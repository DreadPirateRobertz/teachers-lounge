import asyncio
import logging

from google.cloud import pubsub_v1

from app.config import settings
from app.models import IngestJobMessage
from app.processors import route_to_processor

logger = logging.getLogger(__name__)


def _publish_ingest_job_sync(message: IngestJobMessage) -> None:
    """Synchronous Pub/Sub publish. Called via run_in_executor — never call directly from async code."""
    publisher = pubsub_v1.PublisherClient()
    topic_path = publisher.topic_path(settings.gcp_project, settings.pubsub_ingest_topic)
    data = message.model_dump_json().encode()
    msg_id = publisher.publish(topic_path, data).result()
    logger.info("published ingest job %s → msg_id=%s", message.job_id, msg_id)


async def publish_ingest_job(message: IngestJobMessage) -> None:
    """Publish to Pub/Sub without blocking the event loop."""
    loop = asyncio.get_running_loop()
    await loop.run_in_executor(None, _publish_ingest_job_sync, message)


def start_subscriber() -> None:
    """Start a blocking Pub/Sub pull subscriber. Run in a background thread."""
    subscriber = pubsub_v1.SubscriberClient()
    subscription_path = subscriber.subscription_path(
        settings.gcp_project, settings.pubsub_ingest_subscription
    )

    def callback(msg: pubsub_v1.subscriber.message.Message) -> None:
        try:
            payload = IngestJobMessage.model_validate_json(msg.data)
            logger.info(
                "received ingest job job_id=%s mime=%s", payload.job_id, payload.mime_type
            )
            asyncio.run(route_to_processor(payload))
            msg.ack()
        except Exception:
            logger.exception("failed to process job_id=%s", getattr(payload, "job_id", "?"))
            msg.nack()

    streaming_pull = subscriber.subscribe(subscription_path, callback=callback)
    logger.info("pub/sub subscriber started on %s", subscription_path)
    try:
        streaming_pull.result()
    except Exception:
        streaming_pull.cancel()
        raise
