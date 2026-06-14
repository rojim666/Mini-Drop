# Mini-Drop

Mini-Drop is a documentation-first implementation of a small Linux performance
diagnosis platform. The repository now includes a runnable mock vertical slice:

`Web UI -> API Server -> Agent -> mock collector -> Analyzer -> flamegraph -> Web UI`

The mock path keeps the real system contract intact:

1. A user creates a profiling task from the Web UI.
2. The API Server persists the task and every status transition.
3. The Agent heartbeats, claims the task, and writes a mock raw artifact.
4. The Analyzer generates a flamegraph SVG and TopN hotspot JSON.
5. The Web UI shows agent health, task history, audit logs, and analysis output.
6. The plan page can create a continuous profiling window set and reuse the same task/result flow.

## Current Components

- `apps/api-server`: Go + Gin + GORM + SQLite API server
- `apps/agent`: Go mock agent with heartbeat and task loop
- `apps/analyzer`: Python CLI that generates mock flamegraphs and hotspots
- `apps/web`: React + Vite + TypeScript operations dashboard
- `deploy/docker-compose.yml`: one-command PostgreSQL + MinIO demo stack
- `scripts/demo`: smoke test and mock target helpers

## Quick Start

### Docker demo

On Windows PowerShell, the most direct one-command path is:

```powershell
.\scripts\demo\start-compose.ps1
```

The script starts the compose stack and performs a smoke task that verifies
MinIO signed artifact URLs. By default it creates two completed mock tasks so
the task-comparison page is ready for recording immediately.

Make is also supported:

```bash
make demo
```

This compose stack starts PostgreSQL, MinIO, API Server, Agent, Web, and the
bundled mock target. The Agent and API still share a local artifact volume for
simple handoff; when the API returns artifact URLs it uploads the file to MinIO
and returns a temporary signed URL.

Then open:

- Web UI: [http://localhost:4173](http://localhost:4173)
- API health: [http://localhost:8080/healthz](http://localhost:8080/healthz)
- MinIO console: [http://localhost:9001](http://localhost:9001), login `minidrop` / `minidrop123`

If you already have a local API or Agent running on `8080`, start the compose
stack on alternate ports first:

```bash
MINIDROP_API_PORT=18080 MINIDROP_WEB_PORT=14173 MINIDROP_MINIO_PORT=19000 MINIDROP_MINIO_CONSOLE_PORT=19001 make demo
```

The PowerShell compose helper prints matching snapshot and evidence commands
with the same ports. Use those printed commands when the stack is not on the
default `8080` / `4173` / `9000` ports.

For the compose-backed demo, use `PID 1` in the task form. The agent shares the
PID namespace of the bundled `demo-target` container, so PID 1 is a stable mock
workload for the end-to-end flow.

To verify the compose stack from the command line:

```bash
make smoke-demo
make smoke-demo-fail
make smoke-demo-offline
```

To assert that result artifacts are served through MinIO signed URLs:

```bash
make smoke-demo-minio
```

To print a compact acceptance snapshot before recording or presenting the demo:

```bash
make acceptance-snapshot
```

For the recommended 10 to 15 minute recording flow, use the final demo script:

- [Final demo script](docs/demo-script.md)

On Windows PowerShell without `make`, run the helper directly:

```powershell
.\scripts\demo\acceptance-snapshot.ps1
```

For an alternate-port compose stack:

```powershell
.\scripts\demo\acceptance-snapshot.ps1 -ApiPort 18080 -WebPort 14173 -MinioPort 19000
.\scripts\demo\write-demo-evidence.ps1 -ApiPort 18080 -WebPort 14173 -MinioPort 19000
```

On Linux / WSL2 without `make`, use the Bash helper:

```bash
bash ./scripts/demo/acceptance-snapshot.sh
```

If the compose stack is already running but does not yet have two completed
tasks for comparison, seed them and print the snapshot in one command:

```powershell
.\scripts\demo\acceptance-snapshot.ps1 -SeedTasks
```

```bash
bash ./scripts/demo/acceptance-snapshot.sh --seed-tasks
```

The snapshot checks API health, Web reachability, online Agents, completed
tasks, MinIO signed result URLs, and whether the Web task-comparison page has at
least two completed TopN-backed tasks to compare.

If `8080` or `4173` is already used by a local run, start the compose stack on
alternate host ports:

```bash
MINIDROP_API_PORT=18080 MINIDROP_WEB_PORT=14173 MINIDROP_MINIO_PORT=19000 MINIDROP_MINIO_CONSOLE_PORT=19001 make demo
MINIDROP_API_PORT=18080 make smoke-demo
MINIDROP_API_PORT=18080 make smoke-demo-fail
MINIDROP_API_PORT=18080 MINIDROP_WEB_PORT=14173 MINIDROP_MINIO_PORT=19000 make acceptance-snapshot
```

For the agent-offline smoke path, start compose with a shorter offline window:

```bash
MINIDROP_API_PORT=18080 MINIDROP_WEB_PORT=14173 MINIDROP_OFFLINE_AFTER_SEC=6 MINIDROP_OFFLINE_SCAN_SEC=2 make demo
MINIDROP_API_PORT=18080 make smoke-demo-offline
```

To stop the stack:

```bash
.\scripts\demo\stop-compose.ps1
# or
make demo-down
```

### Local development

On Windows PowerShell, the fastest local start path is:

```powershell
.\scripts\demo\start-local.ps1
```

The script starts the API, mock target, Agent, and Web UI. It prints the target
PID to use in the task form.

If the console stays busy, that is normal; the processes are now running in the
background from the same terminal session.

To stop all local demo processes:

```powershell
.\scripts\demo\stop-local.ps1
```

On WSL2 Ubuntu or another Linux host, use the bash version:

```bash
make local
```

If a native Linux Go toolchain is not installed, the script downloads a local
Go toolchain into `tmp/toolchains/`. It also runs `npm ci` for the Web app when
the Linux dependency cache is missing, storing it under `tmp/web-runtime/`
so it does not conflict with Windows `node_modules`.

To stop it:

```bash
make local-down
```

The Linux script also supports the real collector smoke path:

```bash
COLLECTOR_TYPE=perf bash ./scripts/demo/start-local.sh
MINIDROP_API_BASE_URL=http://127.0.0.1:8080 python3 scripts/demo/smoke_e2e.py <printed-pid> agt_local perf
```

If port `8080` or `5173` is already occupied, choose alternate ports:

```bash
API_ADDR=127.0.0.1:18080 WEB_PORT=15173 bash ./scripts/demo/start-local.sh
MINIDROP_API_BASE_URL=http://127.0.0.1:18080 python3 scripts/demo/smoke_e2e.py <printed-pid> agt_local mock-perf
```

Manual startup is also available:

1. Start the API server:

```bash
go run ./apps/api-server
```

2. Start a target process in another terminal:

```bash
python scripts/demo/mock_target.py
```

3. Start the agent:

```bash
go run ./apps/agent
```

4. Start the web app:

```bash
cd apps/web
npm install
npm run dev
```

5. Use the PID of the `mock_target.py` process in the task form.

## Verification

### Tests

```bash
make test
```

Current automated coverage includes:

- task transition validation
- create-task validation
- task lifecycle event persistence
- agent offline / recovery audit log flow
- agent offline active-task failure
- rule-based attribution result from TopN hotspots
- analyzer mock JSON and `perf script` parsing
- compose smoke task helper

### Local smoke helper

After starting the API and Agent locally, run:

```bash
python scripts/demo/smoke_e2e.py <pid> [agent_id]
```

This creates a task and polls until it reaches `DONE` or `FAILED`.
For an expected failure path:

```bash
python scripts/demo/smoke_e2e.py 999999 agt_local mock-perf --expect-status FAILED --expect-reason-contains "target pid not found"
```

### Real `perf` collector on WSL2 / Linux

The default local demo still uses `mock-perf`, which works on Windows. To run
the real collector, start the same services inside WSL2 Ubuntu or another Linux
host, then create a task with `collector_type=perf` in the Web form.

Install `perf` first. On Ubuntu this is usually one of:

```bash
sudo apt-get update
sudo apt-get install linux-tools-common linux-tools-generic
```

If profiling is blocked by kernel permissions, lower the paranoid setting for
the demo session:

```bash
sudo sysctl kernel.perf_event_paranoid=1
```

Before starting the real collector, run the environment check inside WSL2 /
Linux:

```bash
make real-check
python3 scripts/demo/check_perf_env.py
python3 scripts/demo/check_perf_env.py --pid <target-pid>
```

`make real-check` summarizes `perf`, `ebpf-syscall`, and `py-spy` readiness.
The individual checks report Linux/runtime, tool availability, permission
settings such as `perf_event_paranoid`, target PID existence, and a short smoke
command when a PID is provided.

The smoke helper can also request the real collector:

```bash
COLLECTOR_TYPE=perf bash ./scripts/demo/start-local.sh
MINIDROP_API_BASE_URL=http://127.0.0.1:8080 python3 scripts/demo/smoke_e2e.py <pid> [agent_id] perf
```

The real path writes `artifacts/<task_id>/raw/perf.data`, runs
`perf script`, and generates the same `flamegraph.svg` and `topn.json` result
files as the mock path.

When `COLLECTOR_TYPE=perf`, the Linux start script switches to
`scripts/demo/perf_workload.py`, a CPU-burning target that produces a clearer
hotspot stack for the demo.

For a one-command WSL2/Linux smoke after `bash ./scripts/demo/start-local.sh`:

```bash
make smoke-real COLLECTOR_TYPE=perf
make smoke-real COLLECTOR_TYPE=ebpf-syscall
make smoke-real COLLECTOR_TYPE=py-spy
```

The helper reuses `tmp/local-demo/target.pid` by default and runs the matching
environment check before creating the task.

### eBPF syscall collector on WSL2 / Linux

Mini-Drop also has a first P2 collector, `collector_type=ebpf-syscall`. It uses
`bpftrace` tracepoints to count system calls for one target PID and writes the
raw output to `artifacts/<task_id>/raw/ebpf.syscalls.txt`. Analyzer converts the
histogram into the same `topn.json` contract and Web shows an `eBPF 分布` tab.

Install `bpftrace` inside WSL2 Ubuntu or another Linux host:

```bash
sudo apt-get update
sudo apt-get install bpftrace
```

Run the eBPF preflight before creating a task:

```bash
python3 scripts/demo/check_ebpf_env.py
python3 scripts/demo/check_ebpf_env.py --pid <target-pid>
```

For the live demo path, start the local stack with the eBPF collector selected:

```bash
COLLECTOR_TYPE=ebpf-syscall bash ./scripts/demo/start-local.sh
```

In this mode the script starts `scripts/demo/ebpf_workload.sh` instead of the
quiet Python target. The workload runs `dd if=/dev/zero of=/dev/null` in a loop,
prints its PID, and continuously produces `read` / `write` syscalls so the Web
`eBPF 分布` tab shows a visible histogram change. You can smoke-test that path
from the terminal:

```bash
MINIDROP_API_BASE_URL=http://127.0.0.1:8080 python3 scripts/demo/smoke_e2e.py <pid> agt_local ebpf-syscall
```

To tune the demo workload:

```bash
MINIDROP_EBPF_DD_BS=64K COLLECTOR_TYPE=ebpf-syscall bash ./scripts/demo/start-local.sh
```

The collector may require root, mounted tracefs/debugfs, and a kernel with
syscall tracepoints. Windows local development should keep using `mock-perf`;
selecting `ebpf-syscall` outside Linux fails clearly with
`ebpf-syscall collector requires linux`.

### Python user-space collector with py-spy

The first language-level collector is `collector_type=py-spy`. It attaches to a
Python process with `py-spy record --format raw`, writes
`artifacts/<task_id>/raw/pyspy.raw.txt`, and Analyzer converts the Python stack
samples into `flamegraph.svg` plus `topn.json`. The Web detail page adds a
`Python 栈` tab for this collector.

Install `py-spy` in the same environment as the Agent:

```bash
python -m pip install py-spy
```

Run the preflight against a Python target process:

```bash
python scripts/demo/check_pyspy_env.py --pid <target-pid>
```

Then create a task with `collector_type=py-spy`. On Linux you may still need
ptrace permissions, for example running the Agent with enough privileges or
adjusting `kernel.yama.ptrace_scope` for the demo session.

### Continuous profiling windows

The `计划任务` page now creates a minimal continuous profiling profile. Each
profile uses fixed 5-minute windows (`300s`) and materializes each due window as
a normal task, so the existing Agent, Analyzer, flamegraph, TopN, and attribution
views are reused.

The window list endpoint also returns a small aggregate summary for the latest
24 windows: total, done, failed, active, pending, latest status, latest range,
and done ratio. The Web page renders that summary above the window table so a
reviewer can verify continuous profiling health without opening every task.
The same endpoint accepts `status`, `from`, `to`, and `limit` query parameters
for scoped timeline review, and the Web page exposes those filters with a
clickable window strip for drilling into the materialized task.
`GET /api/v1/continuous-profiles/:id/trends` aggregates recent completed window
TopN files into a compact hotspot trend view, so the plan page can show whether
the same function is rising, falling, staying stable, or sitting in a high-risk
band across windows.
The trend payload now also includes a deterministic label and reason so the Web
page can flag sustained high-peak hotspots without a separate heuristic layer.
Continuous profiles can also be paused and resumed from the plan page; the API
persists an audit log for each lifecycle change while keeping existing windows
available for review.

For a Windows-safe demo, choose `collector_type=mock-perf`, enter the printed
mock target PID, and keep the default `5 分钟` interval. The first window is
created immediately so the demo does not need to wait five minutes; later
windows are scheduled by the API background scanner and the Agent polling loop.

### Attribution mini-loop

`GET /api/v1/tasks/:id/results` also returns an `attribution` object when
`topn.json` is readable. The current implementation is a deterministic
tool-driven attribution loop over TopN hotspots, rule matches, sampling
parameters, and seeded baseline rows. It returns a conclusion, confidence,
evidence list, recommendations, source metadata, prompt text, and tool trace so
the Web detail page can show a reproducible "归因建议" tab.

This is intentionally not a remote LLM call yet. The prompt and tool trace are
persisted in SQLite as the auditable contract that a later LLM loop can reuse.
The built-in tools are `get_top_hotspots(task_id)`,
`match_hotspot_rules(topn)`, `compare_with_baseline(task_id)`, and
`get_resource_timeline(task_id)`. The resource timeline tool currently produces
deterministic demo evidence for CPU / IO / memory / wait alignment; it is the
contract that a later real metrics source can replace.

Analyzer writes `perf.script.txt` and `collapsed.txt` for every `perf.data`
analysis. It uses the built-in stack parser and SVG renderer by default. To use
Brendan Gregg's standard FlameGraph tools, set:

```bash
export MINIDROP_STACKCOLLAPSE_PERF=/path/to/stackcollapse-perf.pl
export MINIDROP_FLAMEGRAPH_PL=/path/to/flamegraph.pl
```

If `stackcollapse-perf.pl` produces no stacks, the analyzer falls back to the
built-in parser. If `flamegraph.pl` is missing or fails, it falls back to the
built-in SVG renderer.

Expected failure reasons are explicit: missing `perf` returns
`perf command not found; install linux-tools or linux-perf`, and restrictive
kernel settings return a `perf_event_paranoid=...` message.

## API Surface

Public endpoints:

- `GET /healthz`
- `GET /api/v1/agents`
- `POST /api/v1/agents/heartbeat`
- `POST /api/v1/tasks`
- `GET /api/v1/tasks`
- `GET /api/v1/tasks/:id`
- `GET /api/v1/tasks/:id/results`
- `GET /api/v1/continuous-profiles`
- `POST /api/v1/continuous-profiles`
- `GET /api/v1/continuous-profiles/:id`
- `GET /api/v1/continuous-profiles/:id/windows`
- `GET /api/v1/audit-logs`

Internal mock-agent endpoints:

- `GET /api/v1/internal/tasks/claim`
- `POST /api/v1/internal/tasks/:id/uploading`
- `POST /api/v1/internal/tasks/:id/complete`
- `POST /api/v1/internal/tasks/:id/fail`

## Data and Artifacts

- SQLite database: `data/mini-drop.db`
- Raw artifacts: `artifacts/<task_id>/raw/`
- Analysis artifacts: `artifacts/<task_id>/analysis/`

Docker Compose uses PostgreSQL for the API database and MinIO for browser-facing
artifact downloads. The shared artifact volume remains the Agent/API handoff
path; API uploads files from that volume to MinIO before it signs URLs.

- Postgres database: `mini_drop_postgres`
- MinIO objects: `mini_drop_minio`
- Agent/API handoff artifacts: `mini_drop_artifacts`

Relevant storage environment variables:

- `MINIDROP_STORAGE_BACKEND=local|minio`
- `MINIDROP_MINIO_ENDPOINT=minio:9000`
- `MINIDROP_MINIO_PUBLIC_ENDPOINT=http://localhost:9000`
- `MINIDROP_MINIO_BUCKET=mini-drop-artifacts`
- `MINIDROP_MINIO_REGION=us-east-1`
- `MINIDROP_MINIO_PRESIGN_TTL_SEC=900`

## Next Steps

The repository still follows the documented roadmap:

1. Run the real collector smoke helpers in WSL2 / Ubuntu to validate kernel permissions.
2. Validate the optional standard FlameGraph scripts in the WSL2 / Linux demo path.
3. Replace the deterministic attribution loop with a remote LLM call and wire `get_resource_timeline` to real metrics.
4. Replace the minimal continuous profiling scheduler with a full cron/baseline implementation.

## Acceptance Status

Verified in the current Windows workspace:

- PowerShell local mock demo startup and cleanup with `scripts/demo/start-local.ps1` / `stop-local.ps1`.
- Local mock E2E smoke task with `mock-perf`.
- Docker Compose PostgreSQL API database startup.
- Docker Compose mock demo normal path.
- Docker Compose MinIO signed artifact URL path.
- Docker Compose PID failure path.
- Docker Compose acceptance snapshot with two completed TopN-backed tasks.
- Markdown demo evidence generation with `make demo-evidence` or
  `python scripts/demo/write_demo_evidence.py`.
- Optional WSL2 real-collector preflight evidence in
  `artifacts/demo-evidence.md`.
- Go API/Agent unit tests.
- Python Analyzer unit tests.
- Web production build.
- Linux shell script syntax.

Requires WSL2 / Linux host validation:

- `make smoke-real COLLECTOR_TYPE=perf`
- `make smoke-real COLLECTOR_TYPE=ebpf-syscall`
- `make smoke-real COLLECTOR_TYPE=py-spy`

Current WSL2 preflight on this machine detects a Linux runtime, but still needs
collector prerequisites before the real smoke can pass:

- `perf` is not installed and `kernel.perf_event_paranoid=2` blocks process profiling.
- `bpftrace` is not installed and tracefs is not readable for the current user.
- `py-spy` is not installed.

## Documentation

- [Original Mini-Drop assignment](docs/references/Mini-Drop-题目.md)
- [Original Drop reverse-engineering guide](docs/references/drop系统复刻指南.md)
- [Project brief](docs/design/00-project-brief.md)
- [MVP scope](docs/design/01-mvp-scope.md)
- [Architecture](docs/design/02-architecture.md)
- [State machines and observability](docs/design/03-state-machines-and-observability.md)
- [Development plan](docs/design/04-development-plan.md)
- [Backlog](docs/design/05-backlog.md)
- [Next implementation](docs/design/06-next-implementation.md)
- [Attribution evaluation](docs/design/07-attribution-evaluation.md)
- [Demo runbook](docs/demo-runbook.md)
- [Final demo script](docs/demo-script.md)
