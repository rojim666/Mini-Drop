# 05. Backlog

This backlog is ordered by implementation dependency. Items near the top should
be finished before lower items are started.

## P0: Repository Foundation

- [ ] Create `apps/api-server`.
- [ ] Create `apps/web`.
- [ ] Create `apps/agent`.
- [ ] Create `apps/analyzer`.
- [ ] Create `deploy/docker-compose.yml`.
- [ ] Create `scripts/demo`.
- [ ] Add root `.gitignore`.
- [ ] Add root Makefile with `make demo`.

## P0: API Server Mock Slice

- [ ] Initialize Go module.
- [ ] Add `/healthz`.
- [ ] Add SQLite or PostgreSQL connection.
- [ ] Create initial tables: `agents`, `tasks`, `task_status_events`, `audit_logs`, `analysis_results`.
- [ ] Add `POST /api/v1/tasks`.
- [ ] Add `GET /api/v1/tasks`.
- [ ] Add `GET /api/v1/tasks/:id`.
- [ ] Add task status transition helper.

## P0: Agent Mock Slice

- [ ] Initialize Agent app.
- [ ] Register Agent with API Server.
- [ ] Send heartbeat every 5 seconds.
- [ ] Poll for pending task or accept mock task.
- [ ] Mark task `RUNNING`.
- [ ] Produce mock artifact.
- [ ] Mark task `UPLOADING`.
- [ ] Mark task `DONE`.

## P0: Analyzer Mock Slice

- [ ] Initialize Analyzer app.
- [ ] Add command line entry: `analyzer --task-id <id>`.
- [ ] Generate mock flamegraph SVG.
- [ ] Generate mock TopN JSON.
- [ ] Save artifact path to API Server or shared storage.

## P0: Web Mock Slice

- [ ] Initialize Web app.
- [ ] Create Agent list view.
- [ ] Create task form.
- [ ] Create task list view.
- [ ] Create task detail view.
- [ ] Render mock SVG flamegraph.
- [ ] Poll task status.

## P1: Real perf Collector

- [ ] Validate Linux runtime permissions.
- [ ] Check `perf` availability.
- [ ] Detect `perf_event_paranoid`.
- [ ] Implement `perf record`.
- [ ] Implement timeout and process cleanup.
- [ ] Convert `perf.data` to collapsed stacks.
- [ ] Generate SVG flamegraph.
- [ ] Show real flamegraph in Web.

## P1: Observability

- [ ] Add structured logs to API Server.
- [ ] Add structured logs to Agent.
- [ ] Add structured logs to Analyzer.
- [ ] Persist all status transitions.
- [ ] Persist Agent offline audit log.
- [ ] Persist Agent recovery audit log.

## P1: Tests

- [ ] E2E normal path: valid PID -> flamegraph.
- [ ] E2E error path: invalid PID -> FAILED with reason.
- [ ] E2E error path: Agent offline -> FAILED or no dispatch with reason.
- [ ] Unit tests for status transition helper.
- [ ] Unit tests for input validation.

## P2: eBPF Collector

- [ ] Choose `bpftrace`, `bcc`, or `libbpf-go`.
- [ ] Implement one kernel probe.
- [ ] Add demo workload using `dd`, `fio`, or `stress-ng`.
- [ ] Parse eBPF output into JSON.
- [ ] Display eBPF visualization in Web.

## P2: User-Space Language Collector

- [ ] Choose `py-spy` or `pprof HTTP`.
- [ ] Add demo target program.
- [ ] Generate collector output.
- [ ] Display language-specific visualization.

## P2: Continuous Profiling

- [ ] Add low-frequency recurring sampling.
- [ ] Store time-windowed artifacts.
- [ ] Add Web time window selector.
- [ ] Display one 5-minute window result.

## P3: AI Attribution

- [ ] Define attribution tool interfaces.
- [ ] Create baseline comparison data.
- [ ] Implement attribution prompt and tool loop.
- [ ] Store attribution result with evidence.
- [ ] Write small evaluation report.

