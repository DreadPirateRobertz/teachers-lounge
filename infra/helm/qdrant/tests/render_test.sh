#!/usr/bin/env bash
# Render-time tests for the qdrant Helm chart.
#
# Exercises the PrometheusRule that alerts on Qdrant snapshot CronJob failures.
# Run locally:   infra/helm/qdrant/tests/render_test.sh
# Run in CI:     invoked from .github/workflows/ci.yml (helm-lint job).
#
# Each test renders the chart with a distinct values override and asserts on
# the rendered YAML. Failures abort with a non-zero exit code.
set -euo pipefail

CHART_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HELM="${HELM:-helm}"
FAILED=0

fail() {
  echo "FAIL: $*" >&2
  FAILED=$((FAILED + 1))
}

pass() {
  echo "ok: $*"
}

render() {
  # Render with a deterministic release name so kube_cronjob name matches in expr assertions.
  "$HELM" template qdrant "$CHART_DIR" --namespace teacherslounge "$@"
}

# ── Test 1: default values render a snapshot PrometheusRule ─────────────────
out=$(render)
if ! echo "$out" | grep -q "kind: PrometheusRule"; then
  fail "default render: expected PrometheusRule, not found"
else
  pass "default render includes PrometheusRule"
fi

# ── Test 2: alert name + 25h threshold present in default render ────────────
if ! echo "$out" | grep -q "alert: QdrantSnapshotStale"; then
  fail "default render: expected alert QdrantSnapshotStale"
else
  pass "default render includes QdrantSnapshotStale alert"
fi

if ! echo "$out" | grep -qE "> 25 \* 3600|> 90000"; then
  fail "default render: expected 25h threshold (25*3600 seconds) in alert expr"
else
  pass "default render uses 25h staleness threshold"
fi

# ── Test 3: severity=critical label is present so PD routing fires ──────────
if ! echo "$out" | awk '/alert: QdrantSnapshotStale/,/alert: QdrantSnapshotJobFailed/' | grep -q "severity: critical"; then
  fail "QdrantSnapshotStale must carry severity=critical to route to PagerDuty"
else
  pass "QdrantSnapshotStale routes via severity=critical → PagerDuty"
fi

# ── Test 4: runbook URL annotation points at the restore doc ────────────────
if ! echo "$out" | grep -q "docs/runbooks/qdrant-restore.md"; then
  fail "alert is missing runbook_url pointing at docs/runbooks/qdrant-restore.md"
else
  pass "alert annotates runbook_url for restore runbook"
fi

# ── Test 5: job-failure warning alert is present (Slack route) ──────────────
if ! echo "$out" | grep -q "alert: QdrantSnapshotJobFailed"; then
  fail "expected QdrantSnapshotJobFailed warning alert for Slack routing"
else
  pass "QdrantSnapshotJobFailed warning alert present"
fi

# ── Test 6: disabling alerting removes the PrometheusRule ───────────────────
out_no_alert=$(render --set snapshot.alerting.enabled=false)
if echo "$out_no_alert" | grep -q "kind: PrometheusRule"; then
  fail "alerting.enabled=false: PrometheusRule should be suppressed"
else
  pass "alerting.enabled=false suppresses PrometheusRule"
fi

# ── Test 7: disabling the CronJob also suppresses alerts ────────────────────
out_no_snap=$(render --set snapshot.enabled=false)
if echo "$out_no_snap" | grep -q "kind: PrometheusRule"; then
  fail "snapshot.enabled=false: PrometheusRule should be suppressed"
else
  pass "snapshot.enabled=false suppresses PrometheusRule"
fi

# ── Test 8: rule scoped to the correct cronjob name (release-aware) ─────────
# The CronJob name is "{release}-qdrant-snapshot" (include "qdrant.fullname" . ).
# The CronJob name is the chart fullname + "-snapshot". With release=qdrant
# (set in `render()`) the helper produces "qdrant-qdrant", so the cronjob
# label ends up "qdrant-qdrant-snapshot". If the alert's cronjob matcher
# drifts from the actual CronJob name, the stale-alert silently never fires.
cronjob_name=$(echo "$out" | awk '/kind: CronJob/{found=1} found && /^  name:/{print $2; exit}')
if ! echo "$out" | grep -q "cronjob=\"${cronjob_name}\""; then
  fail "alert expr must scope to cronjob=\"${cronjob_name}\" so it matches the rendered CronJob"
else
  pass "alert expr scoped to cronjob=${cronjob_name}"
fi

# ── Test 9: helm lint passes ────────────────────────────────────────────────
if "$HELM" lint "$CHART_DIR" >/dev/null 2>&1; then
  pass "helm lint clean"
else
  fail "helm lint reported errors"
fi

if [ "$FAILED" -gt 0 ]; then
  echo
  echo "$FAILED test(s) failed" >&2
  exit 1
fi

echo
echo "all qdrant render tests passed"
