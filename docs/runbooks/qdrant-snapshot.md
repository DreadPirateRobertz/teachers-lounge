# Runbook — Qdrant snapshot failures and restore (tl-94n)

Covers both alerts emitted by `tl.qdrant.snapshot`:

- **`QdrantSnapshotJobFailed`** (warning) — a single CronJob run has failed.
- **`QdrantSnapshotCronJobStale`** (critical) — no successful snapshot in 36 h.

Also documents the **restore** procedure for disaster recovery.

---

## 1. Triage — what failed?

```bash
# Most recent 5 snapshot jobs
kubectl -n qdrant get jobs -l app.kubernetes.io/component=snapshot \
  --sort-by=.metadata.creationTimestamp | tail -5

# Pick the latest failing job
FAILED_JOB=$(kubectl -n qdrant get jobs -l app.kubernetes.io/component=snapshot \
  -o jsonpath='{.items[?(@.status.failed>0)].metadata.name}' | awk '{print $NF}')

kubectl -n qdrant logs "job/${FAILED_JOB}"
```

Categorise the failure by which stage of the CronJob script (`infra/helm/qdrant/templates/cronjob-snapshot.yaml`) logged the error:

| Symptom in log                                   | Likely cause                                          | Next step                                                                                 |
|--------------------------------------------------|-------------------------------------------------------|-------------------------------------------------------------------------------------------|
| `curl: ... could not resolve host`               | Qdrant service DNS / networking                       | `kubectl -n qdrant get svc qdrant`; check that the statefulset is healthy.                |
| `curl: (22) ... 5xx`                             | Qdrant server overloaded or mid-upgrade               | Check Qdrant pod logs; retry once cluster is healthy.                                     |
| `failed to create snapshot for <collection>`     | Collection lock or disk pressure                      | `kubectl -n qdrant top pod`; inspect Qdrant storage PVC utilisation.                      |
| `gsutil: ... ServiceException: 401`              | Workload Identity binding broken                      | See §3 (Workload Identity).                                                               |
| `gsutil: ... ServiceException: 403`              | GCS SA missing `roles/storage.objectCreator`          | Re-apply Terraform at `infra/terraform/modules/qdrant-gcs/`.                              |
| Pod OOMKilled                                    | Large collection + memory-backed streaming            | Bump `snapshot.resources.limits.memory` in values-prod.yaml.                              |
| `activeDeadlineSeconds` exceeded                 | Full snapshot cycle > 2 h                             | Raise `activeDeadlineSeconds` OR shard the CronJob per collection.                        |

## 2. Verify backups are actually landing

```bash
# Most recent snapshot directories in GCS
gsutil ls "gs://${GCS_BUCKET}/${GCS_PATH_PREFIX}/" | tail -5

# Per-collection files inside the latest directory
LATEST=$(gsutil ls "gs://${GCS_BUCKET}/${GCS_PATH_PREFIX}/" | tail -1)
gsutil ls -l "${LATEST}"
```

Expect one `*.snapshot` file per live Qdrant collection (currently `curriculum` and `diagrams`).
If a directory is empty or partial, treat it as a failed run even if the Job reported success.

## 3. Workload Identity sanity check

The snapshot pod uses Workload Identity — **no static credentials in the cluster**. If 401s appear:

```bash
# Confirm the K8s SA is annotated
kubectl -n qdrant get sa qdrant -o yaml | grep iam.gke.io

# Should match the GCP SA defined in infra/terraform/modules/qdrant-gcs/main.tf
# e.g. iam.gke.io/gcp-service-account: qdrant-snapshots@<project>.iam.gserviceaccount.com
```

If the annotation is missing, re-apply the Qdrant Helm release — the ServiceAccount
is templated from `values-prod.yaml` Workload Identity annotations.

## 4. Retry a failed run manually

```bash
# Trigger an immediate one-shot run of the CronJob
kubectl -n qdrant create job --from=cronjob/qdrant-qdrant-snapshot qdrant-snapshot-manual-$(date +%s)

# Watch it
kubectl -n qdrant get pods -w -l app.kubernetes.io/component=snapshot
```

## 5. Restore from a snapshot

Qdrant supports restore via the `snapshots/upload` endpoint (or `snapshots/recover` with a URL).
The simplest path is **upload**:

```bash
# 1. Pick a snapshot directory (e.g. the most recent green run)
SNAPSHOT_DIR="gs://${GCS_BUCKET}/${GCS_PATH_PREFIX}/20260411-020000"
COLLECTION="curriculum"
SNAPSHOT_FILE=$(gsutil ls "${SNAPSHOT_DIR}/${COLLECTION}/" | tail -1)

# 2. Copy it down (inside a pod on the cluster, or through a bastion)
gsutil cp "${SNAPSHOT_FILE}" /tmp/restore.snapshot

# 3. Port-forward to Qdrant REST API
kubectl -n qdrant port-forward svc/qdrant 6333:6333 &

# 4. Upload + restore (Qdrant will recreate the collection from the snapshot).
#    WARNING: this replaces the existing collection in-place.
curl -X POST 'http://127.0.0.1:6333/collections/curriculum/snapshots/upload' \
  -H 'Content-Type: multipart/form-data' \
  -F 'snapshot=@/tmp/restore.snapshot'

# 5. Verify
curl -sS 'http://127.0.0.1:6333/collections/curriculum' | jq '.result.points_count'
```

### Alternative: recover via URL (no local download)

If the running cluster has network egress to GCS, you can skip the download and
hand Qdrant a signed URL directly via `POST /collections/{name}/snapshots/recover`.
This is faster for large snapshots but requires an accessible URL — cheaper to
generate via `gsutil signurl` than to download.

## 6. Silencing the alerts during maintenance

If you are intentionally disabling snapshots (e.g. during a Qdrant major-version
upgrade), silence the rules in Alertmanager with a matcher on
`service=qdrant, component=snapshot` for the maintenance window, then **set a
reminder** — the alert will otherwise fire within 36 h of the last good run.

## 7. Escalation

- **First 30 minutes:** on-call engineer works the table in §1.
- **After 30 min with no progress:** page Qdrant SME (currently crew/bean).
- **After 2 h or any data-loss concern:** `gt escalate -s CRITICAL "Qdrant snapshot failure — restore in doubt"`.

Related:
- CronJob source: `infra/helm/qdrant/templates/cronjob-snapshot.yaml`
- Alert rule source: `infra/helm/monitoring/templates/slo-alert-rules.yaml` (group `tl.qdrant.snapshot`)
- GCS bucket / IAM terraform: `infra/terraform/modules/qdrant-gcs/`
