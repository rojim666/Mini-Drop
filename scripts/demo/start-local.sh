#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LOG_DIR="$ROOT/tmp/local-demo"
mkdir -p "$LOG_DIR"

API_ADDR="${API_ADDR:-127.0.0.1:8080}"
WEB_ADDR="${WEB_ADDR:-127.0.0.1}"
WEB_PORT="${WEB_PORT:-5173}"
COLLECTOR_TYPE="${COLLECTOR_TYPE:-mock-perf}"
GO_VERSION="${GO_VERSION:-1.25.5}"
TOOLCHAIN_DIR="$ROOT/tmp/toolchains"
WEB_APP_DIR="$ROOT/apps/web"
WEB_RUNTIME_DIR="$ROOT/tmp/web-runtime"

require_command() {
  local name="$1"
  local hint="$2"
  if ! command -v "$name" >/dev/null 2>&1; then
    echo "Missing required command: $name" >&2
    echo "$hint" >&2
    exit 1
  fi
}

addr_host() {
  local addr="$1"
  echo "${addr%:*}"
}

addr_port() {
  local addr="$1"
  echo "${addr##*:}"
}

port_is_open() {
  local host="$1"
  local port="$2"
  python3 - "$host" "$port" <<'PY'
import socket
import sys

host, port = sys.argv[1], int(sys.argv[2])
with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
    sock.settimeout(0.3)
    raise SystemExit(0 if sock.connect_ex((host, port)) == 0 else 1)
PY
}

require_port_free() {
  local name="$1"
  local host="$2"
  local port="$3"
  if port_is_open "$host" "$port"; then
    echo "$name port is already in use at $host:$port." >&2
    echo "Run bash ./scripts/demo/stop-local.sh first, or choose another port with API_ADDR/WEB_PORT." >&2
    exit 1
  fi
}

detect_go_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *)
      echo "Unsupported WSL architecture for Mini-Drop: $(uname -m)" >&2
      exit 1
      ;;
  esac
}

ensure_linux_go() {
  local go_os=""
  if command -v go >/dev/null 2>&1; then
    go_os="$(go env GOOS 2>/dev/null || true)"
  fi

  if [[ "$go_os" == "linux" ]]; then
    export GOROOT="$(go env GOROOT)"
    export PATH="$GOROOT/bin:$PATH"
    return 0
  fi

  local go_arch
  go_arch="$(detect_go_arch)"
  local go_root="$TOOLCHAIN_DIR/go-${GO_VERSION}-linux-${go_arch}"
  local go_tarball="$TOOLCHAIN_DIR/go${GO_VERSION}.linux-${go_arch}.tar.gz"
  local go_bin="$go_root/go/bin/go"
  if [[ ! -x "$go_bin" ]]; then
    echo "Bootstrapping Go ${GO_VERSION} for Linux under $go_root" >&2
    mkdir -p "$TOOLCHAIN_DIR"
    require_command tar "Install tar inside WSL/Linux."
    require_command curl "Install curl inside WSL/Linux."
    curl -fsSL -o "$go_tarball" "https://go.dev/dl/go${GO_VERSION}.linux-${go_arch}.tar.gz"
    rm -rf "$go_root"
    mkdir -p "$go_root"
    tar -C "$go_root" -xzf "$go_tarball"
  fi

  export GOROOT="$go_root/go"
  export PATH="$GOROOT/bin:$PATH"
  local boot_go_os
  boot_go_os="$(go env GOOS 2>/dev/null || true)"
  if [[ "$boot_go_os" != "linux" ]]; then
    echo "Unable to activate a native Linux Go toolchain (GOOS=$boot_go_os)." >&2
    exit 1
  fi
}

ensure_web_deps() {
  mkdir -p "$WEB_RUNTIME_DIR"
  cp "$WEB_APP_DIR/package.json" "$WEB_RUNTIME_DIR/package.json"
  cp "$WEB_APP_DIR/package-lock.json" "$WEB_RUNTIME_DIR/package-lock.json"
  cp "$WEB_APP_DIR/index.html" "$WEB_RUNTIME_DIR/index.html"
  cp "$WEB_APP_DIR/vite.config.ts" "$WEB_RUNTIME_DIR/vite.config.ts"
  cp "$WEB_APP_DIR/tsconfig.json" "$WEB_RUNTIME_DIR/tsconfig.json"
  cp "$WEB_APP_DIR/tsconfig.app.json" "$WEB_RUNTIME_DIR/tsconfig.app.json"
  cp "$WEB_APP_DIR/tsconfig.node.json" "$WEB_RUNTIME_DIR/tsconfig.node.json"
  rm -rf "$WEB_RUNTIME_DIR/src"
  cp -R "$WEB_APP_DIR/src" "$WEB_RUNTIME_DIR/src"

  local lock_hash=""
  local installed_hash=""
  lock_hash="$(sha256sum "$WEB_APP_DIR/package-lock.json" | awk '{print $1}')"
  installed_hash="$(cat "$WEB_RUNTIME_DIR/.package-lock.sha256" 2>/dev/null || true)"

  if [[ "$lock_hash" == "$installed_hash" \
    && -x "$WEB_RUNTIME_DIR/node_modules/.bin/vite" \
    && -d "$WEB_RUNTIME_DIR/node_modules/rolldown" ]]; then
    return 0
  fi

  echo "Installing web dependencies with npm ci ..." >&2
  rm -rf "$WEB_RUNTIME_DIR/node_modules"
  npm --prefix "$WEB_RUNTIME_DIR" ci --cache "$ROOT/tmp/npm-cache" --prefer-offline
  printf '%s\n' "$lock_hash" >"$WEB_RUNTIME_DIR/.package-lock.sha256"
}

require_command python3 "Install Python 3, for example: sudo apt-get install python3"
require_command npm "Install Node.js/npm inside WSL/Linux."
require_command curl "Install curl, for example: sudo apt-get install curl"
require_port_free "API" "$(addr_host "$API_ADDR")" "$(addr_port "$API_ADDR")"
require_port_free "Web" "$WEB_ADDR" "$WEB_PORT"
ensure_linux_go
ensure_web_deps
if [[ "$COLLECTOR_TYPE" == "perf" ]]; then
  require_command perf "Install perf, for example: sudo apt-get install linux-tools-common linux-tools-generic"
  python3 "$ROOT/scripts/demo/check_perf_env.py"
elif [[ "$COLLECTOR_TYPE" == "ebpf-syscall" ]]; then
  require_command bpftrace "Install bpftrace, for example: sudo apt-get install bpftrace"
  python3 "$ROOT/scripts/demo/check_ebpf_env.py"
elif [[ "$COLLECTOR_TYPE" == "py-spy" ]]; then
  require_command py-spy "Install py-spy, for example: python3 -m pip install py-spy"
  python3 "$ROOT/scripts/demo/check_pyspy_env.py"
fi

start_demo_process() {
  local name="$1"
  shift
  local log_file="$LOG_DIR/${name}.log"
  local cmd=("$@")
  local command_string=""

  printf -v command_string '%q ' "${cmd[@]}"

  setsid bash -lc "cd $(printf '%q' "$ROOT") && exec ${command_string% }" >"$log_file" 2>&1 &

  local pid=$!
  echo "$pid" >"$LOG_DIR/${name}.pid"
  echo "$name started, pid=$pid, log=$log_file"
}

wait_for_target_pid() {
  local pid_file="$LOG_DIR/target.pid"
  local log_file="$LOG_DIR/target.log"
  local deadline=$((SECONDS + 20))
  while (( SECONDS < deadline )); do
    if [[ -f "$log_file" ]]; then
      local target_pid
      target_pid="$(grep -Eo 'target_pid=[0-9]+' "$log_file" | tail -n 1 | cut -d= -f2 || true)"
      if [[ -n "$target_pid" ]]; then
        echo "$target_pid" >"$pid_file"
        return 0
      fi
    fi
    sleep 1
  done

  echo "Target process did not report a PID; check $log_file" >&2
  return 1
}

wait_api_ready() {
  local deadline=$((SECONDS + 30))
  while (( SECONDS < deadline )); do
    if curl -fsS "http://${API_ADDR}/healthz" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "API server did not become ready at http://${API_ADDR}/healthz" >&2
  return 1
}

wait_agent_ready() {
  local agent_id="${MINIDROP_AGENT_ID:-agt_local}"
  local deadline=$((SECONDS + 30))
  while (( SECONDS < deadline )); do
    if python3 - "$API_ADDR" "$agent_id" <<'PY' >/dev/null 2>&1
import json
import sys
import urllib.request

api_addr, agent_id = sys.argv[1], sys.argv[2]
with urllib.request.urlopen(f"http://{api_addr}/api/v1/agents", timeout=2) as response:
    agents = json.loads(response.read().decode("utf-8"))["agents"]
for agent in agents:
    if agent["id"] == agent_id and agent["status"] == "ONLINE":
        raise SystemExit(0)
raise SystemExit(1)
PY
    then
      return 0
    fi
    sleep 1
  done
  echo "Agent did not become ONLINE within 30s; check $LOG_DIR/agent.log" >&2
  return 1
}

if [[ "$COLLECTOR_TYPE" == "perf" ]]; then
  start_demo_process target python3 scripts/demo/perf_workload.py
elif [[ "$COLLECTOR_TYPE" == "ebpf-syscall" ]]; then
  start_demo_process target bash scripts/demo/ebpf_workload.sh
else
  start_demo_process target python3 scripts/demo/mock_target.py
fi
wait_for_target_pid

if [[ "$COLLECTOR_TYPE" == "perf" ]]; then
  python3 "$ROOT/scripts/demo/check_perf_env.py" --pid "$(cat "$LOG_DIR/target.pid")"
elif [[ "$COLLECTOR_TYPE" == "ebpf-syscall" ]]; then
  python3 "$ROOT/scripts/demo/check_ebpf_env.py" --pid "$(cat "$LOG_DIR/target.pid")"
fi

start_demo_process api env \
  MINIDROP_API_ADDR="$API_ADDR" \
  MINIDROP_ALLOWED_ORIGIN="http://${WEB_ADDR}:${WEB_PORT},http://localhost:${WEB_PORT},http://127.0.0.1:${WEB_PORT}" \
  go run ./apps/api-server

wait_api_ready

start_demo_process agent env \
  MINIDROP_API_BASE_URL="http://${API_ADDR}" \
  MINIDROP_PYTHON_BIN="${MINIDROP_PYTHON_BIN:-python3}" \
  MINIDROP_ANALYZER_SCRIPT="$ROOT/apps/analyzer/main.py" \
  MINIDROP_ARTIFACT_DIR="$ROOT/artifacts" \
  MINIDROP_AGENT_ID="${MINIDROP_AGENT_ID:-agt_local}" \
  MINIDROP_AGENT_HOSTNAME="${MINIDROP_AGENT_HOSTNAME:-linux-agent}" \
  MINIDROP_AGENT_IP="${MINIDROP_AGENT_IP:-127.0.0.1}" \
  MINIDROP_AGENT_VERSION="${MINIDROP_AGENT_VERSION:-0.1.0}" \
  go run ./apps/agent

wait_agent_ready

start_demo_process web env \
  VITE_API_BASE_URL="http://${API_ADDR}" \
  npm --prefix "$WEB_RUNTIME_DIR" run dev -- --host "${WEB_ADDR}" --port "${WEB_PORT}"

echo
echo "Mini-Drop local demo is starting."
echo "Web UI: http://localhost:${WEB_PORT}"
echo "API:    http://${API_ADDR}/healthz"
echo "Logs:   $LOG_DIR"
echo
echo "Use this target PID in the Web form:"
cat "$LOG_DIR/target.pid"
echo
echo "Smoke helper:"
echo "MINIDROP_API_BASE_URL=http://${API_ADDR} python3 scripts/demo/smoke_e2e.py $(cat "$LOG_DIR/target.pid") agt_local ${COLLECTOR_TYPE}"
if [[ "$COLLECTOR_TYPE" != "mock-perf" ]]; then
  echo "bash ./scripts/demo/smoke_real_collector.sh ${COLLECTOR_TYPE}"
fi
echo "If you want real perf, start with: COLLECTOR_TYPE=perf bash ./scripts/demo/start-local.sh"
echo "If you want eBPF syscall tracing, start with: COLLECTOR_TYPE=ebpf-syscall bash ./scripts/demo/start-local.sh"
echo "If you want Python user-space stacks, start with: COLLECTOR_TYPE=py-spy bash ./scripts/demo/start-local.sh"
if [[ "$COLLECTOR_TYPE" == "perf" ]]; then
  echo
  echo "If perf is blocked, run: sudo sysctl kernel.perf_event_paranoid=1"
elif [[ "$COLLECTOR_TYPE" == "ebpf-syscall" ]]; then
  echo
  echo "If bpftrace is blocked, check tracefs/debugfs and run the Agent with tracing privileges."
elif [[ "$COLLECTOR_TYPE" == "py-spy" ]]; then
  echo
  echo "If py-spy is blocked on Linux, check ptrace permissions or kernel.yama.ptrace_scope."
fi
