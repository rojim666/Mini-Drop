#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUTPUT="${MINIDROP_RECORDING_CHECKLIST_OUTPUT:-artifacts/recording-checklist.md}"
API_PORT="${MINIDROP_API_PORT:-8080}"
WEB_PORT="${MINIDROP_WEB_PORT:-4173}"
MINIO_PORT="${MINIDROP_MINIO_PORT:-9000}"
MINIO_CONSOLE_PORT="${MINIDROP_MINIO_CONSOLE_PORT:-9001}"
EVIDENCE_PATH="${MINIDROP_DEMO_EVIDENCE_OUTPUT:-artifacts/demo-evidence.md}"

python3 "$ROOT/scripts/demo/write_recording_checklist.py" \
  --output "$OUTPUT" \
  --api-port "$API_PORT" \
  --web-port "$WEB_PORT" \
  --minio-port "$MINIO_PORT" \
  --minio-console-port "$MINIO_CONSOLE_PORT" \
  --evidence-path "$EVIDENCE_PATH"
