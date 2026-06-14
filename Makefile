.PHONY: help demo demo-up demo-down local local-down test build web-build coverage docs-check smoke-local smoke-real smoke-demo smoke-demo-minio smoke-demo-fail smoke-demo-offline acceptance-snapshot demo-evidence recording-checklist submission-notes final-preflight real-preflight real-check perf-check ebpf-check pyspy-check

help:
	@echo "Mini-Drop commands:"
	@echo "  make demo        Build, start, and smoke-test the Docker demo stack"
	@echo "  make demo-up     Build and start the Docker demo stack without smoke test"
	@echo "  make demo-down   Stop the Docker demo stack"
	@echo "  make smoke-demo  Run a Docker Compose smoke task"
	@echo "  make smoke-demo-minio  Verify Docker Compose signed artifact URLs"
	@echo "  make smoke-demo-fail  Verify Docker Compose PID failure path"
	@echo "  make smoke-demo-offline  Verify Docker Compose agent offline path"
	@echo "  make acceptance-snapshot  Print compose demo acceptance evidence"
	@echo "  make demo-evidence  Write artifacts/demo-evidence.md from live demo state"
	@echo "  make recording-checklist  Write artifacts/recording-checklist.md"
	@echo "  make submission-notes  Write artifacts/submission-notes.md"
	@echo "  make final-preflight  Run the final demo preflight and write artifacts/final-preflight.md"
	@echo "  make real-preflight  Write artifacts/real-collector-preflight.md from WSL/Linux checks"
	@echo "  make local       Start the local Linux/WSL demo stack"
	@echo "  make local-down  Stop the local Linux/WSL demo stack"
	@echo "  make test        Run Go tests"
	@echo "  make coverage    Write artifacts/coverage-report.md and enforce required coverage gates"
	@echo "  make build       Build Go binaries and the web app"
	@echo "  make web-build   Build the web app only"
	@echo "  make smoke-local Run the local smoke helper against a PID"
	@echo "  make smoke-real  Run the Linux real-collector smoke helper"
	@echo "  make real-check  Check all real collector prerequisites"
	@echo "  make perf-check  Check local perf collector prerequisites"
	@echo "  make ebpf-check  Check local eBPF syscall collector prerequisites"
	@echo "  make pyspy-check Check local py-spy collector prerequisites"
	@echo "  make docs-check  Check required design docs exist"

demo:
	@powershell -NoProfile -ExecutionPolicy Bypass -File scripts\\demo\\start-compose.ps1

demo-up:
	@powershell -NoProfile -ExecutionPolicy Bypass -File scripts\\demo\\start-compose.ps1 -SkipSmoke

demo-down:
	@powershell -NoProfile -ExecutionPolicy Bypass -File scripts\\demo\\stop-compose.ps1

smoke-demo:
	@powershell -NoProfile -Command "$$apiPort = if ($$env:MINIDROP_API_PORT) { $$env:MINIDROP_API_PORT } else { '8080' }; $$env:MINIDROP_API_BASE_URL = if ($$env:MINIDROP_API_BASE_URL) { $$env:MINIDROP_API_BASE_URL } else { \"http://127.0.0.1:$$apiPort\" }; python scripts\\demo\\smoke_compose.py"

smoke-demo-minio:
	@powershell -NoProfile -Command "$$apiPort = if ($$env:MINIDROP_API_PORT) { $$env:MINIDROP_API_PORT } else { '8080' }; $$env:MINIDROP_API_BASE_URL = if ($$env:MINIDROP_API_BASE_URL) { $$env:MINIDROP_API_BASE_URL } else { \"http://127.0.0.1:$$apiPort\" }; python scripts\\demo\\smoke_compose.py --expect-minio-url"

smoke-demo-fail:
	@powershell -NoProfile -Command "$$apiPort = if ($$env:MINIDROP_API_PORT) { $$env:MINIDROP_API_PORT } else { '8080' }; $$env:MINIDROP_API_BASE_URL = if ($$env:MINIDROP_API_BASE_URL) { $$env:MINIDROP_API_BASE_URL } else { \"http://127.0.0.1:$$apiPort\" }; python scripts\\demo\\smoke_compose.py --pid 999999 --expect-status FAILED --expect-reason-contains \"target pid not found\""

smoke-demo-offline:
	@powershell -NoProfile -Command "$$apiPort = if ($$env:MINIDROP_API_PORT) { $$env:MINIDROP_API_PORT } else { '8080' }; $$env:MINIDROP_API_BASE_URL = if ($$env:MINIDROP_API_BASE_URL) { $$env:MINIDROP_API_BASE_URL } else { \"http://127.0.0.1:$$apiPort\" }; python scripts\\demo\\smoke_agent_offline.py"

acceptance-snapshot:
	@powershell -NoProfile -ExecutionPolicy Bypass -File scripts\\demo\\acceptance-snapshot.ps1

demo-evidence:
	@powershell -NoProfile -ExecutionPolicy Bypass -File scripts\\demo\\write-demo-evidence.ps1

recording-checklist:
	@powershell -NoProfile -ExecutionPolicy Bypass -File scripts\\demo\\write-recording-checklist.ps1

submission-notes:
	@powershell -NoProfile -ExecutionPolicy Bypass -File scripts\\demo\\write-submission-notes.ps1

final-preflight:
	@powershell -NoProfile -ExecutionPolicy Bypass -File scripts\\demo\\final-preflight.ps1

real-preflight:
	@powershell -NoProfile -ExecutionPolicy Bypass -File scripts\\demo\\prepare-real-collectors.ps1

perf-check:
	@powershell -NoProfile -Command "if ($$env:MINIDROP_TARGET_PID) { python scripts\\demo\\check_perf_env.py --pid $$env:MINIDROP_TARGET_PID } else { python scripts\\demo\\check_perf_env.py }"

ebpf-check:
	@powershell -NoProfile -Command "if ($$env:MINIDROP_TARGET_PID) { python scripts\\demo\\check_ebpf_env.py --pid $$env:MINIDROP_TARGET_PID } else { python scripts\\demo\\check_ebpf_env.py }"

pyspy-check:
	@powershell -NoProfile -Command "if ($$env:MINIDROP_TARGET_PID) { python scripts\\demo\\check_pyspy_env.py --pid $$env:MINIDROP_TARGET_PID } else { python scripts\\demo\\check_pyspy_env.py }"

real-check:
	@if [ -n "$$MINIDROP_TARGET_PID" ]; then python3 scripts/demo/check_real_collectors.py --pid "$$MINIDROP_TARGET_PID"; else python3 scripts/demo/check_real_collectors.py; fi

local:
	bash ./scripts/demo/start-local.sh

local-down:
	bash ./scripts/demo/stop-local.sh

test:
	@powershell -NoProfile -Command "$$env:GOPROXY='https://goproxy.cn,direct'; go test ./apps/api-server ./apps/agent ./internal/..."
	@powershell -NoProfile -Command "python -m unittest apps.analyzer.main_test"

coverage:
	@powershell -NoProfile -Command "$$env:GOPROXY='https://goproxy.cn,direct'; python scripts\\demo\\check_coverage.py"

build: web-build
	@powershell -NoProfile -Command "$$env:GOPROXY='https://goproxy.cn,direct'; go build ./apps/api-server ./apps/agent"

web-build:
	@powershell -NoProfile -Command "Set-Location apps\\web; npm run build"

smoke-local:
	@powershell -NoProfile -Command "python scripts\\demo\\smoke_e2e.py $$env:MINIDROP_TARGET_PID $$env:MINIDROP_TARGET_AGENT_ID"

smoke-real:
	bash ./scripts/demo/smoke_real_collector.sh $(COLLECTOR_TYPE)

docs-check:
	@test -f docs/design/00-project-brief.md
	@test -f docs/design/01-mvp-scope.md
	@test -f docs/design/02-architecture.md
	@test -f docs/design/03-state-machines-and-observability.md
	@test -f docs/design/04-development-plan.md
	@test -f docs/design/05-backlog.md
	@test -f docs/design/06-next-implementation.md
	@test -f docs/design/07-attribution-evaluation.md
	@test -f docs/demo-runbook.md
	@test -f docs/demo-script.md
	@echo "Required docs exist."
