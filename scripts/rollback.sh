#!/usr/bin/env bash
set -Eeuo pipefail

DEPLOYMENT="${1:-}"
REVISION="${2:-}"
NAMESPACE="${NAMESPACE:-game-ops}"

if [[ -z "${DEPLOYMENT}" ]]; then
  echo "用法：$0 <game-gateway|login-service|match-service|room-service> [revision]" >&2
  exit 2
fi

kubectl -n "${NAMESPACE}" rollout history "deployment/${DEPLOYMENT}"
if [[ -n "${REVISION}" ]]; then
  kubectl -n "${NAMESPACE}" rollout undo "deployment/${DEPLOYMENT}" --to-revision="${REVISION}"
else
  kubectl -n "${NAMESPACE}" rollout undo "deployment/${DEPLOYMENT}"
fi
kubectl -n "${NAMESPACE}" rollout status "deployment/${DEPLOYMENT}" --timeout=300s

