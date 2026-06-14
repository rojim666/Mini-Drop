# Mini-Drop Demo Runbook

This runbook keeps the two demo paths separate:

- Windows / Docker Desktop: full mock E2E with PostgreSQL, MinIO signed URLs, API, Agent, and Web.
- WSL2 Ubuntu / Linux: native Agent path for real collectors such as `perf`, `ebpf-syscall`, and `py-spy`.

For the actual recording or live review order, use
[`docs/demo-script.md`](demo-script.md). This runbook is the command reference;
the demo script is the narrated walkthrough.

## 1. Windows Compose Demo

Use this path for the default review demo. It does not require Linux kernel
profiling permissions.

```powershell
.\scripts\demo\start-compose.ps1
```

The script builds the stack, starts it in the background, creates two mock
tasks by default, waits for `DONE`, and verifies that result artifacts are
returned as MinIO signed URLs. The two completed tasks make the task-comparison
page ready for a recording without extra manual setup.

Open:

- Web: <http://localhost:4173>
- API: <http://localhost:8080/healthz>
- MinIO console: <http://localhost:9001>
- MinIO login: `minidrop` / `minidrop123`

Use `PID 1` in the Web form. The Agent shares the PID namespace of the bundled
`demo-target` container, so PID 1 is the stable target process.

If default ports are occupied:

```powershell
.\scripts\demo\start-compose.ps1 -ApiPort 18080 -WebPort 14173 -MinioPort 19000 -MinioConsolePort 19001
```

The script prints matching acceptance snapshot, evidence, recording checklist,
submission notes, and final preflight commands with the same ports. Copy those
printed commands when the stack is not using the default `8080` / `4173` /
`9000` ports.

For a faster startup that only creates one smoke task:

```powershell
.\scripts\demo\start-compose.ps1 -SmokeTaskCount 1
```

Stop:

```powershell
.\scripts\demo\stop-compose.ps1
```

Remove demo volumes as well:

```powershell
.\scripts\demo\stop-compose.ps1 -Volumes
```

Equivalent Make targets:

```powershell
make demo
make demo-up
make smoke-demo
make smoke-demo-minio
make smoke-demo-fail
make smoke-demo-offline
make compose-health
make acceptance-snapshot
make demo-evidence
make recording-checklist
make submission-notes
make coverage
make final-preflight
make demo-down
```

`make demo` starts the stack and performs the MinIO signed URL smoke test.
`make demo-up` only starts the stack.
`make compose-health` verifies the six compose services and host port mappings
before recording, so a half-started stack is caught before Web/API evidence is
collected.
`make demo-evidence` writes `artifacts/demo-evidence.md` from the currently
running API/Web/Git state so the final recording has a reproducible evidence
summary. To include WSL2 / Linux real-collector prerequisite evidence, run:

```powershell
.\scripts\demo\write-demo-evidence.ps1 -ApiPort 18080 -WebPort 14173 -MinioPort 19000 -IncludeRealPreflight
```

If collector dependencies are missing, the evidence file records the blocked
collector and the exact install or permission command to run next.
`make recording-checklist` writes `artifacts/recording-checklist.md`, a
page-by-page capture checklist for the final recording.
`make submission-notes` writes `artifacts/submission-notes.md`, including the
expected screenshot file names and a concise commit summary template.
`make final-preflight` runs the lightweight recording gate, then writes
`artifacts/final-preflight.md` with the combined check results, command
outputs, and the final record/no-record decision.
`make coverage` writes `artifacts/coverage-report.md` and enforces the required
coverage gates used by the final preflight.
On Windows without `make`, use the PowerShell snapshot helper directly:

```powershell
.\scripts\demo\check-compose-stack.ps1
.\scripts\demo\acceptance-snapshot.ps1
.\scripts\demo\write-demo-evidence.ps1
.\scripts\demo\write-recording-checklist.ps1
.\scripts\demo\write-submission-notes.ps1
python .\scripts\demo\check_coverage.py
.\scripts\demo\final-preflight.ps1
```

If compose is running on alternate ports, pass the same ports to the snapshot
and evidence helpers:

```powershell
.\scripts\demo\check-compose-stack.ps1 -ApiPort 18080 -WebPort 14173 -MinioPort 19000 -MinioConsolePort 19001
.\scripts\demo\acceptance-snapshot.ps1 -ApiPort 18080 -WebPort 14173 -MinioPort 19000
.\scripts\demo\write-demo-evidence.ps1 -ApiPort 18080 -WebPort 14173 -MinioPort 19000
.\scripts\demo\write-recording-checklist.ps1 -ApiPort 18080 -WebPort 14173 -MinioPort 19000 -MinioConsolePort 19001
.\scripts\demo\write-submission-notes.ps1 -ApiPort 18080 -WebPort 14173 -MinioConsolePort 19001
.\scripts\demo\final-preflight.ps1 -ApiPort 18080 -WebPort 14173 -MinioPort 19000 -MinioConsolePort 19001 -SeedTasks -IncludeRealPreflight
```

On Linux / WSL2 without `make`, use the Bash helper:

```bash
bash ./scripts/demo/check-compose-stack.sh
bash ./scripts/demo/acceptance-snapshot.sh
bash ./scripts/demo/write-demo-evidence.sh
bash ./scripts/demo/write-demo-evidence.sh --include-real-preflight
bash ./scripts/demo/write-recording-checklist.sh
bash ./scripts/demo/write-submission-notes.sh
bash ./scripts/demo/final-preflight.sh
```

If the stack is already running but lacks two completed mock tasks, seed and
check in one command:

```powershell
.\scripts\demo\acceptance-snapshot.ps1 -SeedTasks
```

```bash
bash ./scripts/demo/acceptance-snapshot.sh --seed-tasks
```

`make acceptance-snapshot` prints the same compact pre-recording checklist
covering API health, Web reachability, online Agents, completed tasks, signed
artifact URLs, whether the task-comparison page has enough completed
TopN-backed tasks, whether continuous profiling has materialized windows and
trend data, and each sampled profile's interval/cron/stagger schedule policy.

## 2. WSL2 / Linux Real Collector Demo

Use this path when validating real kernel or language profiling. Run these
commands inside WSL2 Ubuntu or another Linux host.

Start the native local stack with one collector selected:

```bash
COLLECTOR_TYPE=perf bash ./scripts/demo/start-local.sh
COLLECTOR_TYPE=ebpf-syscall bash ./scripts/demo/start-local.sh
COLLECTOR_TYPE=py-spy bash ./scripts/demo/start-local.sh
```

The script prints the target PID and writes it to `tmp/local-demo/target.pid`.

Run the matching smoke helper:

```bash
make smoke-real COLLECTOR_TYPE=perf
make smoke-real COLLECTOR_TYPE=ebpf-syscall
make smoke-real COLLECTOR_TYPE=py-spy
```

Stop:

```bash
bash ./scripts/demo/stop-local.sh
```

## 3. Real Collector Prerequisites

Run preflight checks before creating a real collector task:

```bash
make real-preflight
make real-check
python3 scripts/demo/check_perf_env.py --pid <target-pid>
python3 scripts/demo/check_ebpf_env.py --pid <target-pid>
python3 scripts/demo/check_pyspy_env.py --pid <target-pid>
```

`make real-check` runs all three checks and prints a ready/blocked summary.
Use the individual scripts when you want the full output for one collector.
`make real-preflight` writes `artifacts/real-collector-preflight.md` with the
same readiness summary and the exact install or permission commands that would
unblock the current host.

Typical fixes:

```bash
sudo apt-get update
sudo apt-get install -y linux-tools-common linux-tools-generic bpftrace
python3 -m pip install --user py-spy
sudo sysctl kernel.perf_event_paranoid=1
sudo mount -t tracefs tracefs /sys/kernel/tracing 2>/dev/null || true
```

For `py-spy`, the target PID must be a Python process and the current user must
be allowed to attach to it. On Linux this can require ptrace permissions or a
temporary `kernel.yama.ptrace_scope` adjustment.

## 4. Acceptance Checklist

- Compose starts with PostgreSQL, MinIO, API, Agent, Web, and `demo-target`.
- `make compose-health` reports `compose_stack=OK` with expected port mappings.
- A mock task reaches `PENDING -> RUNNING -> UPLOADING -> DONE`.
- `flamegraph_url` and `topn_url` contain `X-Amz-Signature` and the configured MinIO public port.
- Web task detail can load the flamegraph SVG.
- `make acceptance-snapshot` reports `acceptance=OK` before recording.
- Linux preflight checks explain missing tools or permissions clearly.
- At least the `perf` smoke path reaches `DONE` after WSL2/Linux prerequisites are installed.

## 5. Common Compose Failures

- If `docker compose up` fails while pulling base images, check Docker Desktop
  network settings, registry mirrors, VPN, and proxy configuration. The helper
  scripts use `--pull never` for repeatable local demos, so the first run still
  needs the required base images to exist locally or be pulled successfully once.
- If the script reports success but the smoke talks to an old local API on
  `8080`, stop the old local demo first or choose alternate ports for compose.
- If MinIO artifact URLs fall back to local `/artifacts/...`, confirm the
  compose API container has `MINIDROP_STORAGE_BACKEND=minio` and that the
  MinIO endpoint is reachable from inside the container.
