"""Application configuration loaded from environment variables.

Uses pydantic-settings so every field can be overridden by an environment
variable of the same name (case-insensitive).  An optional ``.env`` file is
also read when present (useful for local development).
"""
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Analytics service runtime configuration.

    Attributes:
        database_url: Async SQLAlchemy connection string for Postgres.
        jwt_secret: Shared secret used to validate JWTs issued by user-service.
        jwt_algorithm: JWT signing algorithm (must match user-service config).
        allowed_origins: Comma-separated list of CORS-allowed origins.
        log_level: Python logging level string (e.g. ``"info"``, ``"debug"``).
        gcp_project: GCP project ID hosting the BigQuery dataset.
        bigquery_dataset: BigQuery dataset name for aggregated analytics.
        redis_url: Redis connection URL used for insight caching.
        redis_insight_ttl: TTL in seconds for cached insights (default 6h).
        qdrant_host: Qdrant cluster hostname.
        qdrant_port: Qdrant gRPC/HTTP port.
        qdrant_api_key: Qdrant API key (None for unauthenticated local instances).
        insights_collection: Qdrant collection name for cross-student insights.
        insight_vector_dim: Dimension of text-embedding-3-large output (3072).
        openai_api_key: OpenAI API key used for text-embedding-3-large.
        anthropic_api_key: Anthropic API key used for Claude Haiku insight generation.
        k_anonymity_threshold: Minimum unique students required to export a topic.
        dp_noise_scale: Laplace noise scale for differential privacy on counts.
    """

    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8")

    database_url: str = "postgresql+asyncpg://tl_app:localdevpassword@localhost:5432/teacherslounge"

    jwt_secret: str = "REPLACE_ME"
    jwt_algorithm: str = "HS256"
    jwt_audience: str = "teacherslounge-services"

    allowed_origins: str = "http://localhost:3000"

    log_level: str = "info"

    # BigQuery
    gcp_project: str = "teachers-lounge"
    bigquery_dataset: str = "tvtutor_analytics"

    # Redis
    redis_url: str = "redis://redis.teachers-lounge.svc.cluster.local:6379/0"
    redis_insight_ttl: int = 21600  # 6 hours

    # Qdrant
    qdrant_host: str = "qdrant.qdrant.svc.cluster.local"
    qdrant_port: int = 6333
    qdrant_api_key: str | None = None
    insights_collection: str = "insights"
    insight_vector_dim: int = 3072  # text-embedding-3-large

    # AI providers
    openai_api_key: str = ""
    anthropic_api_key: str = ""

    # Privacy parameters
    k_anonymity_threshold: int = 10
    dp_noise_scale: float = 1.0


settings = Settings()
