.PHONY: help demo demo-down test build web-build docs-check smoke-local

help:
	@echo "Mini-Drop commands:"
	@echo "  make demo        Build and start the Docker demo stack"
	@echo "  make demo-down   Stop the Docker demo stack"
	@echo "  make test        Run Go tests"
	@echo "  make build       Build Go binaries and the web app"
	@echo "  make web-build   Build the web app only"
	@echo "  make smoke-local Run the local smoke helper against a PID"
	@echo "  make docs-check  Check required design docs exist"

demo:
	docker compose -f deploy/docker-compose.yml up --build -d
	@echo "Mini-Drop demo is starting:"
	@echo "  Web UI: http://localhost:4173"
	@echo "  API:    http://localhost:8080/healthz"
	@echo "Use PID 1 in the UI for the compose-backed mock target."

demo-down:
	docker compose -f deploy/docker-compose.yml down --remove-orphans

test:
	@powershell -NoProfile -Command "$$env:GOPROXY='https://goproxy.cn,direct'; go test ./apps/api-server ./apps/agent ./internal/..."

build: web-build
	@powershell -NoProfile -Command "$$env:GOPROXY='https://goproxy.cn,direct'; go build ./apps/api-server ./apps/agent"

web-build:
	@powershell -NoProfile -Command "Set-Location apps\\web; npm run build"

smoke-local:
	@powershell -NoProfile -Command "python scripts\\demo\\smoke_e2e.py $$env:MINIDROP_TARGET_PID $$env:MINIDROP_TARGET_AGENT_ID"

docs-check:
	@test -f docs/design/00-project-brief.md
	@test -f docs/design/01-mvp-scope.md
	@test -f docs/design/02-architecture.md
	@test -f docs/design/03-state-machines-and-observability.md
	@test -f docs/design/04-development-plan.md
	@test -f docs/design/05-backlog.md
	@echo "Required docs exist."
