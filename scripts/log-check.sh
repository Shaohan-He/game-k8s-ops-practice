#!/usr/bin/env bash
set -Eeuo pipefail

MODE="${1:-compose}"
SERVICE="${2:-}"
PATTERN="${LOG_PATTERN:-error|exception|traceback|timeout|refused}"

if [[ "${MODE}" == "compose" ]]; then
  args=(logs --no-color --since 30m)
  [[ -n "${SERVICE}" ]] && args+=("${SERVICE}")
  docker compose "${args[@]}" 2>&1 | grep -Ein "${PATTERN}" || true
elif [[ "${MODE}" == "k8s" ]]; then
  selector="${SERVICE:+app=${SERVICE}}"
  if [[ -n "${selector}" ]]; then
    kubectl -n game-ops logs -l "${selector}" --all-containers --since=30m --prefix 2>&1 |
      grep -Ein "${PATTERN}" || true
  else
    kubectl -n game-ops get pods
    echo "请指定服务，例如：$0 k8s login-service"
  fi
else
  echo "用法：$0 [compose|k8s] [service]" >&2
  exit 2
fi

