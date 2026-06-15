# Mini-Drop Final Demo Script

This script is for a 10 to 15 minute recording or live review. It keeps the
stable Windows Compose path separate from the WSL2 / Linux real collector path,
so the demo can succeed even when kernel profiling tools are not yet installed.

## 0. Pre-Recording Setup

Use the alternate ports when a local API or another MinIO instance is already
running:

```powershell
.\scripts\demo\start-compose.ps1 -ApiPort 18080 -WebPort 14173 -MinioPort 19000 -MinioConsolePort 19001
```

Generate acceptance and evidence output:

```powershell
.\scripts\demo\check-compose-stack.ps1 -ApiPort 18080 -WebPort 14173 -MinioPort 19000 -MinioConsolePort 19001
.\scripts\demo\acceptance-snapshot.ps1 -ApiPort 18080 -WebPort 14173 -MinioPort 19000 -SeedTasks
.\scripts\demo\write-demo-evidence.ps1 -ApiPort 18080 -WebPort 14173 -MinioPort 19000 -IncludeRealPreflight
.\scripts\demo\write-recording-checklist.ps1 -ApiPort 18080 -WebPort 14173 -MinioPort 19000 -MinioConsolePort 19001
.\scripts\demo\write-submission-notes.ps1 -ApiPort 18080 -WebPort 14173 -MinioConsolePort 19001
python .\scripts\demo\check_coverage.py
.\scripts\demo\capture-submission-artifacts.ps1 -WebPort 14173 -MinioConsolePort 19001
.\scripts\demo\final-preflight.ps1 -ApiPort 18080 -WebPort 14173 -MinioPort 19000 -MinioConsolePort 19001 -IncludeRealPreflight
```

Open these URLs before recording:

- Web: <http://localhost:14173>
- API health: <http://localhost:18080/healthz>
- MinIO console: <http://localhost:19001>
- Evidence file: `artifacts/demo-evidence.md`
- Recording checklist: `artifacts/recording-checklist.md`
- Submission notes: `artifacts/submission-notes.md`
- Screenshot evidence: `artifacts/submission-screenshots/`
- Coverage report: `artifacts/coverage-report.md`
- Final preflight: `artifacts/final-preflight.md`

Expected preflight state on the current machine:

- Mock compose path: ready.
- Final preflight overall status: `OK`.
- Real collector readiness step: non-blocking for Windows compose recording.
- Real `perf`: blocked until `perf` is installed and `perf_event_paranoid` is lowered.
- `ebpf-syscall`: blocked until `bpftrace` and tracefs permissions are ready.
- `py-spy`: blocked until `py-spy` is installed and attach permissions are available.

## 1. Opening Summary

Say:

> Mini-Drop is a small performance diagnosis platform. The Web UI creates a
> profiling task, the API persists it and records status transitions, the Agent
> collects raw artifacts, the Analyzer converts them into a flamegraph and TopN
> hotspots, and the Web UI displays the result.

Show:

- `README.md` or the Web dashboard.
- The high-level chain: `Web -> API Server -> Agent -> Analyzer -> Web`.
- The online Agent count and completed task count on the dashboard.

## 2. Stable Mock E2E Path

Show the Web dashboard:

1. Confirm the left navigation and Tencent Cloud style console layout.
2. Open `机器列表` and show the `compose-agent` heartbeat.
3. Open `历史任务` and show at least two `DONE` tasks.
4. Open one task detail and show:
   - status tag
   - status history
   - flamegraph tab
   - TopN tab
   - attribution tab

Say:

> The mock collector is intentionally deterministic. It proves the product
> contract, storage path, status machine, and visualization path without being
> blocked by Linux kernel permissions.

## 3. Create a New Demo Task

In Web, click `新建采样`:

- Target PID: `1`
- Target machine: `compose-agent` or automatic
- Duration: `15`
- Frequency: `99`
- Collector: `mock-perf`

Submit the task and narrate the state transitions:

```text
PENDING -> RUNNING -> UPLOADING -> DONE
```

If the task is already done before the UI is refreshed, use `历史任务` and the
status history panel to show the recorded transitions.

## 4. Artifact and Storage Evidence

Show:

- `文件分析` page for raw artifact, `flamegraph.svg`, and `topn.json`.
- A signed artifact URL containing `X-Amz-Signature`.
- MinIO console only if reviewers ask to see the object store.

Say:

> In the Compose path, analysis outputs are uploaded to MinIO and returned as
> temporary signed URLs, matching the production-style object storage contract.

## 5. Comparison and Continuous Profiling

Show:

- `任务对比`: compare two completed tasks and point out TopN delta rows.
- `任务对比`: show recurring hotspot aggregate rows across tasks and continuous profiles.
- `计划任务`: show the minimal continuous profiling profile and its materialized
  task windows, interval/cron policy, stagger offset, trend labels, and baseline
  drift.

Say:

> Continuous profiling is implemented as recurring windows that materialize into
> normal tasks, so the same Agent, Analyzer, flamegraph, TopN, and attribution
> path is reused. Interval, cron, and stagger settings are visible from the same
> plan table. Trend labels and baseline drift make repeated hotspots visible
> without opening every single window.

## 6. Failure and Audit Path

Use one of these before recording, or show existing evidence if time is tight:

```powershell
$env:MINIDROP_API_BASE_URL = "http://127.0.0.1:18080"
python scripts\demo\smoke_compose.py --pid 999999 --agent-id agt_compose --expect-status FAILED --expect-reason-contains "target pid not found"
```

Show:

- Failed task row.
- `status_reason`.
- Status history event.
- Agent offline/recovery audit logs if `smoke-demo-offline` has been run.

Say:

> The demo is not just a happy path. Expected failures are persisted with clear
> reasons, and lifecycle events remain inspectable from the UI and API.

## 7. Real Collector Evidence

Open `artifacts/demo-evidence.md` and show `Real Collector Preflight`.

Say:

> The real collector code paths are implemented, but final smoke validation
> depends on Linux host prerequisites. The evidence file records exactly which
> collectors are ready or blocked and prints the next command to run.

If WSL2 dependencies are later installed, run:

```bash
make real-preflight
make real-check
make real-smoke-report
COLLECTOR_TYPE=perf bash ./scripts/demo/start-local.sh
make smoke-real COLLECTOR_TYPE=perf
```

Optional:

```bash
make smoke-real COLLECTOR_TYPE=ebpf-syscall
make smoke-real COLLECTOR_TYPE=py-spy
```

## 8. Closing Checklist

Before finishing, show or mention:

- `compose_stack=OK` from `check-compose-stack`.
- `acceptance=OK` from `acceptance-snapshot`.
- `Real collector readiness` from `final-preflight`, showing READY or the exact
  BLOCKED prerequisites.
- `artifacts/real-smoke-report.md`, showing smoke READY/DONE or the current
  BLOCKED reason.
- `continuous_profiles` and `continuous_profile_samples` lines from
  `acceptance-snapshot`, including each profile's schedule policy.
- `artifacts/demo-evidence.md`.
- `artifacts/submission-notes.md` screenshot manifest.
- `artifacts/submission-screenshots/` contains 10 generated PNGs.
- `artifacts/coverage-report.md` required gates.
- `artifacts/final-preflight.md` overall status.
- Recent Git commits with meaningful messages.
- Tests:

```powershell
go test ./apps/api-server ./apps/agent ./internal/...
python -m unittest apps.analyzer.main_test
npm --prefix apps\web run build
```

Closing sentence:

> The mock product path is complete and repeatable. The real collector path is
> coded and documented, with the remaining work narrowed to WSL2/Linux tool
> installation and permission validation.
