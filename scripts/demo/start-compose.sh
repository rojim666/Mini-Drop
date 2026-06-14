#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE_FILE="$ROOT/deploy/docker-compose.yml"

API_PORT="${MINIDROP_API_PORT:-8080}"
WEB_PORT="${MINIDROP_WEB_PORT:-4173}"
MINIO_PORT="${MINIDROP_MINIO_PORT:-9000}"
MINIO_CONSOLE_PORT="${MINIDROP_MINIO_CONSOLE_PORT:-9001}"
SMOKE_TASK_COUNT="${MINIDROP_COMPOSE_SMOKE_TASKS:-2}"
SKIP_SMOKE="${SKIP_SMOKE:-0}"

require_command() {
  local name="$1"
  local hint="$2"
  if ! command -v "$name" >/dev/null 2>&1; then
    echo "Missing required command: $name" >&2
    echo "$hint" >&2
    exit 1
  fi
}

require_command docker "Install Docker and Docker Compose."
require_command python3 "Install Python 3."
docker compose version >/dev/null

export MINIDROP_API_PORT="$API_PORT"
export MINIDROP_WEB_PORT="$WEB_PORT"
export MINIDROP_MINIO_PORT="$MINIO_PORT"
export MINIDROP_MINIO_CONSOLE_PORT="$MINIO_CONSOLE_PORT"
export MINIDROP_API_BASE_URL="http://127.0.0.1:${API_PORT}"

docker compose -f "$COMPOSE_FILE" up --build --pull never -d

if [[ "$SKIP_SMOKE" != "1" ]]; then
  if [[ "$SMOKE_TASK_COUNT" -lt 1 ]]; then
    echo "MINIDROP_COMPOSE_SMOKE_TASKS must be at least 1 when smoke testing is enabled." >&2
    exit 1
  fi

  for index in $(seq 1 "$SMOKE_TASK_COUNT"); do
    echo "Running compose smoke task ${index}/${SMOKE_TASK_COUNT}..."
    MINIDROP_EXPECT_MINIO_URL=1 python3 "$ROOT/scripts/demo/smoke_compose.py" \
      --pid 1 \
      --agent-id agt_compose \
      --expect-minio-url
  done
fi

echo
echo "Mini-Drop compose demo is ready."
echo "Web UI:        http://localhost:${WEB_PORT}"
echo "API health:    http://localhost:${API_PORT}/healthz"
echo "MinIO console: http://localhost:${MINIO_CONSOLE_PORT}"
echo "MinIO login:   minidrop / minidrop123"
echo
echo "Use PID 1 in the Web task form for the bundled compose target."
echo "Snapshot:      MINIDROP_API_PORT=${API_PORT} MINIDROP_WEB_PORT=${WEB_PORT} MINIDROP_MINIO_PORT=${MINIO_PORT} bash ./scripts/demo/acceptance-snapshot.sh"
echo "Evidence:      MINIDROP_API_PORT=${API_PORT} MINIDROP_WEB_PORT=${WEB_PORT} MINIDROP_MINIO_PORT=${MINIO_PORT} bash ./scripts/demo/write-demo-evidence.sh --include-real-preflight"
echo "Checklist:     MINIDROP_API_PORT=${API_PORT} MINIDROP_WEB_PORT=${WEB_PORT} MINIDROP_MINIO_PORT=${MINIO_PORT} MINIDROP_MINIO_CONSOLE_PORT=${MINIO_CONSOLE_PORT} bash ./scripts/demo/write-recording-checklist.sh"
echo "Submission:    MINIDROP_API_PORT=${API_PORT} MINIDROP_WEB_PORT=${WEB_PORT} MINIDROP_MINIO_CONSOLE_PORT=${MINIO_CONSOLE_PORT} bash ./scripts/demo/write-submission-notes.sh"
echo "Final gate:    bash ./scripts/demo/final-preflight.sh --api-port ${API_PORT} --web-port ${WEB_PORT} --minio-port ${MINIO_PORT} --minio-console-port ${MINIO_CONSOLE_PORT} --include-real-preflight"
echo "Stop command:  bash ./scripts/demo/stop-compose.sh"
