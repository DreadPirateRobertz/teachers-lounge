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
    """

    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8")

    database_url: str = "postgresql+asyncpg://tl_app:localdevpassword@localhost:5432/teacherslounge"

    jwt_secret: str = "REPLACE_ME"
    jwt_algorithm: str = "HS256"
    jwt_audience: str = "teacherslounge-services"

    allowed_origins: str = "http://localhost:3000"

    log_level: str = "info"


settings = Settings()
