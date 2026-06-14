#!/usr/bin/env bash
set -euo pipefail

MODE="${MINIDROP_EBPF_WORKLOAD:-dd}"
DD_BLOCK_SIZE="${MINIDROP_EBPF_DD_BS:-4K}"

echo "target_pid=$$"
echo "workload=${MODE}"

case "$MODE" in
  dd)
    if ! command -v dd >/dev/null 2>&1; then
      echo "Missing required command: dd" >&2
      exit 1
    fi
    echo "command=dd if=/dev/zero of=/dev/null bs=${DD_BLOCK_SIZE} status=none"
    exec dd if=/dev/zero of=/dev/null bs="$DD_BLOCK_SIZE" status=none
    ;;
  *)
    echo "Unsupported MINIDROP_EBPF_WORKLOAD=${MODE}. Supported values: dd" >&2
    exit 2
    ;;
esac
