"""Nightly LLM judge CronJob entry point.

Samples 20 recent tutor interactions and evaluates them with Claude Haiku.
Results are stored in the interaction_quality Postgres table.

Exit code 0 on success, 1 on any unhandled error.
"""
import asyncio
import logging
import sys

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(name)s %(levelname)s %(message)s",
)
logger = logging.getLogger(__name__)


async def main() -> None:
    """Run the nightly LLM judge."""
    from app.llm_judge import run_llm_judge
    from app.database import close_engine

    try:
        judged = await run_llm_judge()
        logger.info("Nightly judge complete: %d interactions judged", judged)
    finally:
        await close_engine()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except Exception:
        logger.exception("Nightly judge job failed")
        sys.exit(1)
