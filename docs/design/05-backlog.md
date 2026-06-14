# 05. Backlog

This backlog is ordered by implementation dependency. Items near the top should
be finished before lower items are started.

## P0: Repository Foundation

- [x] Create `apps/api-server`.
- [x] Create `apps/web`.
- [x] Create `apps/agent`.
- [x] Create `apps/analyzer`.
- [x] Create `deploy/docker-compose.yml`.
- [x] Create `scripts/demo`.
- [x] Add root `.gitignore`.
- [x] Add root Makefile with `make demo`.

## P0: API Server Mock Slice

- [x] Initialize Go module.
- [x] Add `/healthz`.
- [x] Add SQLite or PostgreSQL connection.
- [x] Create initial tables: `agents`, `tasks`, `task_status_events`, `audit_logs`, `analysis_results`.
- [x] Add `POST /api/v1/tasks`.
- [x] Add `GET /api/v1/tasks`.
- [x] Add `GET /api/v1/tasks/:id`.
- [x] Add task status transition helper.

## P0: Agent Mock Slice

- [x] Initialize Agent app.
- [x] Register Agent with API Server.
- [x] Send heartbeat every 5 seconds.
- [x] Poll for pending task or accept mock task.
- [x] Mark task `RUNNING`.
- [x] Produce mock artifact.
- [x] Mark task `UPLOADING`.
- [x] Mark task `DONE`.

## P0: Analyzer Mock Slice

- [x] Initialize Analyzer app.
- [x] Add command line entry: `analyzer --task-id <id>`.
- [x] Generate mock flamegraph SVG.
- [x] Generate mock TopN JSON.
- [x] Save artifact path to API Server or shared storage.

## P0: Web Mock Slice

- [x] Initialize Web app.
- [x] Create Agent list view.
- [x] Create task form.
- [x] Create task list view.
- [x] Create task detail view.
- [x] Render mock SVG flamegraph.
- [x] Poll task status.

## P1: Real perf Collector

- [ ] Validate Linux runtime permissions.
- [x] Check `perf` availability.
- [x] Detect `perf_event_paranoid`.
- [x] Implement `perf record`.
- [x] Implement timeout and process cleanup.
- [x] Convert `perf.data` to collapsed stacks.
- [x] Generate SVG flamegraph.
- [x] Show real flamegraph in Web.
- [x] Add a stable CPU workload for perf demo validation.
- [x] Add real collector smoke helper for WSL2 / Linux.
- [x] Add optional `stackcollapse-perf.pl` and `flamegraph.pl` analyzer integration.

## P1: Delivery Baseline

- [x] Add PostgreSQL deployment mode for API Server.
- [x] Add MinIO object storage and signed artifact URLs.
- [x] Add compose-side readiness checks for Postgres and Web.
- [x] Write one-click demo docs for compose and WSL2 flows.
- [x] Add final demo script for recording and live review.
- [x] Add final preflight report for the recording gate.

## P1: Observability

- [x] Add structured logs to API Server.
- [x] Add structured logs to Agent.
- [x] Add structured logs to Analyzer.
- [x] Log task lifecycle transitions and collector milestones.
- [x] Persist all status transitions.
- [x] Persist Agent offline audit log.
- [x] Persist Agent recovery audit log.

## P1: Tests

- [x] E2E normal path: valid PID -> flamegraph.
- [x] E2E error path: invalid PID -> FAILED with reason.
- [x] E2E error path: Agent offline -> FAILED or no dispatch with reason.
- [x] Unit tests for status transition helper.
- [x] Unit tests for input validation.

## P2: eBPF Collector

- [x] Choose `bpftrace`, `bcc`, or `libbpf-go`.
- [x] Implement one kernel probe.
- [x] Add demo workload using `dd`, `fio`, or `stress-ng`.
- [x] Parse eBPF output into JSON.
- [x] Display eBPF visualization in Web.

## P2: User-Space Language Collector

- [x] Choose `py-spy` or `pprof HTTP`.
- [x] Add demo target program.
- [x] Generate collector output.
- [x] Display language-specific visualization.

## P2: Continuous Profiling

- [x] Add low-frequency recurring sampling.
- [x] Store time-windowed artifacts.
- [x] Add Web time window selector.
- [x] Display one 5-minute window result.

## P3: AI Attribution

- [x] Define attribution result contract.
- [x] Generate rule-based attribution from TopN hotspots.
- [x] Display attribution conclusion, confidence, evidence, and recommendations in Web.
- [x] Add small rule evaluation samples.
- [x] Define attribution tool interfaces.
- [x] Create baseline comparison data.
- [x] Implement attribution prompt and tool loop.
- [x] Store attribution result with evidence.
- [x] Write small evaluation report.
