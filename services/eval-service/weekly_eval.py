"""Weekly evaluation CronJob entry point.

Runs:
  1. RAGAS offline evaluation (faithfulness, relevancy, context precision/recall)
  2. Custom learning effectiveness metric (quiz pre/post tutoring delta)

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
    """Run weekly RAGAS evaluation and learning effectiveness computation."""
    from app.ragas_eval import run_ragas_evaluation
    from app.learning_effectiveness import run_learning_effectiveness
    from app.database import close_engine

    try:
        ragas_result = await run_ragas_evaluation()
        logger.info("Weekly RAGAS done: %s", ragas_result)

        effectiveness = await run_learning_effectiveness()
        logger.info("Weekly learning effectiveness done: %d topics", len(effectiveness))
    finally:
        await close_engine()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except Exception:
        logger.exception("Weekly eval job failed")
        sys.exit(1)
