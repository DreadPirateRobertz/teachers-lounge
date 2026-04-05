"""Configuration for the eval service — loaded from environment variables."""
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Eval service runtime configuration.

    All values are read from environment variables (or a .env file in development).
    """

    # Postgres (Cloud SQL via Cloud SQL Auth Proxy)
    database_url: str = "postgresql+asyncpg://postgres:postgres@localhost:5432/teacherslounge"

    # BigQuery
    bigquery_project: str = "teachers-lounge"
    bigquery_dataset: str = "analytics"

    # Anthropic API (for LLM judge via direct SDK — bypasses AI Gateway)
    anthropic_api_key: str = ""
    judge_model: str = "claude-haiku-4-5-20251001"

    # Eval parameters
    ragas_weekly_sample_size: int = 100   # interactions sampled per RAGAS run
    llm_judge_nightly_sample: int = 20   # interactions sampled per nightly run
    faithfulness_alert_threshold: float = 0.7  # alert if RAGAS faithfulness drops below
    effectiveness_flag_threshold: float = 0.1  # flag topics below this effectiveness

    model_config = SettingsConfigDict(env_file=".env", extra="ignore")


settings = Settings()
