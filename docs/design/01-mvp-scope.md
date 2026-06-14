# 01. MVP Scope

## 范围原则

MVP 的目标不是覆盖题目里的每一个高级能力，而是做出一个可复现、可演示、可解释的 Mini-Drop。

优先级排序：

1. 主链路必须通。
2. 状态必须可见。
3. 失败必须可解释。
4. 演示必须稳定。
5. 扩展点必须真实，不做空壳。

## 必做能力

### Web UI

- Agent 列表页：显示 Agent 在线/离线状态。
- 任务创建表单：输入 PID、采样时长、采样率。
- 任务列表页：显示任务状态、创建时间、目标 PID。
- 任务详情页：展示状态历史、火焰图、TopN 热点。

### API Server

- 提供 REST API 给 Web UI。
- 任务创建、查询、列表、详情。
- 状态迁移落库，每次迁移带 `reason`。
- Agent 心跳登记。
- Agent 30 秒无心跳判离线。
- 离线/恢复写审计日志。

### Agent

- 启动后定期向 Server 心跳。
- 拉取或接收任务。
- 对目标 PID 执行 `perf` 采样。
- 采集结束后上传产物到对象存储或本地存储。
- 上报任务结果。

### Analyzer

- 接收任务 ID 和原始数据路径。
- 将 `perf.data` 或折叠栈转换为火焰图 SVG。
- 生成 TopN 热点 JSON。
- 写回分析结果路径和状态。

### Storage

- MVP 使用 MinIO 或本地文件目录。
- 存放原始采集数据、火焰图 SVG、TopN JSON。

### Tests

- 至少 1 条正常端到端路径。
- 至少 2 条异常路径：
  - PID 不存在。
  - Agent 离线或采集超时。

## 扩展能力选择

### 第一扩展：eBPF 采集器

目标：使用 `bpftrace` 或 `bcc` 实现一个真实可演示的 IO 或调度观测。

推荐最小方案：

- 用 `bpftrace` 观测 block IO latency 或 syscall 分布。
- 演示时用 `dd` / `fio` / `stress-ng` 制造变化。
- Web 上展示一个简单分布图或时间序列。

### 第二扩展：用户态语言级采集器

推荐顺序：

1. `py-spy`：适合 Python demo，接入相对简单。
2. `pprof HTTP`：适合 Go demo 服务。
3. `async-profiler`：适合 Java，但环境复杂度更高。

MVP 推荐选 `py-spy` 或 `pprof HTTP`。

### 第三扩展：Continuous Profiling

最小版定义：

- Agent 以低频定时采集。
- 每段数据按时间窗口存储。
- Web 上可以选择最近某个 5 分钟窗口查看结果。

不要一开始做复杂的时序检索和聚合，先做固定窗口。

## 加分项选择

优先选择智能归因，但只做可验证的小闭环。

### 智能归因最小版

输入：

- TopN 热点函数。
- 采样参数。
- 资源曲线。
- 历史 baseline，可先用手工构造数据。

约束：

- LLM 不能直接编造结论。
- LLM 只能调用我们定义的工具，例如：
  - `get_top_hotspots(task_id)`
  - `compare_with_baseline(task_id)`
  - `get_resource_timeline(task_id)`

输出：

- 归因结论。
- 证据列表。
- 置信度。
- 可复核的指标来源。

## 暂不做或后做

- 完整用户/组权限系统。
- 多租户。
- 复杂对象存储权限。
- 自研火焰图组件。
- 大规模任务调度。
- Java HPROF 深度分析。
- 多 Agent 跨机器高级调度。

这些内容可以写进“如果再有 7 天我会做什么”。

