#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REGISTRY="${IMAGE_REGISTRY:-game-k8s-ops-practice}"
TAG="${IMAGE_TAG:-1.0.0}"
SERVICES=(game-gateway login-service match-service room-service)

for service in "${SERVICES[@]}"; do
  image="${REGISTRY}/${service}:${TAG}"
  echo "[build] ${image}"
  docker build \
    --file "${ROOT_DIR}/services/${service}/Dockerfile" \
    --tag "${image}" \
    "${ROOT_DIR}"
done

echo "镜像构建完成：${REGISTRY}/*:${TAG}"

