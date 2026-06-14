import { useEffect, useMemo, useState } from "react";
import {
  AlertCircle,
  ActivitySquare,
  BrainCircuit,
  ExternalLink,
  FolderOpen,
  GitCompareArrows,
  History,
  LayoutDashboard,
  Loader2,
  Monitor,
  RadioTower,
  Rows3,
  Search,
  Timer,
  X,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

import {
  createContinuousProfile,
  createTask,
  getAgents,
  getAuditLogs,
  getContinuousProfileTrends,
  getContinuousProfileWindows,
  getContinuousProfiles,
  getTask,
  getTasks,
  setContinuousProfileEnabled,
} from "./api";
import type {
  Agent,
  AuditLog,
  ContinuousProfile,
  ContinuousTrend,
  ContinuousWindowFilters,
  ContinuousWindow,
  ContinuousWindowSummary,
  CreateContinuousProfileInput,
  CreateTaskInput,
  Hotspot,
  Task,
} from "./types";
import "./App.css";

type NavKey = "home" | "quick" | "machines" | "history" | "files" | "schedule" | "compare";
type WindowRangeKey = "latest" | "1h" | "6h" | "24h";

const defaultWindowFilters: ContinuousWindowFilters & { range: WindowRangeKey } = {
  status: "ALL",
  range: "latest",
  limit: 24,
};

const defaultTaskInput: CreateTaskInput = {
  target_pid: 1,
  sample_duration_sec: 15,
  sample_rate_hz: 99,
  collector_type: "mock-perf",
};

const defaultContinuousInput: CreateContinuousProfileInput = {
  name: "默认 5 分钟窗口",
  target_pid: 1,
  sample_duration_sec: 15,
  sample_rate_hz: 49,
  collector_type: "mock-perf",
  interval_sec: 300,
};

const topLinks: Array<{ key: NavKey; label: string; icon: LucideIcon }> = [
  { key: "home", label: "首页", icon: LayoutDashboard },
  { key: "quick", label: "快速接入", icon: ActivitySquare },
  { key: "machines", label: "机器列表", icon: RadioTower },
  { key: "history", label: "历史任务", icon: History },
  { key: "files", label: "文件分析", icon: FolderOpen },
  { key: "schedule", label: "计划任务", icon: Timer },
  { key: "compare", label: "任务对比", icon: GitCompareArrows },
];

function App() {
  const [activeNav, setActiveNav] = useState<NavKey>("home");
  const [agents, setAgents] = useState<Agent[]>([]);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [profiles, setProfiles] = useState<ContinuousProfile[]>([]);
  const [windows, setWindows] = useState<ContinuousWindow[]>([]);
  const [windowSummary, setWindowSummary] = useState<ContinuousWindowSummary | null>(null);
  const [trend, setTrend] = useState<ContinuousTrend | null>(null);
  const [windowFilters, setWindowFilters] = useState<ContinuousWindowFilters & { range: WindowRangeKey }>(defaultWindowFilters);
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);
  const [selectedTask, setSelectedTask] = useState<Task | null>(null);
  const [selectedProfileId, setSelectedProfileId] = useState<string | null>(null);
  const [compareBaseTaskId, setCompareBaseTaskId] = useState<string | null>(null);
  const [compareTargetTaskId, setCompareTargetTaskId] = useState<string | null>(null);
  const [compareBaseTask, setCompareBaseTask] = useState<Task | null>(null);
  const [compareTargetTask, setCompareTargetTask] = useState<Task | null>(null);
  const [taskInput, setTaskInput] = useState<CreateTaskInput>(defaultTaskInput);
  const [continuousInput, setContinuousInput] = useState<CreateContinuousProfileInput>(defaultContinuousInput);
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [loading, setLoading] = useState(true);
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

  useEffect(() => {
    if (!selectedProfileId && profiles.length > 0) {
      setSelectedProfileId(profiles[0].id);
    }
  }, [profiles, selectedProfileId]);

  useEffect(() => {
    const doneTasks = tasks.filter((task) => task.status === "DONE");
    if (!compareBaseTaskId && doneTasks.length > 0) {
      setCompareBaseTaskId(doneTasks[0].id);
    }
    if (!compareTargetTaskId && doneTasks.length > 1) {
      setCompareTargetTaskId(doneTasks[1].id);
    }
  }, [tasks, compareBaseTaskId, compareTargetTaskId]);

  useEffect(() => {
    let alive = true;

    const loadCompareTask = async (taskId: string | null, setter: (task: Task | null) => void) => {
      if (!taskId) {
        setter(null);
        return;
      }
      try {
        const data = await getTask(taskId);
        if (alive) {
          setter(data.task);
        }
      } catch (compareError) {
        if (alive) {
          setter(null);
          setError((compareError as Error).message);
        }
      }
    };

    void loadCompareTask(compareBaseTaskId, setCompareBaseTask);
    void loadCompareTask(compareTargetTaskId, setCompareTargetTask);

    return () => {
      alive = false;
    };
  }, [compareBaseTaskId, compareTargetTaskId]);

  useEffect(() => {
    if (!selectedProfileId) {
      setWindows([]);
      setWindowSummary(null);
      setTrend(null);
      return;
    }

    let alive = true;
    const loadWindows = async () => {
      try {
        const [data, trendData] = await Promise.all([
          getContinuousProfileWindows(selectedProfileId, buildWindowQuery(windowFilters)),
          getContinuousProfileTrends(selectedProfileId, 12),
        ]);
        if (alive) {
          setWindows(data.windows);
          setWindowSummary(data.summary ?? null);
          setTrend(trendData);
        }
      } catch (profileError) {
        if (alive) {
          setError((profileError as Error).message);
        }
      }
    };

    void loadWindows();
    const interval = window.setInterval(loadWindows, 3000);
    return () => {
      alive = false;
      window.clearInterval(interval);
    };
  }, [selectedProfileId, windowFilters]);

  async function refreshOverview(initial = false) {
    try {
      if (initial) {
        setLoading(true);
      }

      const [agentData, taskData, profileData, auditData] = await Promise.all([
        getAgents(),
        getTasks(),
        getContinuousProfiles(),
        getAuditLogs(),
      ]);
      setAgents(agentData.agents);
      setTasks(taskData.tasks);
      setProfiles(profileData.profiles);
      setAuditLogs(auditData.audit_logs);
      setError(null);
    } catch (overviewError) {
      setError((overviewError as Error).message);
    } finally {
      setLoading(false);
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

  async function handleCreateContinuousProfile(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    try {
      setSubmitting(true);
      const payload = {
        ...continuousInput,
        target_agent_id: continuousInput.target_agent_id || undefined,
      };
      const response = await createContinuousProfile(payload);
      setSelectedProfileId(response.profile.id);
      setActiveNav("schedule");
      await refreshOverview();
    } catch (submitError) {
      setError((submitError as Error).message);
    } finally {
      setSubmitting(false);
    }
  }

  async function handleToggleContinuousProfile(profileId: string, enabled: boolean) {
    try {
      await setContinuousProfileEnabled(profileId, enabled);
      await refreshOverview();
    } catch (toggleError) {
      setError((toggleError as Error).message);
    }
  }

  const stats = useMemo(() => {
    const onlineAgents = agents.filter((agent) => agent.status === "ONLINE").length;
    const failedTasks = tasks.filter((task) => task.status === "FAILED").length;
    const runningTasks = tasks.filter((task) => task.status === "RUNNING" || task.status === "UPLOADING").length;
    const doneTasks = tasks.filter((task) => task.status === "DONE").length;
    return { onlineAgents, failedTasks, runningTasks, doneTasks };
  }, [agents, tasks]);

  const showDropLandingLoader = activeNav === "home" && loading && agents.length === 0 && tasks.length === 0;

  return (
    <div className="drop-app">
      <TopNav
        activeNav={activeNav}
        setActiveNav={setActiveNav}
        onCreateTask={() => setCreateDialogOpen(true)}
      />

      <div className="console-layout">
        <SideRail
          activeNav={activeNav}
          setActiveNav={setActiveNav}
          stats={stats}
          agentCount={agents.length}
          onCreateTask={() => setCreateDialogOpen(true)}
        />

        <section className="console-workspace">
          <main className="console-shell">
            {showDropLandingLoader ? (
              <DropLandingLoader />
            ) : (
              <>
                {activeNav === "home" ? (
                  <section className="hero-panel">
                    <div className="hero-copy">
                      <div className="eyebrow-row">
                        <span className="eyebrow-chip">Mini-Drop</span>
                        <span className="eyebrow-chip muted">一站式性能优化平台</span>
                      </div>
                      <h1>首页</h1>
                      <p>面向 Linux 主机和容器场景的按需采样、火焰图分析、热点定位和智能归因演示平台。</p>
                      <div className="hero-actions">
                        <button className="primary-button" type="button" onClick={() => setCreateDialogOpen(true)}>
                          <ActivitySquare size={14} />
                          新建采样
                        </button>
                        <button className="secondary-button" type="button" onClick={() => setActiveNav("compare")}>
                          <GitCompareArrows size={14} />
                          查看任务对比
                        </button>
                      </div>
                    </div>
                    <div className="hero-metrics">
                      <MetricCard label="在线探针" value={stats.onlineAgents} suffix={`/${agents.length}`} tone="blue" />
                      <MetricCard label="进行中" value={stats.runningTasks} tone="orange" />
                      <MetricCard label="已完成" value={stats.doneTasks} tone="green" />
                      <MetricCard label="失败" value={stats.failedTasks} tone="red" />
                    </div>
                  </section>
                ) : null}

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
                    loading={loading}
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
                  <SchedulePage
                    profiles={profiles}
                    selectedProfileId={selectedProfileId}
                    setSelectedProfileId={setSelectedProfileId}
                    windows={windows}
                    windowSummary={windowSummary}
                    windowFilters={windowFilters}
                    setWindowFilters={setWindowFilters}
                    trend={trend}
                    agents={agents}
                    continuousInput={continuousInput}
                    setContinuousInput={setContinuousInput}
                    submitting={submitting}
                    onSubmit={handleCreateContinuousProfile}
                    onToggleProfile={handleToggleContinuousProfile}
                    onOpenTask={(taskId) => {
                      setSelectedTaskId(taskId);
                      setActiveNav("history");
                    }}
                  />
                ) : null}
                {activeNav === "compare" ? (
                  <ComparePage
                    tasks={tasks}
                    baseTaskId={compareBaseTaskId}
                    targetTaskId={compareTargetTaskId}
                    baseTaskDetail={compareBaseTask}
                    targetTaskDetail={compareTargetTask}
                    setBaseTaskId={setCompareBaseTaskId}
                    setTargetTaskId={setCompareTargetTaskId}
                    onOpenTask={(taskId) => {
                      setSelectedTaskId(taskId);
                      setActiveNav("history");
                    }}
                  />
                ) : null}
              </>
            )}
          </main>

          <footer className="console-footer">
            <div>Mini-Drop Demo · Performance diagnosis workbench</div>
            <div>Mock E2E ready · WSL2 real collectors pending validation</div>
          </footer>
        </section>
      </div>

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
  onCreateTask,
}: {
  activeNav: NavKey;
  setActiveNav: (key: NavKey) => void;
  onCreateTask: () => void;
}) {
  return (
    <header className="top-nav">
      <button className="brand" type="button" onClick={() => setActiveNav("home")}>
        <span className="brand-mark">
          <ActivitySquare size={20} />
        </span>
        <span className="brand-text">
          Mini-Drop <small>性能优化平台</small>
        </span>
      </button>

      <nav className="top-links" aria-label="Mini-Drop primary navigation">
        {topLinks.map((item) => {
          return (
            <button
              key={item.key}
              type="button"
              className={activeNav === item.key ? "active" : ""}
              onClick={() => setActiveNav(item.key)}
            >
              {item.label}
            </button>
          );
        })}
      </nav>

      <div className="top-actions">
        <button type="button" className="top-action secondary" onClick={() => setActiveNav("history")}>
          帮助
        </button>
        <button type="button" className="top-action secondary" onClick={() => setActiveNav("compare")}>
          用户组
        </button>
        <span className="top-user">未登录</span>
        <button type="button" className="top-action primary" onClick={onCreateTask}>
          <ActivitySquare size={14} />
          新建采样
        </button>
      </div>
    </header>
  );
}

function SideRail({
  activeNav,
  setActiveNav,
  stats,
  agentCount,
  onCreateTask,
}: {
  activeNav: NavKey;
  setActiveNav: (key: NavKey) => void;
  stats: { onlineAgents: number; failedTasks: number; runningTasks: number; doneTasks: number };
  agentCount: number;
  onCreateTask: () => void;
}) {
  return (
    <aside className="side-nav">
      <div className="side-product">
        <span>PerfOps Workbench</span>
        <strong>Mini-Drop</strong>
        <p>Mock E2E · WSL2 perf · eBPF / py-spy extensible</p>
      </div>

      <button className="side-primary-action" type="button" onClick={onCreateTask}>
        <ActivitySquare size={16} />
        新建采样
      </button>

      <nav className="side-menu" aria-label="Mini-Drop primary navigation">
        {topLinks.map((item) => {
          const Icon = item.icon;
          return (
            <button
              key={item.key}
              type="button"
              className={activeNav === item.key ? "active" : ""}
              onClick={() => setActiveNav(item.key)}
            >
              <span className="nav-icon">
                <Icon size={15} />
              </span>
              <span>{item.label}</span>
            </button>
          );
        })}
      </nav>

      <div className="side-panel">
        <div className="side-panel-head">
          <span>运行概况</span>
          <strong>Live</strong>
        </div>
        <div className="side-stats">
          <div>
            <span>在线探针</span>
            <strong>
              {stats.onlineAgents}/{agentCount}
            </strong>
          </div>
          <div>
            <span>进行中</span>
            <strong>{stats.runningTasks}</strong>
          </div>
          <div>
            <span>已完成</span>
            <strong>{stats.doneTasks}</strong>
          </div>
          <div>
            <span>失败</span>
            <strong>{stats.failedTasks}</strong>
          </div>
        </div>
        <div className="side-note">
          优先跑通 mock 采样，再把 perf、eBPF、py-spy 逐步接入 Linux / WSL2。
        </div>
      </div>
    </aside>
  );
}

function HomePage({
  agents,
  tasks,
  loading,
  onOpenMachines,
  onOpenHistory,
}: {
  agents: Agent[];
  tasks: Task[];
  loading: boolean;
  onOpenMachines: () => void;
  onOpenHistory: () => void;
}) {
  const comparableTasks = tasks.filter((task) => task.status === "DONE");
  return (
    <div className="page-stack">
      <section className="content-grid">
        <div className="console-card report-card">
          <CardHeader title="我的机器" action="查看全部" onAction={onOpenMachines} />
          {loading ? <LoadingBlock /> : <AgentTable agents={agents.slice(0, 5)} density="home" />}
        </div>
        <div className="console-card report-card">
          <CardHeader title="最近任务" action="查看历史" onAction={onOpenHistory} />
          {loading ? <LoadingBlock /> : <TaskTable tasks={tasks.slice(0, 6)} onSelect={() => onOpenHistory()} density="home" />}
        </div>
      </section>

      <section className="console-card narrative-card">
        <CardHeader title="Demo 证据链" />
        <div className="narrative-grid">
          <div>
            <span className="narrative-label">主链路</span>
            <strong>{"Web -> API Server -> Agent -> Analyzer -> Web"}</strong>
          </div>
          <div>
            <span className="narrative-label">当前可对比任务</span>
            <strong>{comparableTasks.length} 个</strong>
          </div>
          <div>
            <span className="narrative-label">推荐采集器</span>
            <strong>mock-perf / perf</strong>
          </div>
          <div>
            <span className="narrative-label">下一步验收</span>
            <strong>WSL2 真实 perf smoke</strong>
          </div>
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
          <p>选择在线 Agent，输入目标 PID 和采样参数，即可创建一次 CPU 采样任务。</p>
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
          <p>查看任务状态机、变更原因、分析产物和 TopN 热点。</p>
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

function ComparePage({
  tasks,
  baseTaskId,
  targetTaskId,
  baseTaskDetail,
  targetTaskDetail,
  setBaseTaskId,
  setTargetTaskId,
  onOpenTask,
}: {
  tasks: Task[];
  baseTaskId: string | null;
  targetTaskId: string | null;
  baseTaskDetail: Task | null;
  targetTaskDetail: Task | null;
  setBaseTaskId: (taskId: string) => void;
  setTargetTaskId: (taskId: string) => void;
  onOpenTask: (taskId: string) => void;
}) {
  const comparableTasks = tasks.filter((task) => task.status === "DONE");
  const baseTaskSummary = comparableTasks.find((task) => task.id === baseTaskId) ?? comparableTasks[0] ?? null;
  const targetTaskSummary =
    comparableTasks.find((task) => task.id === targetTaskId && task.id !== baseTaskSummary?.id) ??
    comparableTasks.find((task) => task.id !== baseTaskSummary?.id) ??
    comparableTasks[0] ??
    null;
  const baseTask = baseTaskDetail?.id === baseTaskSummary?.id ? baseTaskDetail : baseTaskSummary;
  const targetTask = targetTaskDetail?.id === targetTaskSummary?.id ? targetTaskDetail : targetTaskSummary;
  const diffRows = buildHotspotDiff(baseTask?.result?.hotspots ?? [], targetTask?.result?.hotspots ?? []);
  const canCompare = Boolean(baseTask?.result?.hotspots?.length && targetTask?.result?.hotspots?.length && baseTask.id !== targetTask.id);
  const aggregateRows = buildCrossTaskHotspotAggregate(comparableTasks);

  return (
    <div className="page-stack">
      <section className="page-title-row">
        <div>
          <h1>任务对比</h1>
          <p>选择两个已完成任务，对比采样参数、产物链接和 TopN 热点变化。</p>
        </div>
      </section>

      <section className="content-grid compare-grid">
        <div className="console-card compare-selector-card">
          <CardHeader title="基线任务" />
          <TaskCompareSelector
            label="基线任务"
            tasks={comparableTasks}
            value={baseTaskSummary?.id ?? ""}
            onChange={setBaseTaskId}
          />
          <TaskSummaryPanel task={baseTask} onOpenTask={onOpenTask} />
        </div>
        <div className="console-card compare-selector-card">
          <CardHeader title="对比任务" />
          <TaskCompareSelector
            label="对比任务"
            tasks={comparableTasks}
            value={targetTaskSummary?.id ?? ""}
            onChange={setTargetTaskId}
          />
          <TaskSummaryPanel task={targetTask} onOpenTask={onOpenTask} />
        </div>
      </section>

      <div className="console-card">
        <CardHeader title="跨任务热点聚合" />
        {aggregateRows.length === 0 ? (
          <EmptyBlock text="暂无可聚合的 TopN 热点，请先完成至少一个包含分析结果的任务。" />
        ) : (
          <table className="data-table aggregate-table">
            <thead>
              <tr>
                <th>函数 / 事件</th>
                <th>覆盖任务</th>
                <th>平均占比</th>
                <th>峰值</th>
                <th>样本总数</th>
                <th>最近任务</th>
              </tr>
            </thead>
            <tbody>
              {aggregateRows.map((row) => (
                <tr key={row.name}>
                  <td title={row.name}>{row.name}</td>
                  <td>
                    {row.taskCount} / {comparableTasks.length}
                  </td>
                  <td>{row.averagePercent.toFixed(2)}%</td>
                  <td>{row.peakPercent.toFixed(2)}%</td>
                  <td>{row.samples}</td>
                  <td>
                    <button type="button" className="inline-action" onClick={() => onOpenTask(row.latestTaskId)}>
                      {row.latestTaskId}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div className="console-card">
        <CardHeader title="热点差异" />
        {comparableTasks.length < 2 ? (
          <EmptyBlock text="至少需要两个已完成且包含 TopN 的任务，才能进行对比。" />
        ) : !canCompare ? (
          <EmptyBlock text="正在加载任务 TopN，或所选任务缺少可对比热点。" />
        ) : diffRows.length === 0 ? (
          <EmptyBlock text="两个任务暂无可对比热点。" />
        ) : (
          <table className="data-table compare-table">
            <thead>
              <tr>
                <th>函数 / 事件</th>
                <th>基线占比</th>
                <th>对比占比</th>
                <th>变化</th>
                <th>基线样本</th>
                <th>对比样本</th>
              </tr>
            </thead>
            <tbody>
              {diffRows.map((row) => (
                <tr key={row.name}>
                  <td title={row.name}>{row.name}</td>
                  <td>{row.basePercent.toFixed(2)}%</td>
                  <td>{row.targetPercent.toFixed(2)}%</td>
                  <td>
                    <span className={`delta-pill ${row.deltaPercent > 0 ? "up" : row.deltaPercent < 0 ? "down" : ""}`}>
                      {formatDelta(row.deltaPercent)}%
                    </span>
                  </td>
                  <td>{row.baseSamples}</td>
                  <td>{row.targetSamples}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

function TaskCompareSelector({
  label,
  tasks,
  value,
  onChange,
}: {
  label: string;
  tasks: Task[];
  value: string;
  onChange: (taskId: string) => void;
}) {
  return (
    <label className="compare-selector">
      <span>{label}</span>
      <select value={value} onChange={(event) => onChange(event.target.value)} disabled={tasks.length === 0}>
        {tasks.length === 0 ? <option value="">暂无可选任务</option> : null}
        {tasks.map((task) => (
          <option key={task.id} value={task.id}>
            {task.id} / {task.collector_type} / PID {task.target_pid}
          </option>
        ))}
      </select>
    </label>
  );
}

function TaskSummaryPanel({ task, onOpenTask }: { task: Task | null; onOpenTask: (taskId: string) => void }) {
  if (!task) {
    return <EmptyBlock text="暂无已完成任务，或任务详情尚未加载。" />;
  }

  const top = task.result?.hotspots?.[0];

  return (
    <div className="compare-summary">
      <dl className="info-list">
        <div>
          <dt>任务 ID</dt>
          <dd>{task.id}</dd>
        </div>
        <div>
          <dt>采集器</dt>
          <dd>{task.collector_type}</dd>
        </div>
        <div>
          <dt>采样参数</dt>
          <dd>
            {task.sample_rate_hz}Hz / {task.sample_duration_sec}s / PID {task.target_pid}
          </dd>
        </div>
        <div>
          <dt>Top 热点</dt>
          <dd>{top ? `${top.function} (${top.percent}%)` : "-"}</dd>
        </div>
      </dl>
      <div className="compare-links">
        <ArtifactLink href={task.result?.flamegraph_url ?? ""} label="flamegraph.svg" />
        <ArtifactLink href={task.result?.topn_url ?? ""} label="topn.json" />
        <button type="button" onClick={() => onOpenTask(task.id)}>
          查看详情
        </button>
      </div>
    </div>
  );
}

function SchedulePage({
  profiles,
  selectedProfileId,
  setSelectedProfileId,
  windows,
  windowSummary,
  windowFilters,
  setWindowFilters,
  trend,
  agents,
  continuousInput,
  setContinuousInput,
  submitting,
  onSubmit,
  onToggleProfile,
  onOpenTask,
}: {
  profiles: ContinuousProfile[];
  selectedProfileId: string | null;
  setSelectedProfileId: (profileId: string) => void;
  windows: ContinuousWindow[];
  windowSummary: ContinuousWindowSummary | null;
  windowFilters: ContinuousWindowFilters & { range: WindowRangeKey };
  setWindowFilters: React.Dispatch<React.SetStateAction<ContinuousWindowFilters & { range: WindowRangeKey }>>;
  trend: ContinuousTrend | null;
  agents: Agent[];
  continuousInput: CreateContinuousProfileInput;
  setContinuousInput: React.Dispatch<React.SetStateAction<CreateContinuousProfileInput>>;
  submitting: boolean;
  onSubmit: (event: React.FormEvent<HTMLFormElement>) => void;
  onToggleProfile: (profileId: string, enabled: boolean) => Promise<void>;
  onOpenTask: (taskId: string) => void;
}) {
  const selectedProfile = profiles.find((profile) => profile.id === selectedProfileId) ?? null;

  return (
    <div className="page-stack">
      <section className="page-title-row">
        <div>
          <h1>计划任务</h1>
          <p>固定 5 分钟窗口持续采样，窗口结果会落成普通任务，继续复用同一套火焰图和热点详情。</p>
        </div>
      </section>

      <section className="content-grid schedule-grid">
        <div className="console-card">
          <CardHeader title="连续计划" />
          <table className="data-table">
            <thead>
              <tr>
                <th>名称</th>
                <th>目标 PID</th>
                <th>采集器</th>
                <th>间隔</th>
                <th>状态</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {profiles.length === 0 ? (
                <tr>
                  <td colSpan={6}>
                    <EmptyBlock text="暂无连续计划，请先创建一个 5 分钟窗口计划。" />
                  </td>
                </tr>
              ) : (
                profiles.map((profile) => (
                  <tr
                    key={profile.id}
                    className={selectedProfileId === profile.id ? "selected-row" : ""}
                    onClick={() => setSelectedProfileId(profile.id)}
                  >
                    <td>{profile.name}</td>
                    <td>{profile.target_pid}</td>
                    <td>{profile.collector_type}</td>
                    <td>{profile.interval_sec}s</td>
                    <td>{profile.enabled ? "运行中" : "已停用"}</td>
                    <td>
                      <button
                        className="inline-action"
                        type="button"
                        onClick={(event) => {
                          event.stopPropagation();
                          void onToggleProfile(profile.id, !profile.enabled);
                        }}
                      >
                        {profile.enabled ? "停用" : "启用"}
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        <div className="console-card form-card">
          <div className="schedule-form-head">
            <label>
              <span>计划名称</span>
              <input
                type="text"
                value={continuousInput.name}
                onChange={(event) => setContinuousInput((current) => ({ ...current, name: event.target.value }))}
              />
            </label>
            <label>
              <span>窗口间隔</span>
              <select
                value={continuousInput.interval_sec}
                onChange={(event) => setContinuousInput((current) => ({ ...current, interval_sec: Number(event.target.value) }))}
              >
                <option value={300}>5 分钟</option>
                <option value={600}>10 分钟</option>
                <option value={1800}>30 分钟</option>
              </select>
            </label>
          </div>
          <TaskForm
            agents={agents}
            taskInput={{
              target_pid: continuousInput.target_pid,
              target_agent_id: continuousInput.target_agent_id,
              sample_duration_sec: continuousInput.sample_duration_sec,
              sample_rate_hz: continuousInput.sample_rate_hz,
              collector_type: continuousInput.collector_type,
            }}
            setTaskInput={(updater) =>
              setContinuousInput((current) => {
                const nextBase =
                  typeof updater === "function"
                    ? updater({
                        target_pid: current.target_pid,
                        target_agent_id: current.target_agent_id,
                        sample_duration_sec: current.sample_duration_sec,
                        sample_rate_hz: current.sample_rate_hz,
                        collector_type: current.collector_type,
                      })
                    : updater;
                return {
                  ...current,
                  ...nextBase,
                };
              })
            }
            submitting={submitting}
            onSubmit={onSubmit}
            mode="continuous"
          />
        </div>
      </section>

      <div className="console-card">
        <CardHeader title={selectedProfile ? `5 分钟窗口 - ${selectedProfile.name}` : "5 分钟窗口"} />
        <div className="window-filter-bar">
          <label>
            <span>状态</span>
            <select
              value={windowFilters.status ?? "ALL"}
              onChange={(event) =>
                setWindowFilters((current) => ({
                  ...current,
                  status: event.target.value as ContinuousWindowFilters["status"],
                }))
              }
            >
              <option value="ALL">全部状态</option>
              <option value="PENDING">等待中</option>
              <option value="RUNNING">执行中</option>
              <option value="UPLOADING">上传中</option>
              <option value="DONE">已完成</option>
              <option value="FAILED">失败</option>
            </select>
          </label>
          <label>
            <span>时间范围</span>
            <select
              value={windowFilters.range}
              onChange={(event) =>
                setWindowFilters((current) => ({
                  ...current,
                  range: event.target.value as WindowRangeKey,
                }))
              }
            >
              <option value="latest">最近窗口</option>
              <option value="1h">最近 1 小时</option>
              <option value="6h">最近 6 小时</option>
              <option value="24h">最近 24 小时</option>
            </select>
          </label>
          <label>
            <span>显示数量</span>
            <select
              value={windowFilters.limit ?? 24}
              onChange={(event) =>
                setWindowFilters((current) => ({
                  ...current,
                  limit: Number(event.target.value),
                }))
              }
            >
              <option value={12}>12 个窗口</option>
              <option value={24}>24 个窗口</option>
              <option value={48}>48 个窗口</option>
            </select>
          </label>
        </div>
        <div className="window-summary-strip">
          <div>
            <span>总窗口</span>
            <strong>{windowSummary?.total_windows ?? windows.length}</strong>
          </div>
          <div>
            <span>已完成</span>
            <strong>{windowSummary?.done_windows ?? 0}</strong>
          </div>
          <div>
            <span>失败</span>
            <strong>{windowSummary?.failed_windows ?? 0}</strong>
          </div>
          <div>
            <span>完成率</span>
            <strong>{formatPercent(windowSummary?.done_ratio ?? 0)}</strong>
          </div>
          <div>
            <span>最新状态</span>
            <strong>
              <StatusTag value={windowSummary?.latest_status ?? "NONE"} />
            </strong>
          </div>
          <div className="window-summary-range">
            <span>最近窗口</span>
            <strong>
              {windowSummary?.latest_window_start_at && windowSummary.latest_window_end_at
                ? `${formatDate(windowSummary.latest_window_start_at)} - ${formatDate(windowSummary.latest_window_end_at)}`
                : "暂无"}
            </strong>
          </div>
        </div>
        <div className="window-timeline">
          {windows.length === 0 ? (
            <EmptyBlock text="当前筛选条件下暂无窗口。" />
          ) : (
            windows.map((window) => (
              <button
                key={window.id}
                className={`window-timeline-item ${window.status.toLowerCase()}`}
                type="button"
                onClick={() => onOpenTask(window.task_id)}
                title={`${formatDate(window.window_start_at)} - ${formatDate(window.window_end_at)} / ${window.status_reason}`}
              >
                <span>{formatTimeOnly(window.window_start_at)}</span>
                <strong>{statusText(window.status)}</strong>
              </button>
            ))
          )}
        </div>
        <TrendPanel trend={trend} onOpenTask={onOpenTask} />
        <table className="data-table">
          <thead>
            <tr>
              <th>窗口</th>
              <th>任务</th>
              <th>开始</th>
              <th>结束</th>
              <th>状态</th>
              <th>原因</th>
            </tr>
          </thead>
          <tbody>
            {windows.length === 0 ? (
              <tr>
                <td colSpan={6}>
                  <EmptyBlock text="选中计划后，这里会展示最近的 5 分钟窗口。" />
                </td>
              </tr>
            ) : (
              windows.map((window) => (
                <tr key={window.id} onClick={() => onOpenTask(window.task_id)}>
                  <td>{window.id}</td>
                  <td className="link-cell">{window.task_id}</td>
                  <td>{formatDate(window.window_start_at)}</td>
                  <td>{formatDate(window.window_end_at)}</td>
                  <td>
                    <StatusTag value={window.status} />
                  </td>
                  <td title={window.status_reason}>{window.status_reason}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function TrendPanel({
  trend,
  onOpenTask,
}: {
  trend: ContinuousTrend | null;
  onOpenTask: (taskId: string) => void;
}) {
  return (
    <div className="trend-panel">
      <div className="card-header">
        <h2>跨窗口热点趋势</h2>
        <span className="trend-hint">最近完成窗口</span>
      </div>
      {!trend || trend.windows.length === 0 || trend.series.length === 0 ? (
        <EmptyBlock text="当前没有足够的已完成窗口来生成热点趋势。" />
      ) : (
        <div className="trend-body">
          <div className="trend-window-row">
            {trend.windows.map((window) => (
              <button
                key={window.window_id}
                type="button"
                className="trend-window-chip"
                onClick={() => onOpenTask(window.task_id)}
                title={`${formatDate(window.window_start_at)} - ${formatDate(window.window_end_at)}`}
              >
                <span>{formatTimeOnly(window.window_start_at)}</span>
                <strong>{window.status}</strong>
              </button>
            ))}
          </div>
          <table className="data-table compact trend-table">
            <thead>
              <tr>
                <th>函数</th>
                <th>自动标注</th>
                <th>平均占比</th>
                <th>峰值</th>
                <th>变化量</th>
                <th>基线偏差</th>
                <th>趋势</th>
              </tr>
            </thead>
            <tbody>
              {trend.series.map((series) => (
                <tr key={series.function}>
                  <td>{series.function}</td>
                  <td>
                    <span className={`trend-label ${series.severity}`} title={series.reason}>
                      {series.label}
                    </span>
                  </td>
                  <td>{series.average.toFixed(1)}%</td>
                  <td>{series.peak.toFixed(1)}%</td>
                  <td className={series.delta >= 0 ? "trend-positive" : "trend-negative"}>
                    {series.delta >= 0 ? "+" : ""}
                    {series.delta.toFixed(1)}%
                  </td>
                  <td>
                    <BaselineBadge series={series} />
                  </td>
                  <td>
                    <div className="trend-sparkline">
                      {series.points.map((point) => (
                        <button
                          key={point.window_id}
                          type="button"
                          className="trend-bar"
                          style={{ height: `${Math.max(point.percent, 3)}%` }}
                          title={`${point.percent.toFixed(1)}% / ${point.samples} samples`}
                          onClick={() => onOpenTask(point.task_id)}
                        />
                      ))}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function BaselineBadge({ series }: { series: ContinuousTrend["series"][number] }) {
  if (!series.baseline) {
    return <span className="baseline-badge none">无基线</span>;
  }

  const delta = series.baseline.delta_percent;
  const title = `${series.baseline.description}：峰值 ${series.baseline.actual_percent.toFixed(1)}%，基线 ${series.baseline.expected_percent.toFixed(1)}%。${series.baseline.reason}`;
  return (
    <span className={`baseline-badge ${series.baseline.status}`} title={title}>
      {delta >= 0 ? "+" : ""}
      {delta.toFixed(1)}%
    </span>
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
  mode = "task",
}: {
  agents: Agent[];
  taskInput: CreateTaskInput;
  setTaskInput: React.Dispatch<React.SetStateAction<CreateTaskInput>>;
  submitting: boolean;
  onSubmit: (event: React.FormEvent<HTMLFormElement>) => void;
  mode?: "task" | "continuous";
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
          <option value="perf">perf (Linux / WSL2)</option>
          <option value="ebpf-syscall">eBPF syscall (Linux / WSL2)</option>
          <option value="py-spy">py-spy (Python)</option>
        </select>
      </label>

      <div className="form-actions">
        <button className="primary-button" type="submit" disabled={submitting}>
          {submitting ? "提交中..." : mode === "continuous" ? "创建连续计划" : "提交任务"}
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

function AgentTable({ agents, density }: { agents: Agent[]; density?: "home" }) {
  const isHomeDensity = density === "home";

  return (
    <table className={`data-table agent-table ${isHomeDensity ? "home-density" : ""}`}>
      <thead>
        <tr>
          <th>机器名</th>
          <th>IP</th>
          {!isHomeDensity ? <th>版本</th> : null}
          <th>状态</th>
          <th>最近心跳</th>
        </tr>
      </thead>
      <tbody>
        {agents.length === 0 ? (
          <tr>
            <td colSpan={isHomeDensity ? 4 : 5}>
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
              {!isHomeDensity ? <td>{agent.version}</td> : null}
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
  density,
}: {
  tasks: Task[];
  selectedTaskId?: string | null;
  onSelect: (taskId: string) => void;
  density?: "home";
}) {
  const isHomeDensity = density === "home";

  return (
    <table className={`data-table task-table ${isHomeDensity ? "home-density" : ""}`}>
      <thead>
        <tr>
          <th>任务 ID</th>
          <th>目标 PID</th>
          {!isHomeDensity ? <th>机器</th> : null}
          {!isHomeDensity ? <th>采集器</th> : null}
          <th>状态</th>
          {!isHomeDensity ? <th>原因</th> : null}
          <th>更新时间</th>
        </tr>
      </thead>
      <tbody>
        {tasks.length === 0 ? (
          <tr>
            <td colSpan={isHomeDensity ? 4 : 7}>
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
              {!isHomeDensity ? <td>{task.target_agent_id}</td> : null}
              {!isHomeDensity ? <td>{task.collector_type}</td> : null}
              <td>
                <StatusTag value={task.status} />
              </td>
              {!isHomeDensity ? <td title={task.status_reason}>{task.status_reason}</td> : null}
              <td>{formatDate(task.updated_at)}</td>
            </tr>
          ))
        )}
      </tbody>
    </table>
  );
}

function TaskDetail({ task }: { task: Task | null }) {
  const [activeTab, setActiveTab] = useState<"basic" | "flame" | "hotspots" | "ebpf" | "pyspy" | "attribution">("basic");

  useEffect(() => {
    setActiveTab("basic");
  }, [task?.id]);

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
        {task.collector_type === "ebpf-syscall" ? (
          <button className={activeTab === "ebpf" ? "active" : ""} type="button" onClick={() => setActiveTab("ebpf")}>
            eBPF 分布
          </button>
        ) : null}
        {task.collector_type === "py-spy" ? (
          <button className={activeTab === "pyspy" ? "active" : ""} type="button" onClick={() => setActiveTab("pyspy")}>
            Python 栈
          </button>
        ) : null}
        <button
          className={activeTab === "attribution" ? "active" : ""}
          type="button"
          onClick={() => setActiveTab("attribution")}
        >
          归因建议
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
                <th>{task.collector_type === "ebpf-syscall" ? "系统调用" : "函数"}</th>
                <th>{task.collector_type === "ebpf-syscall" ? "次数" : "样本数"}</th>
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

      {activeTab === "ebpf" ? (
        task.result?.hotspots?.length ? (
          <EBPFDistributionPanel task={task} />
        ) : (
          <EmptyBlock text="eBPF 任务完成后会展示系统调用分布。" />
        )
      ) : null}

      {activeTab === "pyspy" ? (
        task.result?.hotspots?.length ? (
          <PySpyPanel task={task} />
        ) : (
          <EmptyBlock text="py-spy 任务完成后会展示 Python 用户态栈。" />
        )
      ) : null}

      {activeTab === "attribution" ? (
        task.result?.attribution ? (
          <AttributionPanel attribution={task.result.attribution} />
        ) : (
          <EmptyBlock text="任务完成后会展示归因建议。" />
        )
      ) : null}
    </div>
  );
}

function PySpyPanel({ task }: { task: Task }) {
  const hotspots = task.result?.hotspots ?? [];
  const top = hotspots[0];

  return (
    <div className="pyspy-panel">
      <div className="pyspy-summary">
        <div className="pyspy-icon">
          <Rows3 size={20} />
        </div>
        <div>
          <span>Python 用户态栈</span>
          <strong>{top ? `${top.function} 占 ${top.percent}%` : "等待采集结果"}</strong>
        </div>
        <ArtifactLink href={task.result?.flamegraph_url ?? ""} label="flamegraph.svg" />
      </div>
      <table className="data-table compact">
        <thead>
          <tr>
            <th>Python 栈帧</th>
            <th>样本数</th>
            <th>占比</th>
          </tr>
        </thead>
        <tbody>
          {hotspots.slice(0, 8).map((item) => (
            <tr key={item.function}>
              <td>{item.function}</td>
              <td>{item.samples}</td>
              <td>{item.percent}%</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function EBPFDistributionPanel({ task }: { task: Task }) {
  const hotspots = task.result?.hotspots ?? [];
  const maxSamples = Math.max(...hotspots.map((item) => item.samples), 1);

  return (
    <div className="ebpf-panel">
      <div className="ebpf-summary">
        <div className="ebpf-icon">
          <RadioTower size={20} />
        </div>
        <div>
          <span>eBPF syscall 分布</span>
          <strong>
            {task.sample_duration_sec}s 窗口 / {hotspots.length} 个系统调用
          </strong>
        </div>
        <ArtifactLink href={task.result?.topn_url ?? ""} label="topn.json" />
      </div>

      <div className="syscall-bars">
        {hotspots.slice(0, 8).map((item) => (
          <div className="syscall-bar-row" key={item.function}>
            <span>{item.function}</span>
            <div className="syscall-bar-track">
              <div className="syscall-bar-fill" style={{ width: `${Math.max((item.samples / maxSamples) * 100, 3)}%` }} />
            </div>
            <strong>{item.samples}</strong>
            <em>{item.percent}%</em>
          </div>
        ))}
      </div>
    </div>
  );
}

function AttributionPanel({ attribution }: { attribution: NonNullable<Task["result"]>["attribution"] }) {
  if (!attribution) {
    return <EmptyBlock text="任务完成后会展示归因建议。" />;
  }

  return (
    <div className="attribution-panel">
      <div className="attribution-summary">
        <div className="attribution-icon">
          <BrainCircuit size={20} />
        </div>
        <div>
          <span>规则归因结论</span>
          <strong>{attribution.conclusion}</strong>
        </div>
        <div className="confidence-meter">
          <span>置信度</span>
          <strong>{Math.round(attribution.confidence * 100)}%</strong>
        </div>
      </div>

      <div className="attribution-grid">
        <section className="attribution-section">
          <h3>证据列表</h3>
          <table className="data-table compact evidence-table">
            <thead>
              <tr>
                <th>类型</th>
                <th>证据</th>
                <th>函数</th>
                <th>占比</th>
              </tr>
            </thead>
            <tbody>
              {attribution.evidence.map((item, index) => (
                <tr key={`${item.kind}-${index}`}>
                  <td>{evidenceKindText(item.kind)}</td>
                  <td>{item.detail}</td>
                  <td>{item.function ?? "-"}</td>
                  <td>{typeof item.percent === "number" ? `${item.percent}%` : "-"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>

        <section className="attribution-section">
          <h3>优化建议</h3>
          <div className="recommendation-list">
            {attribution.recommendations.map((item, index) => (
              <div className="recommendation-item" key={item}>
                <span>{index + 1}</span>
                <p>{item}</p>
              </div>
            ))}
          </div>
          <dl className="source-list">
            <div>
              <dt>采样来源</dt>
              <dd>
                {attribution.source.collector_type} / {attribution.source.sample_rate_hz}Hz /{" "}
                {attribution.source.sample_duration_sec}s
              </dd>
            </div>
            <div>
              <dt>TopN 文件</dt>
              <dd>{attribution.source.topn_path}</dd>
            </div>
            {attribution.persisted_at ? (
              <div>
                <dt>已持久化</dt>
                <dd>{formatDate(attribution.persisted_at)}</dd>
              </div>
            ) : null}
          </dl>
        </section>

        <section className="attribution-section">
          <h3>工具轨迹</h3>
          <div className="tool-trace-list">
            {(attribution.tool_trace ?? []).map((item, index) => (
              <div className="tool-trace-item" key={`${item.name}-${index}`}>
                <strong>{item.name}</strong>
                <span>{item.input}</span>
                <p>{item.output}</p>
              </div>
            ))}
            {(attribution.tool_trace ?? []).length === 0 ? <EmptyBlock text="当前结果没有工具轨迹。" /> : null}
          </div>
          {attribution.prompt ? (
            <div className="prompt-box">
              <h4>Prompt</h4>
              <pre>{attribution.prompt}</pre>
            </div>
          ) : null}
        </section>
      </div>
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
  if (!href) {
    return <span className="artifact-link muted-link">{label}</span>;
  }

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
    NONE: "暂无",
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

function buildWindowQuery(filters: ContinuousWindowFilters & { range: WindowRangeKey }): ContinuousWindowFilters {
  const query: ContinuousWindowFilters = {
    status: filters.status,
    limit: filters.limit,
  };
  if (filters.range !== "latest") {
    const hours: Record<Exclude<WindowRangeKey, "latest">, number> = {
      "1h": 1,
      "6h": 6,
      "24h": 24,
    };
    const now = new Date();
    query.from = new Date(now.getTime() - hours[filters.range] * 60 * 60 * 1000).toISOString();
    query.to = now.toISOString();
  }
  return query;
}

function formatPercent(value: number) {
  if (!Number.isFinite(value)) {
    return "0%";
  }
  return `${Math.round(value * 100)}%`;
}

function evidenceKindText(value: string) {
  const map: Record<string, string> = {
    top_hotspot: "Top 热点",
    supporting_hotspot: "辅助热点",
    baseline: "历史 baseline",
    resource_timeline: "资源时间线",
    rule_match: "规则命中",
    sampling: "采样参数",
    topn: "TopN",
  };
  return map[value] ?? value;
}

function buildHotspotDiff(baseHotspots: Hotspot[], targetHotspots: Hotspot[]) {
  const names = new Set<string>();
  const baseMap = new Map(baseHotspots.map((item) => [item.function, item]));
  const targetMap = new Map(targetHotspots.map((item) => [item.function, item]));
  baseHotspots.slice(0, 10).forEach((item) => names.add(item.function));
  targetHotspots.slice(0, 10).forEach((item) => names.add(item.function));

  return Array.from(names)
    .map((name) => {
      const base = baseMap.get(name);
      const target = targetMap.get(name);
      const basePercent = Number(base?.percent ?? 0);
      const targetPercent = Number(target?.percent ?? 0);
      return {
        name,
        basePercent,
        targetPercent,
        deltaPercent: targetPercent - basePercent,
        baseSamples: Number(base?.samples ?? 0),
        targetSamples: Number(target?.samples ?? 0),
      };
    })
    .sort((left, right) => Math.abs(right.deltaPercent) - Math.abs(left.deltaPercent));
}

function buildCrossTaskHotspotAggregate(tasks: Task[]) {
  const summary = new Map<string, {
    name: string;
    taskCount: number;
    totalPercent: number;
    peakPercent: number;
    samples: number;
    latestTaskId: string;
    latestCreatedAt: number;
  }>();

  for (const task of tasks) {
    const hotspots = task.result?.hotspots ?? [];
    const createdAt = Date.parse(task.created_at) || 0;
    for (const hotspot of hotspots) {
      const current = summary.get(hotspot.function);
      if (!current) {
        summary.set(hotspot.function, {
          name: hotspot.function,
          taskCount: 1,
          totalPercent: Number(hotspot.percent ?? 0),
          peakPercent: Number(hotspot.percent ?? 0),
          samples: Number(hotspot.samples ?? 0),
          latestTaskId: task.id,
          latestCreatedAt: createdAt,
        });
        continue;
      }

      current.taskCount += 1;
      current.totalPercent += Number(hotspot.percent ?? 0);
      current.peakPercent = Math.max(current.peakPercent, Number(hotspot.percent ?? 0));
      current.samples += Number(hotspot.samples ?? 0);
      if (createdAt >= current.latestCreatedAt) {
        current.latestTaskId = task.id;
        current.latestCreatedAt = createdAt;
      }
    }
  }

  return Array.from(summary.values())
    .map((item) => ({
      ...item,
      averagePercent: item.totalPercent / item.taskCount,
    }))
    .sort((left, right) => right.peakPercent - left.peakPercent || right.taskCount - left.taskCount)
    .slice(0, 6);
}

function formatDelta(value: number) {
  const normalized = Number.isFinite(value) ? value : 0;
  return `${normalized > 0 ? "+" : ""}${normalized.toFixed(2)}`;
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

function formatTimeOnly(value: string) {
  if (!value) {
    return "-";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}

export default App;
