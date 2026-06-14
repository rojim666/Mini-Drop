# 06. Next Implementation

## 下一步只做一件事

实现 mock 端到端链路。

目标不是马上接 `perf`，而是先证明系统能从 Web/接口层一路走到 Analyzer，并把结果展示回来。

## Mock 链路定义

```text
POST /api/v1/tasks
  -> create task PENDING
  -> mock agent marks RUNNING
  -> mock agent writes fake artifact
  -> mock analyzer writes fake flamegraph.svg and topn.json
  -> task becomes DONE
  -> Web can read and display result
```

## 为什么先 mock

真实 `perf`、eBPF、容器权限、Linux 内核参数都会带来环境变量。如果第一天就碰这些，容易被底层问题卡住，看不到系统整体。

Mock 链路先把产品骨架跑起来，之后替换采集器即可。

## 第一批代码任务

1. API Server:
   - `/healthz`
   - `POST /api/v1/tasks`
   - `GET /api/v1/tasks`
   - `GET /api/v1/tasks/:id`
   - 状态迁移 helper

2. Agent:
   - 心跳接口先 mock。
   - 能把新任务推进到 `DONE`。

3. Analyzer:
   - 生成一个固定 SVG。
   - 生成一个固定 TopN JSON。

4. Web:
   - 任务表单。
   - 任务列表。
   - 任务详情。

## 验收口径

一次演示能完成：

1. 创建任务。
2. 看到任务状态变化。
3. 打开任务详情。
4. 看到假火焰图。
5. 看到假 TopN 热点。

