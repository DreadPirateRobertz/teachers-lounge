"""Tests that verify the ruff.toml lint configuration is present and correct.

These tests act as a living contract for the lint gate: if the config file
is removed, renamed, or stripped of required sections the tests fail
immediately rather than silently regressing to a weaker ruleset.
"""

import subprocess
import sys
from pathlib import Path

import pytest

# Root of this service (parent of the tests/ directory).
SERVICE_ROOT = Path(__file__).parent.parent


class TestRuffTomlPresent:
    """Verify ruff.toml exists and contains required top-level keys."""

    def test_ruff_toml_exists(self):
        """ruff.toml must be present at the service root."""
        assert (SERVICE_ROOT / "ruff.toml").is_file(), (
            "ruff.toml not found — lint rules are not being enforced"
        )

    def test_ruff_toml_contains_docstring_rule(self):
        """ruff.toml must select the D (pydocstyle) rule group."""
        config_text = (SERVICE_ROOT / "ruff.toml").read_text()
        assert '"D"' in config_text, (
            'ruff.toml must include "D" in the select list to enforce docstring rules'
        )

    def test_ruff_toml_contains_complexity_rule(self):
        """ruff.toml must select the C90 (McCabe complexity) rule group."""
        config_text = (SERVICE_ROOT / "ruff.toml").read_text()
        assert '"C90"' in config_text, (
            'ruff.toml must include "C90" in the select list to enforce complexity limits'
        )

    def test_ruff_toml_contains_isort_rule(self):
        """ruff.toml must select the I (isort) rule group."""
        config_text = (SERVICE_ROOT / "ruff.toml").read_text()
        assert '"I"' in config_text, (
            'ruff.toml must include "I" in the select list to enforce import ordering'
        )

    def test_ruff_toml_specifies_google_convention(self):
        """ruff.toml must configure Google-style docstring convention."""
        config_text = (SERVICE_ROOT / "ruff.toml").read_text()
        assert 'convention = "google"' in config_text, (
            "ruff.toml must set pydocstyle convention to google"
        )

    def test_ruff_toml_sets_max_complexity(self):
        """ruff.toml must define a max-complexity threshold."""
        config_text = (SERVICE_ROOT / "ruff.toml").read_text()
        assert "max-complexity" in config_text, "ruff.toml must set [lint.mccabe] max-complexity"


class TestRuffPassesClean:
    """Verify the service passes ruff with the configured ruleset."""

    def test_ruff_check_passes(self):
        """Running ruff check on the service must exit 0 (no violations).

        This test runs ruff as a subprocess so it picks up the full ruff.toml
        configuration exactly as CI does.  If it fails, ruff --fix can auto-
        correct most issues; the remainder need manual attention.
        """
        result = subprocess.run(
            [sys.executable, "-m", "ruff", "check", "--quiet", str(SERVICE_ROOT)],
            capture_output=True,
            text=True,
        )
        assert result.returncode == 0, f"ruff found violations:\n{result.stdout}\n{result.stderr}"
