from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Application settings loaded from environment variables and .env file."""

    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8")

    # Database
    database_url: str = "postgresql+asyncpg://postgres:postgres@localhost:5432/teachers_lounge"

    # AI Gateway (LiteLLM proxy)
    ai_gateway_url: str = "http://ai-gateway.teachers-lounge.svc.cluster.local:4000"
    ai_gateway_key: str = "REPLACE_ME"

    # Model aliases (defined in LiteLLM config)
    tutor_primary_model: str = "tutor-primary"
    tutor_fast_model: str = "tutor-fast"

    # JWT validation (shared secret from User Service)
    # User Service signs tokens with this secret; all services validate with it.
    jwt_secret: str = "REPLACE_ME"
    jwt_algorithm: str = "HS256"
    jwt_audience: str = "teacherslounge-services"

    # CORS — comma-separated list of allowed origins
    allowed_origins: str = "http://localhost:3000"

    # Service
    service_host: str = "0.0.0.0"
    service_port: int = 8080
    log_level: str = "info"

    # Conversation limits
    # "last 10 exchanges" = 10 student messages + 10 tutor replies = 20 messages
    max_history_messages: int = 20
    max_message_length: int = 8000     # chars per student message
    # Context window management (tl-8co)
    context_summarise_threshold: int = 40   # total messages above which older context is summarised
    model_context_limit: int = 131072       # target model context window in tokens (128k)
    context_token_warn_ratio: float = 0.8   # log WARNING at this fraction of context limit

    # User Service — for learning profile reads/writes
    user_service_url: str = "http://user-service.teachers-lounge.svc.cluster.local:8080"

    # Redis — session history cache
    redis_url: str = "redis://redis.teachers-lounge.svc.cluster.local:6379/0"
    # TTL for cached session history snapshots (seconds)
    session_history_cache_ttl: int = 300  # 5 minutes

    # Search Service — called by the agentic RAG pipeline for curriculum retrieval
    search_service_url: str = "http://search-service.teachers-lounge.svc.cluster.local:8080"

    # RAG pipeline
    rag_chunk_limit: int = 8  # max curriculum chunks retrieved per query

    # Diagram retrieval (Phase 6 CLIP search)
    diagram_limit: int = 1  # diagrams embedded per tutor response
    # GCS signed URL expiry in seconds (used when generating pre-signed URLs)
    gcs_signed_url_expiry: int = 3600

    # GCP project for signing GCS URLs
    gcp_project: str = "tvtutor-prod"


settings = Settings()
