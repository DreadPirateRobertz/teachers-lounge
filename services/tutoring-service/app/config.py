from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8")

    # Database
    database_url: str = "postgresql+asyncpg://postgres:postgres@localhost:5432/teachers_lounge"

    # AI Gateway (LiteLLM proxy)
    ai_gateway_url: str = "http://ai-gateway.teachers-lounge.svc.cluster.local:4000"
    ai_gateway_key: str = "REPLACE_ME"

    # Model aliases (defined in LiteLLM config)
    tutor_primary_model: str = "tutor-primary"
    tutor_fast_model: str = "tutor-fast"

    # Service
    service_host: str = "0.0.0.0"
    service_port: int = 8080
    log_level: str = "info"

    # Conversation limits
    max_history_messages: int = 50     # messages kept in context window
    max_message_length: int = 8000     # chars per student message


settings = Settings()
