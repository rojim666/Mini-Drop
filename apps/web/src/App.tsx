import { useEffect, useMemo, useState } from "react";
import {
  AlertCircle,
  BarChart3,
  Bell,
  ChevronDown,
  Cloud,
  ExternalLink,
  FileSearch,
  History,
  LayoutDashboard,
  Loader2,
  Monitor,
  Play,
  RefreshCw,
  Search,
  Server,
  Settings,
  X,
} from "lucide-react";

import { createTask, getAgents, getAuditLogs, getTask, getTasks } from "./api";
import type { Agent, AuditLog, CreateTaskInput, Task } from "./types";
import "./App.css";

type NavKey = "home" | "quick" | "machines" | "history" | "files" | "schedule" | "compare";

const navItems: Array<{ key: NavKey; label: string; icon: React.ReactNode }> = [
  { key: "home", label: "首页", icon: <LayoutDashboard size={16} /> },
  { key: "quick", label: "快速接入", icon: <Play size={16} /> },
  { key: "machines", label: "机器列表", icon: <Server size={16} /> },
  { key: "history", label: "历史任务", icon: <History size={16} /> },
  { key: "files", label: "文件分析", icon: <FileSearch size={16} /> },
  { key: "schedule", label: "计划任务", icon: <Settings size={16} /> },
  { key: "compare", label: "任务对比", icon: <BarChart3 size={16} /> },
];

const defaultTaskInput: CreateTaskInput = {
  target_pid: 1,
  sample_duration_sec: 15,
  sample_rate_hz: 99,
  collector_type: "mock-perf",
};

function App() {
  const [activeNav, setActiveNav] = useState<NavKey>("home");
  const [agents, setAgents] = useState<Agent[]>([]);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);
  const [selectedTask, setSelectedTask] = useState<Task | null>(null);
  const [taskInput, setTaskInput] = useState<CreateTaskInput>(defaultTaskInput);
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    void refreshOverview(true);
  }, []);

  useEffect(() => {
    const interval = window.setInterval(() => {
      void refreshOverview(false);
    }, 3000);
    return () => window.clearInterval(interval);
  }, []);

  useEffect(() => {
    if (!selectedTaskId) {
      setSelectedTask(null);
      return;
    }

    let alive = true;
    const loadDetail = async () => {
      try {
        const data = await getTask(selectedTaskId);
        if (alive) {
          setSelectedTask(data.task);
        }
      } catch (detailError) {
        if (alive) {
          setError((detailError as Error).message);
        }
      }
    };

    void loadDetail();
    const interval = window.setInterval(loadDetail, 3000);
    return () => {
      alive = false;
      window.clearInterval(interval);
    };
  }, [selectedTaskId]);

  useEffect(() => {
    if (!selectedTaskId && tasks.length > 0) {
      setSelectedTaskId(tasks[0].id);
    }
  }, [tasks, selectedTaskId]);

  async function refreshOverview(initial = false) {
    try {
      if (initial) {
        setLoading(true);
      } else {
        setRefreshing(true);
      }

      const [agentData, taskData, auditData] = await Promise.all([getAgents(), getTasks(), getAuditLogs()]);
      setAgents(agentData.agents);
      setTasks(taskData.tasks);
      setAuditLogs(auditData.audit_logs);
      setError(null);
    } catch (overviewError) {
      setError((overviewError as Error).message);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }

  async function handleCreateTask(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    try {
      setSubmitting(true);
      const payload = {
        ...taskInput,
        target_agent_id: taskInput.target_agent_id || undefined,
      };
      const response = await createTask(payload);
      setSelectedTaskId(response.task.id);
      setActiveNav("history");
      setCreateDialogOpen(false);
      await refreshOverview();
    } catch (submitError) {
      setError((submitError as Error).message);
    } finally {
      setSubmitting(false);
    }
  }

  const stats = useMemo(() => {
    const onlineAgents = agents.filter((agent) => agent.status === "ONLINE").length;
    const failedTasks = tasks.filter((task) => task.status === "FAILED").length;
    const runningTasks = tasks.filter((task) => task.status === "RUNNING" || task.status === "UPLOADING").length;
    const doneTasks = tasks.filter((task) => task.status === "DONE").length;
    return { onlineAgents, failedTasks, runningTasks, doneTasks };
  }, [agents, tasks]);

  const currentTitle = navItems.find((item) => item.key === activeNav)?.label ?? "首页";
  const showDropLandingLoader = activeNav === "home" && loading && agents.length === 0 && tasks.length === 0;

  return (
    <div className="drop-app">
      <TopNav activeNav={activeNav} setActiveNav={setActiveNav} />

      <main className="console-shell">
        {showDropLandingLoader ? (
          <DropLandingLoader />
        ) : (
          <>
            <div className="breadcrumb">Drop 雨滴 / {currentTitle}</div>

            {error ? (
              <div className="console-alert">
                <AlertCircle size={16} />
                <span>{error}</span>
              </div>
            ) : null}

            {activeNav === "home" ? (
              <HomePage
                agents={agents}
                tasks={tasks}
                stats={stats}
                loading={loading}
                refreshing={refreshing}
                onRefresh={() => void refreshOverview()}
                onCreate={() => setCreateDialogOpen(true)}
                onOpenMachines={() => setActiveNav("machines")}
                onOpenHistory={() => setActiveNav("history")}
              />
            ) : null}

            {activeNav === "quick" ? (
              <QuickAccessPage
                agents={agents}
                taskInput={taskInput}
                setTaskInput={setTaskInput}
                submitting={submitting}
                onSubmit={handleCreateTask}
              />
            ) : null}

            {activeNav === "machines" ? <MachinesPage agents={agents} auditLogs={auditLogs} /> : null}

            {activeNav === "history" ? (
              <HistoryPage
                tasks={tasks}
                selectedTask={selectedTask}
                selectedTaskId={selectedTaskId}
                setSelectedTaskId={setSelectedTaskId}
                onCreate={() => setCreateDialogOpen(true)}
              />
            ) : null}

            {activeNav === "files" ? <FilesPage selectedTask={selectedTask} tasks={tasks} /> : null}
            {activeNav === "schedule" ? (
              <PlaceholderPage title="计划任务" description="持续 Profiling 和定时采样将在下一阶段接入。" />
            ) : null}
            {activeNav === "compare" ? (
              <PlaceholderPage title="任务对比" description="后续用于对比两个任务的火焰图、TopN 热点和资源曲线。" />
            ) : null}
          </>
        )}
      </main>

      <footer className="console-footer">
        <div>Copyright © Tencent Cloud. All Rights Reserved. 腾讯云 版权所有</div>
        <div>负责团队 @ CSIG质量部 - 专项性能工程中心</div>
      </footer>

      {createDialogOpen ? (
        <CreateTaskDialog
          agents={agents}
          taskInput={taskInput}
          setTaskInput={setTaskInput}
          submitting={submitting}
          onSubmit={handleCreateTask}
          onClose={() => setCreateDialogOpen(false)}
        />
      ) : null}
    </div>
  );
}

function DropLandingLoader() {
  return (
    <section className="drop-loading-screen">
      <div className="drop-spinner" />
      <span>加载中...</span>
    </section>
  );
}

function TopNav({
  activeNav,
  setActiveNav,
}: {
  activeNav: NavKey;
  setActiveNav: (key: NavKey) => void;
}) {
  return (
    <header className="top-nav">
      <button className="brand" type="button" onClick={() => setActiveNav("home")}>
        <span className="brand-mark">
          <Cloud size={28} />
        </span>
        <span className="brand-text">Drop 雨滴</span>
      </button>

      <nav className="primary-nav">
        {navItems.map((item) => (
          <button
            key={item.key}
            type="button"
            className={activeNav === item.key ? "active" : ""}
            onClick={() => setActiveNav(item.key)}
          >
            <span className="nav-icon">{item.icon}</span>
            {item.label}
          </button>
        ))}
      </nav>

      <div className="top-actions">
        <button type="button">
          进群咨询
          <ExternalLink size={13} />
        </button>
        <button type="button">用户组</button>
        <button type="button" className="login-button">
          未登录
          <ChevronDown size={14} />
        </button>
      </div>
    </header>
  );
}

function HomePage({
  agents,
  tasks,
  stats,
  loading,
  refreshing,
  onRefresh,
  onCreate,
  onOpenMachines,
  onOpenHistory,
}: {
  agents: Agent[];
  tasks: Task[];
  stats: { onlineAgents: number; failedTasks: number; runningTasks: number; doneTasks: number };
  loading: boolean;
  refreshing: boolean;
  onRefresh: () => void;
  onCreate: () => void;
  onOpenMachines: () => void;
  onOpenHistory: () => void;
}) {
  return (
    <div className="page-stack">
      <section className="page-title-row">
        <div>
          <h1>首页</h1>
          <p>按需采集 Linux 进程性能数据，生成火焰图、热点函数和状态审计。</p>
        </div>
        <div className="title-actions">
          <button className="secondary-button" type="button" onClick={onRefresh} disabled={refreshing}>
            <RefreshCw size={15} className={refreshing ? "spin" : ""} />
            刷新
          </button>
          <button className="primary-button" type="button" onClick={onCreate}>
            新建采样
          </button>
        </div>
      </section>

      <section className="metric-grid">
        <MetricCard label="在线机器" value={stats.onlineAgents} suffix={`/${agents.length}`} tone="blue" />
        <MetricCard label="执行中任务" value={stats.runningTasks} tone="orange" />
        <MetricCard label="完成任务" value={stats.doneTasks} tone="green" />
        <MetricCard label="失败任务" value={stats.failedTasks} tone="red" />
      </section>

      <section className="content-grid">
        <div className="console-card">
          <CardHeader title="我的机器" action="查看全部" onAction={onOpenMachines} />
          {loading ? <LoadingBlock /> : <AgentTable agents={agents.slice(0, 5)} />}
        </div>
        <div className="console-card">
          <CardHeader title="最近任务" action="查看历史" onAction={onOpenHistory} />
          {loading ? <LoadingBlock /> : <TaskTable tasks={tasks.slice(0, 6)} onSelect={() => onOpenHistory()} />}
        </div>
      </section>
    </div>
  );
}

function QuickAccessPage({
  agents,
  taskInput,
  setTaskInput,
  submitting,
  onSubmit,
}: {
  agents: Agent[];
  taskInput: CreateTaskInput;
  setTaskInput: React.Dispatch<React.SetStateAction<CreateTaskInput>>;
  submitting: boolean;
  onSubmit: (event: React.FormEvent<HTMLFormElement>) => void;
}) {
  return (
    <div className="page-stack">
      <section className="page-title-row">
        <div>
          <h1>快速接入</h1>
          <p>选择已在线 Agent，输入目标 PID 和采样参数，快速创建一次 CPU 采样任务。</p>
        </div>
      </section>
      <div className="console-card form-card">
        <TaskForm agents={agents} taskInput={taskInput} setTaskInput={setTaskInput} submitting={submitting} onSubmit={onSubmit} />
      </div>
    </div>
  );
}

function MachinesPage({ agents, auditLogs }: { agents: Agent[]; auditLogs: AuditLog[] }) {
  return (
    <div className="page-stack">
      <section className="page-title-row">
        <div>
          <h1>机器列表</h1>
          <p>Agent 每 5 秒心跳一次，超过 30 秒未上报会被标记为离线。</p>
        </div>
      </section>
      <div className="console-card">
        <CardHeader title="Agent 状态" />
        <AgentTable agents={agents} />
      </div>
      <div className="console-card">
        <CardHeader title="审计日志" />
        <AuditTable auditLogs={auditLogs} />
      </div>
    </div>
  );
}

function HistoryPage({
  tasks,
  selectedTask,
  selectedTaskId,
  setSelectedTaskId,
  onCreate,
}: {
  tasks: Task[];
  selectedTask: Task | null;
  selectedTaskId: string | null;
  setSelectedTaskId: (taskId: string) => void;
  onCreate: () => void;
}) {
  return (
    <div className="page-stack">
      <section className="page-title-row">
        <div>
          <h1>历史任务</h1>
          <p>查看任务状态机、状态变更原因、分析产物和 TopN 热点。</p>
        </div>
        <button className="primary-button" type="button" onClick={onCreate}>
          新建采样
        </button>
      </section>

      <div className="console-card">
        <div className="toolbar-row">
          <div className="search-box">
            <Search size={15} />
            <span>按任务 ID / PID 搜索</span>
          </div>
          <button className="secondary-button" type="button">
            导出
          </button>
        </div>
        <TaskTable tasks={tasks} selectedTaskId={selectedTaskId} onSelect={setSelectedTaskId} />
      </div>

      <TaskDetail task={selectedTask} />
    </div>
  );
}

function FilesPage({ selectedTask, tasks }: { selectedTask: Task | null; tasks: Task[] }) {
  const doneTasks = tasks.filter((task) => task.status === "DONE");
  return (
    <div className="page-stack">
      <section className="page-title-row">
        <div>
          <h1>文件分析</h1>
          <p>集中查看原始采集文件、火焰图 SVG 和热点 JSON。</p>
        </div>
      </section>
      <div className="console-card">
        <CardHeader title="产物列表" />
        <table className="data-table">
          <thead>
            <tr>
              <th>任务 ID</th>
              <th>原始数据</th>
              <th>火焰图</th>
              <th>TopN</th>
            </tr>
          </thead>
          <tbody>
            {doneTasks.length === 0 ? (
              <tr>
                <td colSpan={4}>
                  <EmptyBlock text="暂无已完成分析产物。" />
                </td>
              </tr>
            ) : (
              doneTasks.map((task) => (
                <tr key={task.id}>
                  <td>{task.id}</td>
                  <td>{task.raw_artifact_url ? <ArtifactLink href={task.raw_artifact_url} label="raw" /> : "-"}</td>
                  <td>
                    {task.id === selectedTask?.id && selectedTask.result?.flamegraph_url ? (
                      <ArtifactLink href={selectedTask.result.flamegraph_url} label="flamegraph.svg" />
                    ) : (
                      "打开历史任务查看"
                    )}
                  </td>
                  <td>
                    {task.id === selectedTask?.id && selectedTask.result?.topn_url ? (
                      <ArtifactLink href={selectedTask.result.topn_url} label="topn.json" />
                    ) : (
                      "打开历史任务查看"
                    )}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function PlaceholderPage({ title, description }: { title: string; description: string }) {
  return (
    <div className="page-stack">
      <section className="page-title-row">
        <div>
          <h1>{title}</h1>
          <p>{description}</p>
        </div>
      </section>
      <div className="console-card placeholder-card">
        <Bell size={32} />
        <strong>功能规划中</strong>
        <span>当前 demo 先完成按需采样主链路，后续会逐步补齐该模块。</span>
      </div>
    </div>
  );
}

function CreateTaskDialog(props: {
  agents: Agent[];
  taskInput: CreateTaskInput;
  setTaskInput: React.Dispatch<React.SetStateAction<CreateTaskInput>>;
  submitting: boolean;
  onSubmit: (event: React.FormEvent<HTMLFormElement>) => void;
  onClose: () => void;
}) {
  return (
    <div className="modal-backdrop">
      <section className="modal-card">
        <div className="modal-title">
          <div>
            <h2>新建采样</h2>
            <p>创建任务后，Agent 会领取任务并推进状态机。</p>
          </div>
          <button type="button" className="icon-button" onClick={props.onClose} aria-label="关闭">
            <X size={18} />
          </button>
        </div>
        <TaskForm {...props} />
      </section>
    </div>
  );
}

function TaskForm({
  agents,
  taskInput,
  setTaskInput,
  submitting,
  onSubmit,
}: {
  agents: Agent[];
  taskInput: CreateTaskInput;
  setTaskInput: React.Dispatch<React.SetStateAction<CreateTaskInput>>;
  submitting: boolean;
  onSubmit: (event: React.FormEvent<HTMLFormElement>) => void;
}) {
  return (
    <form className="drop-form" onSubmit={onSubmit}>
      <label>
        <span>目标 PID</span>
        <input
          type="number"
          min={1}
          value={taskInput.target_pid}
          onChange={(event) => setTaskInput((current) => ({ ...current, target_pid: Number(event.target.value) }))}
        />
      </label>

      <label>
        <span>目标机器</span>
        <select
          value={taskInput.target_agent_id ?? ""}
          onChange={(event) =>
            setTaskInput((current) => ({ ...current, target_agent_id: event.target.value || undefined }))
          }
        >
          <option value="">自动选择在线机器</option>
          {agents.map((agent) => (
            <option key={agent.id} value={agent.id}>
              {agent.hostname} / {agent.ip} / {agent.status}
            </option>
          ))}
        </select>
      </label>

      <div className="form-line">
        <label>
          <span>采样时长（秒）</span>
          <input
            type="number"
            min={1}
            max={300}
            value={taskInput.sample_duration_sec}
            onChange={(event) =>
              setTaskInput((current) => ({ ...current, sample_duration_sec: Number(event.target.value) }))
            }
          />
        </label>
        <label>
          <span>采样频率（Hz）</span>
          <input
            type="number"
            min={1}
            max={999}
            value={taskInput.sample_rate_hz}
            onChange={(event) => setTaskInput((current) => ({ ...current, sample_rate_hz: Number(event.target.value) }))}
          />
        </label>
      </div>

      <label>
        <span>采集器类型</span>
        <select
          value={taskInput.collector_type}
          onChange={(event) => setTaskInput((current) => ({ ...current, collector_type: event.target.value }))}
        >
          <option value="mock-perf">mock-perf</option>
          <option value="perf" disabled>
            perf（下一阶段）
          </option>
          <option value="ebpf" disabled>
            eBPF（规划中）
          </option>
        </select>
      </label>

      <div className="form-actions">
        <button className="primary-button" type="submit" disabled={submitting}>
          {submitting ? "提交中..." : "提交任务"}
        </button>
      </div>
    </form>
  );
}

function MetricCard({ label, value, suffix, tone }: { label: string; value: number; suffix?: string; tone: string }) {
  return (
    <div className={`metric-card ${tone}`}>
      <span>{label}</span>
      <strong>
        {value}
        {suffix ? <em>{suffix}</em> : null}
      </strong>
    </div>
  );
}

function CardHeader({ title, action, onAction }: { title: string; action?: string; onAction?: () => void }) {
  return (
    <div className="card-header">
      <h2>{title}</h2>
      {action ? (
        <button type="button" onClick={onAction}>
          {action}
        </button>
      ) : null}
    </div>
  );
}

function AgentTable({ agents }: { agents: Agent[] }) {
  return (
    <table className="data-table">
      <thead>
        <tr>
          <th>机器名</th>
          <th>IP</th>
          <th>版本</th>
          <th>状态</th>
          <th>最近心跳</th>
        </tr>
      </thead>
      <tbody>
        {agents.length === 0 ? (
          <tr>
            <td colSpan={5}>
              <EmptyBlock text="暂无 Agent 心跳，请先启动 agent 服务。" />
            </td>
          </tr>
        ) : (
          agents.map((agent) => (
            <tr key={agent.id}>
              <td>
                <div className="name-cell">
                  <Monitor size={15} />
                  <div>
                    <strong>{agent.hostname}</strong>
                    <span>{agent.id}</span>
                  </div>
                </div>
              </td>
              <td>{agent.ip}</td>
              <td>{agent.version}</td>
              <td>
                <StatusTag value={agent.status} />
              </td>
              <td>{formatDate(agent.last_heartbeat_at)}</td>
            </tr>
          ))
        )}
      </tbody>
    </table>
  );
}

function TaskTable({
  tasks,
  selectedTaskId,
  onSelect,
}: {
  tasks: Task[];
  selectedTaskId?: string | null;
  onSelect: (taskId: string) => void;
}) {
  return (
    <table className="data-table">
      <thead>
        <tr>
          <th>任务 ID</th>
          <th>目标 PID</th>
          <th>机器</th>
          <th>采集器</th>
          <th>状态</th>
          <th>更新时间</th>
        </tr>
      </thead>
      <tbody>
        {tasks.length === 0 ? (
          <tr>
            <td colSpan={6}>
              <EmptyBlock text="暂无任务，请点击新建采样。" />
            </td>
          </tr>
        ) : (
          tasks.map((task) => (
            <tr
              key={task.id}
              className={selectedTaskId === task.id ? "selected-row" : ""}
              onClick={() => onSelect(task.id)}
            >
              <td className="link-cell">{task.id}</td>
              <td>{task.target_pid}</td>
              <td>{task.target_agent_id}</td>
              <td>{task.collector_type}</td>
              <td>
                <StatusTag value={task.status} />
              </td>
              <td>{formatDate(task.updated_at)}</td>
            </tr>
          ))
        )}
      </tbody>
    </table>
  );
}

function TaskDetail({ task }: { task: Task | null }) {
  const [activeTab, setActiveTab] = useState<"basic" | "flame" | "hotspots">("basic");

  if (!task) {
    return (
      <div className="console-card">
        <EmptyBlock text="选择一个任务查看详情。" />
      </div>
    );
  }

  return (
    <div className="console-card task-detail-card">
      <div className="detail-head">
        <div>
          <h2>任务详情</h2>
          <p>{task.id}</p>
        </div>
        <StatusTag value={task.status} />
      </div>

      <div className="tabs">
        <button className={activeTab === "basic" ? "active" : ""} type="button" onClick={() => setActiveTab("basic")}>
          基本信息
        </button>
        <button className={activeTab === "flame" ? "active" : ""} type="button" onClick={() => setActiveTab("flame")}>
          火焰图
        </button>
        <button className={activeTab === "hotspots" ? "active" : ""} type="button" onClick={() => setActiveTab("hotspots")}>
          热点 TopN
        </button>
      </div>

      {activeTab === "basic" ? (
        <div className="basic-layout">
          <dl className="info-list">
            <div>
              <dt>目标 PID</dt>
              <dd>{task.target_pid}</dd>
            </div>
            <div>
              <dt>目标机器</dt>
              <dd>{task.target_agent_id}</dd>
            </div>
            <div>
              <dt>采样参数</dt>
              <dd>
                {task.sample_rate_hz}Hz / {task.sample_duration_sec}s
              </dd>
            </div>
            <div>
              <dt>状态原因</dt>
              <dd>{task.status_reason}</dd>
            </div>
          </dl>
          <div className="event-list">
            <h3>状态历史</h3>
            {(task.events ?? []).map((event) => (
              <div className="event-item" key={event.id}>
                <span>{event.to_status}</span>
                <p>{event.reason}</p>
                <time>{formatDate(event.created_at)}</time>
              </div>
            ))}
          </div>
        </div>
      ) : null}

      {activeTab === "flame" ? (
        task.result?.flamegraph_url ? (
          <object className="flame-frame" data={task.result.flamegraph_url} type="image/svg+xml">
            火焰图加载失败
          </object>
        ) : (
          <EmptyBlock text="任务完成后会展示火焰图。" />
        )
      ) : null}

      {activeTab === "hotspots" ? (
        task.result?.hotspots?.length ? (
          <table className="data-table compact">
            <thead>
              <tr>
                <th>函数</th>
                <th>样本数</th>
                <th>占比</th>
              </tr>
            </thead>
            <tbody>
              {task.result.hotspots.map((hotspot) => (
                <tr key={hotspot.function}>
                  <td>{hotspot.function}</td>
                  <td>{hotspot.samples}</td>
                  <td>{hotspot.percent}%</td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <EmptyBlock text="任务完成后会展示 TopN 热点。" />
        )
      ) : null}
    </div>
  );
}

function AuditTable({ auditLogs }: { auditLogs: AuditLog[] }) {
  return (
    <table className="data-table">
      <thead>
        <tr>
          <th>动作</th>
          <th>对象</th>
          <th>原因</th>
          <th>时间</th>
        </tr>
      </thead>
      <tbody>
        {auditLogs.length === 0 ? (
          <tr>
            <td colSpan={4}>
              <EmptyBlock text="暂无离线/恢复审计日志。" />
            </td>
          </tr>
        ) : (
          auditLogs.map((log) => (
            <tr key={log.id}>
              <td>{log.action}</td>
              <td>
                {log.entity_type} / {log.entity_id}
              </td>
              <td>{log.reason}</td>
              <td>{formatDate(log.created_at)}</td>
            </tr>
          ))
        )}
      </tbody>
    </table>
  );
}

function StatusTag({ value }: { value: string }) {
  return <span className={`status-tag ${value.toLowerCase()}`}>{statusText(value)}</span>;
}

function ArtifactLink({ href, label }: { href: string; label: string }) {
  return (
    <a className="artifact-link" href={href} target="_blank" rel="noreferrer">
      {label}
      <ExternalLink size={13} />
    </a>
  );
}

function LoadingBlock() {
  return (
    <div className="loading-block">
      <Loader2 className="spin" size={18} />
      加载中...
    </div>
  );
}

function EmptyBlock({ text }: { text: string }) {
  return <div className="empty-block">{text}</div>;
}

function statusText(value: string) {
  const map: Record<string, string> = {
    ONLINE: "在线",
    OFFLINE: "离线",
    UNKNOWN: "未知",
    PENDING: "等待中",
    RUNNING: "执行中",
    UPLOADING: "上传中",
    DONE: "已完成",
    FAILED: "失败",
  };
  return map[value] ?? value;
}

function formatDate(value: string) {
  if (!value) {
    return "-";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(new Date(value));
}

export default App;
