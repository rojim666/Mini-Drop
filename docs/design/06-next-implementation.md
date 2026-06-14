# 06. Next Implementation

## Focus

Keep moving from the current demo to the next most useful end-to-end slice:

1. Make the demo easy to start on Windows with one command.
2. Keep WSL2 / Linux real collectors runnable and clearly documented.
3. Keep the acceptance path explicit: mock compose demo, then real `perf`, then the other collectors.

## Current State

The repository now has:

- a mock compose demo with PostgreSQL, MinIO signed artifact URLs, and two seeded DONE tasks for comparison
- a native local demo for Windows mock profiling
- a native Linux / WSL2 demo path for `perf`, `ebpf-syscall`, and `py-spy`
- per-collector preflight scripts
- a runbook that separates compose and Linux flows

## Next Implementation Target

The next slice should improve demo readiness rather than add another collector:

1. Keep Windows compose startup one-command and smoke-verified.
2. Keep Linux / WSL2 startup one-command for the real collectors.
3. Keep the runbook and README synchronized with the actual scripts.

## Acceptance Shape

This stage is complete when:

- `.\scripts\demo\start-compose.ps1` starts the compose stack, seeds comparison-ready mock tasks, and smoke-tests MinIO signed URLs.
- `.\scripts\demo\acceptance-snapshot.ps1` gives Windows users the same pre-recording evidence as `make acceptance-snapshot`.
- `bash ./scripts/demo/start-local.sh` still works for Linux / WSL2 mock or real collectors.
- `docs/demo-runbook.md` matches the real commands and ports.
- README points users to the right path for Windows compose and WSL2 real collection.
