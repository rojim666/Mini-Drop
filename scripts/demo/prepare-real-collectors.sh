#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUTPUT="${MINIDROP_REAL_COLLECTOR_PREFLIGHT_OUTPUT:-artifacts/real-collector-preflight.md}"
COLLECTORS="${MINIDROP_REAL_COLLECTORS:-perf,ebpf-syscall,py-spy}"
PID="${MINIDROP_TARGET_PID:-0}"
INSTALL=0
STRICT=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --collectors)
      COLLECTORS="$2"
      shift 2
      ;;
    --pid)
      PID="$2"
      shift 2
      ;;
    --output)
      OUTPUT="$2"
      shift 2
      ;;
    --install)
      INSTALL=1
      shift
      ;;
    --strict)
      STRICT=1
      shift
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

args=(
  "$ROOT/scripts/demo/prepare_real_collectors.py"
  --collectors "$COLLECTORS"
  --output "$OUTPUT"
)

if [[ "$PID" -gt 0 ]]; then
  args+=(--pid "$PID")
fi

if [[ "$INSTALL" == "1" ]]; then
  args+=(--install)
fi

set +e
python3 "${args[@]}"
status=$?
set -e

if [[ "$status" -ne 0 && "$STRICT" == "1" ]]; then
  exit "$status"
fi

if [[ "$status" -ne 0 ]]; then
  echo "Real collector prerequisites are blocked; see ${OUTPUT} for the generated report." >&2
fi
