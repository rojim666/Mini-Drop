# 04. Development Plan

## 开发节奏

Mini-Drop 采用文档驱动和垂直切片开发。每个阶段都要保持一个可运行 demo，而不是先堆模块再最后联调。

核心节奏：

1. 先固定接口、状态机和 artifact 契约。
2. 先跑通最小端到端链路。
3. 再接真实采集器和对象存储。
4. 扩展能力只在主链路稳定后加入。
5. 每个失败路径都必须有可解释 reason 和可演示证据。

## 阶段 1：文档和骨架

目标：

- 整理题目、复刻指南、MVP 范围和架构图。
- 建立 `apps/api-server`、`apps/agent`、`apps/analyzer`、`apps/web`、`deploy`、`scripts/demo`。
- 固定数据模型、状态机和本地 artifact 目录。

当前状态：已完成。

验收证据：

- README 可说明项目目标和启动方式。
- `docs/design` 下有架构、状态机、backlog、下一步计划。
- 根目录 Makefile 和 PowerShell / Bash demo 脚本已存在。

## 阶段 2：Mock 主链路

目标：

```text
Web -> API Server -> Agent -> mock collector -> Analyzer -> Web
```

实现内容：

- Web 创建采样任务。
- API 持久化任务和状态事件。
- Agent 心跳、领取任务、推进状态。
- Mock collector 写 raw artifact。
- Analyzer 生成 `flamegraph.svg` 和 `topn.json`。
- Web 轮询任务状态并展示结果。

当前状态：已完成。

验收证据：

- `scripts/demo/start-local.ps1` 可在 Windows 本地启动 mock demo。
- `scripts/demo/smoke_e2e.py` 可创建任务并等待 `DONE`。
- Web 可以展示 Agent、任务列表、任务详情、火焰图和 TopN。

## 阶段 3：真实 perf 采集器

目标：

在 Linux / WSL2 Ubuntu 中接入真实 `perf record`，保持 Web/API/Artifact 契约不变。

实现内容：

- `collector_type=perf` 走真实 Linux collector。
- 非 Linux 环境直接失败并返回清晰 reason。
- 检查 PID、`perf` 命令、`perf_event_paranoid`。
- 执行 `perf record -F <rate> -g -p <pid> -o perf.data -- sleep <duration>`。
- Analyzer 执行 `perf script`，生成 collapsed stacks、火焰图和 TopN。
- 支持可选标准 FlameGraph Perl 工具，失败时回退到内置解析器。

当前状态：代码已完成，仍需要在具备权限的 WSL2 / Linux 环境做最终 smoke 验证。

验收命令：

```bash
make real-check
COLLECTOR_TYPE=perf bash ./scripts/demo/start-local.sh
make smoke-real COLLECTOR_TYPE=perf
```

## 阶段 4：状态机、心跳和审计

目标：

让演示不仅“能跑”，还可以解释每次状态变化和失败原因。

实现内容：

- 任务状态迁移 helper。
- `task_status_events` 全量记录。
- Agent 5 秒心跳。
- Server 离线扫描。
- Agent 离线和恢复写 `audit_logs`。
- PID 不存在、Agent 离线、采集器缺失等异常路径都有 reason。

当前状态：已完成。

验收命令：

```bash
make smoke-demo-fail
make smoke-demo-offline
go test ./apps/api-server ./apps/agent ./internal/...
```

## 阶段 5：交付底座

目标：

满足 `docker compose up` 一键启动和对象存储下载的交付基线。

实现内容：

- Compose 启动 PostgreSQL、MinIO、API、Agent、Web、demo target。
- API 支持 PostgreSQL。
- API 支持 MinIO 上传和签名 URL。
- PowerShell 和 Bash 脚本分别覆盖 Windows / Linux 启动路径。
- README 和 runbook 说明端口、账号、PID、常见失败。

当前状态：已完成 Windows compose mock 路径。

验收命令：

```powershell
.\scripts\demo\start-compose.ps1
python scripts\demo\smoke_compose.py --pid 1 --agent-id agt_compose --expect-minio-url
```

## 阶段 6：eBPF 扩展

目标：

实现一个真实可演示的内核态采集器。

实现内容：

- `collector_type=ebpf-syscall`。
- Linux 下使用 `bpftrace` 统计目标 PID 的 syscall 分布。
- Demo workload 使用 `dd` 循环制造明显 syscall 变化。
- Analyzer 将 syscall 分布转成 TopN 和 SVG。
- Web 增加 `eBPF 分布` 展示。

当前状态：代码已完成，仍需要具备 `bpftrace` 和 tracefs 权限的 WSL2 / Linux 环境最终验证。

验收命令：

```bash
python3 scripts/demo/check_ebpf_env.py --pid <target-pid>
COLLECTOR_TYPE=ebpf-syscall bash ./scripts/demo/start-local.sh
make smoke-real COLLECTOR_TYPE=ebpf-syscall
```

## 阶段 7：用户态语言采集器

目标：

实现一个与 `perf` 语义不同的用户态语言采集器。

实现内容：

- `collector_type=py-spy`。
- 使用 `py-spy record --format raw` 采集 Python 进程。
- Analyzer 解析 Python 栈样本。
- Web 增加 `Python 栈` 展示。

当前状态：代码已完成，仍需要安装 `py-spy` 并在 Linux / WSL2 上验证 attach 权限。

验收命令：

```bash
python3 scripts/demo/check_pyspy_env.py --pid <python-target-pid>
COLLECTOR_TYPE=py-spy bash ./scripts/demo/start-local.sh
make smoke-real COLLECTOR_TYPE=py-spy
```

## 阶段 8：Continuous Profiling

目标：

实现最小持续 profiling：固定窗口、低频采样、结果可回溯。

实现内容：

- Web `计划任务` 页面创建 continuous profile。
- API 创建 profile 和 window。
- 每个到期 window materialize 为普通 task。
- 结果复用原有 Agent、Analyzer、火焰图、TopN 和归因展示。
- API 返回最近 24 个窗口的聚合摘要：总数、完成数、失败数、活跃数、等待数、最新状态和完成率。
- Web 在窗口时间轴表格上方展示摘要，便于 demo 评审快速判断持续采样健康度。
- API 支持按 `status`、`from`、`to`、`limit` 筛选窗口列表，Web 提供状态、时间范围和显示数量筛选。
- Web 提供可点击窗口时间轴条，点击任一窗口可回溯到其 materialized task 详情。
- API 聚合最近完成窗口的 TopN 热点趋势，Web 在计划页展示跨窗口函数占比、峰值和变化量。
- API 为趋势结果补充确定性自动标注与原因，Web 直接标出持续高位、基线偏高、升高、回落、小幅波动或平稳。
- API 为趋势结果接入 seeded baseline 对比，Web 直接展示基线偏差。
- API 和 Web 支持 continuous profile 启用/停用，并写入审计日志，停用后不再调度新窗口。
- API 对未指定目标机器的任务和 continuous profile 使用最少活跃任务优先的自动 Agent 调度策略。
- API 和 Web 支持 interval / cron 两种 continuous profile 调度方式，并支持秒级错峰窗口，Agent 只领取已到 `window_start_at` 的窗口任务。

当前状态：已完成最小版，并补齐窗口聚合摘要、筛选、可点击时间轴、跨窗口热点趋势、细粒度自动标注、baseline 对比、profile 生命周期控制、cron 表达式和错峰窗口策略。
任务对比页已补充最小跨任务热点聚合视图，用于查看已完成任务中的重复热点、覆盖任务数、平均占比和峰值。
任务对比页已补充最小跨 profile 聚合视图，用于区分跨连续计划反复出现的热点和单计划异常。
调度策略已补齐最小版：自动目标选择会优先分配给活跃任务数更少的在线 Agent。
调度表达式当前由内置轻量解析器支持常见五字段 cron、`@every` 和错峰秒数；后续如需完整生产级 cron 方言，可替换为专用 cron 库并保留现有 API 字段。

## 阶段 9：智能归因

目标：

做一个可验证、可审计的小闭环，而不是直接让 LLM 编结论。

实现内容：

- Analyzer / API 读取 TopN。
- 匹配热点规则。
- 与 baseline 样例对比。
- 输出 conclusion、confidence、evidence、recommendations、source、tool trace。
- Web `归因建议` tab 展示完整证据链。

当前状态：规则驱动版本已完成。

下一步增强：

- 接入真实资源时间线。
- 接入远程 LLM，但只允许它调用结构化工具。
- 扩充评测样例和评分报告。

## 当前下一步

当前项目主线已经从“实现功能”进入“交付验证和演示收口”：

1. 在 WSL2 / Linux 中安装真实采集器依赖，运行 `make real-check`。
2. 跑通至少 `perf` 的真实 smoke：`make smoke-real COLLECTOR_TYPE=perf`。
3. 尽量跑通 `ebpf-syscall` 和 `py-spy` 的真实 smoke。
4. 继续打磨 Web 控制台风格和演示路径。
5. 按 `docs/demo-script.md` 录制最终演示，并补截图和提交说明。
6. 录制前运行 `make final-preflight`，用 `artifacts/final-preflight.md` 汇总静态检查、自动测试、验收快照和交付材料生成状态。

## 两周交付排期

| Day | 目标 | 当前状态 |
|---|---|---|
| D1 | 文档、目录、数据表、API 草案 | 完成 |
| D2 | API Server、DB migrate、Web 骨架 | 完成 |
| D3 | Agent 心跳和任务创建 mock | 完成 |
| D4 | mock 端到端链路 | 完成 |
| D5 | 接入真实 perf | 代码完成，待 Linux 验证 |
| D6 | Analyzer 生成火焰图 | 完成 |
| D7 | Web 展示火焰图和 TopN | 完成 |
| D8 | 状态机和状态历史 | 完成 |
| D9 | Agent 离线/恢复和审计日志 | 完成 |
| D10 | 异常路径和集成测试 | 完成 |
| D11 | eBPF 采集器 | 代码完成，待 Linux 验证 |
| D12 | Continuous Profiling 最小版 | 完成 |
| D13 | 用户态采集器和智能归因 | 完成最小版 |
| D14 | README、runbook、演示脚本、录屏准备 | 演示脚本、录制清单、提交说明模板和 final preflight 已补，最终人工录屏待完成 |

## Commit 规则

每个 commit 要解释“为什么”，避免没有信息量的提交。

推荐：

- `docs: align demo runbook with compose scripts`
- `agent: add linux perf collector preflight`
- `analyzer: parse perf script into collapsed stacks`
- `web: show attribution evidence in task detail`
- `deploy: serve artifacts through minio signed urls`

避免：

- `update`
- `fix`
- `wip`
- `change`
