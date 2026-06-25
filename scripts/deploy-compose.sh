#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

if [[ ! -f .env ]]; then
  cp .env.example .env
  echo "已从 .env.example 创建 .env，请勿在真实环境沿用默认密码。"
fi

docker compose config --quiet
docker compose up --detach --build
docker compose ps

echo "入口：http://localhost:8080  Grafana：http://localhost:3000  Prometheus：http://localhost:9090"

