#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

API_PORT="${MINIDROP_API_PORT:-8080}"
WEB_PORT="${MINIDROP_WEB_PORT:-80}"
MINIO_PORT="${MINIDROP_MINIO_PORT:-9000}"
MINIO_CONSOLE_PORT="${MINIDROP_MINIO_CONSOLE_PORT:-9001}"

python3 "$ROOT/scripts/demo/check_compose_stack.py" \
  --api-port "$API_PORT" \
  --web-port "$WEB_PORT" \
  --minio-port "$MINIO_PORT" \
  --minio-console-port "$MINIO_CONSOLE_PORT"
