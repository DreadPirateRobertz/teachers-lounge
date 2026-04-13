# Qdrant Snapshot Restore Runbook

Runbook for `QdrantSnapshotStale` and `QdrantSnapshotJobFailed` alerts from the
`tl-qdrant-snapshot-alerts` PrometheusRule (defined in
`infra/helm/qdrant/templates/prometheusrule-snapshot.yaml`).

- **Alert source:** kube-state-metrics via kube-prometheus-stack
- **Pager route:** `severity=critical` → PagerDuty; `severity=warning` → Slack `#alerts`
- **Snapshot artifacts:** `gs://teachers-lounge-qdrant-snapshots/snapshots/<YYYYMMDD-HHMMSS>/<collection>/<file>`
- **On-call owner:** `bean` (infra) — escalate to Mayor if Dolt/GKE are also impacted.

---

## 1. Triage (first 5 minutes)

Identify which alert fired:

```bash
# Active alerts
kubectl -n monitoring port-forward svc/alertmanager-operated 9093 &
curl -s localhost:9093/api/v2/alerts | jq '.[] | select(.labels.alertname | startswith("QdrantSnapshot"))'
```

### `QdrantSnapshotJobFailed` (warning)

A single snapshot `Job` owned by the CronJob has failed. The 25h stale-alert
has **not** yet fired — you have time before on-call is paged. Inspect Job
logs and decide whether to let the next scheduled run recover or intervene.

### `QdrantSnapshotStale` (critical — pages on-call)

No successful snapshot in the last 25 h. This is the disaster-recovery SLO.
Do not page-ack and walk away — work the issue until a fresh snapshot lands.

---

## 2. Inspect the CronJob and latest Job

```bash
kubectl -n teacherslounge get cronjob qdrant-qdrant-snapshot
kubectl -n teacherslounge get jobs -l app.kubernetes.io/component=snapshot --sort-by=.metadata.creationTimestamp
kubectl -n teacherslounge logs job/<latest-job-name>
```

Most common failure modes:

| Symptom in logs                                   | Likely cause                         | Fix                                                      |
|---------------------------------------------------|--------------------------------------|----------------------------------------------------------|
| `curl: (7) Failed to connect to qdrant ...`       | Qdrant pods unhealthy                | Check `kubectl get pods -l app.kubernetes.io/name=qdrant` — see §5. |
| `gsutil ... 403 AccessDenied`                     | Workload Identity binding drifted    | Re-apply the terraform module `infra/terraform/modules/qdrant-gcs`. |
| `gsutil ... 404 bucket not found`                 | Wrong `snapshot.gcsBucket` per env   | Confirm `values-prod.yaml` override.                     |
| `activeDeadlineSeconds` exceeded (Job `DeadlineExceeded`) | Snapshot size grew past 2 h budget | Bump `activeDeadlineSeconds` or split by collection.     |
| `jq: error ... .result.collections`               | Qdrant API contract changed          | Pin Qdrant image tag; file a bead for a migration.       |

---

## 3. List available snapshots in GCS

```bash
gsutil ls -r gs://teachers-lounge-qdrant-snapshots/snapshots/ | head -40

# Most recent snapshot directory
LATEST=$(gsutil ls gs://teachers-lounge-qdrant-snapshots/snapshots/ | sort | tail -1)
echo "latest snapshot set: ${LATEST}"
gsutil ls -l "${LATEST}"
```

If `LATEST` is > 25 h old, the page is correct and a manual snapshot must be
taken **before** any restore (see §4). A restore without a fresh snapshot
trades one lost window for another.

---

## 4. Trigger a manual snapshot (preferred before any restore)

Always prefer capturing the current state before destructive actions:

```bash
kubectl -n teacherslounge create job --from=cronjob/qdrant-qdrant-snapshot \
  qdrant-snapshot-manual-$(date -u +%Y%m%d-%H%M%S)

# Tail the job's logs
kubectl -n teacherslounge logs -f job/qdrant-snapshot-manual-<ts>
```

Verify the new snapshot landed in GCS before proceeding.

---

## 5. Restore a collection from a snapshot

Qdrant supports snapshot restore via `PUT /collections/{name}/snapshots/upload`
with a URL body. The target Qdrant pod must be able to reach the signed URL.

```bash
# Pick the snapshot to restore
SNAP_URI="gs://teachers-lounge-qdrant-snapshots/snapshots/20260411-020000/courses/courses-20260411.snapshot"
COLLECTION="courses"

# Sign a short-lived URL (10 min) so the pod can pull it
SIGNED=$(gsutil signurl -d 10m -m GET \
  ~/.config/gcloud/qdrant-snapshots-sa.json "${SNAP_URI}" | tail -1 | awk '{print $NF}')

# Port-forward to one replica (restore is node-local; repeat on each replica)
kubectl -n teacherslounge port-forward pod/qdrant-qdrant-0 6333:6333 &

curl -X PUT "http://localhost:6333/collections/${COLLECTION}/snapshots/recover" \
  -H 'Content-Type: application/json' \
  -d "{\"location\":\"${SIGNED}\"}"
```

Repeat for each replica (`qdrant-qdrant-0`, `qdrant-qdrant-1`, `qdrant-qdrant-2`).
Qdrant's distributed mode replicates after the first successful load but
per-replica restore is faster and avoids replication-lag surprises.

---

## 6. Verify restore

```bash
curl -s http://localhost:6333/collections/${COLLECTION} | jq '.result | {points_count, status}'
# points_count should match the snapshot's manifest; status should be "green"

# Smoke-test a known query (uses the ingestion-service health check)
kubectl -n teacherslounge exec deploy/ingestion-service -- \
  curl -sf "http://qdrant-qdrant:6333/collections/${COLLECTION}/points/count" \
  -H 'Content-Type: application/json' -d '{"exact": true}'
```

Resolve the PagerDuty incident only after both checks pass on **all** replicas.

---

## 7. Rollback

If a restore makes things worse (wrong snapshot, corrupt data), the pre-restore
state is already captured from §4. Re-run §5 pointing at that manual snapshot.

For a full cluster rebuild (last resort):

```bash
helm -n teacherslounge uninstall qdrant    # deletes PVCs — destructive
helm -n teacherslounge install qdrant infra/helm/qdrant \
  -f infra/helm/qdrant/values-prod.yaml
# Then restore §5 for each collection.
```

---

## 8. Post-incident

- Link the PagerDuty incident to the bead tracking the failure.
- Update this runbook if a new failure mode was encountered (§2 table).
- If the snapshot CronJob repeatedly fails for the same reason, file a
  follow-up bead to fix the root cause — do not silence the alert.
