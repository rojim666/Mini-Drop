#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

API_PORT="${MINIDROP_API_PORT:-8080}"
WEB_PORT="${MINIDROP_WEB_PORT:-80}"
MINIO_PORT="${MINIDROP_MINIO_PORT:-9000}"
OUTPUT="${MINIDROP_DEMO_EVIDENCE_OUTPUT:-artifacts/demo-evidence.md}"
INCLUDE_REAL_PREFLIGHT="${MINIDROP_INCLUDE_REAL_PREFLIGHT:-0}"
REAL_COLLECTORS="${MINIDROP_REAL_COLLECTORS:-perf,ebpf-syscall,py-spy}"

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
    --output)
      OUTPUT="$2"
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

args=("$ROOT/scripts/demo/write_demo_evidence.py" --output "$OUTPUT")
if [[ "$INCLUDE_REAL_PREFLIGHT" == "1" ]]; then
  args+=(--include-real-preflight --real-collectors "$REAL_COLLECTORS")
fi

python3 "${args[@]}"
