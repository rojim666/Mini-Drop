#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

API_PORT="${MINIDROP_API_PORT:-8080}"
WEB_PORT="${MINIDROP_WEB_PORT:-4173}"
MINIO_PORT="${MINIDROP_MINIO_PORT:-9000}"
MINIO_CONSOLE_PORT="${MINIDROP_MINIO_CONSOLE_PORT:-9001}"
OUTPUT="${MINIDROP_FINAL_PREFLIGHT_OUTPUT:-artifacts/final-preflight.md}"
SEED_TASKS=0
SEED_TASK_COUNT="${MINIDROP_ACCEPTANCE_SEED_TASKS:-2}"
TARGET_PID="${MINIDROP_TARGET_PID:-1}"
TARGET_AGENT_ID="${MINIDROP_TARGET_AGENT_ID:-agt_compose}"
INCLUDE_REAL_PREFLIGHT=0
REAL_COLLECTORS="${MINIDROP_REAL_COLLECTORS:-perf,ebpf-syscall,py-spy}"
SKIP_LIVE=0
SKIP_TESTS=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --api-port)
      API_PORT="$2"
      shift 2
      ;;
    --web-port)
      WEB_PORT="$2"
      shift 2
      ;;
    --minio-port)
      MINIO_PORT="$2"
      shift 2
      ;;
    --minio-console-port)
      MINIO_CONSOLE_PORT="$2"
      shift 2
      ;;
    --output)
      OUTPUT="$2"
      shift 2
      ;;
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
    --include-real-preflight)
      INCLUDE_REAL_PREFLIGHT=1
      shift
      ;;
    --real-collectors)
      REAL_COLLECTORS="$2"
      shift 2
      ;;
    --skip-live)
      SKIP_LIVE=1
      shift
      ;;
    --skip-tests)
      SKIP_TESTS=1
      shift
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

args=(
  "$ROOT/scripts/demo/run_final_preflight.py"
  --api-port "$API_PORT"
  --web-port "$WEB_PORT"
  --minio-port "$MINIO_PORT"
  --minio-console-port "$MINIO_CONSOLE_PORT"
  --output "$OUTPUT"
  --seed-task-count "$SEED_TASK_COUNT"
  --target-pid "$TARGET_PID"
  --agent-id "$TARGET_AGENT_ID"
  --real-collectors "$REAL_COLLECTORS"
)

if [[ "$SEED_TASKS" == "1" ]]; then
  args+=(--seed-tasks)
fi

if [[ "$INCLUDE_REAL_PREFLIGHT" == "1" ]]; then
  args+=(--include-real-preflight)
fi

if [[ "$SKIP_LIVE" == "1" ]]; then
  args+=(--skip-live)
fi

if [[ "$SKIP_TESTS" == "1" ]]; then
  args+=(--skip-tests)
fi

python3 "${args[@]}"
