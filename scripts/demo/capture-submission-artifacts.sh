#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WEB_PORT="${MINIDROP_WEB_PORT:-4173}"
MINIO_CONSOLE_PORT="${MINIDROP_MINIO_CONSOLE_PORT:-9001}"
OUTPUT_DIR="${MINIDROP_SUBMISSION_SCREENSHOT_DIR:-artifacts/submission-screenshots}"
EVIDENCE_PATH="${MINIDROP_DEMO_EVIDENCE_OUTPUT:-artifacts/demo-evidence.md}"
COVERAGE_PATH="${MINIDROP_COVERAGE_OUTPUT:-artifacts/coverage-report.md}"
BROWSER_CHANNEL="${MINIDROP_SCREENSHOT_BROWSER_CHANNEL:-auto}"

python3 "$ROOT/scripts/demo/capture_submission_artifacts.py" \
  --web-base "http://localhost:${WEB_PORT}" \
  --minio-console-base "http://localhost:${MINIO_CONSOLE_PORT}" \
  --output-dir "$OUTPUT_DIR" \
  --evidence-path "$EVIDENCE_PATH" \
  --coverage-path "$COVERAGE_PATH" \
  --browser-channel "$BROWSER_CHANNEL"
