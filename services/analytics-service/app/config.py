from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8")

    database_url: str = "postgresql+asyncpg://tl_app:localdevpassword@localhost:5432/teacherslounge"

    jwt_secret: str = "REPLACE_ME"
    jwt_algorithm: str = "HS256"

    allowed_origins: str = "http://localhost:3000"

    log_level: str = "info"


settings = Settings()
