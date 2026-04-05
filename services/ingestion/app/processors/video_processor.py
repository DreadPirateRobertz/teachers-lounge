"""Video and audio transcription pipeline.

Handles MP4, MOV, MP3, and WAV files.  Video files have their audio
track extracted by ffmpeg before transcription.  Transcription is
performed by either OpenAI Whisper API or Google Cloud Speech-to-Text,
selected at runtime via the ``transcription_provider`` config setting.

Timestamps are preserved in chunk metadata so the frontend can seek to
the correct position when a student references a specific chunk.

Pipeline:

1. Download from GCS to local temp file.
2. If video: extract audio track to WAV via ffmpeg.
3. Transcribe with configured provider → list of timed segments.
4. Group segments into chunks of at most ``chunk_max_tokens`` tokens,
   preserving start/end timestamps.
5. Embed and store via shared :mod:`app.processors.common` pipeline.
"""
import asyncio
import logging
import subprocess
import tempfile
from pathlib import Path
from uuid import UUID

from app.config import settings
from app.models import IngestJobMessage, ProcessingStatus
from app.processors.common import (
    download_from_gcs,
    embed_and_store,
    make_chunk,
)
from app.services import db

logger = logging.getLogger(__name__)

# MIME types that carry a video stream (audio-only types skip ffmpeg)
_VIDEO_MIMES = {"video/mp4", "video/quicktime"}


async def process_video(job: IngestJobMessage) -> dict:
    """Full video/audio processing pipeline.

    Downloads the file, optionally extracts audio, transcribes with the
    configured provider, chunks by timestamp, then embeds and stores.

    Args:
        job: Ingest job message from the Pub/Sub subscription.

    Returns:
        Dict with keys: ``status``, ``job_id``, ``chunk_count``, ``processor``.

    Raises:
        RuntimeError: If ffmpeg is not installed or transcription fails.
        Exception: Any exception marks the material FAILED.
    """
    logger.info("video_processor: starting job_id=%s file=%s", job.job_id, job.filename)
    await db.update_material_status(job.material_id, ProcessingStatus.PROCESSING)

    local_path: Path | None = None
    audio_path: Path | None = None
    try:
        local_path = await download_from_gcs(job.gcs_path, job.job_id)

        # Extract audio if this is a video file
        if job.mime_type in _VIDEO_MIMES:
            audio_path = await asyncio.get_running_loop().run_in_executor(
                None, _extract_audio_ffmpeg, local_path, job.job_id
            )
            transcribe_path = audio_path
        else:
            transcribe_path = local_path

        # Transcribe
        provider = settings.transcription_provider
        if provider == "openai":
            segments = await _transcribe_openai(transcribe_path)
        elif provider == "google":
            segments = await asyncio.get_running_loop().run_in_executor(
                None, _transcribe_google_sync, transcribe_path
            )
        else:
            raise ValueError(f"unknown transcription_provider: {provider!r}")

        logger.info("video_processor: job_id=%s got %d transcript segments", job.job_id, len(segments))

        chunks = _build_timestamped_chunks(segments, job.material_id, job.course_id)
        logger.info("video_processor: job_id=%s built %d chunks", job.job_id, len(chunks))

        chunk_count = await embed_and_store(chunks, job.material_id)
        logger.info("video_processor: complete job_id=%s chunks=%d", job.job_id, chunk_count)
        return {
            "status": "complete",
            "job_id": str(job.job_id),
            "chunk_count": chunk_count,
            "processor": "video",
        }

    except Exception:
        logger.exception("video_processor: failed job_id=%s", job.job_id)
        await db.update_material_status(job.material_id, ProcessingStatus.FAILED)
        raise
    finally:
        if local_path is not None:
            local_path.unlink(missing_ok=True)
        if audio_path is not None:
            audio_path.unlink(missing_ok=True)


# ── Audio extraction ──────────────────────────────────────────────────────────


def _extract_audio_ffmpeg(video_path: Path, job_id: UUID) -> Path:
    """Extract the audio track from a video file using ffmpeg.

    Outputs a 16 kHz mono WAV file, which is the format preferred by
    Whisper for best transcription accuracy.

    Args:
        video_path: Path to the input video file (MP4/MOV).
        job_id: Used to name the temporary output file.

    Returns:
        Path to the extracted WAV file.

    Raises:
        RuntimeError: If ffmpeg is not available or returns a non-zero exit code.
    """
    out = tempfile.NamedTemporaryFile(
        delete=False, suffix=".wav", prefix=f"audio-{job_id}-"
    )
    out.close()
    out_path = Path(out.name)

    cmd = [
        "ffmpeg", "-y",
        "-i", str(video_path),
        "-vn",                  # drop video stream
        "-acodec", "pcm_s16le", # 16-bit PCM
        "-ar", "16000",         # 16 kHz sample rate
        "-ac", "1",             # mono
        str(out_path),
    ]
    result = subprocess.run(cmd, capture_output=True, timeout=300)
    if result.returncode != 0:
        raise RuntimeError(
            f"ffmpeg failed (exit {result.returncode}): {result.stderr.decode()[:500]}"
        )
    logger.info("extracted audio from %s → %s", video_path.name, out_path.name)
    return out_path


# ── Transcription providers ───────────────────────────────────────────────────


async def _transcribe_openai(audio_path: Path) -> list[dict]:
    """Transcribe audio using the OpenAI Whisper API.

    Splits files larger than 24 MB into ``audio_segment_max_seconds``-
    length segments to stay within the API's 25 MB file size limit.
    Timestamps are adjusted per-segment so they reflect position in the
    original file.

    Args:
        audio_path: Path to the audio file (WAV/MP3/M4A/etc.).

    Returns:
        List of segment dicts, each with keys:
        ``text`` (str), ``start`` (float seconds), ``end`` (float seconds).
    """
    from openai import AsyncOpenAI

    max_bytes = 24 * 1024 * 1024  # 24 MB — leave headroom under the 25 MB limit
    file_size = audio_path.stat().st_size

    if file_size <= max_bytes:
        return await _whisper_request(audio_path, offset_seconds=0.0)

    # Split into segments using ffmpeg, transcribe each, merge
    segment_paths = await asyncio.get_running_loop().run_in_executor(
        None, _split_audio_ffmpeg, audio_path, settings.audio_segment_max_seconds
    )
    try:
        all_segments: list[dict] = []
        offset = 0.0
        for seg_path in segment_paths:
            seg_segments = await _whisper_request(seg_path, offset_seconds=offset)
            all_segments.extend(seg_segments)
            # Advance offset by segment duration
            if seg_segments:
                offset = seg_segments[-1]["end"]
        return all_segments
    finally:
        for p in segment_paths:
            p.unlink(missing_ok=True)


async def _whisper_request(audio_path: Path, offset_seconds: float) -> list[dict]:
    """Send one audio file to the OpenAI Whisper API.

    Args:
        audio_path: Path to the audio file to transcribe.
        offset_seconds: Time offset to add to all returned timestamps
            (used when the file is a split segment of a longer recording).

    Returns:
        List of segment dicts with ``text``, ``start``, and ``end`` keys.
    """
    from openai import AsyncOpenAI

    client = AsyncOpenAI(api_key=settings.openai_api_key)
    try:
        with open(audio_path, "rb") as f:
            response = await client.audio.transcriptions.create(
                model=settings.whisper_model,
                file=f,
                response_format="verbose_json",
            )
        return [
            {
                "text": seg.text.strip(),
                "start": seg.start + offset_seconds,
                "end": seg.end + offset_seconds,
            }
            for seg in (response.segments or [])
            if seg.text.strip()
        ]
    finally:
        await client.close()


def _transcribe_google_sync(audio_path: Path) -> list[dict]:
    """Transcribe audio using Google Cloud Speech-to-Text.

    Uses ``long_running_recognize`` for files longer than one minute,
    which requires uploading to GCS.  For short clips (< 60 s) uses the
    synchronous ``recognize`` API with inline audio bytes.

    Args:
        audio_path: Path to a 16 kHz mono WAV file.

    Returns:
        List of segment dicts with ``text``, ``start``, and ``end`` keys.
        ``start`` and ``end`` are in seconds and derived from word-level
        timestamps when available.
    """
    from google.cloud import speech

    client = speech.SpeechClient()
    audio_bytes = audio_path.read_bytes()

    config = speech.RecognitionConfig(
        encoding=speech.RecognitionConfig.AudioEncoding.LINEAR16,
        sample_rate_hertz=16000,
        language_code="en-US",
        enable_word_time_offsets=True,
        enable_automatic_punctuation=True,
    )
    audio = speech.RecognitionAudio(content=audio_bytes)

    # Choose sync vs async based on file size (proxy for duration)
    if len(audio_bytes) > 10 * 1024 * 1024:  # > 10 MB ≈ > ~5 min at 16kHz mono
        operation = client.long_running_recognize(config=config, audio=audio)
        response = operation.result(timeout=600)
    else:
        response = client.recognize(config=config, audio=audio)

    segments: list[dict] = []
    for result in response.results:
        if not result.alternatives:
            continue
        alt = result.alternatives[0]
        words = alt.words
        if words:
            start = words[0].start_time.total_seconds()
            end = words[-1].end_time.total_seconds()
        else:
            # No word timing — assign sequential indices
            idx = len(segments)
            start = float(idx * 30)
            end = start + 30.0
        segments.append({"text": alt.transcript.strip(), "start": start, "end": end})

    return segments


def _split_audio_ffmpeg(audio_path: Path, segment_seconds: int) -> list[Path]:
    """Split an audio file into fixed-duration segments using ffmpeg.

    Args:
        audio_path: Path to the audio file to split.
        segment_seconds: Maximum duration of each output segment in seconds.

    Returns:
        List of Paths to the segment files, in order.  Caller is responsible
        for deleting them.

    Raises:
        RuntimeError: If ffmpeg returns a non-zero exit code.
    """
    out_dir = Path(tempfile.mkdtemp(prefix="audio-split-"))
    pattern = str(out_dir / "seg-%03d.wav")

    cmd = [
        "ffmpeg", "-y",
        "-i", str(audio_path),
        "-f", "segment",
        "-segment_time", str(segment_seconds),
        "-c", "copy",
        pattern,
    ]
    result = subprocess.run(cmd, capture_output=True, timeout=600)
    if result.returncode != 0:
        raise RuntimeError(
            f"ffmpeg split failed (exit {result.returncode}): {result.stderr.decode()[:500]}"
        )

    return sorted(out_dir.glob("seg-*.wav"))


# ── Chunk building ────────────────────────────────────────────────────────────


def _build_timestamped_chunks(
    segments: list[dict],
    material_id: UUID,
    course_id: UUID,
) -> list[dict]:
    """Group transcript segments into chunks preserving timestamps.

    Segments are accumulated until the combined text would exceed
    ``chunk_max_tokens * 4`` characters, at which point the buffer is
    emitted as a chunk.  Start and end timestamps of the first and last
    segment in each group are stored in the chunk ``metadata`` field so
    that the frontend can seek to the source position.

    Args:
        segments: List of dicts with ``text``, ``start``, ``end`` keys.
        material_id: UUID of the parent material.
        course_id: UUID of the course.

    Returns:
        List of chunk dicts with ``metadata`` containing ``start_time``
        and ``end_time`` (seconds as float).
    """
    max_chars = settings.chunk_max_tokens * 4
    chunks: list[dict] = []

    buf_texts: list[str] = []
    buf_len = 0
    buf_start: float | None = None
    buf_end: float | None = None

    def _emit() -> None:
        if not buf_texts:
            return
        chunks.append(make_chunk(
            content=" ".join(buf_texts),
            material_id=material_id,
            course_id=course_id,
            chapter=None,
            section=None,
            page=None,
            content_type="text",
            metadata={"start_time": buf_start, "end_time": buf_end},
        ))

    for seg in segments:
        text = seg["text"]
        text_len = len(text)

        if buf_len + text_len > max_chars and buf_texts:
            _emit()
            buf_texts = []
            buf_len = 0
            buf_start = seg["start"]
            buf_end = seg["end"]

        if not buf_texts:
            buf_start = seg["start"]

        buf_texts.append(text)
        buf_len += text_len
        buf_end = seg["end"]

    _emit()
    return chunks
