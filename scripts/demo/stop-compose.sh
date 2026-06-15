#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE_FILE="$ROOT/deploy/docker-compose.yml"

API_PORT="${MINIDROP_API_PORT:-8080}"
WEB_PORT="${MINIDROP_WEB_PORT:-4173}"
MINIO_PORT="${MINIDROP_MINIO_PORT:-9000}"
MINIO_CONSOLE_PORT="${MINIDROP_MINIO_CONSOLE_PORT:-9001}"
VOLUMES="${VOLUMES:-0}"

export MINIDROP_API_PORT="$API_PORT"
export MINIDROP_WEB_PORT="$WEB_PORT"
export MINIDROP_MINIO_PORT="$MINIO_PORT"
export MINIDROP_MINIO_CONSOLE_PORT="$MINIO_CONSOLE_PORT"

args=(compose -f "$COMPOSE_FILE" down --remove-orphans)
if [[ "$VOLUMES" == "1" ]]; then
  args+=(--volumes)
fi

docker "${args[@]}"
