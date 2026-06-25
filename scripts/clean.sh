#!/usr/bin/env bash
set -Eeuo pipefail

MODE="${1:-compose}"

case "${MODE}" in
  compose)
    docker compose down
    echo "Compose 容器和网络已删除，数据卷保留。使用 '$0 compose-volumes' 同时删除数据。"
    ;;
  compose-volumes)
    docker compose down --volumes
    ;;
  k8s)
    kubectl delete namespace game-ops --ignore-not-found
    ;;
  *)
    echo "用法：$0 [compose|compose-volumes|k8s]" >&2
    exit 2
    ;;
esac

