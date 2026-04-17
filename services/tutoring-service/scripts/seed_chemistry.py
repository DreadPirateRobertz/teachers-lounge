"""CLI runner that seeds the chemistry chapter of the global concept graph (tl-mhd).

Usage (from ``services/tutoring-service``)::

    python -m scripts.seed_chemistry

The database URL is read from ``DATABASE_URL`` (via ``app.config.settings``)
unless ``--database-url`` is passed explicitly. The command is idempotent:
re-running tops the graph up without duplicating rows.
"""

from __future__ import annotations

import argparse
import asyncio
import sys

from sqlalchemy.ext.asyncio import async_sessionmaker, create_async_engine

from app.config import settings
from app.seeds.chemistry import CHEMISTRY_CONCEPTS, SeedResult, seed_chemistry_concepts


def _build_parser() -> argparse.ArgumentParser:
    """Construct the argparse parser for the seed-chemistry CLI."""
    return argparse.ArgumentParser(
        prog="seed_chemistry",
        description=(
            f"Seed the global concept_graph with {len(CHEMISTRY_CONCEPTS)} chemistry "
            "concepts (categories + leaves). Idempotent on concept_id."
        ),
    )


def _add_args(parser: argparse.ArgumentParser) -> argparse.ArgumentParser:
    """Attach shared arguments; split out to keep ``--help`` output stable for tests."""
    parser.add_argument(
        "--database-url",
        default=None,
        help="SQLAlchemy async URL; defaults to settings.database_url / $DATABASE_URL.",
    )
    return parser


async def _run(database_url: str) -> SeedResult:
    """Connect to ``database_url`` and run the seed in a single transaction.

    Args:
        database_url: SQLAlchemy-compatible async database URL.

    Returns:
        :class:`SeedResult` summarising inserted vs. skipped counts.
    """
    engine = create_async_engine(database_url)
    session_factory = async_sessionmaker(engine, expire_on_commit=False)
    try:
        async with session_factory() as session:
            result = await seed_chemistry_concepts(session)
            await session.commit()
    finally:
        await engine.dispose()
    return result


def main(argv: list[str] | None = None) -> int:
    """Parse argv, run the seed, print a summary, and return a shell exit code."""
    parser = _add_args(_build_parser())
    args = parser.parse_args(argv)

    database_url = args.database_url or settings.database_url
    result = asyncio.run(_run(database_url))

    print(
        f"seeded concept_graph: {result.inserted} concepts inserted, "
        f"{result.skipped} skipped."
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
