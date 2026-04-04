from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    qdrant_host: str = "qdrant.qdrant.svc.cluster.local"
    qdrant_port: int = 6333
    qdrant_api_key: str | None = None

    curriculum_collection: str = "curriculum"
    embedding_dim: int = 1024

    default_search_limit: int = 10
    max_search_limit: int = 50

    # OpenAI embeddings (Phase 2). When None, falls back to random unit vector stub.
    openai_api_key: str | None = None
    openai_embedding_model: str = "text-embedding-3-large"

    # Number of candidates fetched per signal before RRF fusion + final limit
    sparse_rerank_limit: int = 20


settings = Settings()
