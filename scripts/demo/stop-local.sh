#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LOG_DIR="$ROOT/tmp/local-demo"
NAMES=(web agent api target)

for name in "${NAMES[@]}"; do
  pid_file="$LOG_DIR/${name}.pid"
  if [[ -f "$pid_file" ]]; then
    pid="$(cat "$pid_file")"
    if [[ -n "$pid" ]]; then
      kill -- "-$pid" >/dev/null 2>&1 || kill "$pid" >/dev/null 2>&1 || true
      sleep 1
      kill -9 -- "-$pid" >/dev/null 2>&1 || kill -9 "$pid" >/dev/null 2>&1 || true
    fi
    rm -f "$pid_file"
    echo "$name stopped"
  fi
done
