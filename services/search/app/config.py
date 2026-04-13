from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Application settings loaded from environment variables and .env file."""

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

    # Re-ranking — Cohere rerank-english-v3.0 via AI Gateway (LiteLLM).
    # Set to empty string to disable re-ranking (falls back to RRF order).
    rerank_model: str = "rerank-english-v3.0"
    rerank_top_n: int = 10

    default_search_limit: int = 10
    max_search_limit: int = 50

    # OpenAI embeddings (Phase 2). When None, falls back to random unit vector stub.
    openai_api_key: str | None = None
    openai_embedding_model: str = "text-embedding-3-large"

    # Number of candidates fetched per signal before RRF fusion + final limit
    sparse_rerank_limit: int = 20

    # Query expansion via AI Gateway (tl-afb) — used for short (<5-token)
    # follow-up queries when the caller supplies conversation context turns.
    tutor_fast_model: str = "tutor-fast"
    query_expansion_short_threshold: int = 5
    query_expansion_max_context_turns: int = 6
    query_expansion_max_tokens: int = 128

    # Diagram (CLIP) collection — Phase 6 multi-modal RAG
    diagrams_collection: str = "diagrams"
    clip_model: str = "openai/clip-vit-base-patch32"
    clip_embedding_dim: int = 768
    default_diagram_limit: int = 3
    max_diagram_limit: int = 10


settings = Settings()
