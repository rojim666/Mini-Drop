export interface Agent {
  id: string;
  hostname: string;
  ip: string;
  version: string;
  status: "UNKNOWN" | "ONLINE" | "OFFLINE";
  last_heartbeat_at: string;
}

export interface UserProfile {
  username: string;
  tenant: string;
  region: string;
}

export interface LoginRequest {
  username: string;
  password: string;
  tenant: string;
  region: string;
}

export interface LoginResponse {
  token: string;
  expires_at: string;
  user: UserProfile;
}

export interface TaskEvent {
  id: string;
  from_status: string;
  to_status: string;
  reason: string;
  created_at: string;
}

export interface Hotspot {
  function: string;
  samples: number;
  percent: number;
}

export interface AttributionEvidence {
  kind: string;
  detail: string;
  function?: string;
  samples?: number;
  percent?: number;
  resource_timeline?: ResourceTimeline;
}

export interface AttributionSource {
  task_id: string;
  collector_type: string;
  sample_duration_sec: number;
  sample_rate_hz: number;
  topn_path: string;
  resource_timeline_path?: string;
}

export interface ResourceTimelinePoint {
  offset_sec: number;
  value: number;
  samples: number;
}

export interface ResourceTimeline {
  source: string;
  signal: string;
  alignment: string;
  summary: string;
  window_sec: number;
  top_function: string;
  peak_percent: number;
  points: ResourceTimelinePoint[];
}

export interface AttributionResult {
  conclusion: string;
  confidence: number;
  analysis_engine: "ai" | "rule" | string;
  model?: string;
  fallback_reason?: string;
  evidence: AttributionEvidence[];
  recommendations: string[];
  source: AttributionSource;
  resource_timeline?: ResourceTimeline;
  tool_trace?: AttributionToolCall[];
  prompt?: string;
  persisted_at?: string;
}

export interface AttributionToolCall {
  name: string;
  input: string;
  output: string;
}

export interface TaskResult {
  flamegraph_url: string;
  topn_url: string;
  summary: string;
  hotspots: Hotspot[];
  attribution?: AttributionResult | null;
}

export interface Task {
  id: string;
  target_pid: number;
  target_agent_id: string;
  sample_duration_sec: number;
  sample_rate_hz: number;
  collector_type: string;
  continuous_profile_id?: string;
  continuous_window_id?: string;
  status: "PENDING" | "RUNNING" | "UPLOADING" | "DONE" | "FAILED";
  status_reason: string;
  created_at: string;
  updated_at: string;
  started_at?: string;
  finished_at?: string;
  raw_artifact_url?: string;
  analysis_artifact_url?: string;
  events?: TaskEvent[];
  result?: TaskResult | null;
}

export interface AuditLog {
  id: string;
  entity_type: string;
  entity_id: string;
  action: string;
  reason: string;
  created_at: string;
}

export interface CreateTaskInput {
  target_pid: number;
  target_agent_id?: string;
  sample_duration_sec: number;
  sample_rate_hz: number;
  collector_type: string;
}

export interface ContinuousProfile {
  id: string;
  name: string;
  target_pid: number;
  target_agent_id: string;
  sample_duration_sec: number;
  sample_rate_hz: number;
  collector_type: string;
  window_duration_sec: number;
  interval_sec: number;
  schedule_mode: "interval" | "cron";
  cron_expression: string;
  stagger_sec: number;
  enabled: boolean;
  last_window_start_at?: string;
  last_scheduled_at?: string;
  created_at: string;
  updated_at: string;
}

export interface ContinuousWindow {
  id: string;
  profile_id: string;
  task_id: string;
  window_start_at: string;
  window_end_at: string;
  status: "PENDING" | "RUNNING" | "UPLOADING" | "DONE" | "FAILED";
  status_reason: string;
  created_at: string;
  updated_at: string;
}

export interface ContinuousWindowSummary {
  total_windows: number;
  done_windows: number;
  failed_windows: number;
  running_windows: number;
  pending_windows: number;
  latest_status: "NONE" | "PENDING" | "RUNNING" | "UPLOADING" | "DONE" | "FAILED";
  latest_status_reason: string;
  latest_window_start_at?: string;
  latest_window_end_at?: string;
  done_ratio: number;
}

export interface ContinuousWindowFilters {
  status?: "ALL" | "PENDING" | "RUNNING" | "UPLOADING" | "DONE" | "FAILED";
  from?: string;
  to?: string;
  limit?: number;
}

export interface ContinuousTrendWindow {
  window_id: string;
  task_id: string;
  window_start_at: string;
  window_end_at: string;
  status: "DONE";
}

export interface ContinuousTrendPoint {
  window_id: string;
  task_id: string;
  percent: number;
  samples: number;
}

export interface ContinuousTrendBaseline {
  status: "above" | "within" | "below";
  description: string;
  expected_percent: number;
  actual_percent: number;
  delta_percent: number;
  reason: string;
}

export interface ContinuousTrendSeries {
  function: string;
  average: number;
  peak: number;
  delta: number;
  label: string;
  severity: "critical" | "warning" | "success" | "normal";
  reason: string;
  baseline?: ContinuousTrendBaseline;
  points: ContinuousTrendPoint[];
}

export interface ContinuousTrend {
  windows: ContinuousTrendWindow[];
  series: ContinuousTrendSeries[];
}

export interface CreateContinuousProfileInput {
  name: string;
  target_pid: number;
  target_agent_id?: string;
  sample_duration_sec: number;
  sample_rate_hz: number;
  collector_type: string;
  interval_sec: number;
  schedule_mode: "interval" | "cron";
  cron_expression?: string;
  stagger_sec: number;
}
