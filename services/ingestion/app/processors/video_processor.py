"""Video and audio transcription pipeline.

Handles MP4, MOV (video) and MP3, WAV (audio). Video files have audio
extracted via ffmpeg before transcription. Transcription uses self-hosted
Whisper on GKE GPU; Google Speech-to-Text is used as fallback when Whisper
is unavailable. Output: timestamped transcript chunks for citation support.
"""
import asyncio
import logging
import subprocess
import tempfile
from pathlib import Path
from uuid import UUID

import httpx

# Expose `speech` at module level so tests can patch it directly.
# The actual import is deferred to first use to avoid requiring
# google-cloud-speech at import time.
try:
    from google.cloud import speech  # type: ignore[import-untyped]
except ImportError:  # pragma: no cover
    speech = None  # type: ignore[assignment]

from app.config import settings
from app.models import IngestJobMessage, ProcessingStatus
from app.services import db, embeddings, gcs, qdrant
from app.services.chunking import flush_segments as _flush_segments

logger = logging.getLogger(__name__)

# Video MIME types that require audio extraction before transcription
_VIDEO_MIMES = {"video/mp4", "video/quicktime"}
# Audio MIME types passed directly to transcription
_AUDIO_MIMES = {"audio/mpeg", "audio/wav"}

# Maximum audio file size for in-memory Google STT (1 minute @ 16kHz = ~1.9 MB)
_STT_INLINE_MAX_BYTES = 10 * 1024 * 1024  # 10 MB

# Confidence threshold below which we fall back to Form Parser for OCR
_WHISPER_TRANSCRIPT_KEY = "segments"


async def process_video(job: IngestJobMessage) -> dict:
    """Full video/audio transcription pipeline.

    Downloads the source file, extracts audio from video if needed,
    transcribes with Whisper (self-hosted) or Google STT (fallback),
    then chunks the timestamped transcript and writes to Qdrant + Postgres.

    Args:
        job: Pub/Sub message describing the upload.

    Returns:
        Dict with status, job_id, chunk_count, and processor key.
    """
    logger.info("video_processor: starting job_id=%s file=%s", job.job_id, job.filename)
    await db.update_material_status(job.material_id, ProcessingStatus.PROCESSING)

    source_path: Path | None = None
    audio_path: Path | None = None

    try:
        source_path = await gcs.download_file(job.gcs_path, job.job_id)

        # Extract audio track from video files
        if job.mime_type in _VIDEO_MIMES:
            loop = asyncio.get_running_loop()
            audio_path = await loop.run_in_executor(
                None, _extract_audio, source_path, job.job_id
            )
        else:
            audio_path = source_path

        # Transcribe — try Whisper first, fall back to Google STT
        transcript_segments = await _transcribe(audio_path)

        logger.info(
            "video_processor: job_id=%s transcribed %d segments",
            job.job_id, len(transcript_segments),
        )

        # Build chunks from transcript segments
        max_chars = settings.chunk_max_tokens * 4
        overlap_chars = settings.chunk_overlap_tokens * 4
        chunks = _flush_segments(
            transcript_segments, job.material_id, job.course_id, max_chars, overlap_chars
        )

        logger.info("video_processor: job_id=%s built %d chunks", job.job_id, len(chunks))

        if not chunks:
            await db.update_material_status(job.material_id, ProcessingStatus.COMPLETE, chunk_count=0)
            return {"status": "complete", "job_id": str(job.job_id), "chunk_count": 0, "processor": "video"}

        texts = [c["content"] for c in chunks]
        vectors = await embeddings.embed_texts(texts)

        chunk_ids = [c["id"] for c in chunks]
        payloads = [
            {
                "chunk_id": str(c["id"]),
                "material_id": str(c["material_id"]),
                "course_id": str(c["course_id"]),
                "content": c["content"],
                "chapter": c.get("chapter"),
                "section": c.get("section"),
                "page": c.get("page"),
                "content_type": c.get("content_type", "text"),
                "start_time_ms": c.get("metadata", {}).get("start_time_ms"),
            }
            for c in chunks
        ]
        await qdrant.upsert_chunks(chunk_ids, vectors, payloads)
        await db.insert_chunks(chunks)
        await db.update_material_status(job.material_id, ProcessingStatus.COMPLETE, chunk_count=len(chunks))

        logger.info("video_processor: complete job_id=%s chunks=%d", job.job_id, len(chunks))
        return {
            "status": "complete",
            "job_id": str(job.job_id),
            "chunk_count": len(chunks),
            "processor": "video",
        }

    except Exception:
        logger.exception("video_processor: failed job_id=%s", job.job_id)
        await db.update_material_status(job.material_id, ProcessingStatus.FAILED)
        raise
    finally:
        for p in (source_path, audio_path):
            if p and p != source_path:
                try:
                    p.unlink(missing_ok=True)
                except Exception:
                    pass
        if source_path:
            try:
                source_path.unlink(missing_ok=True)
            except Exception:
                pass


# ── Audio extraction ───────────────────────────────────────────────────────────


def _extract_audio(source_path: Path, job_id: UUID) -> Path:
    """Extract a mono 16 kHz PCM WAV audio track from a video file via ffmpeg.

    Args:
        source_path: Local path to the video file (mp4, mov, etc.).
        job_id: Job UUID used for naming the temp output file.

    Returns:
        Path to the extracted WAV file. Caller is responsible for cleanup.

    Raises:
        subprocess.CalledProcessError: If ffmpeg exits with a non-zero status.
    """
    tmp = tempfile.NamedTemporaryFile(
        delete=False, suffix=".wav", prefix=f"audio-{job_id}-"
    )
    tmp.close()
    audio_path = Path(tmp.name)

    cmd = [
        "ffmpeg", "-y",
        "-i", str(source_path),
        "-vn",                    # no video
        "-acodec", "pcm_s16le",   # 16-bit PCM
        "-ar", "16000",           # 16 kHz sample rate (optimal for Whisper / STT)
        "-ac", "1",               # mono
        str(audio_path),
    ]
    logger.info("extracting audio: %s", " ".join(cmd))
    result = subprocess.run(cmd, capture_output=True, text=True)
    if result.returncode != 0:
        audio_path.unlink(missing_ok=True)
        raise subprocess.CalledProcessError(
            result.returncode, cmd, result.stdout, result.stderr
        )
    logger.info("extracted audio → %s", audio_path)
    return audio_path


# ── Transcription ─────────────────────────────────────────────────────────────


async def _transcribe(audio_path: Path) -> list[dict]:
    """Transcribe audio using Whisper (primary) or Google STT (fallback).

    Returns a list of segment dicts with 'text', 'chapter', 'section',
    'page', 'content_type', and 'metadata' keys. The 'metadata' field
    contains 'start_time_ms' for citation purposes.

    Args:
        audio_path: Local path to the audio file (WAV preferred).

    Returns:
        List of segment dicts compatible with _flush_segments.
    """
    try:
        return await _transcribe_whisper(audio_path)
    except Exception as exc:
        logger.warning(
            "whisper transcription failed (%s), falling back to Google STT", exc
        )
        return await _transcribe_google_stt(audio_path)


async def _transcribe_whisper(audio_path: Path) -> list[dict]:
    """Transcribe audio using self-hosted Whisper ASR webservice.

    POSTs the audio file to ``{whisper_endpoint}/asr?output=json``.
    The service returns a JSON object with a ``segments`` array, each element
    having ``start`` (seconds), ``end`` (seconds), and ``text`` fields.

    Args:
        audio_path: Local path to the audio file.

    Returns:
        List of segment dicts with start_time_ms in metadata.

    Raises:
        httpx.HTTPError: If the HTTP request fails or returns a non-2xx status.
        ValueError: If the response JSON is missing expected fields.
    """
    url = f"{settings.whisper_endpoint}/asr"
    params = {"output": "json", "task": "transcribe"}

    async with httpx.AsyncClient(timeout=settings.whisper_timeout_seconds) as client:
        with open(audio_path, "rb") as fh:
            files = {"audio_file": (audio_path.name, fh, "audio/wav")}
            response = await client.post(url, params=params, files=files)

    response.raise_for_status()
    data = response.json()

    raw_segments = data.get(_WHISPER_TRANSCRIPT_KEY, [])
    if not raw_segments and "text" in data:
        # Some Whisper builds return a flat text without segment timestamps
        return [_transcript_seg(data["text"], start_ms=0)]

    return [
        _transcript_seg(
            seg.get("text", "").strip(),
            start_ms=int(seg.get("start", 0) * 1000),
        )
        for seg in raw_segments
        if seg.get("text", "").strip()
    ]


async def _transcribe_google_stt(audio_path: Path) -> list[dict]:
    """Transcribe audio using Google Cloud Speech-to-Text v1.

    Uses synchronous recognition for audio under _STT_INLINE_MAX_BYTES, and
    ``long_running_recognize`` (with local content) for larger files. Word-level
    timestamps are requested and aggregated into sentence-level segments.

    Args:
        audio_path: Local path to the audio file.

    Returns:
        List of segment dicts with start_time_ms in metadata.

    Raises:
        RuntimeError: If google-cloud-speech is not installed.
    """
    if speech is None:  # pragma: no cover
        raise RuntimeError(
            "google-cloud-speech is not installed — "
            "add it to requirements.txt to enable the Google STT fallback"
        )

    audio_bytes = audio_path.read_bytes()

    # Detect encoding from file extension
    suffix = audio_path.suffix.lower()
    if suffix in (".wav",):
        encoding = speech.RecognitionConfig.AudioEncoding.LINEAR16
        sample_rate = 16000
    elif suffix in (".mp3",):
        encoding = speech.RecognitionConfig.AudioEncoding.MP3
        sample_rate = 0  # auto-detect for mp3
    else:
        encoding = speech.RecognitionConfig.AudioEncoding.ENCODING_UNSPECIFIED
        sample_rate = 0

    config = speech.RecognitionConfig(
        encoding=encoding,
        sample_rate_hertz=sample_rate if sample_rate else None,
        language_code="en-US",
        enable_word_time_offsets=True,
        model="video",
    )
    audio = speech.RecognitionAudio(content=audio_bytes)

    loop = asyncio.get_running_loop()

    if len(audio_bytes) <= _STT_INLINE_MAX_BYTES:
        response = await loop.run_in_executor(
            None, lambda: speech.SpeechClient().recognize(config=config, audio=audio)
        )
    else:
        operation = await loop.run_in_executor(
            None,
            lambda: speech.SpeechClient().long_running_recognize(config=config, audio=audio),
        )
        response = await loop.run_in_executor(
            None, lambda: operation.result(timeout=settings.whisper_timeout_seconds)
        )

    segments: list[dict] = []
    for result in response.results:
        if not result.alternatives:
            continue
        alt = result.alternatives[0]
        text = alt.transcript.strip()
        if not text:
            continue

        # Use the start time of the first word for chunk citation
        start_ms = 0
        if alt.words:
            start_nanos = alt.words[0].start_time.total_seconds() * 1000
            start_ms = int(start_nanos)

        segments.append(_transcript_seg(text, start_ms=start_ms))

    return segments


def _transcript_seg(text: str, start_ms: int) -> dict:
    """Build a segment dict for a transcript sentence group.

    Args:
        text: Transcript text for this segment.
        start_ms: Start timestamp in milliseconds for citation.

    Returns:
        Segment dict with 'metadata' containing 'start_time_ms'.
    """
    return {
        "text": text,
        "chapter": None,
        "section": None,
        "page": None,
        "content_type": "transcript",
        "metadata": {"start_time_ms": start_ms},
    }
