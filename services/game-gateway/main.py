import logging
import uuid

import httpx
from fastapi import HTTPException, Request, Response

from common.app import create_app
from common.config import get_settings

app = create_app("Game Gateway")
settings = get_settings()
logger = logging.getLogger("game-gateway")

ROUTES = {
    "/login": settings.login_service_url,
    "/logout": settings.login_service_url,
    "/match": settings.match_service_url,
    "/match/status": settings.match_service_url,
    "/room/create": settings.room_service_url,
    "/room/join": settings.room_service_url,
    "/room/leave": settings.room_service_url,
    "/room/status": settings.room_service_url,
}


async def proxy(request: Request, target_base: str) -> Response:
    request_id = request.headers.get("x-request-id", str(uuid.uuid4()))
    headers = {
        "content-type": request.headers.get("content-type", "application/json"),
        "x-request-id": request_id,
        "x-forwarded-for": request.client.host if request.client else "unknown",
    }
    body = await request.body()
    try:
        async with httpx.AsyncClient(timeout=5.0) as client:
            upstream = await client.request(
                request.method,
                f"{target_base}{request.url.path}",
                params=request.query_params,
                content=body,
                headers=headers,
            )
        return Response(
            content=upstream.content,
            status_code=upstream.status_code,
            media_type=upstream.headers.get("content-type"),
            headers={"x-request-id": request_id},
        )
    except httpx.RequestError as exc:
        logger.exception("Upstream request failed", extra={"request_id": request_id})
        raise HTTPException(status_code=502, detail="上游服务不可用") from exc


def create_proxy_endpoint(target_base: str):
    async def endpoint(request: Request) -> Response:
        return await proxy(request, target_base)

    return endpoint


for route, target in ROUTES.items():
    app.add_api_route(
        route,
        endpoint=create_proxy_endpoint(target),
        methods=["GET", "POST"],
        tags=["gateway"],
    )

