from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    # GCS
    gcs_raw_bucket: str = "tvtutor-raw-uploads"
    gcp_project: str = "tvtutor-prod"

    # Pub/Sub
    pubsub_ingest_topic: str = "ingest-jobs"
    pubsub_ingest_subscription: str = "ingest-jobs-sub"

    # Postgres
    database_url: str = "postgresql+asyncpg://postgres:postgres@localhost:5432/tvtutor"

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


settings = Settings()
