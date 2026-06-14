# 04. Development Plan

## 开发节奏

这不是先写一堆模块再联调的项目。正确节奏是：

1. 先定义接口和数据结构。
2. 写最薄的端到端链路。
3. 每天保持一个可运行 demo。
4. 扩展能力只在主链路稳定后加。

## 第一阶段：文档和骨架

目标：

- 完成 docs。
- 确定 MVP 范围。
- 确定接口和数据表。
- 搭建仓库目录。

验收：

- README 能说明项目是什么。
- 设计文档能指导开发。
- 后续每个功能都能找到对应文档依据。

## 第二阶段：最小主链路

目标：

`Web -> API -> Agent -> mock collector -> Analyzer -> Web`

先不用真实 `perf`，用 mock artifact 跑通流程。

验收：

- Web 可以创建任务。
- Server 可以保存任务。
- Agent 可以拿到任务。
- Analyzer 可以生成一个假 SVG。
- Web 可以展示结果。

## 第三阶段：接入 perf

目标：

用真实 `perf` 替换 mock collector。

验收：

- 输入真实 PID。
- Agent 生成真实采集产物。
- Analyzer 生成火焰图 SVG。
- Web 能展示火焰图。

## 第四阶段：状态机、心跳、审计

目标：

- 实现任务状态机。
- 实现 Agent 心跳。
- 实现离线判定。
- 实现审计日志。

验收：

- 任务每次状态迁移都落库。
- Web 可见状态历史。
- 停掉 Agent 后 30 秒内显示离线。
- Agent 恢复后写恢复审计日志。

## 第五阶段：eBPF 扩展

目标：

实现一个真实可演示的 eBPF 采集器。

推荐路径：

- 先用 `bpftrace`。
- 采集 block IO 或调度延迟。
- 用 `dd` / `fio` / `stress-ng` 制造变化。

验收：

- 演示视频中能现场制造异常。
- Web 上能看到 eBPF 数据变化。

## 第六阶段：用户态语言级采集器

目标：

接入 `py-spy` 或 `pprof HTTP`。

验收：

- 有一个 demo Python 或 Go 服务。
- 采集器能生成不同于 perf 的结果。
- Web 上有独立展示形态。

## 第七阶段：Continuous Profiling

目标：

做最小版持续 profiling。

验收：

- Agent 定时低频采样。
- 结果按时间窗口保存。
- Web 能选择一个固定 5 分钟窗口查看。

## 第八阶段：智能归因

目标：

做一个可验证的 LLM 归因小闭环。

验收：

- LLM 输入来自结构化工具结果。
- 输出包含证据。
- 至少有 3 个评测样例。

## 两周建议排期

| Day | 目标 |
|---|---|
| D1 | 文档、目录、数据表、API 草案 |
| D2 | API Server 骨架、DB migrate、Web 骨架 |
| D3 | Agent 心跳 mock、任务创建 mock |
| D4 | mock 端到端链路跑通 |
| D5 | 接入真实 perf |
| D6 | Analyzer 生成火焰图 |
| D7 | Web 展示火焰图，第一次端到端验收 |
| D8 | 状态机和状态历史 |
| D9 | Agent 离线/恢复和审计日志 |
| D10 | 异常路径和集成测试 |
| D11 | eBPF 采集器 |
| D12 | Continuous Profiling 最小版 |
| D13 | 智能归因或用户态采集器 |
| D14 | 文档、README、演示脚本、录屏 |

## Commit 规则

每个 commit 要解释“为什么”：

- `docs: define MVP scope and architecture`
- `api: persist task status events`
- `agent: add heartbeat loop for offline detection`
- `analyzer: generate flamegraph svg from perf output`
- `web: show task status history`

避免：

- `update`
- `fix`
- `wip`
- `change`

## 当前下一步

在代码层面，下一步应该创建项目骨架：

```text
apps/
  api-server/
  web/
  agent/
  analyzer/
deploy/
  docker-compose.yml
scripts/
  demo/
```

然后先实现 mock 端到端链路。
