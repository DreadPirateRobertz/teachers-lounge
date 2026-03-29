from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    qdrant_host: str = "qdrant.qdrant.svc.cluster.local"
    qdrant_port: int = 6333
    qdrant_api_key: str | None = None

    curriculum_collection: str = "curriculum"

    # Embedding — OpenAI text-embedding-3-small via AI Gateway (LiteLLM)
    # Dimension uses the model's native 1536-dim output (no truncation for small model).
    # Phase 4+ migration to self-hosted uses same config; only gateway URL changes.
    ai_gateway_url: str = "http://litellm-service.ai-gateway.svc.cluster.local:4000"
    embedding_model: str = "text-embedding-3-small"
    embedding_dim: int = 1536

    default_search_limit: int = 10
    max_search_limit: int = 50


settings = Settings()
