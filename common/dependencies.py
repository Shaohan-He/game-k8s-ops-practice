import json
import logging
from datetime import UTC, datetime
from typing import Any

import aiomysql
from aiokafka import AIOKafkaProducer
from redis.asyncio import Redis

from common.config import Settings

logger = logging.getLogger(__name__)


class Infrastructure:
    def __init__(self, settings: Settings):
        self.settings = settings
        self.redis: Redis | None = None
        self.mysql: aiomysql.Pool | None = None
        self.kafka: AIOKafkaProducer | None = None

    async def connect(self) -> None:
        self.redis = Redis.from_url(self.settings.redis_url, decode_responses=True)
        self.mysql = await aiomysql.create_pool(
            host=self.settings.mysql_host,
            port=self.settings.mysql_port,
            user=self.settings.mysql_user,
            password=self.settings.mysql_password,
            db=self.settings.mysql_database,
            autocommit=True,
            minsize=1,
            maxsize=5,
        )
        self.kafka = AIOKafkaProducer(
            bootstrap_servers=self.settings.kafka_bootstrap_servers,
            value_serializer=lambda value: json.dumps(
                value, ensure_ascii=False
            ).encode("utf-8"),
        )
        await self.kafka.start()

    async def close(self) -> None:
        if self.kafka:
            await self.kafka.stop()
        if self.mysql:
            self.mysql.close()
            await self.mysql.wait_closed()
        if self.redis:
            await self.redis.aclose()

    async def health(self) -> dict[str, str]:
        result = {"redis": "down", "mysql": "down", "kafka": "down"}
        try:
            if self.redis and await self.redis.ping():
                result["redis"] = "up"
        except Exception:
            logger.exception("Redis health check failed")
        try:
            if self.mysql:
                async with self.mysql.acquire() as conn:
                    async with conn.cursor() as cursor:
                        await cursor.execute("SELECT 1")
                result["mysql"] = "up"
        except Exception:
            logger.exception("MySQL health check failed")
        try:
            if self.kafka:
                await self.kafka.client.force_metadata_update()
                result["kafka"] = "up"
        except Exception:
            logger.exception("Kafka health check failed")
        return result

    async def publish(self, topic: str, event: str, data: dict[str, Any]) -> None:
        if not self.kafka:
            raise RuntimeError("Kafka producer is not initialized")
        payload = {
            "event": event,
            "service": self.settings.service_name,
            "timestamp": datetime.now(UTC).isoformat(),
            "data": data,
        }
        try:
            await self.kafka.send_and_wait(topic, payload)
        except Exception:
            logger.exception("Kafka publish failed", extra={"event": event})
            try:
                await self.kafka.send_and_wait(
                    "game.exception.events",
                    {
                        "event": "publish_failed",
                        "service": self.settings.service_name,
                        "timestamp": datetime.now(UTC).isoformat(),
                        "data": {"target_topic": topic, "original_event": event},
                    },
                )
            except Exception:
                logger.exception("Exception event publish failed")
            raise

