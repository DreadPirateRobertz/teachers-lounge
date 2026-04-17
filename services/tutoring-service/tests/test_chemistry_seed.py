"""Tests for the chemistry seed data and :mod:`app.seeds.chemistry`."""

from __future__ import annotations

import re
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from app.seeds.chemistry import (
    CHEMISTRY_CONCEPTS,
    SUBJECT,
    SeedResult,
    seed_chemistry_concepts,
)

# ── Structural invariants (pure data) ─────────────────────────────────────────


def test_expected_concept_count():
    """The curriculum is exactly 52 concepts (6 categories + 46 leaves) — drift canary."""
    assert len(CHEMISTRY_CONCEPTS) == 52


def test_subject_is_chemistry():
    """All seeded rows share ``subject='chemistry'`` so the catalog is subject-filterable."""
    assert SUBJECT == "chemistry"


def test_concept_ids_are_unique():
    """``concept_id`` is the seed's public key — duplicates would break joins."""
    ids = [c.concept_id for c in CHEMISTRY_CONCEPTS]
    assert len(ids) == len(set(ids))


def test_labels_are_unique():
    """Labels feed the RAG prereq-gap note; duplicates would produce ambiguous phrasing."""
    labels = [c.label for c in CHEMISTRY_CONCEPTS]
    assert len(labels) == len(set(labels))


def test_paths_are_unique():
    """ltree paths must be unique so each concept has its own node in the hierarchy."""
    paths = [c.path for c in CHEMISTRY_CONCEPTS]
    assert len(paths) == len(set(paths))


def test_concept_ids_are_valid_slugs():
    """Slugs must be lowercase ASCII with underscores — stable in URLs and code."""
    slug_re = re.compile(r"^[a-z][a-z0-9_]*$")
    for c in CHEMISTRY_CONCEPTS:
        assert slug_re.match(c.concept_id), f"bad slug: {c.concept_id!r}"


def test_paths_are_valid_ltree():
    """Every path must match Postgres' ltree label regex."""
    label_regex = re.compile(r"^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$")
    for c in CHEMISTRY_CONCEPTS:
        assert label_regex.match(c.path), f"bad ltree path: {c.path!r}"


def test_paths_rooted_in_chemistry():
    """All concepts live under the ``chemistry`` root of the hierarchy."""
    for c in CHEMISTRY_CONCEPTS:
        assert c.path.startswith("chemistry."), f"path outside chemistry root: {c.path}"


def test_stereochemistry_branch_is_populated():
    """Spec called out stereochemistry.chirality — verify that branch is seeded."""
    stereo = {c.path for c in CHEMISTRY_CONCEPTS if ".stereochemistry" in c.path}
    assert "chemistry.organic.stereochemistry" in stereo  # category node
    assert "chemistry.organic.stereochemistry.chirality" in stereo  # leaf


def test_chirality_ancestors_are_in_seed():
    """Spec exit criterion: ancestors of chirality include organic-chem + stereochem rows."""
    paths = {c.path for c in CHEMISTRY_CONCEPTS}
    assert "chemistry.organic" in paths
    assert "chemistry.organic.stereochemistry" in paths


def test_category_paths_are_depth_two():
    """Category nodes sit directly under ``chemistry.*`` and one level deeper for branches."""
    categories = [c for c in CHEMISTRY_CONCEPTS if c.path.count(".") in (1, 2) and c.label in {
        "Foundations of Chemistry",
        "General Chemistry",
        "Organic Chemistry",
        "Stereochemistry",
        "Reaction Mechanisms",
        "Analytical Chemistry",
    }]
    assert len(categories) == 6


# ── seed_chemistry_concepts — fake async session ──────────────────────────────


class _ScalarsWrapper:
    """Stand-in for the object returned by ``result.scalars()``."""

    def __init__(self, items):
        self._items = list(items)

    def all(self):
        return list(self._items)


class _ResultWrapper:
    """Stand-in for the object returned by ``session.execute(select(...))``."""

    def __init__(self, scalars):
        self._scalars = list(scalars)

    def scalars(self):
        return _ScalarsWrapper(self._scalars)


class FakeAsyncSession:
    """Minimal async-session stand-in that tracks add() + flush() + execute()."""

    def __init__(self, existing_concept_ids=None):
        self.added: list = []
        self.flush_count = 0
        self._existing = set(existing_concept_ids or [])

    async def execute(self, stmt):
        return _ResultWrapper(scalars=sorted(self._existing))

    def add(self, row):
        self.added.append(row)
        self._existing.add(row.concept_id)

    async def flush(self):
        self.flush_count += 1


@pytest.mark.asyncio
async def test_seed_inserts_all_on_empty_graph():
    """Empty-graph run inserts every concept and nothing is skipped."""
    session = FakeAsyncSession()
    result = await seed_chemistry_concepts(session)

    assert isinstance(result, SeedResult)
    assert result.inserted == len(CHEMISTRY_CONCEPTS)
    assert result.skipped == 0
    assert len(session.added) == len(CHEMISTRY_CONCEPTS)
    assert session.flush_count >= 1


@pytest.mark.asyncio
async def test_seed_is_idempotent():
    """Re-running after a full seed inserts zero and skips everything."""
    session = FakeAsyncSession()
    await seed_chemistry_concepts(session)
    session.added.clear()

    second = await seed_chemistry_concepts(session)
    assert second.inserted == 0
    assert second.skipped == len(CHEMISTRY_CONCEPTS)
    assert session.added == []


@pytest.mark.asyncio
async def test_seed_tops_up_partial_state():
    """With some concepts already present, only the missing ones are inserted."""
    preexisting = {"atomic_structure", "periodic_table"}
    session = FakeAsyncSession(existing_concept_ids=preexisting)

    result = await seed_chemistry_concepts(session)

    assert result.inserted == len(CHEMISTRY_CONCEPTS) - len(preexisting)
    assert result.skipped == len(preexisting)


@pytest.mark.asyncio
async def test_seed_marks_subject_chemistry():
    """Every inserted row must carry ``subject='chemistry'``."""
    session = FakeAsyncSession()
    await seed_chemistry_concepts(session)
    assert all(r.subject == SUBJECT for r in session.added)


# ── CLI runner ────────────────────────────────────────────────────────────────


def test_cli_help_mentions_concept_count(capsys):
    """``--help`` advertises how many concepts will be seeded for operator clarity."""
    from scripts import seed_chemistry as cli

    with pytest.raises(SystemExit):
        cli.main(["--help"])

    out = capsys.readouterr().out
    assert str(len(CHEMISTRY_CONCEPTS)) in out


def test_cli_runs_seed_and_reports(capsys):
    """Happy path: CLI invokes the async runner, prints a summary, exits 0."""
    from scripts import seed_chemistry as cli

    fake_result = SeedResult(inserted=50, skipped=0)

    async def _fake_run(url):
        assert url  # some non-empty URL was resolved
        return fake_result

    with patch.object(cli, "_run", _fake_run):
        exit_code = cli.main(
            ["--database-url", "postgresql+asyncpg://test/test"]
        )

    captured = capsys.readouterr()
    assert exit_code == 0
    assert "50 concepts inserted" in captured.out


def test_cli_uses_settings_database_url_when_absent(capsys):
    """Omitting ``--database-url`` falls back to ``settings.database_url``."""
    from scripts import seed_chemistry as cli

    received = {}

    async def _fake_run(url):
        received["url"] = url
        return SeedResult(inserted=0, skipped=50)

    with (
        patch.object(cli, "_run", _fake_run),
        patch.object(cli.settings, "database_url", "postgresql+asyncpg://default/x"),
    ):
        exit_code = cli.main([])

    assert exit_code == 0
    assert received["url"] == "postgresql+asyncpg://default/x"


@pytest.mark.asyncio
async def test_cli_run_connects_and_dispatches():
    """``_run`` creates an engine, opens a session, calls the seeder, commits, disposes."""
    from scripts import seed_chemistry as cli

    seeded: dict = {}

    fake_engine = MagicMock()
    fake_engine.dispose = AsyncMock()
    fake_session = MagicMock()
    fake_session.commit = AsyncMock()
    fake_session.__aenter__ = AsyncMock(return_value=fake_session)
    fake_session.__aexit__ = AsyncMock(return_value=False)
    fake_factory = MagicMock(return_value=fake_session)

    async def _fake_seed(db):
        seeded["called"] = True
        return SeedResult(inserted=50, skipped=0)

    with (
        patch.object(cli, "create_async_engine", return_value=fake_engine),
        patch.object(cli, "async_sessionmaker", return_value=fake_factory),
        patch.object(cli, "seed_chemistry_concepts", _fake_seed),
    ):
        result = await cli._run("postgresql+asyncpg://test/x")

    assert result.inserted == 50
    assert seeded.get("called") is True
    fake_session.commit.assert_awaited_once()
    fake_engine.dispose.assert_awaited_once()
