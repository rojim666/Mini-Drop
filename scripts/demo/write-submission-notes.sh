#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUTPUT="${MINIDROP_SUBMISSION_NOTES_OUTPUT:-artifacts/submission-notes.md}"
API_PORT="${MINIDROP_API_PORT:-8080}"
WEB_PORT="${MINIDROP_WEB_PORT:-80}"
MINIO_CONSOLE_PORT="${MINIDROP_MINIO_CONSOLE_PORT:-9001}"
EVIDENCE_PATH="${MINIDROP_DEMO_EVIDENCE_OUTPUT:-artifacts/demo-evidence.md}"
CHECKLIST_PATH="${MINIDROP_RECORDING_CHECKLIST_OUTPUT:-artifacts/recording-checklist.md}"
ATTRIBUTION_EVALUATION_PATH="${MINIDROP_ATTRIBUTION_EVALUATION_OUTPUT:-artifacts/attribution-evaluation-report.md}"

python3 "$ROOT/scripts/demo/write_submission_notes.py" \
  --output "$OUTPUT" \
  --api-port "$API_PORT" \
  --web-port "$WEB_PORT" \
  --minio-console-port "$MINIO_CONSOLE_PORT" \
  --evidence-path "$EVIDENCE_PATH" \
  --checklist-path "$CHECKLIST_PATH" \
  --attribution-evaluation-path "$ATTRIBUTION_EVALUATION_PATH"
