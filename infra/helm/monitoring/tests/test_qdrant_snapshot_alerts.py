"""Smoke tests for the Qdrant snapshot PrometheusRule additions (tl-94n).

The raw template uses Go templating and cannot be parsed as YAML without
Helm, but we can still make high-value text assertions that catch common
regressions:

* The alert group and both alert names exist.
* Severity labels are set (critical for stale, warning for single-failure).
* ``runbook_url`` annotations point to the committed runbook file.
* The runbook file itself exists and references both alert names.

These tests run under plain ``pytest`` with no extra dependencies and
execute in milliseconds — they are intended to run in CI alongside the
service test suites.
"""
from __future__ import annotations

from pathlib import Path

import pytest

REPO_ROOT = Path(__file__).resolve().parents[4]
RULES_FILE = (
    REPO_ROOT / "infra" / "helm" / "monitoring" / "templates" / "slo-alert-rules.yaml"
)
RUNBOOK_FILE = REPO_ROOT / "docs" / "runbooks" / "qdrant-snapshot.md"
RUNBOOK_URL_SUFFIX = "docs/runbooks/qdrant-snapshot.md"


@pytest.fixture(scope="module")
def rules_text() -> str:
    """Load the alert-rules template once per test module."""
    assert RULES_FILE.exists(), f"missing {RULES_FILE}"
    return RULES_FILE.read_text(encoding="utf-8")


@pytest.fixture(scope="module")
def runbook_text() -> str:
    """Load the snapshot runbook once per test module."""
    assert RUNBOOK_FILE.exists(), f"missing {RUNBOOK_FILE}"
    return RUNBOOK_FILE.read_text(encoding="utf-8")


class TestAlertGroup:
    """Cover presence and shape of the new ``tl.qdrant.snapshot`` rule group."""

    def test_group_declared(self, rules_text: str) -> None:
        """The new group must be declared so kube-prometheus-stack picks it up."""
        assert "name: tl.qdrant.snapshot" in rules_text

    def test_stale_alert_present(self, rules_text: str) -> None:
        """Critical staleness alert must be named so on-call can silence it."""
        assert "alert: QdrantSnapshotCronJobStale" in rules_text

    def test_failed_alert_present(self, rules_text: str) -> None:
        """Warning single-failure alert must be named."""
        assert "alert: QdrantSnapshotJobFailed" in rules_text


class TestAlertLabels:
    """Severity routing depends on exact label values — verify them."""

    def test_stale_is_critical(self, rules_text: str) -> None:
        """``QdrantSnapshotCronJobStale`` must carry ``severity: critical``."""
        idx = rules_text.index("alert: QdrantSnapshotCronJobStale")
        window = rules_text[idx : idx + 800]
        assert "severity: critical" in window
        assert "service: qdrant" in window
        assert "component: snapshot" in window

    def test_failed_is_warning(self, rules_text: str) -> None:
        """``QdrantSnapshotJobFailed`` must carry ``severity: warning``."""
        idx = rules_text.index("alert: QdrantSnapshotJobFailed")
        window = rules_text[idx : idx + 800]
        assert "severity: warning" in window
        assert "service: qdrant" in window
        assert "component: snapshot" in window


class TestAlertExpressions:
    """Spot-check PromQL selectors to catch accidental metric-name drift."""

    def test_stale_uses_cronjob_metric(self, rules_text: str) -> None:
        """Staleness alert must read ``kube_cronjob_status_last_successful_time``."""
        idx = rules_text.index("alert: QdrantSnapshotCronJobStale")
        window = rules_text[idx : idx + 800]
        assert "kube_cronjob_status_last_successful_time" in window
        assert '".*qdrant.*snapshot.*"' in window

    def test_failed_uses_job_metric(self, rules_text: str) -> None:
        """Single-failure alert must read ``kube_job_status_failed``."""
        idx = rules_text.index("alert: QdrantSnapshotJobFailed")
        window = rules_text[idx : idx + 800]
        assert "kube_job_status_failed" in window


class TestRunbookWiring:
    """Catch broken ``runbook_url`` annotations before they page on-call."""

    def test_both_alerts_reference_runbook(self, rules_text: str) -> None:
        """Both alerts should cite the committed snapshot runbook."""
        # There is exactly one runbook file for this work — both alerts share it.
        url_hits = rules_text.count(RUNBOOK_URL_SUFFIX)
        assert url_hits >= 2, f"expected ≥2 runbook_url refs, got {url_hits}"

    def test_runbook_file_exists_and_covers_both_alerts(
        self, runbook_text: str
    ) -> None:
        """The runbook must mention both alert names so readers can search for them."""
        assert "QdrantSnapshotCronJobStale" in runbook_text
        assert "QdrantSnapshotJobFailed" in runbook_text

    def test_runbook_has_restore_section(self, runbook_text: str) -> None:
        """The runbook must include a restore procedure — that is the point of tl-94n."""
        assert "Restore" in runbook_text or "restore" in runbook_text
        assert "snapshots/upload" in runbook_text or "snapshots/recover" in runbook_text
