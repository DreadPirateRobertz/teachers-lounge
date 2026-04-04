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


settings = Settings()
