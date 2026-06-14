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

## Current Components

- `apps/api-server`: Go + Gin + GORM + SQLite API server
- `apps/agent`: Go mock agent with heartbeat and task loop
- `apps/analyzer`: Python CLI that generates mock flamegraphs and hotspots
- `apps/web`: React + Vite + TypeScript operations dashboard
- `deploy/docker-compose.yml`: one-command demo stack
- `scripts/demo`: smoke test and mock target helpers

## Quick Start

### Docker demo

```bash
make demo
```

Then open:

- Web UI: [http://localhost:4173](http://localhost:4173)
- API health: [http://localhost:8080/healthz](http://localhost:8080/healthz)

For the compose-backed demo, use `PID 1` in the task form. The agent shares the
PID namespace of the bundled `demo-target` container, so PID 1 is a stable mock
workload for the end-to-end flow.

To stop the stack:

```bash
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

### Local smoke helper

After starting the API and Agent locally, run:

```bash
python scripts/demo/smoke_e2e.py <pid> [agent_id]
```

This creates a task and polls until it reaches `DONE` or `FAILED`.

## API Surface

Public endpoints:

- `GET /healthz`
- `GET /api/v1/agents`
- `POST /api/v1/agents/heartbeat`
- `POST /api/v1/tasks`
- `GET /api/v1/tasks`
- `GET /api/v1/tasks/:id`
- `GET /api/v1/tasks/:id/results`
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

## Next Steps

The repository still follows the documented roadmap:

1. Replace the mock collector with real `perf record` in WSL2 / Ubuntu.
2. Preserve the same status machine and artifact contracts.
3. Add Postgres + MinIO deployment mode.
4. Extend with eBPF, continuous profiling, user-space collectors, and AI attribution.

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
