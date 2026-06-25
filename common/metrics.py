import time
from collections.abc import Awaitable, Callable

from fastapi import Request, Response
from prometheus_client import CONTENT_TYPE_LATEST, Counter, Histogram, generate_latest

REQUEST_COUNT = Counter(
    "game_http_requests_total",
    "HTTP 请求总数",
    ["service", "method", "path", "status"],
)
REQUEST_LATENCY = Histogram(
    "game_http_request_duration_seconds",
    "HTTP 请求耗时",
    ["service", "method", "path"],
)
BUSINESS_EVENTS = Counter(
    "game_business_events_total",
    "业务事件总数",
    ["service", "event", "result"],
)


def metrics_response() -> Response:
    return Response(generate_latest(), media_type=CONTENT_TYPE_LATEST)


def metrics_middleware(service_name: str) -> Callable:
    async def middleware(
        request: Request,
        call_next: Callable[[Request], Awaitable[Response]],
    ) -> Response:
        started = time.perf_counter()
        status = 500
        path = request.url.path
        try:
            response = await call_next(request)
            status = response.status_code
            return response
        finally:
            REQUEST_COUNT.labels(service_name, request.method, path, str(status)).inc()
            REQUEST_LATENCY.labels(service_name, request.method, path).observe(
                time.perf_counter() - started
            )

    return middleware

