import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI

from common.config import get_settings
from common.dependencies import Infrastructure
from common.logging import configure_logging
from common.metrics import metrics_middleware, metrics_response


def create_app(title: str) -> FastAPI:
    settings = get_settings()
    configure_logging(settings.log_level)
    logger = logging.getLogger(settings.service_name)

    @asynccontextmanager
    async def lifespan(app: FastAPI):
        infrastructure = Infrastructure(settings)
        app.state.infrastructure = infrastructure
        try:
            await infrastructure.connect()
            logger.info("Infrastructure connected", extra={"service": settings.service_name})
            yield
        except Exception:
            logger.exception(
                "Service startup failed", extra={"service": settings.service_name}
            )
            raise
        finally:
            await infrastructure.close()

    app = FastAPI(title=title, version="1.0.0", lifespan=lifespan)
    app.middleware("http")(metrics_middleware(settings.service_name))

    @app.get("/health", tags=["ops"])
    async def health():
        dependencies = await app.state.infrastructure.health()
        healthy = all(value == "up" for value in dependencies.values())
        return {
            "status": "ok" if healthy else "degraded",
            "service": settings.service_name,
            "dependencies": dependencies,
        }

    @app.get("/metrics", include_in_schema=False)
    async def metrics():
        return metrics_response()

    return app

