#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

require_command() {
  local name="$1"
  if ! command -v "${name}" >/dev/null 2>&1; then
    echo "missing required command: ${name}" >&2
    exit 1
  fi
}

render_kustomize() {
  local path="$1"
  echo "[validate] kubectl kustomize ${path}"
  kubectl kustomize "${ROOT_DIR}/${path}" >/dev/null
}

require_command python
require_command go
require_command kubectl

cd "${ROOT_DIR}"

echo "[validate] python -m compileall"
python -m compileall -q common services

echo "[validate] go test ./..."
(
  cd "${ROOT_DIR}/operator"
  export GOCACHE="${GOCACHE:-${ROOT_DIR}/.go-cache}"
  export GOMODCACHE="${GOMODCACHE:-${ROOT_DIR}/.gomodcache}"
  if [[ -n "${APPDATA:-}" ]]; then
    export APPDATA="${GO_APPDATA:-${ROOT_DIR}/.go-appdata}"
  fi
  go test ./...
)

render_kustomize "k8s"
render_kustomize "operator/config/default"
render_kustomize "k8s-v2"

echo "[validate] all checks passed"