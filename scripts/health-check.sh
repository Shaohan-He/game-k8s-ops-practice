#!/usr/bin/env bash
set -Eeuo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
SERVICES=(
  "${BASE_URL}/health"
  "http://localhost:8001/health"
  "http://localhost:8002/health"
  "http://localhost:8003/health"
)

failed=0
for url in "${SERVICES[@]}"; do
  echo "[check] ${url}"
  if ! curl --fail --silent --show-error --max-time 5 "${url}"; then
    failed=1
  fi
  echo
done

if [[ "${failed}" -ne 0 ]]; then
  echo "存在健康检查失败的服务。" >&2
  exit 1
fi

echo "全部健康检查请求成功。"

