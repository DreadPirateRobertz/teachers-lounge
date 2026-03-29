from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    qdrant_host: str = "qdrant.qdrant.svc.cluster.local"
    qdrant_port: int = 6333
    qdrant_api_key: str = ""

    curriculum_collection: str = "curriculum"
    embedding_dim: int = 1024

    default_search_limit: int = 10
    max_search_limit: int = 50


settings = Settings()
