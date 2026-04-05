"""Tests for video and audio transcription pipeline."""
import subprocess
import uuid
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from app.models import IngestJobMessage, ProcessingStatus
from app.processors.video_processor import (
    _extract_audio,
    _transcript_seg,
    _transcribe_google_stt,
    _transcribe_whisper,
    process_video,
)


def _make_job(
    mime_type: str = "video/mp4",
    filename: str = "lecture.mp4",
) -> IngestJobMessage:
    """Build a minimal IngestJobMessage for video processor tests."""
    return IngestJobMessage(
        job_id=uuid.uuid4(),
        user_id=uuid.uuid4(),
        course_id=uuid.uuid4(),
        material_id=uuid.uuid4(),
        gcs_path=f"gs://tvtutor-raw-uploads/u/c/j/{filename}",
        mime_type=mime_type,
        filename=filename,
    )


# ── Segment builder ───────────────────────────────────────────────────────────


class TestTranscriptSeg:
    def test_fields(self):
        """_transcript_seg builds segment dict with correct fields."""
        seg = _transcript_seg("Hello everyone.", start_ms=3500)
        assert seg["text"] == "Hello everyone."
        assert seg["content_type"] == "transcript"
        assert seg["metadata"]["start_time_ms"] == 3500
        assert seg["chapter"] is None
        assert seg["section"] is None
        assert seg["page"] is None

    def test_zero_start(self):
        """start_ms=0 is stored correctly for the first segment."""
        seg = _transcript_seg("Opening remarks.", start_ms=0)
        assert seg["metadata"]["start_time_ms"] == 0


# ── Audio extraction ──────────────────────────────────────────────────────────


class TestExtractAudio:
    def test_successful_extraction(self, tmp_path):
        """ffmpeg success returns a WAV path."""
        source = tmp_path / "video.mp4"
        source.write_bytes(b"fake-video")
        job_id = uuid.uuid4()

        with patch("app.processors.video_processor.subprocess.run") as mock_run:
            mock_run.return_value = MagicMock(returncode=0, stdout="", stderr="")
            audio_path = _extract_audio(source, job_id)

        assert audio_path.suffix == ".wav"
        # Verify ffmpeg was called with correct flags
        cmd = mock_run.call_args.args[0]
        assert "ffmpeg" in cmd
        assert "-ar" in cmd
        assert "16000" in cmd
        assert "-ac" in cmd
        assert "1" in cmd
        audio_path.unlink(missing_ok=True)

    def test_ffmpeg_failure_raises(self, tmp_path):
        """Non-zero ffmpeg exit raises CalledProcessError."""
        source = tmp_path / "bad.mp4"
        source.write_bytes(b"corrupt")
        job_id = uuid.uuid4()

        with patch("app.processors.video_processor.subprocess.run") as mock_run:
            mock_run.return_value = MagicMock(
                returncode=1, stdout="", stderr="No such file"
            )
            with pytest.raises(subprocess.CalledProcessError):
                _extract_audio(source, job_id)


# ── Whisper transcription ─────────────────────────────────────────────────────


class TestTranscribeWhisper:
    @pytest.mark.asyncio
    async def test_happy_path_with_segments(self, tmp_path):
        """Successful Whisper response with segments is parsed correctly."""
        audio = tmp_path / "audio.wav"
        audio.write_bytes(b"fake-wav")

        mock_response = MagicMock()
        mock_response.raise_for_status = MagicMock()
        mock_response.json.return_value = {
            "segments": [
                {"start": 0.0, "end": 5.2, "text": "  Hello everyone.  "},
                {"start": 5.2, "end": 10.1, "text": "Welcome to the lecture."},
            ]
        }

        with patch("app.processors.video_processor.httpx.AsyncClient") as mock_client_cls:
            mock_client = AsyncMock()
            mock_client.post = AsyncMock(return_value=mock_response)
            mock_client.__aenter__ = AsyncMock(return_value=mock_client)
            mock_client.__aexit__ = AsyncMock(return_value=None)
            mock_client_cls.return_value = mock_client

            segments = await _transcribe_whisper(audio)

        assert len(segments) == 2
        assert segments[0]["text"] == "Hello everyone."
        assert segments[0]["metadata"]["start_time_ms"] == 0
        assert segments[1]["metadata"]["start_time_ms"] == 5200
        assert segments[0]["content_type"] == "transcript"

    @pytest.mark.asyncio
    async def test_flat_text_response(self, tmp_path):
        """Whisper response without segments array falls back to flat text."""
        audio = tmp_path / "audio.wav"
        audio.write_bytes(b"fake-wav")

        mock_response = MagicMock()
        mock_response.raise_for_status = MagicMock()
        mock_response.json.return_value = {"text": "Simple flat transcript."}

        with patch("app.processors.video_processor.httpx.AsyncClient") as mock_client_cls:
            mock_client = AsyncMock()
            mock_client.post = AsyncMock(return_value=mock_response)
            mock_client.__aenter__ = AsyncMock(return_value=mock_client)
            mock_client.__aexit__ = AsyncMock(return_value=None)
            mock_client_cls.return_value = mock_client

            segments = await _transcribe_whisper(audio)

        assert len(segments) == 1
        assert "Simple flat transcript." in segments[0]["text"]
        assert segments[0]["metadata"]["start_time_ms"] == 0

    @pytest.mark.asyncio
    async def test_http_error_raises(self, tmp_path):
        """HTTP error from Whisper endpoint propagates as an exception."""
        import httpx

        audio = tmp_path / "audio.wav"
        audio.write_bytes(b"fake-wav")

        with patch("app.processors.video_processor.httpx.AsyncClient") as mock_client_cls:
            mock_client = AsyncMock()
            mock_client.post = AsyncMock(
                side_effect=httpx.ConnectError("connection refused")
            )
            mock_client.__aenter__ = AsyncMock(return_value=mock_client)
            mock_client.__aexit__ = AsyncMock(return_value=None)
            mock_client_cls.return_value = mock_client

            with pytest.raises(httpx.ConnectError):
                await _transcribe_whisper(audio)


# ── Google STT fallback ───────────────────────────────────────────────────────


class TestTranscribeGoogleStt:
    @pytest.mark.asyncio
    async def test_happy_path(self, tmp_path):
        """Google STT response with word timestamps produces segments."""
        audio = tmp_path / "audio.wav"
        audio.write_bytes(b"\x00" * 1000)  # minimal WAV-ish content

        mock_alt = MagicMock()
        mock_alt.transcript = "This is the lecture content."
        first_word = MagicMock()
        first_word.start_time.total_seconds.return_value = 1.5
        mock_alt.words = [first_word]

        mock_result = MagicMock()
        mock_result.alternatives = [mock_alt]

        mock_response = MagicMock()
        mock_response.results = [mock_result]

        mock_speech_client = MagicMock()
        mock_speech_client.recognize.return_value = mock_response

        with patch("app.processors.video_processor.speech") as mock_speech_mod:
            mock_speech_mod.SpeechClient.return_value = mock_speech_client
            mock_speech_mod.RecognitionConfig = MagicMock()
            mock_speech_mod.RecognitionConfig.AudioEncoding = MagicMock()
            mock_speech_mod.RecognitionConfig.AudioEncoding.LINEAR16 = "LINEAR16"
            mock_speech_mod.RecognitionConfig.AudioEncoding.MP3 = "MP3"
            mock_speech_mod.RecognitionConfig.AudioEncoding.ENCODING_UNSPECIFIED = "UNSPEC"
            mock_speech_mod.RecognitionAudio = MagicMock()

            segments = await _transcribe_google_stt(audio)

        assert len(segments) == 1
        assert "lecture content" in segments[0]["text"]
        assert segments[0]["metadata"]["start_time_ms"] == 1500

    @pytest.mark.asyncio
    async def test_empty_results_returns_empty_list(self, tmp_path):
        """Empty STT response returns an empty segment list."""
        audio = tmp_path / "audio.wav"
        audio.write_bytes(b"\x00" * 100)

        mock_response = MagicMock()
        mock_response.results = []

        mock_speech_client = MagicMock()
        mock_speech_client.recognize.return_value = mock_response

        with patch("app.processors.video_processor.speech") as mock_speech_mod:
            mock_speech_mod.SpeechClient.return_value = mock_speech_client
            mock_speech_mod.RecognitionConfig = MagicMock()
            mock_speech_mod.RecognitionConfig.AudioEncoding = MagicMock()
            mock_speech_mod.RecognitionConfig.AudioEncoding.LINEAR16 = "LINEAR16"
            mock_speech_mod.RecognitionConfig.AudioEncoding.MP3 = "MP3"
            mock_speech_mod.RecognitionConfig.AudioEncoding.ENCODING_UNSPECIFIED = "UNSPEC"
            mock_speech_mod.RecognitionAudio = MagicMock()

            segments = await _transcribe_google_stt(audio)

        assert segments == []


# ── Full pipeline (mocked externals) ─────────────────────────────────────────


class TestProcessVideoPipeline:
    @pytest.mark.asyncio
    @patch("app.processors.video_processor.qdrant")
    @patch("app.processors.video_processor.embeddings")
    @patch("app.processors.video_processor.db")
    @patch("app.processors.video_processor.gcs")
    @patch("app.processors.video_processor._transcribe")
    @patch("app.processors.video_processor._extract_audio")
    async def test_mp4_happy_path(
        self, mock_extract, mock_transcribe, mock_gcs, mock_db, mock_embed, mock_qdrant, tmp_path
    ):
        """MP4 job runs audio extraction, transcribes, chunks, and returns complete."""
        job = _make_job("video/mp4", "lecture.mp4")

        fake_source = tmp_path / "video.mp4"
        fake_source.write_bytes(b"fake-video")
        fake_audio = tmp_path / "audio.wav"
        fake_audio.write_bytes(b"fake-wav")

        mock_gcs.download_file = AsyncMock(return_value=fake_source)
        mock_extract.return_value = fake_audio
        mock_transcribe.return_value = [
            _transcript_seg("Lecture segment one.", start_ms=0),
            _transcript_seg("Lecture segment two.", start_ms=5000),
        ]
        mock_embed.embed_texts = AsyncMock(return_value=[[0.1] * 1024, [0.2] * 1024])
        mock_db.update_material_status = AsyncMock()
        mock_db.insert_chunks = AsyncMock()
        mock_qdrant.upsert_chunks = AsyncMock()

        result = await process_video(job)

        assert result["status"] == "complete"
        assert result["processor"] == "video"
        assert result["chunk_count"] >= 1
        mock_extract.assert_called_once()

        calls = mock_db.update_material_status.call_args_list
        assert calls[0].args == (job.material_id, ProcessingStatus.PROCESSING)
        assert calls[-1].args[1] == ProcessingStatus.COMPLETE

    @pytest.mark.asyncio
    @patch("app.processors.video_processor.qdrant")
    @patch("app.processors.video_processor.embeddings")
    @patch("app.processors.video_processor.db")
    @patch("app.processors.video_processor.gcs")
    @patch("app.processors.video_processor._transcribe")
    async def test_mp3_skips_audio_extraction(
        self, mock_transcribe, mock_gcs, mock_db, mock_embed, mock_qdrant, tmp_path
    ):
        """Audio files skip ffmpeg extraction and go directly to transcription."""
        job = _make_job("audio/mpeg", "lecture.mp3")

        fake_audio = tmp_path / "audio.mp3"
        fake_audio.write_bytes(b"fake-mp3")

        mock_gcs.download_file = AsyncMock(return_value=fake_audio)
        mock_transcribe.return_value = [
            _transcript_seg("Audio transcript.", start_ms=0),
        ]
        mock_embed.embed_texts = AsyncMock(return_value=[[0.1] * 1024])
        mock_db.update_material_status = AsyncMock()
        mock_db.insert_chunks = AsyncMock()
        mock_qdrant.upsert_chunks = AsyncMock()

        result = await process_video(job)

        assert result["status"] == "complete"
        # _extract_audio should NOT have been called for audio/* MIME
        # (verified by checking _transcribe received the source path directly)
        mock_transcribe.assert_awaited_once_with(fake_audio)

    @pytest.mark.asyncio
    @patch("app.processors.video_processor.db")
    @patch("app.processors.video_processor.gcs")
    async def test_gcs_failure_marks_failed(self, mock_gcs, mock_db):
        """GCS download failure marks the material as FAILED."""
        job = _make_job()
        mock_gcs.download_file = AsyncMock(side_effect=Exception("network error"))
        mock_db.update_material_status = AsyncMock()

        with pytest.raises(Exception, match="network error"):
            await process_video(job)

        calls = mock_db.update_material_status.call_args_list
        assert calls[-1].args == (job.material_id, ProcessingStatus.FAILED)

    @pytest.mark.asyncio
    @patch("app.processors.video_processor.qdrant")
    @patch("app.processors.video_processor.embeddings")
    @patch("app.processors.video_processor.db")
    @patch("app.processors.video_processor.gcs")
    @patch("app.processors.video_processor._transcribe")
    @patch("app.processors.video_processor._extract_audio")
    async def test_empty_transcript_completes_with_zero_chunks(
        self, mock_extract, mock_transcribe, mock_gcs, mock_db, mock_embed, mock_qdrant, tmp_path
    ):
        """Empty transcript returns complete with chunk_count=0."""
        job = _make_job("video/mp4")

        fake_source = tmp_path / "silent.mp4"
        fake_source.write_bytes(b"silent")
        fake_audio = tmp_path / "silent.wav"
        fake_audio.write_bytes(b"silent")

        mock_gcs.download_file = AsyncMock(return_value=fake_source)
        mock_extract.return_value = fake_audio
        mock_transcribe.return_value = []
        mock_db.update_material_status = AsyncMock()

        result = await process_video(job)

        assert result["status"] == "complete"
        assert result["chunk_count"] == 0
        calls = mock_db.update_material_status.call_args_list
        assert calls[-1].args[1] == ProcessingStatus.COMPLETE

    @pytest.mark.asyncio
    @patch("app.processors.video_processor.qdrant")
    @patch("app.processors.video_processor.embeddings")
    @patch("app.processors.video_processor.db")
    @patch("app.processors.video_processor.gcs")
    @patch("app.processors.video_processor._transcribe_google_stt")
    @patch("app.processors.video_processor._transcribe_whisper")
    async def test_whisper_failure_falls_back_to_stt(
        self, mock_whisper, mock_stt, mock_gcs, mock_db, mock_embed, mock_qdrant, tmp_path
    ):
        """When Whisper raises, Google STT fallback is used."""
        import httpx
        job = _make_job("audio/wav", "lecture.wav")

        fake_audio = tmp_path / "audio.wav"
        fake_audio.write_bytes(b"fake-wav")
        mock_gcs.download_file = AsyncMock(return_value=fake_audio)

        mock_whisper.side_effect = httpx.ConnectError("whisper down")
        mock_stt.return_value = [_transcript_seg("Fallback transcript.", start_ms=0)]

        mock_embed.embed_texts = AsyncMock(return_value=[[0.1] * 1024])
        mock_db.update_material_status = AsyncMock()
        mock_db.insert_chunks = AsyncMock()
        mock_qdrant.upsert_chunks = AsyncMock()

        result = await process_video(job)

        assert result["status"] == "complete"
        mock_stt.assert_awaited_once()
