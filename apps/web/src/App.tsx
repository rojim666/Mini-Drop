import { useEffect, useMemo, useRef, useState } from "react";
import {
  AlertCircle,
  ActivitySquare,
  BrainCircuit,
  Building2,
  CheckCircle2,
  ExternalLink,
  FolderOpen,
  GitCompareArrows,
  Globe2,
  History,
  KeyRound,
  LayoutDashboard,
  Loader2,
  LockKeyhole,
  Monitor,
  PauseCircle,
  PlayCircle,
  RadioTower,
  Rows3,
  Search,
  Server,
  ShieldCheck,
  Timer,
  UserRound,
  X,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

import {
  createContinuousProfile,
  createTask,
  clearAuthSession,
  getAgents,
  getAuditLogs,
  getContinuousProfileTrends,
  getContinuousProfileWindows,
  getContinuousProfiles,
  getAIConfig,
  getCurrentUser,
  getStoredAuthToken,
  getStoredUserProfile,
  getTask,
  getTasks,
  login,
  setContinuousProfileEnabled,
  storeAuthSession,
  updateAIConfig,
} from "./api";
import type {
  Agent,
  AIConfig,
  AuditLog,
  ContinuousProfile,
  ContinuousTrend,
  ContinuousWindowFilters,
  ContinuousWindow,
  ContinuousWindowSummary,
  CreateContinuousProfileInput,
  CreateTaskInput,
  Hotspot,
  LoginRequest,
  Task,
  UpdateAIConfigInput,
  UserProfile,
} from "./types";
import "./App.css";

type NavKey = "home" | "quick" | "machines" | "history" | "files" | "schedule" | "ai" | "compare";
type WindowRangeKey = "latest" | "1h" | "6h" | "24h";
type FormFeedback = { tone: "success" | "error"; message: string };
const navKeys: NavKey[] = ["home", "quick", "machines", "history", "files", "schedule", "ai", "compare"];

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
  name: "默认连续剖析计划",
  target_pid: 1,
  sample_duration_sec: 15,
  sample_rate_hz: 49,
  collector_type: "mock-perf",
  interval_sec: 300,
  schedule_mode: "interval",
  cron_expression: "*/5 * * * *",
  stagger_sec: 0,
};

const defaultAIConfigInput: UpdateAIConfigInput = {
  enabled: false,
  base_url: "https://api.openai.com/v1",
  api_key: "",
  model: "gpt-4o-mini",
  timeout_sec: 20,
  max_tokens: 800,
};

const topLinks: Array<{ key: NavKey; label: string; icon: LucideIcon }> = [
  { key: "home", label: "首页", icon: LayoutDashboard },
  { key: "quick", label: "快速接入", icon: ActivitySquare },
  { key: "machines", label: "机器列表", icon: RadioTower },
  { key: "history", label: "历史任务", icon: History },
  { key: "files", label: "文件分析", icon: FolderOpen },
  { key: "schedule", label: "计划任务", icon: Timer },
  { key: "ai", label: "智能分析", icon: BrainCircuit },
  { key: "compare", label: "任务对比", icon: GitCompareArrows },
];

function readNavFromHash(): NavKey {
  const value = window.location.hash.replace(/^#\/?/, "");
  return navKeys.includes(value as NavKey) ? (value as NavKey) : "home";
}

function App() {
  const [authChecking, setAuthChecking] = useState(() => getStoredAuthToken() !== "");
  const [authenticated, setAuthenticated] = useState(() => getStoredAuthToken() !== "");
  const [currentUser, setCurrentUser] = useState<UserProfile | null>(() => getStoredUserProfile());
  const [loginError, setLoginError] = useState<string | null>(null);
  const [loginSubmitting, setLoginSubmitting] = useState(false);
  const [activeNav, setActiveNavState] = useState<NavKey>(readNavFromHash());
  const [agents, setAgents] = useState<Agent[]>([]);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [profiles, setProfiles] = useState<ContinuousProfile[]>([]);
  const [windows, setWindows] = useState<ContinuousWindow[]>([]);
  const [windowSummary, setWindowSummary] = useState<ContinuousWindowSummary | null>(null);
  const [trend, setTrend] = useState<ContinuousTrend | null>(null);
  const [aiConfig, setAIConfig] = useState<AIConfig | null>(null);
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
  const [aiConfigInput, setAIConfigInput] = useState<UpdateAIConfigInput>(defaultAIConfigInput);
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [aiConfigFeedback, setAIConfigFeedback] = useState<FormFeedback | null>(null);
  const activeNavRef = useRef<NavKey>(activeNav);

  const setActiveNav = (key: NavKey) => {
    setActiveNavState(key);
    if (window.location.hash !== `#${key}`) {
      window.history.replaceState(null, "", `#${key}`);
    }
  };

  useEffect(() => {
    activeNavRef.current = activeNav;
  }, [activeNav]);

  useEffect(() => {
    if (!getStoredAuthToken()) {
      setAuthChecking(false);
      setAuthenticated(false);
      return;
    }

    let alive = true;
    const verifySession = async () => {
      try {
        const data = await getCurrentUser();
        if (alive) {
          setCurrentUser(data.user);
          setAuthenticated(true);
          setLoginError(null);
        }
      } catch {
        clearAuthSession();
        if (alive) {
          setCurrentUser(null);
          setAuthenticated(false);
        }
      } finally {
        if (alive) {
          setAuthChecking(false);
        }
      }
    };

    void verifySession();
    return () => {
      alive = false;
    };
  }, []);

  useEffect(() => {
    if (!authenticated) {
      return;
    }
    void refreshOverview(true);
  }, [authenticated]);

  useEffect(() => {
    const onHashChange = () => setActiveNavState(readNavFromHash());
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, []);

  useEffect(() => {
    if (!authenticated) {
      return;
    }
    const interval = window.setInterval(() => {
      void refreshOverview(false);
    }, 3000);
    return () => window.clearInterval(interval);
  }, [authenticated]);

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
    if (!authenticated) {
      return;
    }

    try {
      if (initial) {
        setLoading(true);
      }

      const [agentData, taskData, profileData, auditData, aiData] = await Promise.all([
        getAgents(),
        getTasks(),
        getContinuousProfiles(),
        getAuditLogs(),
        getAIConfig(),
      ]);
      setAgents(agentData.agents);
      setTasks(taskData.tasks);
      setProfiles(profileData.profiles);
      setAuditLogs(auditData.audit_logs);
      setAIConfig(aiData);
      if (initial || activeNavRef.current !== "ai") {
        setAIConfigInput({
          enabled: aiData.enabled,
          base_url: aiData.base_url,
          api_key: "",
          model: aiData.model,
          timeout_sec: aiData.timeout_sec,
          max_tokens: aiData.max_tokens,
        });
      }
      setError(null);
    } catch (overviewError) {
      const message = (overviewError as Error).message;
      if (message.includes("401") || message.toLowerCase().includes("auth")) {
        clearAuthSession();
        setAuthenticated(false);
        setCurrentUser(null);
        setLoginError("登录状态已失效，请重新登录。");
      } else {
        setError(message);
      }
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
        cron_expression: continuousInput.schedule_mode === "cron" ? continuousInput.cron_expression || "*/5 * * * *" : undefined,
        interval_sec: continuousInput.schedule_mode === "cron" ? continuousInput.interval_sec || 300 : continuousInput.interval_sec,
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

  async function handleSaveAIConfig(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    try {
      setSubmitting(true);
      setAIConfigFeedback(null);
      const config = await updateAIConfig(aiConfigInput);
      setAIConfig(config);
      setAIConfigInput({
        enabled: config.enabled,
        base_url: config.base_url,
        api_key: "",
        model: config.model,
        timeout_sec: config.timeout_sec,
        max_tokens: config.max_tokens,
      });
      setError(null);
      setAIConfigFeedback({
        tone: "success",
        message: config.enabled ? "LLM 配置已保存，后续完成的任务会尝试使用真实 AI 归因。" : "配置已保存，当前继续使用规则兜底。",
      });
    } catch (configError) {
      const message = (configError as Error).message;
      setError(null);
      setAIConfigFeedback({ tone: "error", message });
    } finally {
      setSubmitting(false);
    }
  }

  const setAIConfigInputFromForm: React.Dispatch<React.SetStateAction<UpdateAIConfigInput>> = (value) => {
    setAIConfigFeedback(null);
    setAIConfigInput(value);
  };

  const stats = useMemo(() => {
    const onlineAgents = agents.filter((agent) => agent.status === "ONLINE").length;
    const failedTasks = tasks.filter((task) => task.status === "FAILED").length;
    const runningTasks = tasks.filter((task) => task.status === "RUNNING" || task.status === "UPLOADING").length;
    const doneTasks = tasks.filter((task) => task.status === "DONE").length;
    return { onlineAgents, failedTasks, runningTasks, doneTasks };
  }, [agents, tasks]);

  const showDropLandingLoader = activeNav === "home" && loading && agents.length === 0 && tasks.length === 0;

  async function handleLogin(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const payload: LoginRequest = {
      username: String(new FormData(form).get("username") ?? ""),
      password: String(new FormData(form).get("password") ?? ""),
      tenant: String(new FormData(form).get("tenant") ?? "local-demo"),
      region: String(new FormData(form).get("region") ?? "local"),
    };

    try {
      setLoginSubmitting(true);
      setLoginError(null);
      const session = await login(payload);
      storeAuthSession(session);
      setCurrentUser(session.user);
      setAuthenticated(true);
      if (!window.location.hash) {
        window.history.replaceState(null, "", "#home");
      }
    } catch (authError) {
      clearAuthSession();
      setAuthenticated(false);
      setCurrentUser(null);
      setLoginError((authError as Error).message);
    } finally {
      setLoginSubmitting(false);
    }
  }

  function handleLogout() {
    clearAuthSession();
    setCurrentUser(null);
    setAuthenticated(false);
  }

  if (authChecking) {
    return <DropLandingLoader />;
  }

  if (!authenticated) {
    return <LoginPage error={loginError} submitting={loginSubmitting} onLogin={handleLogin} />;
  }

  return (
    <div className="drop-app">
      <TopNav
        activeNav={activeNav}
        setActiveNav={setActiveNav}
        onCreateTask={() => setCreateDialogOpen(true)}
        onLogout={handleLogout}
        user={currentUser}
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
                    aiConfig={aiConfig}
                    onOpenAIConfig={() => setActiveNav("ai")}
                    onOpenTask={(taskId) => {
                      setSelectedTaskId(taskId);
                      setActiveNav("history");
                    }}
                  />
                ) : null}
                {activeNav === "ai" ? (
                  <AISettingsPage
                    config={aiConfig}
                    input={aiConfigInput}
                    setInput={setAIConfigInputFromForm}
                    feedback={aiConfigFeedback}
                    submitting={submitting}
                    tasks={tasks}
                    onSubmit={handleSaveAIConfig}
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

function LoginPage({
  error,
  submitting,
  onLogin,
}: {
  error: string | null;
  submitting: boolean;
  onLogin: (event: React.FormEvent<HTMLFormElement>) => void;
}) {
  return (
    <main className="login-shell">
      <header className="login-topbar">
        <div className="login-brand">
          <span className="brand-mark">
            <ActivitySquare size={22} />
          </span>
          <div>
            <strong>Mini-Drop</strong>
            <span>性能优化平台</span>
          </div>
        </div>
        <nav className="login-links" aria-label="登录页辅助导航">
          <a href="#quick">快速接入</a>
          <a href="#machines">节点管理</a>
          <a href="#history">任务中心</a>
          <a href="#files">制品分析</a>
        </nav>
      </header>

      <section className="login-main">
        <section className="login-brief">
          <div className="login-brief-copy">
            <span className="login-kicker">Mini-Drop Console</span>
            <h1>登录性能诊断控制台</h1>
            <p>统一管理 Agent、采样任务、火焰图制品和 AI 归因结果，面向本地 Demo 与 WSL2 真实 perf 验证场景。</p>
          </div>

          <div className="login-signal-grid">
            <div>
              <span>任务链路</span>
              <strong>PENDING → DONE</strong>
            </div>
            <div>
              <span>采集器</span>
              <strong>mock-perf / perf</strong>
            </div>
            <div>
              <span>归因模式</span>
              <strong>AI + 规则兜底</strong>
            </div>
          </div>

          <div className="login-status-panel">
            <div className="login-status-head">
              <ShieldCheck size={18} />
              <div>
                <strong>演示环境状态</strong>
                <span>Compose 五服务验收视图</span>
              </div>
            </div>
            <div className="login-service-list">
              <div>
                <Server size={16} />
                <span>API Server</span>
                <strong>18080</strong>
              </div>
              <div>
                <RadioTower size={16} />
                <span>Agent</span>
                <strong>drop_agent</strong>
              </div>
              <div>
                <BrainCircuit size={16} />
                <span>Analyzer</span>
                <strong>ready</strong>
              </div>
              <div>
                <FolderOpen size={16} />
                <span>MinIO / Local</span>
                <strong>artifacts</strong>
              </div>
            </div>
          </div>
        </section>

        <section className="login-card" aria-label="控制台登录">
          <div className="login-card-head">
            <div>
              <h2>账号登录</h2>
              <p>本地演示租户</p>
            </div>
            <span className="login-edition">Demo</span>
          </div>

          <div className="login-tabs" role="tablist" aria-label="登录方式">
            <button className="active" type="button">
              账号密码
            </button>
            <button type="button">企业 SSO</button>
            <button type="button">访问密钥</button>
          </div>

          <form className="login-form" onSubmit={onLogin}>
            <label>
              <span>账号</span>
              <div className="login-field">
                <UserRound size={16} />
                <input name="username" type="text" defaultValue="demo" autoComplete="username" />
              </div>
            </label>
            <label>
              <span>密码</span>
              <div className="login-field">
                <LockKeyhole size={16} />
                <input name="password" type="password" defaultValue="minidrop" autoComplete="current-password" />
              </div>
            </label>
            <label>
              <span>登录范围</span>
              <div className="login-field">
                <Building2 size={16} />
                <select name="tenant" defaultValue="local-demo">
                  <option value="local-demo">本地演示租户 / default</option>
                  <option value="wsl2-perf">WSL2 perf 验证 / ubuntu</option>
                </select>
              </div>
            </label>
            <label>
              <span>地域</span>
              <div className="login-field">
                <Globe2 size={16} />
                <select name="region" defaultValue="local">
                  <option value="local">本地开发环境</option>
                  <option value="compose">Docker Compose 环境</option>
                  <option value="wsl2">WSL2 Ubuntu</option>
                </select>
              </div>
            </label>

            <div className="login-form-row">
              <label className="login-checkbox">
                <input type="checkbox" defaultChecked />
                <span>保持登录状态</span>
              </label>
              <a href="#home">查看控制台入口</a>
            </div>

            {error ? (
              <div className="login-error" role="alert">
                <AlertCircle size={15} />
                <span>{error}</span>
              </div>
            ) : null}

            <button className="primary-button" type="submit" disabled={submitting}>
              {submitting ? (
                <>
                  <Loader2 className="spin" size={14} />
                  登录中
                </>
              ) : (
                "登录控制台"
              )}
            </button>
          </form>

          <div className="login-security-box">
            <KeyRound size={16} />
            <div>
              <strong>安全校验</strong>
              <span>当前为本地 Demo 登录，不会向外部认证服务发送账号信息。</span>
            </div>
          </div>

          <div className="login-checklist">
            <div>
              <CheckCircle2 size={15} />
              <span>Docker Compose E2E</span>
            </div>
            <div>
              <CheckCircle2 size={15} />
              <span>Agent 心跳与任务状态机</span>
            </div>
            <div>
              <CheckCircle2 size={15} />
              <span>火焰图、TopN 与 AI 归因</span>
            </div>
          </div>
        </section>

        <aside className="login-ops-rail" aria-label="演示验收信息">
          <section className="login-ops-card">
            <div className="login-ops-head">
              <Timer size={17} />
              <div>
                <strong>端到端验证</strong>
                <span>Compose demo checklist</span>
              </div>
            </div>
            <div className="login-step-list">
              <div className="login-step-item active">
                <CheckCircle2 size={14} />
                <span>5 个服务 healthy</span>
              </div>
              <div className="login-step-item active">
                <CheckCircle2 size={14} />
                <span>drop_agent 在线</span>
              </div>
              <div className="login-step-item">
                <ActivitySquare size={14} />
                <span>创建 CPU 采样任务</span>
              </div>
              <div className="login-step-item">
                <BrainCircuit size={14} />
                <span>AI 归因建议展示</span>
              </div>
            </div>
          </section>

          <section className="login-ops-card">
            <div className="login-ops-head">
              <Monitor size={17} />
              <div>
                <strong>访问端点</strong>
                <span>Local compose runtime</span>
              </div>
            </div>
            <dl className="login-endpoint-list">
              <div>
                <dt>Web</dt>
                <dd>localhost:14173</dd>
              </div>
              <div>
                <dt>API</dt>
                <dd>localhost:18080</dd>
              </div>
              <div>
                <dt>MinIO</dt>
                <dd>localhost:19000</dd>
              </div>
            </dl>
          </section>
        </aside>
      </section>

      <footer className="login-footer">
        <span>Mini-Drop Demo</span>
        <span>性能采样</span>
        <span>制品分析</span>
        <span>智能归因</span>
      </footer>
    </main>
  );
}

function TopNav({
  activeNav,
  setActiveNav,
  onCreateTask,
  onLogout,
  user,
}: {
  activeNav: NavKey;
  setActiveNav: (key: NavKey) => void;
  onCreateTask: () => void;
  onLogout: () => void;
  user: UserProfile | null;
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
        <span className="top-user" title={user ? `${user.tenant} / ${user.region}` : "未登录"}>
          {user ? `${user.username} · ${user.tenant}` : "Guest"}
        </span>
        <button type="button" className="top-action secondary" onClick={onLogout}>
          退出
        </button>
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
  const failedTasks = tasks.filter((task) => task.status === "FAILED");
  return (
    <div className="page-stack">
      <section className="content-grid">
        <div className="console-card report-card">
          <CardHeader title="我的机器" action="查看全部" onAction={onOpenMachines} />
          {loading ? <LoadingBlock /> : <HomeAgentPanel agents={agents} />}
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

      <section className="delivery-grid">
        <div className="console-card delivery-card">
          <CardHeader title="真实采集器状态" />
          <div className="collector-readiness-list">
            <ReadinessRow name="perf" status="BLOCKED" reason="缺少 perf，且 perf_event_paranoid=2" />
            <ReadinessRow name="eBPF" status="BLOCKED" reason="缺少 bpftrace / tracefs 权限" />
            <ReadinessRow name="py-spy" status="BLOCKED" reason="缺少 py-spy 或 attach 权限" />
          </div>
        </div>

        <div className="console-card delivery-card">
          <CardHeader title="下一步命令" />
          <div className="command-stack">
            <code>make real-preflight</code>
            <code>make real-check</code>
            <code>make smoke-real COLLECTOR_TYPE=perf</code>
          </div>
          <div className="delivery-note">
            当前 Windows Compose 演示可直接使用；真实 smoke 需要在 WSL2 / Linux 装好采集工具后执行。
          </div>
        </div>

        <div className="console-card delivery-card">
          <CardHeader title="异常路径证据" />
          <div className="failure-evidence">
            <strong>{failedTasks.length}</strong>
            <span>条 FAILED 任务</span>
            <p>PID 不存在、Agent 离线和采集器环境错误都会落状态原因与事件历史。</p>
          </div>
        </div>
      </section>
    </div>
  );
}

function HomeAgentPanel({ agents }: { agents: Agent[] }) {
  const visibleAgents = agents.slice(0, 5);
  const onlineAgents = agents.filter((agent) => agent.status === "ONLINE").length;
  const offlineAgents = agents.filter((agent) => agent.status === "OFFLINE").length;
  const latestHeartbeat = agents
    .map((agent) => agent.last_heartbeat_at)
    .filter(Boolean)
    .sort((a, b) => new Date(b).getTime() - new Date(a).getTime())[0];
  const dropAgents = agents.filter((agent) => agent.id === "drop_agent" || agent.hostname.includes("drop-agent")).length;

  return (
    <div className="home-agent-panel">
      <AgentTable agents={visibleAgents} density="home" />
      <div className="home-agent-summary">
        <div>
          <span>在线 / 总数</span>
          <strong>
            {onlineAgents}/{agents.length}
          </strong>
        </div>
        <div>
          <span>离线</span>
          <strong>{offlineAgents}</strong>
        </div>
        <div>
          <span>最近心跳</span>
          <strong>{latestHeartbeat ? formatDate(latestHeartbeat) : "暂无"}</strong>
        </div>
        <div>
          <span>Drop Agent</span>
          <strong>{dropAgents}</strong>
        </div>
      </div>
    </div>
  );
}

function ReadinessRow({ name, status, reason }: { name: string; status: "READY" | "BLOCKED"; reason: string }) {
  return (
    <div className="readiness-row">
      <span>{name}</span>
      <StatusTag value={status === "READY" ? "DONE" : "FAILED"} />
      <p>{reason}</p>
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
                  <td>{task.raw_artifact_url ? <ArtifactLink href={task.raw_artifact_url} label={rawArtifactLabel(task)} /> : "-"}</td>
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
  const profileRows = buildCrossProfileHotspotAggregate(comparableTasks);

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
        <CardHeader title="跨 profile 热点聚合" />
        {profileRows.length === 0 ? (
          <EmptyBlock text="暂无来自连续计划的 TopN 热点，完成连续窗口后可查看跨 profile 聚合。" />
        ) : (
          <table className="data-table profile-aggregate-table">
            <thead>
              <tr>
                <th>函数 / 事件</th>
                <th>覆盖 profile</th>
                <th>覆盖任务</th>
                <th>平均占比</th>
                <th>峰值</th>
                <th>最近任务</th>
              </tr>
            </thead>
            <tbody>
              {profileRows.map((row) => (
                <tr key={row.name}>
                  <td title={row.name}>{row.name}</td>
                  <td>{row.profileCount}</td>
                  <td>{row.taskCount}</td>
                  <td>{row.averagePercent.toFixed(2)}%</td>
                  <td>{row.peakPercent.toFixed(2)}%</td>
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
  aiConfig,
  onOpenAIConfig,
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
  aiConfig: AIConfig | null;
  onOpenAIConfig: () => void;
  onOpenTask: (taskId: string) => void;
}) {
  const selectedProfile = profiles.find((profile) => profile.id === selectedProfileId) ?? null;
  const scheduleMode = continuousInput.schedule_mode ?? "interval";
  const enabledProfiles = profiles.filter((profile) => profile.enabled).length;
  const activeWindows =
    (windowSummary?.running_windows ?? 0) + (windowSummary?.pending_windows ?? 0);
  const latestWindowRange =
    windowSummary?.latest_window_start_at && windowSummary.latest_window_end_at
      ? `${formatDate(windowSummary.latest_window_start_at)} - ${formatDate(windowSummary.latest_window_end_at)}`
      : "暂无窗口";

  return (
    <div className="page-stack">
      <section className="page-title-row">
        <div>
          <h1>计划任务</h1>
          <p>按固定间隔或 cron 表达式持续采样，窗口结果会落成普通任务，继续复用同一套火焰图和热点详情。</p>
        </div>
      </section>

      <section className="schedule-overview-strip">
        <div>
          <span>计划总数</span>
          <strong>{profiles.length}</strong>
        </div>
        <div>
          <span>运行中</span>
          <strong>{enabledProfiles}</strong>
        </div>
        <div>
          <span>待处理窗口</span>
          <strong>{activeWindows}</strong>
        </div>
        <div>
          <span>当前计划</span>
          <strong title={selectedProfile?.name ?? "暂无"}>{selectedProfile?.name ?? "暂无"}</strong>
        </div>
        <div className="schedule-overview-wide">
          <span>最近窗口</span>
          <strong>{latestWindowRange}</strong>
        </div>
      </section>

      <section className="content-grid schedule-grid">
        <div className="console-card schedule-plan-card">
          <CardHeader title="连续计划" />
          {profiles.length === 0 ? (
            <div className="schedule-empty-state">
              <div className="schedule-empty-icon">
                <Timer size={20} />
              </div>
              <div>
                <strong>暂无连续剖析计划</strong>
                <p>先用默认 mock-perf 创建一个 5 分钟窗口，验证时间轴和趋势分析；Linux / WSL2 验收时再切换 perf、eBPF 或 py-spy。</p>
              </div>
              <dl className="schedule-empty-kv">
                <div>
                  <dt>默认窗口</dt>
                  <dd>{formatSeconds(continuousInput.interval_sec)}</dd>
                </div>
                <div>
                  <dt>采样参数</dt>
                  <dd>
                    {continuousInput.sample_duration_sec}s / {continuousInput.sample_rate_hz}Hz
                  </dd>
                </div>
                <div>
                  <dt>采集器</dt>
                  <dd>{continuousInput.collector_type}</dd>
                </div>
              </dl>
            </div>
          ) : (
            <div className="schedule-plan-list">
              <div className="schedule-plan-head" aria-hidden="true">
                <span className="schedule-plan-head-select">
                  <span>计划名称</span>
                  <span>采集配置</span>
                  <span>状态</span>
                </span>
                <span>操作</span>
              </div>
              {profiles.map((profile) => (
                <div
                  key={profile.id}
                  className={`schedule-plan-row ${selectedProfileId === profile.id ? "selected" : ""}`}
                >
                  <button type="button" className="schedule-plan-select" onClick={() => setSelectedProfileId(profile.id)}>
                    <span className="schedule-plan-main">
                      <strong title={profile.name}>{profile.name}</strong>
                      <em>{profile.id}</em>
                    </span>
                    <span className="schedule-plan-meta">
                      <span>PID {profile.target_pid}</span>
                      <span>{profile.collector_type}</span>
                      <span>{formatSchedulePolicy(profile)}</span>
                      <span>错峰 {formatStagger(profile.stagger_sec)}</span>
                    </span>
                    <span className={`schedule-state ${profile.enabled ? "enabled" : "paused"}`}>
                      {profile.enabled ? "运行中" : "已停用"}
                    </span>
                  </button>
                  <button
                    type="button"
                    className="schedule-row-action"
                    aria-label={`${profile.enabled ? "停用" : "启用"} ${profile.name}`}
                    onClick={(event) => {
                      event.stopPropagation();
                      void onToggleProfile(profile.id, !profile.enabled);
                    }}
                  >
                    {profile.enabled ? <PauseCircle size={14} /> : <PlayCircle size={14} />}
                    {profile.enabled ? "停用" : "启用"}
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="schedule-side-stack">
          <div className="console-card schedule-ai-card">
            <CardHeader title="智能分析配置" action="配置 LLM" onAction={onOpenAIConfig} />
            <AIConfigSummary config={aiConfig} compact />
          </div>

          <div className="console-card schedule-create-card">
            <CardHeader title="创建连续计划" />
            <div className="schedule-create-body">
              <div className="schedule-form-head">
                <div className="form-section-title">调度策略</div>
                <label>
                  <span>计划名称</span>
                  <input
                    type="text"
                    value={continuousInput.name}
                    onChange={(event) => setContinuousInput((current) => ({ ...current, name: event.target.value }))}
                  />
                </label>
                <div className="schedule-mode-row">
                  <span>调度方式</span>
                  <div className="segmented-control" role="group" aria-label="调度方式">
                    <button
                      type="button"
                      className={scheduleMode === "interval" ? "active" : ""}
                      onClick={() => setContinuousInput((current) => ({ ...current, schedule_mode: "interval" }))}
                    >
                      固定间隔
                    </button>
                    <button
                      type="button"
                      className={scheduleMode === "cron" ? "active" : ""}
                      onClick={() => setContinuousInput((current) => ({ ...current, schedule_mode: "cron" }))}
                    >
                      Cron
                    </button>
                  </div>
                </div>
                {scheduleMode === "interval" ? (
                  <label>
                    <span>窗口间隔</span>
                    <select
                      value={continuousInput.interval_sec}
                      onChange={(event) =>
                        setContinuousInput((current) => ({ ...current, interval_sec: Number(event.target.value) }))
                      }
                    >
                      <option value={300}>5 分钟</option>
                      <option value={600}>10 分钟</option>
                      <option value={1800}>30 分钟</option>
                    </select>
                  </label>
                ) : (
                  <label>
                    <span>Cron 表达式</span>
                    <input
                      type="text"
                      value={continuousInput.cron_expression ?? ""}
                      onChange={(event) =>
                        setContinuousInput((current) => ({ ...current, cron_expression: event.target.value }))
                      }
                      placeholder="*/5 * * * *"
                    />
                  </label>
                )}
                <label>
                  <span>错峰启动</span>
                  <select
                    value={continuousInput.stagger_sec}
                    onChange={(event) =>
                      setContinuousInput((current) => ({ ...current, stagger_sec: Number(event.target.value) }))
                    }
                  >
                    <option value={0}>不延迟</option>
                    <option value={15}>15 秒</option>
                    <option value={30}>30 秒</option>
                    <option value={60}>60 秒</option>
                    <option value={120}>120 秒</option>
                  </select>
                </label>
                <div className="schedule-hint-strip">
                  <span>{scheduleMode === "cron" ? "cron" : "interval"}</span>
                  <strong>{previewSchedule(continuousInput)}</strong>
                </div>
              </div>
              <div className="schedule-task-section">
                <div className="form-section-title">采集参数</div>
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
            </div>
          </div>
        </div>
      </section>

      <div className="console-card schedule-window-card">
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
        <div className="window-detail-head">
          <h3>窗口明细</h3>
          <span>{windows.length > 0 ? `${windows.length} 条窗口记录` : "暂无窗口记录"}</span>
        </div>
        <div className="table-scroll">
          <table className="data-table window-table">
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
          <div className="table-scroll trend-table-scroll">
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

function AISettingsPage({
  config,
  input,
  setInput,
  feedback,
  submitting,
  tasks,
  onSubmit,
}: {
  config: AIConfig | null;
  input: UpdateAIConfigInput;
  setInput: React.Dispatch<React.SetStateAction<UpdateAIConfigInput>>;
  feedback: FormFeedback | null;
  submitting: boolean;
  tasks: Task[];
  onSubmit: (event: React.FormEvent<HTMLFormElement>) => void;
}) {
  const aiTaskCount = tasks.filter((task) => task.result?.attribution?.analysis_engine === "ai").length;
  const fallbackTaskCount = tasks.filter((task) => task.result?.attribution?.fallback_reason).length;
  const aiState = config ? aiConfigState(config) : null;

  return (
    <div className="page-stack">
      <section className="page-title-row">
        <div>
          <h1>智能分析</h1>
          <p>配置 OpenAI 兼容的 LLM 归因通道。任务完成后，Server 会用 TopN、火焰图摘要和资源时间线生成可追踪建议。</p>
        </div>
      </section>

      <section className="ai-overview-strip">
        <div>
          <span>当前状态</span>
          <strong>{aiState?.label ?? "加载中"}</strong>
        </div>
        <div>
          <span>API Key</span>
          <strong>{config?.api_key_configured ? "已配置" : "未配置"}</strong>
        </div>
        <div>
          <span>模型</span>
          <strong>{config?.model ?? input.model}</strong>
        </div>
        <div>
          <span>AI 归因任务</span>
          <strong>{aiTaskCount}</strong>
        </div>
        <div>
          <span>规则兜底</span>
          <strong>{fallbackTaskCount}</strong>
        </div>
      </section>

      <section className="content-grid ai-grid">
        <div className="console-card ai-config-card">
          <CardHeader title="LLM 接入配置" />
          <form className="ai-config-form" onSubmit={onSubmit}>
            <div className="ai-toggle-row">
              <div>
                <strong>启用真实 AI 归因</strong>
                <span>关闭时仍会使用内置规则生成建议；开启后必须配置 API Key。</span>
              </div>
              <label className="switch-control">
                <input
                  type="checkbox"
                  checked={input.enabled}
                  onChange={(event) => setInput((current) => ({ ...current, enabled: event.target.checked }))}
                />
                <span />
              </label>
            </div>

            {feedback ? (
              <div className={`ai-form-feedback ${feedback.tone}`} role={feedback.tone === "error" ? "alert" : "status"}>
                {feedback.tone === "error" ? <AlertCircle size={15} /> : <CheckCircle2 size={15} />}
                <span>{feedback.message}</span>
              </div>
            ) : null}

            <label>
              <span>Base URL</span>
              <input
                type="url"
                value={input.base_url}
                onChange={(event) => setInput((current) => ({ ...current, base_url: event.target.value }))}
                placeholder="https://api.openai.com/v1"
              />
            </label>
            <label>
              <span>API Key</span>
              <input
                type="password"
                value={input.api_key}
                onChange={(event) => setInput((current) => ({ ...current, api_key: event.target.value }))}
                placeholder={config?.api_key_configured ? `${config.api_key_display}（留空不会保存旧值）` : "sk-..."}
              />
            </label>
            <div className="form-line">
              <label>
                <span>模型</span>
                <input
                  type="text"
                  value={input.model}
                  onChange={(event) => setInput((current) => ({ ...current, model: event.target.value }))}
                  placeholder="gpt-4o-mini"
                />
              </label>
              <label>
                <span>超时（秒）</span>
                <input
                  type="number"
                  min={3}
                  max={120}
                  value={input.timeout_sec}
                  onChange={(event) => setInput((current) => ({ ...current, timeout_sec: Number(event.target.value) }))}
                />
              </label>
            </div>
            <label>
              <span>最大输出 Token</span>
              <input
                type="number"
                min={128}
                max={4096}
                value={input.max_tokens}
                onChange={(event) => setInput((current) => ({ ...current, max_tokens: Number(event.target.value) }))}
              />
            </label>
            <div className="form-actions">
              <button className="primary-button" type="submit" disabled={submitting}>
                {submitting ? "保存中..." : "保存配置"}
              </button>
            </div>
          </form>
        </div>

        <div className="console-card ai-status-card">
          <CardHeader title="运行状态" />
          <AIConfigSummary config={config} />
        </div>
      </section>

      <section className="console-card ai-explain-card">
        <CardHeader title="归因链路" />
        <div className="ai-flow-grid">
          <div>
            <span>1</span>
            <strong>Analyzer 产物</strong>
            <p>读取 TopN、资源时间线和采集参数。</p>
          </div>
          <div>
            <span>2</span>
            <strong>规则基线</strong>
            <p>先生成可解释的兜底结论和证据。</p>
          </div>
          <div>
            <span>3</span>
            <strong>LLM 归因</strong>
            <p>启用后调用配置模型，返回 JSON 建议。</p>
          </div>
          <div>
            <span>4</span>
            <strong>结果落库</strong>
            <p>Web 详情页展示来源、置信度和工具轨迹。</p>
          </div>
        </div>
      </section>
    </div>
  );
}

function AIConfigSummary({ config, compact = false }: { config: AIConfig | null; compact?: boolean }) {
  if (!config) {
    return <LoadingBlock />;
  }

  const aiState = aiConfigState(config);

  return (
    <div className={`ai-config-summary ${compact ? "compact" : ""}`}>
      <div className="ai-status-banner">
        <div className="attribution-icon">
          <BrainCircuit size={18} />
        </div>
        <div>
          <span>{config.provider}</span>
          <strong>{aiState.summary}</strong>
        </div>
        <span className={`ai-mode-tag ${aiState.tone}`}>{aiState.label}</span>
      </div>

      <dl className="info-list ai-info-list">
        <div>
          <dt>Base URL</dt>
          <dd>{config.base_url}</dd>
        </div>
        <div>
          <dt>Endpoint</dt>
          <dd>{config.endpoint}</dd>
        </div>
        <div>
          <dt>模型</dt>
          <dd>{config.model}</dd>
        </div>
        <div>
          <dt>密钥状态</dt>
          <dd>{config.api_key_configured ? config.api_key_display || "已配置" : "未配置"}</dd>
        </div>
        {!compact ? (
          <>
            <div>
              <dt>超时</dt>
              <dd>{config.timeout_sec}s</dd>
            </div>
            <div>
              <dt>最大 Token</dt>
              <dd>{config.max_tokens}</dd>
            </div>
            <div>
              <dt>配置来源</dt>
              <dd>{config.source}</dd>
            </div>
            <div>
              <dt>更新时间</dt>
              <dd>{config.updated_at ? formatDate(config.updated_at) : "环境变量默认值"}</dd>
            </div>
          </>
        ) : null}
      </dl>

      {!compact && config.notes?.length ? (
        <div className="ai-note-list">
          {config.notes.map((note) => (
            <div key={note}>
              <CheckCircle2 size={14} />
              <span>{note}</span>
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function aiConfigState(config: AIConfig) {
  if (config.enabled && config.api_key_configured) {
    return {
      label: "已启用",
      summary: "真实 AI 归因已启用",
      tone: "enabled",
    };
  }

  if (config.enabled) {
    return {
      label: "待配置",
      summary: "AI 已打开，等待 API Key",
      tone: "warning",
    };
  }

  return {
    label: "规则兜底",
    summary: "当前使用规则兜底",
    tone: "standby",
  };
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
          <option value="">自动选择空闲在线机器</option>
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

      <div className="task-detail-body">
        <section className="task-detail-main">
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
        </section>

        <TaskAdviceRail task={task} />
      </div>
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

function TaskAdviceRail({ task }: { task: Task }) {
  const attribution = task.result?.attribution;
  const recommendations = attribution?.recommendations ?? [];
  const topHotspot = task.result?.hotspots?.[0];

  return (
    <aside className="task-advice-rail">
      <div className="task-advice-head">
        <div className="attribution-icon">
          <BrainCircuit size={18} />
        </div>
        <div>
          <span>{attributionSourceLabel(attribution)}</span>
          <strong>{attribution?.conclusion ?? adviceFallbackTitle(task)}</strong>
        </div>
      </div>

      <dl className="advice-kv">
        <div>
          <dt>来源</dt>
          <dd>{attributionSourceDetail(attribution)}</dd>
        </div>
        <div>
          <dt>置信度</dt>
          <dd>{attribution ? `${Math.round(attribution.confidence * 100)}%` : "-"}</dd>
        </div>
        <div>
          <dt>Top 热点</dt>
          <dd>{topHotspot ? `${topHotspot.function} / ${topHotspot.percent}%` : "-"}</dd>
        </div>
        <div>
          <dt>原始产物</dt>
          <dd>
            <ArtifactLink href={task.raw_artifact_url ?? ""} label={rawArtifactLabel(task)} />
          </dd>
        </div>
      </dl>

      <div className="advice-list">
        {recommendations.length ? (
          recommendations.slice(0, 4).map((item, index) => (
            <div className="advice-item" key={item}>
              <span>{index + 1}</span>
              <p>{item}</p>
            </div>
          ))
        ) : (
          <div className="advice-empty">{adviceFallbackText(task)}</div>
        )}
      </div>
    </aside>
  );
}

function adviceFallbackTitle(task: Task) {
  if (task.status === "DONE") {
    return "分析完成，等待规则结果刷新";
  }
  if (task.status === "FAILED") {
    return "任务失败，请先处理采集条件";
  }
  return "采样链路执行中";
}

function adviceFallbackText(task: Task) {
  if (task.status === "FAILED") {
    return task.status_reason || "检查 PID、Agent 在线状态、采集器权限和命令是否可用。";
  }
  if (task.status === "DONE") {
    return "火焰图和 TopN 已生成；若建议暂未出现，请等待下一次详情刷新。";
  }
  return "任务完成后会在这里显示规则归因、证据摘要和优化建议。";
}

function attributionSourceLabel(attribution?: NonNullable<NonNullable<Task["result"]>["attribution"]> | null) {
  if (!attribution) {
    return "AI / 规则建议";
  }
  return attribution.analysis_engine === "ai" ? "真实 AI 分析" : "规则兜底分析";
}

function attributionSourceDetail(attribution?: NonNullable<NonNullable<Task["result"]>["attribution"]> | null) {
  if (!attribution) {
    return "-";
  }
  if (attribution.analysis_engine === "ai") {
    return attribution.model ? `AI · ${attribution.model}` : "AI";
  }
  if (attribution.fallback_reason) {
    return `规则兜底 · ${attribution.model || "AI 未配置"}`;
  }
  return "规则兜底";
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

  const timeline = attribution.resource_timeline;

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
          <span>{attributionSourceDetail(attribution)}</span>
          <strong>{Math.round(attribution.confidence * 100)}%</strong>
        </div>
      </div>

      {timeline ? <ResourceTimelineView timeline={timeline} path={attribution.source.resource_timeline_path} /> : null}

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
            {attribution.source.resource_timeline_path ? (
              <div>
                <dt>时间线</dt>
                <dd>{attribution.source.resource_timeline_path}</dd>
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

function ResourceTimelineView({
  timeline,
  path,
}: {
  timeline: NonNullable<NonNullable<Task["result"]>["attribution"]>["resource_timeline"];
  path?: string;
}) {
  if (!timeline) {
    return null;
  }
  const maxValue = Math.max(...timeline.points.map((point) => point.value), 1);

  return (
    <section className="resource-timeline-panel">
      <div className="resource-timeline-head">
        <div>
          <span>资源时间线</span>
          <strong>{timeline.summary}</strong>
        </div>
        <div className="resource-timeline-meta">
          <span>{timeline.source}</span>
          <span>{timeline.signal}</span>
          <span>{timeline.alignment}</span>
        </div>
      </div>
      <div className="resource-timeline-chart" aria-label="资源时间线点位">
        {timeline.points.map((point, index) => (
          <div className="resource-timeline-point" key={`${point.offset_sec}-${index}`}>
            <div className="resource-timeline-bar">
              <span style={{ height: `${Math.max((point.value / maxValue) * 100, 4)}%` }} />
            </div>
            <em>{point.offset_sec}s</em>
          </div>
        ))}
      </div>
      <dl className="resource-timeline-stats">
        <div>
          <dt>Top 函数</dt>
          <dd>{timeline.top_function}</dd>
        </div>
        <div>
          <dt>峰值</dt>
          <dd>{timeline.peak_percent}%</dd>
        </div>
        <div>
          <dt>窗口</dt>
          <dd>{timeline.window_sec}s</dd>
        </div>
        <div>
          <dt>来源</dt>
          <dd>{path ?? timeline.source}</dd>
        </div>
      </dl>
    </section>
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

function rawArtifactLabel(task: Task) {
  if (task.collector_type === "perf") {
    return "perf.data";
  }
  if (task.collector_type === "ebpf-syscall") {
    return "ebpf.syscalls.txt";
  }
  if (task.collector_type === "py-spy") {
    return "pyspy.raw.txt";
  }
  return "mock.perf.data.json";
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

function formatSchedulePolicy(profile: ContinuousProfile) {
  if (profile.schedule_mode === "cron") {
    return profile.cron_expression || "cron 未配置";
  }
  return formatSeconds(profile.interval_sec);
}

function formatStagger(seconds: number) {
  if (!seconds) {
    return "无";
  }
  return `+${formatSeconds(seconds)}`;
}

function previewSchedule(input: CreateContinuousProfileInput) {
  const stagger = input.stagger_sec ? `，错峰 ${formatSeconds(input.stagger_sec)}` : "";
  if (input.schedule_mode === "cron") {
    return `${input.cron_expression || "*/5 * * * *"}${stagger}`;
  }
  return `每 ${formatSeconds(input.interval_sec)} 生成窗口${stagger}`;
}

function formatSeconds(seconds: number) {
  if (seconds >= 60 && seconds % 60 === 0) {
    return `${seconds / 60} 分钟`;
  }
  return `${seconds} 秒`;
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

function buildCrossProfileHotspotAggregate(tasks: Task[]) {
  const summary = new Map<string, {
    name: string;
    profileIds: Set<string>;
    taskCount: number;
    totalPercent: number;
    peakPercent: number;
    latestTaskId: string;
    latestCreatedAt: number;
  }>();

  for (const task of tasks) {
    if (!task.continuous_profile_id) {
      continue;
    }
    const createdAt = Date.parse(task.created_at) || 0;
    for (const hotspot of task.result?.hotspots ?? []) {
      const current = summary.get(hotspot.function);
      if (!current) {
        summary.set(hotspot.function, {
          name: hotspot.function,
          profileIds: new Set([task.continuous_profile_id]),
          taskCount: 1,
          totalPercent: Number(hotspot.percent ?? 0),
          peakPercent: Number(hotspot.percent ?? 0),
          latestTaskId: task.id,
          latestCreatedAt: createdAt,
        });
        continue;
      }

      current.profileIds.add(task.continuous_profile_id);
      current.taskCount += 1;
      current.totalPercent += Number(hotspot.percent ?? 0);
      current.peakPercent = Math.max(current.peakPercent, Number(hotspot.percent ?? 0));
      if (createdAt >= current.latestCreatedAt) {
        current.latestTaskId = task.id;
        current.latestCreatedAt = createdAt;
      }
    }
  }

  return Array.from(summary.values())
    .map((item) => ({
      name: item.name,
      profileCount: item.profileIds.size,
      taskCount: item.taskCount,
      averagePercent: item.totalPercent / item.taskCount,
      peakPercent: item.peakPercent,
      latestTaskId: item.latestTaskId,
    }))
    .sort((left, right) => right.profileCount - left.profileCount || right.peakPercent - left.peakPercent)
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
