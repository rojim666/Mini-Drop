# Mini-Drop

Mini-Drop 是一个面向 Linux 性能诊断场景的多组件 Demo。它按“Web UI + Server + Agent + Analyzer”的架构实现了从创建采样任务、Agent 采集、Analyzer 生成火焰图/热点数据，到 Web 展示和智能归因的闭环。

当前仓库已经具备可运行的 mock 演示链路：

```text
Web UI -> API Server -> Agent -> Collector -> Analyzer -> Flamegraph/TopN -> Web UI
```

这条 mock 链路保留了真实系统的协议和状态机：

1. 用户在 Web 控制台创建 Profiling 任务。
2. API Server 持久化任务，并记录每一次状态迁移。
3. Agent 每 5 秒心跳，领取任务，执行采集器。
4. Analyzer 把原始采集产物转换为 `flamegraph.svg` 和 `topn.json`。
5. Web 展示 Agent 健康、任务状态、审计日志、火焰图、TopN 热点和归因建议。
6. 连续剖析页面可以创建周期性窗口，复用同一套任务和结果展示链路。

## 当前能力

- Docker Compose 一键启动 PostgreSQL、MinIO、API Server、Agent 和 Web。
- Web 登录、Agent 列表、任务创建、任务详情、审计日志、火焰图、TopN 热点。
- 任务状态机：`PENDING -> RUNNING -> UPLOADING -> DONE / FAILED`，每次迁移都落库并带 `reason`。
- Agent 心跳和离线扫描：30 秒无心跳判定离线，恢复时写审计日志。
- 本地 mock collector：Windows / Docker Desktop 可稳定演示。
- Linux / WSL2 真实采集器：`perf`、`ebpf-syscall`、`py-spy`，带 preflight 和 blocked 报告。
- Analyzer 支持 mock JSON、`perf.data`、eBPF syscall 直方图、py-spy raw stack。
- Continuous Profiling：创建周期性窗口、查看窗口状态、热点趋势和基线漂移。
- AI 归因入口：支持 OpenAI-compatible LLM 配置，未配置或失败时回退到规则归因。
- 工程基线：Go / Python 单测、覆盖率报告、Compose smoke、最终验收 preflight 和提交材料生成脚本。

## 目录结构

```text
apps/
  api-server/      Go + Gin + GORM API 服务
  agent/           Go Agent，负责心跳、任务领取和采集器调度
  analyzer/        Python CLI，负责产物解析和可视化数据生成
  web/             React + Vite + TypeScript 控制台
deploy/
  docker-compose.yml
docs/
  design/          设计文档
  references/      题目与复刻参考
scripts/demo/      启动、smoke、验收和证据生成脚本
```

## 快速启动

### Windows PowerShell 推荐方式

如果机器上没有 `make`，直接运行 PowerShell 脚本：

```powershell
.\scripts\demo\start-compose.ps1
```

脚本会启动 Compose 栈，并自动跑一次 smoke 任务。默认会创建可用于演示的 mock 任务和结果。

启动后打开：

- Web 控制台：[http://localhost](http://localhost)
- 登录账号：`demo` / `minidrop`
- API 健康检查：[http://localhost:8080/healthz](http://localhost:8080/healthz)
- MinIO 控制台：[http://localhost:9001](http://localhost:9001)，账号 `minidrop` / `minidrop123`

停止 Compose 栈：

```powershell
.\scripts\demo\stop-compose.ps1
```

### 使用 Make

如果本机已经安装 `make`：

```bash
make demo
```

停止：

```bash
make demo-down
```

### 端口冲突处理

如果 `8080`、`80`、`9000` 或 `9001` 已被占用，可以使用备用端口：

```powershell
.\scripts\demo\start-compose.ps1 -ApiPort 18080 -WebPort 14173 -MinioPort 19000 -MinioConsolePort 19001
```

备用端口对应：

- Web 控制台：[http://localhost:14173](http://localhost:14173)
- API 健康检查：[http://localhost:18080/healthz](http://localhost:18080/healthz)
- MinIO 控制台：[http://localhost:19001](http://localhost:19001)

如果使用 Make，也可以通过环境变量指定：

```bash
MINIDROP_API_PORT=18080 MINIDROP_WEB_PORT=14173 MINIDROP_MINIO_PORT=19000 MINIDROP_MINIO_CONSOLE_PORT=19001 make demo
```

## Web 使用流程

1. 打开 Web 控制台并登录。
2. 在“我的机器”确认 `drop_agent` 为 `ONLINE`。
3. 进入“计划任务”或任务创建区域。
4. Windows / Docker 演示选择 `mock-perf`，PID 可使用 `1`。
5. Linux / WSL2 真机采集选择 `perf`、`ebpf-syscall` 或 `py-spy`，PID 使用目标进程 PID。
6. 创建任务后观察状态从 `PENDING` 进入 `RUNNING`、`UPLOADING`，最终为 `DONE` 或 `FAILED`。
7. 打开任务详情查看火焰图、TopN 热点、原始产物下载链接和 AI/规则归因建议。

## 本地开发启动

### Windows 本地 mock 链路

```powershell
.\scripts\demo\start-local.ps1
```

脚本会启动 API、mock target、Agent 和 Web，并打印任务表单里可使用的目标 PID。

停止：

```powershell
.\scripts\demo\stop-local.ps1
```

### Linux / WSL2 本地链路

```bash
make local
```

停止：

```bash
make local-down
```

如果端口被占用：

```bash
API_ADDR=127.0.0.1:18080 WEB_PORT=15173 bash ./scripts/demo/start-local.sh
MINIDROP_API_BASE_URL=http://127.0.0.1:18080 python3 scripts/demo/smoke_e2e.py <printed-pid> agt_local mock-perf
```

## 真实采集器

Windows 和 Docker Desktop 默认走 `mock-perf`，真实采集器建议在 WSL2 Ubuntu 或 Linux 主机上验收。

### perf

安装依赖：

```bash
sudo apt-get update
sudo apt-get install linux-tools-common linux-tools-generic
```

如果内核权限阻止采集，可在演示会话中降低限制：

```bash
sudo sysctl kernel.perf_event_paranoid=1
```

启动并 smoke：

```bash
COLLECTOR_TYPE=perf bash ./scripts/demo/start-local.sh
make smoke-real COLLECTOR_TYPE=perf
```

真实 `perf` 路径会生成：

- `artifacts/<task_id>/raw/perf.data`
- `artifacts/<task_id>/analysis/perf.script.txt`
- `artifacts/<task_id>/analysis/collapsed.txt`
- `artifacts/<task_id>/analysis/flamegraph.svg`
- `artifacts/<task_id>/analysis/topn.json`

### eBPF syscall

安装：

```bash
sudo apt-get update
sudo apt-get install bpftrace
```

检查：

```bash
python3 scripts/demo/check_ebpf_env.py --pid <target-pid>
```

启动并 smoke：

```bash
COLLECTOR_TYPE=ebpf-syscall bash ./scripts/demo/start-local.sh
make smoke-real COLLECTOR_TYPE=ebpf-syscall
```

该采集器使用 bpftrace syscall tracepoint，输出 syscall 分布并在 Web 中展示 eBPF 直方图。

### py-spy

安装：

```bash
python -m pip install py-spy
```

检查：

```bash
python scripts/demo/check_pyspy_env.py --pid <target-pid>
```

启动并 smoke：

```bash
COLLECTOR_TYPE=py-spy bash ./scripts/demo/start-local.sh
make smoke-real COLLECTOR_TYPE=py-spy
```

该采集器面向 Python 进程，输出 Python 用户态调用栈，并在 Web 中展示独立的 Python 栈视图。

### 真实采集器 preflight

统一检查 `perf`、`ebpf-syscall`、`py-spy`：

```bash
make real-preflight
make real-check
make real-smoke-report
```

在 Windows 上，这些脚本会尽量进入 WSL2 检查；如果依赖或权限不满足，会生成 `BLOCKED` 报告，而不是让脚本直接崩溃。报告路径：

- `artifacts/real-collector-preflight.md`
- `artifacts/real-smoke-report.md`

## AI 归因配置

Web 控制台中有“智能分析”页面，可配置 OpenAI-compatible LLM：

- Base URL，例如 `https://api.openai.com/v1`
- API Key
- Model，例如 `gpt-4o-mini`
- Timeout
- 是否启用真实 AI 归因

也可以通过环境变量配置：

```powershell
$env:MINIDROP_AI_ENABLED = "true"
$env:MINIDROP_AI_API_KEY = "<your-key>"
$env:MINIDROP_AI_MODEL = "gpt-4o-mini"
$env:MINIDROP_AI_BASE_URL = "https://api.openai.com/v1"
.\scripts\demo\start-compose.ps1 -ApiPort 18080 -WebPort 14173 -MinioPort 19000 -MinioConsolePort 19001
```

如果没有配置 LLM，或者远程模型超时、返回非法 JSON，系统会自动回退到规则归因，并在结果中标记 `analysis_engine=rule` 和 fallback reason。

## 常用验证命令

### 单测与覆盖率

```bash
make test
make coverage
```

覆盖率报告会写入：

```text
artifacts/coverage-report.md
```

### Compose smoke

```bash
make compose-health
make smoke-demo
make smoke-demo-minio
make smoke-demo-fail
make smoke-demo-offline
```

如果使用备用 API 端口：

```powershell
$env:MINIDROP_API_PORT = "18080"
$env:MINIDROP_API_BASE_URL = "http://127.0.0.1:18080"
python scripts\demo\smoke_compose.py --pid 1 --agent-id drop_agent --expect-minio-url
python scripts\demo\smoke_compose.py --pid 999999 --agent-id drop_agent --expect-status FAILED --expect-reason-contains "target pid not found"
```

### 最终演示验收

```bash
make acceptance-snapshot
make recording-checklist
make submission-notes
make capture-submission-artifacts
make attribution-evaluation
make final-preflight
```

对应产物：

- `artifacts/demo-evidence.md`
- `artifacts/recording-checklist.md`
- `artifacts/submission-notes.md`
- `artifacts/submission-screenshots/`
- `artifacts/attribution-evaluation-report.md`
- `artifacts/final-preflight.md`

## API 概览

认证：

- `POST /api/v1/auth/login`
- `GET /api/v1/auth/me`

核心接口：

- `GET /healthz`
- `GET /api/v1/agents`
- `POST /api/v1/agents/heartbeat`
- `POST /api/v1/tasks`
- `GET /api/v1/tasks`
- `GET /api/v1/tasks/:id`
- `GET /api/v1/tasks/:id/results`
- `GET /api/v1/audit-logs`

Continuous Profiling：

- `GET /api/v1/continuous-profiles`
- `POST /api/v1/continuous-profiles`
- `GET /api/v1/continuous-profiles/:id`
- `GET /api/v1/continuous-profiles/:id/windows`
- `GET /api/v1/continuous-profiles/:id/trends`

AI 配置：

- `GET /api/v1/ai-config`
- `POST /api/v1/ai-config`

Agent 内部接口：

- `GET /api/v1/internal/tasks/claim`
- `POST /api/v1/internal/tasks/:id/uploading`
- `POST /api/v1/internal/tasks/:id/complete`
- `POST /api/v1/internal/tasks/:id/fail`

## 数据与存储

本地开发默认：

- SQLite 数据库：`data/mini-drop.db`
- 原始产物：`artifacts/<task_id>/raw/`
- 分析产物：`artifacts/<task_id>/analysis/`

Docker Compose 默认：

- PostgreSQL 数据卷：`mini_drop_postgres`
- MinIO 数据卷：`mini_drop_minio`
- Agent/API 共享产物卷：`mini_drop_artifacts`

常用存储环境变量：

- `MINIDROP_STORAGE_BACKEND=local|minio`
- `MINIDROP_MINIO_ENDPOINT=minio:9000`
- `MINIDROP_MINIO_PUBLIC_ENDPOINT=http://localhost:9000`
- `MINIDROP_MINIO_BUCKET=mini-drop-artifacts`
- `MINIDROP_MINIO_REGION=us-east-1`
- `MINIDROP_MINIO_PRESIGN_TTL_SEC=900`

## 当前验收状态

已在当前 Windows 开发环境验证：

- PowerShell 本地 mock demo 启停。
- Docker Compose PostgreSQL + MinIO + API + Agent + Web 启动。
- Compose 正常路径：创建任务并进入 `DONE`。
- Compose 异常路径：不存在 PID 进入 `FAILED`，reason 清晰。
- MinIO signed URL 结果下载。
- Agent 离线 smoke。
- Web production build。
- Go API/Agent 单测。
- Python Analyzer 单测。
- 覆盖率报告生成。
- AI 归因评估报告生成。
- 最终 preflight 和提交材料生成脚本。

仍需要在 WSL2 / Linux 主机上完成真实采集器成功验收：

- `make smoke-real COLLECTOR_TYPE=perf`
- `make smoke-real COLLECTOR_TYPE=ebpf-syscall`
- `make smoke-real COLLECTOR_TYPE=py-spy`

当前机器的真实采集器 preflight 可能会因为依赖或权限进入 `BLOCKED`，典型原因包括：

- `perf` 未安装或 `kernel.perf_event_paranoid` 过高。
- `bpftrace` 未安装或 tracefs/debugfs 不可读。
- `py-spy` 未安装或 ptrace 权限不足。

## 文档导航

- [项目题目](docs/references/Mini-Drop-题目.md)
- [Drop 复刻参考](docs/references/drop系统复刻指南.md)
- [项目简报](docs/design/00-project-brief.md)
- [MVP 范围](docs/design/01-mvp-scope.md)
- [系统架构](docs/design/02-architecture.md)
- [状态机与可观测性](docs/design/03-state-machines-and-observability.md)
- [开发计划](docs/design/04-development-plan.md)
- [Backlog](docs/design/05-backlog.md)
- [下一步实现](docs/design/06-next-implementation.md)
- [归因评估](docs/design/07-attribution-evaluation.md)
- [演示 Runbook](docs/demo-runbook.md)
- [最终演示脚本](docs/demo-script.md)
