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

python3 - "$API_BASE" "$AGENT_ID" "$TARGET_PID" "$ROOT" <<'PY'
import json
import sys
import urllib.error
import urllib.request
from pathlib import Path

api_base, agent_id, target_pid, root = sys.argv[1:5]
sys.path.insert(0, str(Path(root) / "scripts" / "demo"))
from api_auth import auth_headers  # noqa: E402


def fail(message: str, hint: str, code: int) -> None:
    print(f"FAIL: {message}", file=sys.stderr)
    print(f"hint: {hint}", file=sys.stderr)
    raise SystemExit(code)


def request_json(path: str, auth: bool = True) -> dict:
    headers = auth_headers(api_base) if auth else {}
    req = urllib.request.Request(f"{api_base.rstrip('/')}{path}", method="GET", headers=headers)
    with urllib.request.urlopen(req, timeout=10) as response:
        return json.loads(response.read().decode("utf-8"))


try:
    health = request_json("/healthz", auth=False)
except (RuntimeError, urllib.error.URLError, urllib.error.HTTPError, TimeoutError) as exc:
    fail(
        f"API is not reachable at {api_base}: {exc}",
        "Start the local stack first: COLLECTOR_TYPE=perf bash ./scripts/demo/start-local.sh",
        10,
    )

if health.get("status") != "ok":
    fail(
        f"API health returned unexpected payload: {health}",
        "Check API logs under tmp/local-demo/api.log.",
        11,
    )
print(f"OK: API health is ok at {api_base}")

try:
    agents = request_json("/api/v1/agents").get("agents", [])
except (RuntimeError, urllib.error.URLError, urllib.error.HTTPError, TimeoutError) as exc:
    fail(
        f"Cannot list agents from {api_base}: {exc}",
        "Verify login credentials or set MINIDROP_AUTH_TOKEN.",
        12,
    )

observed = [f"{item.get('id', '<unknown>')}:{item.get('status', '<unknown>')}" for item in agents]
if not any(item.get("id") == agent_id and item.get("status") == "ONLINE" for item in agents):
    fail(
        f"Agent {agent_id} is not ONLINE; observed agents: {', '.join(observed) if observed else 'none'}",
        "Wait for heartbeat or set MINIDROP_AGENT_ID to the running Agent ID.",
        13,
    )
print(f"OK: Agent {agent_id} is ONLINE")

try:
    pid = int(target_pid)
except ValueError:
    fail(f"Target PID {target_pid!r} is not an integer", "Set MINIDROP_TARGET_PID=<pid>.", 14)

if pid <= 0:
    fail("Target PID is missing", "Start scripts/demo/start-local.sh or set MINIDROP_TARGET_PID=<pid>.", 15)
print(f"OK: Target PID {pid} is selected for smoke")
PY

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
