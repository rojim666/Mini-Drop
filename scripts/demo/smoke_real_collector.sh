#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LOG_DIR="${LOG_DIR:-$ROOT/tmp/local-demo}"
API_ADDR="${API_ADDR:-127.0.0.1:8080}"
API_BASE="${MINIDROP_API_BASE_URL:-http://${API_ADDR}}"
AGENT_ID="${MINIDROP_AGENT_ID:-agt_local}"
COLLECTOR_TYPE="${1:-${COLLECTOR_TYPE:-perf}}"
TARGET_PID="${MINIDROP_TARGET_PID:-}"

if [[ -z "$TARGET_PID" && -f "$LOG_DIR/target.pid" ]]; then
  TARGET_PID="$(cat "$LOG_DIR/target.pid")"
fi

if [[ -z "$TARGET_PID" ]]; then
  echo "Missing target PID." >&2
  echo "Start bash ./scripts/demo/start-local.sh first or set MINIDROP_TARGET_PID=<pid>." >&2
  exit 1
fi

case "$COLLECTOR_TYPE" in
  perf)
    python3 "$ROOT/scripts/demo/check_perf_env.py" --pid "$TARGET_PID"
    ;;
  ebpf-syscall)
    python3 "$ROOT/scripts/demo/check_ebpf_env.py" --pid "$TARGET_PID"
    ;;
  py-spy)
    python3 "$ROOT/scripts/demo/check_pyspy_env.py" --pid "$TARGET_PID"
    ;;
  *)
    echo "Unsupported real collector: $COLLECTOR_TYPE" >&2
    echo "Supported values: perf, ebpf-syscall, py-spy" >&2
    exit 2
    ;;
esac

MINIDROP_API_BASE_URL="$API_BASE" \
  python3 "$ROOT/scripts/demo/smoke_e2e.py" "$TARGET_PID" "$AGENT_ID" "$COLLECTOR_TYPE"
