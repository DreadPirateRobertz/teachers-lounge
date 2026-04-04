import logging

from app.models import IngestJobMessage

logger = logging.getLogger(__name__)


async def process_video(job: IngestJobMessage) -> dict:
    """
    Stub: Video and audio processing pipeline.

    Phase 2 full implementation will:
    - Download from GCS
    - Video (MP4/MOV): ffmpeg → extract audio track → transcription
    - Audio (MP3/WAV): direct transcription
    - Transcription: Google Speech-to-Text or Whisper (configurable)
    - Segment transcript by timestamp → chunking (max 512 tokens per chunk)
    - Preserve timestamp metadata in chunk payload (for playback seek)
    - Generate embeddings → write to Qdrant curriculum collection
    - Update materials table: status=complete, chunk_count=N
    """
    logger.info("video_processor: processing %s (job_id=%s)", job.filename, job.job_id)
    return {"status": "stub", "job_id": str(job.job_id), "processor": "video"}
