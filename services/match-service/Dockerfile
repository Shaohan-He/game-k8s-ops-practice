FROM python:3.12.10-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PYTHONPATH=/app

WORKDIR /app

RUN groupadd --gid 10001 app \
    && useradd --uid 10001 --gid app --no-log-init --create-home app

COPY requirements.txt ./
RUN pip install --no-cache-dir --requirement requirements.txt

COPY --chown=app:app common/ ./common/
COPY --chown=app:app services/match-service/main.py ./main.py

USER app
EXPOSE 8002
HEALTHCHECK --interval=30s --timeout=3s --start-period=15s --retries=3 \
    CMD ["python", "-c", "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8002/health', timeout=2)"]
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8002"]

