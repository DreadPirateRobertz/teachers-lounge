from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Application settings loaded from environment variables and .env file."""

    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    # GCS
    gcs_raw_bucket: str = "tvtutor-raw-uploads"
    gcp_project: str = "tvtutor-prod"

    # Pub/Sub
    pubsub_ingest_topic: str = "ingest-jobs"
    pubsub_ingest_subscription: str = "ingest-jobs-sub"

    # Postgres
    database_url: str = "postgresql+asyncpg://postgres:postgres@localhost:5432/tvtutor"

    # JWT validation (shared secret from User Service)
    jwt_secret: str = "REPLACE_ME"
    jwt_algorithm: str = "HS256"
    jwt_audience: str = "teacherslounge-services"

    # Upload limits
    max_upload_bytes: int = 500 * 1024 * 1024  # 500 MB

    # Qdrant
    qdrant_host: str = "qdrant.qdrant.svc.cluster.local"
    qdrant_port: int = 6333
    qdrant_api_key: str | None = None
    curriculum_collection: str = "curriculum"

    # OpenAI Embeddings
    openai_api_key: str = ""
    embedding_model: str = "text-embedding-3-large"
    embedding_dim: int = 1024
    embedding_batch_size: int = 100

    # Chunking
    chunk_max_tokens: int = 512
    chunk_overlap_tokens: int = 64

    # CLIP diagram embeddings (Phase 6)
    diagrams_collection: str = "diagrams"
    clip_model: str = "openai/clip-vit-base-patch32"
    clip_embedding_dim: int = 768
    gcs_figures_bucket: str = "tvtutor-raw-uploads"  # figures extracted to same bucket

    # Video/audio transcription
    transcription_provider: str = "openai"
    whisper_model: str = "whisper-1"
    audio_segment_max_seconds: int = 600
    whisper_endpoint: str = "http://whisper-service.whisper.svc.cluster.local:9000"
    whisper_timeout_seconds: int = 600

    # Google Document AI (OCR / image processing)
    document_ai_location: str = "us"
    document_ai_processor_name: str = ""
    document_ai_ocr_processor_id: str = ""
    document_ai_form_processor_id: str = ""
    document_ai_low_confidence_threshold: float = 0.7


settings = Settings()
