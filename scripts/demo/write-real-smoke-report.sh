#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COLLECTORS="${MINIDROP_REAL_COLLECTORS:-${COLLECTOR_TYPE:-perf,ebpf-syscall,py-spy}}"
OUTPUT="${MINIDROP_REAL_SMOKE_OUTPUT:-artifacts/real-smoke-report.md}"
API_BASE="${MINIDROP_API_BASE_URL:-http://127.0.0.1:8080}"
AGENT_ID="${MINIDROP_AGENT_ID:-agt_local}"
ALLOW_BLOCKED=0
SKIP_SMOKE=0
PID="${MINIDROP_TARGET_PID:-0}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --collectors)
      COLLECTORS="$2"
      shift 2
      ;;
    --output)
      OUTPUT="$2"
      shift 2
      ;;
    --api-base)
      API_BASE="$2"
      shift 2
      ;;
    --agent-id)
      AGENT_ID="$2"
      shift 2
      ;;
    --pid)
      PID="$2"
      shift 2
      ;;
    --skip-smoke)
      SKIP_SMOKE=1
      shift
      ;;
    --allow-blocked)
      ALLOW_BLOCKED=1
      shift
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

args=(
  "$ROOT/scripts/demo/write_real_smoke_report.py"
  --collectors "$COLLECTORS"
  --output "$OUTPUT"
  --api-base "$API_BASE"
  --agent-id "$AGENT_ID"
)

if [[ "$PID" != "0" && -n "$PID" ]]; then
  args+=(--pid "$PID")
fi

if [[ "$SKIP_SMOKE" == "1" ]]; then
  args+=(--skip-smoke)
fi

if [[ "$ALLOW_BLOCKED" == "1" ]]; then
  args+=(--allow-blocked)
fi

python3 "${args[@]}"
