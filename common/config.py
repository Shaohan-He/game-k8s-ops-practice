from functools import lru_cache

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    service_name: str = "unknown-service"
    service_port: int = 8000
    log_level: str = "INFO"

    redis_url: str = "redis://redis:6379/0"

    mysql_host: str = "mysql"
    mysql_port: int = 3306
    mysql_database: str = "game_ops"
    mysql_user: str = "game"
    mysql_password: str = "game_password"

    kafka_bootstrap_servers: str = "kafka:9092"

    login_service_url: str = "http://login-service:8001"
    match_service_url: str = "http://match-service:8002"
    room_service_url: str = "http://room-service:8003"

    model_config = SettingsConfigDict(env_file=".env", extra="ignore")


@lru_cache
def get_settings() -> Settings:
    return Settings()

