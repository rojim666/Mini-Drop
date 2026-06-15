#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

API_PORT="${MINIDROP_API_PORT:-8080}"
WEB_PORT="${MINIDROP_WEB_PORT:-4173}"
MINIO_PORT="${MINIDROP_MINIO_PORT:-9000}"
SEED_TASKS="${SEED_TASKS:-0}"
SEED_TASK_COUNT="${MINIDROP_ACCEPTANCE_SEED_TASKS:-2}"
TARGET_PID="${MINIDROP_TARGET_PID:-1}"
TARGET_AGENT_ID="${MINIDROP_TARGET_AGENT_ID:-agt_compose}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --seed-tasks)
      SEED_TASKS=1
      shift
      ;;
    --seed-task-count)
      SEED_TASK_COUNT="$2"
      shift 2
      ;;
    --pid)
      TARGET_PID="$2"
      shift 2
      ;;
    --agent-id)
      TARGET_AGENT_ID="$2"
      shift 2
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

export MINIDROP_API_PORT="$API_PORT"
export MINIDROP_WEB_PORT="$WEB_PORT"
export MINIDROP_MINIO_PORT="$MINIO_PORT"
export MINIDROP_API_BASE_URL="${MINIDROP_API_BASE_URL:-http://127.0.0.1:${API_PORT}}"

if [[ "$SEED_TASKS" == "1" ]]; then
  if [[ "$SEED_TASK_COUNT" -lt 1 ]]; then
    echo "MINIDROP_ACCEPTANCE_SEED_TASKS must be at least 1 when --seed-tasks is set." >&2
    exit 1
  fi

  for index in $(seq 1 "$SEED_TASK_COUNT"); do
    echo "Seeding acceptance task ${index}/${SEED_TASK_COUNT}..."
    MINIDROP_EXPECT_MINIO_URL=1 python3 "$ROOT/scripts/demo/smoke_compose.py" \
      --pid "$TARGET_PID" \
      --agent-id "$TARGET_AGENT_ID" \
      --expect-minio-url
  done
fi

python3 "$ROOT/scripts/demo/acceptance_snapshot.py"
