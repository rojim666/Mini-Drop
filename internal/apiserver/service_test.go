package apiserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mini-drop/internal/minidrop"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	svc, err := New(ctx, Config{
		Addr:                 "127.0.0.1:0",
		DataDir:              filepath.Join(dir, "data"),
		DBPath:               filepath.Join(dir, "data", "mini-drop.db"),
		ArtifactDir:          filepath.Join(dir, "artifacts"),
		AllowedOrigin:        "http://127.0.0.1:5173",
		OfflineAfter:         30 * time.Second,
		OfflineCheckInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func TestTaskLifecycleHappyPath(t *testing.T) {
	svc := newTestService(t)
	router := svc.Router()

	heartbeat := map[string]any{
		"agent_id": "agt_demo",
		"hostname": "demo-host",
		"ip":       "127.0.0.1",
		"version":  "0.1.0",
	}
	performJSON(t, router, http.MethodPost, "/api/v1/agents/heartbeat", heartbeat, http.StatusOK)

	createBody := map[string]any{
		"target_pid":          1234,
		"target_agent_id":     "agt_demo",
		"sample_duration_sec": 15,
		"sample_rate_hz":      99,
		"collector_type":      "mock-perf",
	}
	createResp := performJSON(t, router, http.MethodPost, "/api/v1/tasks", createBody, http.StatusCreated)
	taskID := createResp["task"].(map[string]any)["id"].(string)

	claimResp := performJSON(t, router, http.MethodGet, "/api/v1/internal/tasks/claim?agent_id=agt_demo", nil, http.StatusOK)
	if got := claimResp["task"].(map[string]any)["status"].(string); got != string(minidrop.TaskStatusRunning) {
		t.Fatalf("expected RUNNING after claim, got %s", got)
	}

	performJSON(t, router, http.MethodPost, "/api/v1/internal/tasks/"+taskID+"/uploading", map[string]any{
		"reason":            "mock artifact ready",
		"raw_artifact_path": "tsk/raw/mock.perf.data",
	}, http.StatusOK)

	performJSON(t, router, http.MethodPost, "/api/v1/internal/tasks/"+taskID+"/complete", map[string]any{
		"reason":            "artifact uploaded and flamegraph generated",
		"raw_artifact_path": "tsk/raw/mock.perf.data",
		"flamegraph_path":   "tsk/analysis/flamegraph.svg",
		"topn_path":         "tsk/analysis/topn.json",
		"summary":           "Synthetic CPU profile ready",
	}, http.StatusOK)

	taskResp := performJSON(t, router, http.MethodGet, "/api/v1/tasks/"+taskID, nil, http.StatusOK)
	task := taskResp["task"].(map[string]any)
	if got := task["status"].(string); got != string(minidrop.TaskStatusDone) {
		t.Fatalf("expected DONE, got %s", got)
	}

	events := task["events"].([]any)
	if len(events) != 4 {
		t.Fatalf("expected 4 status events, got %d", len(events))
	}
}

func TestCreateTaskValidation(t *testing.T) {
	input := minidrop.CreateTaskInput{
		TargetPID:         0,
		SampleDurationSec: 0,
		SampleRateHz:      0,
		CollectorType:     "",
	}
	input.Normalize()
	if err := minidrop.ValidateCreateTaskInput(input); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestCreateTaskValidationAllowsPerfCollector(t *testing.T) {
	input := minidrop.CreateTaskInput{
		TargetPID:         1234,
		SampleDurationSec: 15,
		SampleRateHz:      99,
		CollectorType:     "perf",
	}
	input.Normalize()
	if err := minidrop.ValidateCreateTaskInput(input); err != nil {
		t.Fatalf("expected perf collector to be accepted: %v", err)
	}
}

func TestCreateTaskValidationAllowsEBPFSyscallCollector(t *testing.T) {
	input := minidrop.CreateTaskInput{
		TargetPID:         1234,
		SampleDurationSec: 15,
		SampleRateHz:      99,
		CollectorType:     "ebpf-syscall",
	}
	input.Normalize()
	if err := minidrop.ValidateCreateTaskInput(input); err != nil {
		t.Fatalf("expected ebpf-syscall collector to be accepted: %v", err)
	}
}

func TestCreateTaskValidationAllowsPySpyCollector(t *testing.T) {
	input := minidrop.CreateTaskInput{
		TargetPID:         1234,
		SampleDurationSec: 15,
		SampleRateHz:      99,
		CollectorType:     "py-spy",
	}
	input.Normalize()
	if err := minidrop.ValidateCreateTaskInput(input); err != nil {
		t.Fatalf("expected py-spy collector to be accepted: %v", err)
	}
}

func TestCreateTaskValidationRejectsUnsupportedCollector(t *testing.T) {
	input := minidrop.CreateTaskInput{
		TargetPID:         1234,
		SampleDurationSec: 15,
		SampleRateHz:      99,
		CollectorType:     "ebpf",
	}
	input.Normalize()
	if err := minidrop.ValidateCreateTaskInput(input); err == nil {
		t.Fatal("expected unsupported collector to be rejected")
	}
}

func TestOfflineAuditLifecycle(t *testing.T) {
	svc := newTestService(t)
	router := svc.Router()

	performJSON(t, router, http.MethodPost, "/api/v1/agents/heartbeat", map[string]any{
		"agent_id": "agt_audit",
		"hostname": "demo-host",
		"ip":       "127.0.0.1",
		"version":  "0.1.0",
	}, http.StatusOK)

	oldHeartbeat := time.Now().UTC().Add(-35 * time.Second)
	if err := svc.DB().Model(&minidrop.Agent{}).Where("id = ?", "agt_audit").Update("last_heartbeat_at", oldHeartbeat).Error; err != nil {
		t.Fatalf("age heartbeat: %v", err)
	}

	if err := svc.reconcileOfflineAgents(time.Now().UTC()); err != nil {
		t.Fatalf("reconcile offline agents: %v", err)
	}

	performJSON(t, router, http.MethodPost, "/api/v1/agents/heartbeat", map[string]any{
		"agent_id": "agt_audit",
		"hostname": "demo-host",
		"ip":       "127.0.0.1",
		"version":  "0.1.0",
	}, http.StatusOK)

	resp := performJSON(t, router, http.MethodGet, "/api/v1/audit-logs", nil, http.StatusOK)
	logs := resp["audit_logs"].([]any)
	if len(logs) < 2 {
		t.Fatalf("expected at least 2 audit logs, got %d", len(logs))
	}
}

func TestOfflineAgentFailsActiveTasks(t *testing.T) {
	svc := newTestService(t)
	router := svc.Router()

	performJSON(t, router, http.MethodPost, "/api/v1/agents/heartbeat", map[string]any{
		"agent_id": "agt_offline_tasks",
		"hostname": "demo-host",
		"ip":       "127.0.0.1",
		"version":  "0.1.0",
	}, http.StatusOK)

	createResp := performJSON(t, router, http.MethodPost, "/api/v1/tasks", map[string]any{
		"target_pid":          1234,
		"target_agent_id":     "agt_offline_tasks",
		"sample_duration_sec": 15,
		"sample_rate_hz":      99,
		"collector_type":      "mock-perf",
	}, http.StatusCreated)
	taskID := createResp["task"].(map[string]any)["id"].(string)

	oldHeartbeat := time.Now().UTC().Add(-35 * time.Second)
	if err := svc.DB().Model(&minidrop.Agent{}).Where("id = ?", "agt_offline_tasks").Update("last_heartbeat_at", oldHeartbeat).Error; err != nil {
		t.Fatalf("age heartbeat: %v", err)
	}

	if err := svc.reconcileOfflineAgents(time.Now().UTC()); err != nil {
		t.Fatalf("reconcile offline agents: %v", err)
	}

	taskResp := performJSON(t, router, http.MethodGet, "/api/v1/tasks/"+taskID, nil, http.StatusOK)
	task := taskResp["task"].(map[string]any)
	if got := task["status"].(string); got != string(minidrop.TaskStatusFailed) {
		t.Fatalf("expected FAILED after agent offline, got %s", got)
	}
	if got := task["status_reason"].(string); got != "target agent offline" {
		t.Fatalf("expected offline reason, got %q", got)
	}

	events := task["events"].([]any)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestTaskResultsIncludeRuleAttribution(t *testing.T) {
	svc := newTestService(t)
	router := svc.Router()

	performJSON(t, router, http.MethodPost, "/api/v1/agents/heartbeat", map[string]any{
		"agent_id": "agt_attr",
		"hostname": "demo-host",
		"ip":       "127.0.0.1",
		"version":  "0.1.0",
	}, http.StatusOK)

	createResp := performJSON(t, router, http.MethodPost, "/api/v1/tasks", map[string]any{
		"target_pid":          5678,
		"target_agent_id":     "agt_attr",
		"sample_duration_sec": 15,
		"sample_rate_hz":      99,
		"collector_type":      "mock-perf",
	}, http.StatusCreated)
	taskID := createResp["task"].(map[string]any)["id"].(string)

	performJSON(t, router, http.MethodGet, "/api/v1/internal/tasks/claim?agent_id=agt_attr", nil, http.StatusOK)
	performJSON(t, router, http.MethodPost, "/api/v1/internal/tasks/"+taskID+"/uploading", map[string]any{
		"raw_artifact_path": taskID + "/raw/mock.perf.data.json",
	}, http.StatusOK)

	topNRelPath := filepath.ToSlash(filepath.Join(taskID, "analysis", "topn.json"))
	topNAbsPath := filepath.Join(svc.cfg.ArtifactDir, filepath.FromSlash(topNRelPath))
	if err := os.MkdirAll(filepath.Dir(topNAbsPath), 0o755); err != nil {
		t.Fatalf("create topn dir: %v", err)
	}
	topN := []hotspotPayload{
		{Function: "main.expensiveLoop", Samples: 80, Percent: 64.0},
		{Function: "runtime.mallocgc", Samples: 20, Percent: 16.0},
	}
	data, err := json.Marshal(topN)
	if err != nil {
		t.Fatalf("marshal topn: %v", err)
	}
	if err := os.WriteFile(topNAbsPath, data, 0o644); err != nil {
		t.Fatalf("write topn: %v", err)
	}

	performJSON(t, router, http.MethodPost, "/api/v1/internal/tasks/"+taskID+"/complete", map[string]any{
		"raw_artifact_path": taskID + "/raw/mock.perf.data.json",
		"flamegraph_path":   filepath.ToSlash(filepath.Join(taskID, "analysis", "flamegraph.svg")),
		"topn_path":         topNRelPath,
		"summary":           "Synthetic CPU profile ready",
	}, http.StatusOK)

	resp := performJSON(t, router, http.MethodGet, "/api/v1/tasks/"+taskID+"/results", nil, http.StatusOK)
	result := resp["result"].(map[string]any)
	attribution := result["attribution"].(map[string]any)
	if got := attribution["conclusion"].(string); !strings.Contains(got, "单个热点") {
		t.Fatalf("expected single hotspot conclusion, got %q", got)
	}
	source := attribution["source"].(map[string]any)
	if got := source["task_id"].(string); got != taskID {
		t.Fatalf("expected source task_id %s, got %s", taskID, got)
	}
	evidence := attribution["evidence"].([]any)
	if len(evidence) < 3 {
		t.Fatalf("expected attribution evidence, got %d items", len(evidence))
	}
	foundTimeline := false
	for _, item := range evidence {
		row := item.(map[string]any)
		if row["kind"] == "resource_timeline" {
			foundTimeline = true
			if !strings.Contains(row["detail"].(string), "CPU") {
				t.Fatalf("expected resource timeline detail to mention CPU alignment, got %+v", row)
			}
		}
	}
	if !foundTimeline {
		t.Fatalf("expected resource timeline evidence, got %+v", evidence)
	}
	trace := attribution["tool_trace"].([]any)
	if len(trace) < 4 {
		t.Fatalf("expected attribution tool trace, got %d items", len(trace))
	}
	foundTimelineTool := false
	for _, item := range trace {
		row := item.(map[string]any)
		if row["name"] == "get_resource_timeline" {
			foundTimelineTool = true
		}
	}
	if !foundTimelineTool {
		t.Fatalf("expected get_resource_timeline tool trace, got %+v", trace)
	}
	if prompt := attribution["prompt"].(string); !strings.Contains(prompt, taskID) || !strings.Contains(prompt, "timeline=") {
		t.Fatalf("expected prompt to reference task id, got %q", prompt)
	}

	var persisted minidrop.AttributionResult
	if err := svc.DB().First(&persisted, "task_id = ?", taskID).Error; err != nil {
		t.Fatalf("expected persisted attribution result: %v", err)
	}

	secondResp := performJSON(t, router, http.MethodGet, "/api/v1/tasks/"+taskID+"/results", nil, http.StatusOK)
	secondResult := secondResp["result"].(map[string]any)
	secondAttribution := secondResult["attribution"].(map[string]any)
	if _, ok := secondAttribution["persisted_at"].(string); !ok {
		t.Fatalf("expected second attribution response to include persisted_at: %+v", secondAttribution)
	}
}

func TestBuildAttributionRuleSamples(t *testing.T) {
	task := minidrop.Task{
		ID:                "tsk_eval",
		CollectorType:     "perf",
		SampleDurationSec: 15,
		SampleRateHz:      99,
	}

	tests := []struct {
		name       string
		hotspots   []hotspotPayload
		wantPhrase string
	}{
		{
			name: "single hotspot",
			hotspots: []hotspotPayload{
				{Function: "main.spinCPU", Samples: 60, Percent: 60},
				{Function: "runtime.nanotime", Samples: 10, Percent: 10},
			},
			wantPhrase: "单个热点",
		},
		{
			name: "runtime scheduling",
			hotspots: []hotspotPayload{
				{Function: "runtime.schedule", Samples: 25, Percent: 35},
				{Function: "runtime.park_m", Samples: 15, Percent: 21},
			},
			wantPhrase: "runtime / scheduler",
		},
		{
			name: "io storage",
			hotspots: []hotspotPayload{
				{Function: "sqlite3_step", Samples: 24, Percent: 32},
				{Function: "write", Samples: 18, Percent: 24},
			},
			wantPhrase: "IO / storage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAttribution(task, "tsk_eval/analysis/topn.json", tt.hotspots)
			if got == nil {
				t.Fatal("expected attribution")
			}
			if !strings.Contains(got.Conclusion, tt.wantPhrase) {
				t.Fatalf("expected conclusion to contain %q, got %q", tt.wantPhrase, got.Conclusion)
			}
			if got.Confidence <= 0 || len(got.Evidence) == 0 || len(got.Recommendations) == 0 {
				t.Fatalf("expected confidence, evidence and recommendations, got %+v", got)
			}
		})
	}
}

func TestBuildAttributionIncludesBaselineEvidence(t *testing.T) {
	task := minidrop.Task{
		ID:                "tsk_baseline",
		CollectorType:     minidrop.CollectorMockPerf,
		SampleDurationSec: 15,
		SampleRateHz:      99,
	}
	hotspots := []hotspotPayload{
		{Function: "storage.writeArtifacts", Samples: 42, Percent: 42},
		{Function: "handler.profileTask", Samples: 20, Percent: 20},
	}
	baselines := []minidrop.AttributionBaseline{
		{
			CollectorType:   minidrop.CollectorMockPerf,
			FunctionPattern: "storage",
			ExpectedPercent: 12,
			Description:     "storage baseline",
		},
	}

	got := buildAttributionWithBaselines(task, "tsk_baseline/analysis/topn.json", hotspots, baselines)
	if got == nil {
		t.Fatal("expected attribution")
	}
	if len(got.ToolTrace) < 3 {
		t.Fatalf("expected tool trace, got %+v", got.ToolTrace)
	}
	foundBaseline := false
	for _, item := range got.Evidence {
		if item.Kind == "baseline" && strings.Contains(item.Detail, "storage baseline") {
			foundBaseline = true
		}
	}
	if !foundBaseline {
		t.Fatalf("expected baseline evidence, got %+v", got.Evidence)
	}
	foundTimeline := false
	for _, item := range got.Evidence {
		if item.Kind == "resource_timeline" && strings.Contains(item.Detail, "IO/storage") {
			foundTimeline = true
		}
	}
	if !foundTimeline {
		t.Fatalf("expected IO resource timeline evidence, got %+v", got.Evidence)
	}
	if !strings.Contains(got.Prompt, "baseline=storage baseline") || !strings.Contains(got.Prompt, "timeline=io_pressure") {
		t.Fatalf("expected prompt to include baseline summary, got %q", got.Prompt)
	}
}

func TestTaskTransitionValidation(t *testing.T) {
	if err := minidrop.ValidateTaskTransition(minidrop.TaskStatusPending, minidrop.TaskStatusRunning); err != nil {
		t.Fatalf("expected valid transition: %v", err)
	}
	if err := minidrop.ValidateTaskTransition(minidrop.TaskStatusPending, minidrop.TaskStatusDone); err == nil {
		t.Fatal("expected invalid transition")
	}
}

func TestCreateContinuousProfileCreatesInitialWindow(t *testing.T) {
	svc := newTestService(t)
	router := svc.Router()

	performJSON(t, router, http.MethodPost, "/api/v1/agents/heartbeat", map[string]any{
		"agent_id": "agt_schedule",
		"hostname": "demo-host",
		"ip":       "127.0.0.1",
		"version":  "0.1.0",
	}, http.StatusOK)

	resp := performJSON(t, router, http.MethodPost, "/api/v1/continuous-profiles", map[string]any{
		"name":                "five minute window",
		"target_pid":          4321,
		"target_agent_id":     "agt_schedule",
		"sample_duration_sec": 15,
		"sample_rate_hz":      99,
		"collector_type":      "mock-perf",
		"interval_sec":        300,
	}, http.StatusCreated)

	profile := resp["profile"].(map[string]any)
	if got := profile["window_duration_sec"].(float64); got != 300 {
		t.Fatalf("expected 300 second window, got %v", got)
	}

	profilesResp := performJSON(t, router, http.MethodGet, "/api/v1/continuous-profiles", nil, http.StatusOK)
	profiles := profilesResp["profiles"].([]any)
	if len(profiles) != 1 {
		t.Fatalf("expected 1 continuous profile, got %d", len(profiles))
	}
	profileID := profiles[0].(map[string]any)["id"].(string)

	windowsResp := performJSON(t, router, http.MethodGet, "/api/v1/continuous-profiles/"+profileID+"/windows", nil, http.StatusOK)
	windows := windowsResp["windows"].([]any)
	if len(windows) != 1 {
		t.Fatalf("expected initial window, got %d", len(windows))
	}
	window := windows[0].(map[string]any)
	if got := window["status"].(string); got != string(minidrop.TaskStatusPending) {
		t.Fatalf("expected pending window, got %s", got)
	}
	summary := windowsResp["summary"].(map[string]any)
	if got := summary["total_windows"].(float64); got != 1 {
		t.Fatalf("expected 1 total window, got %v", got)
	}
	if got := summary["pending_windows"].(float64); got != 1 {
		t.Fatalf("expected 1 pending window, got %v", got)
	}
	if got := summary["latest_status"].(string); got != string(minidrop.TaskStatusPending) {
		t.Fatalf("expected latest pending status, got %s", got)
	}
}

func TestContinuousWindowTaskStatusSyncsToWindow(t *testing.T) {
	svc := newTestService(t)
	router := svc.Router()

	performJSON(t, router, http.MethodPost, "/api/v1/agents/heartbeat", map[string]any{
		"agent_id": "agt_schedule_sync",
		"hostname": "demo-host",
		"ip":       "127.0.0.1",
		"version":  "0.1.0",
	}, http.StatusOK)

	createResp := performJSON(t, router, http.MethodPost, "/api/v1/continuous-profiles", map[string]any{
		"name":                "five minute window",
		"target_pid":          4321,
		"target_agent_id":     "agt_schedule_sync",
		"sample_duration_sec": 15,
		"sample_rate_hz":      99,
		"collector_type":      "mock-perf",
		"interval_sec":        300,
	}, http.StatusCreated)
	profileID := createResp["profile"].(map[string]any)["id"].(string)

	windowsResp := performJSON(t, router, http.MethodGet, "/api/v1/continuous-profiles/"+profileID+"/windows", nil, http.StatusOK)
	windows := windowsResp["windows"].([]any)
	window := windows[0].(map[string]any)
	windowID := window["id"].(string)
	taskID := window["task_id"].(string)

	performJSON(t, router, http.MethodGet, "/api/v1/internal/tasks/claim?agent_id=agt_schedule_sync", nil, http.StatusOK)
	performJSON(t, router, http.MethodPost, "/api/v1/internal/tasks/"+taskID+"/uploading", map[string]any{
		"reason":            "continuous window ready",
		"raw_artifact_path": taskID + "/raw/mock.perf.data.json",
	}, http.StatusOK)
	performJSON(t, router, http.MethodPost, "/api/v1/internal/tasks/"+taskID+"/complete", map[string]any{
		"reason":            "artifact uploaded and flamegraph generated",
		"raw_artifact_path": taskID + "/raw/mock.perf.data.json",
		"flamegraph_path":   taskID + "/analysis/flamegraph.svg",
		"topn_path":         taskID + "/analysis/topn.json",
		"summary":           "Synthetic CPU profile ready",
	}, http.StatusOK)

	taskResp := performJSON(t, router, http.MethodGet, "/api/v1/tasks/"+taskID, nil, http.StatusOK)
	task := taskResp["task"].(map[string]any)
	if got := task["continuous_profile_id"].(string); got != profileID {
		t.Fatalf("expected continuous_profile_id %s, got %s", profileID, got)
	}
	if got := task["continuous_window_id"].(string); got != windowID {
		t.Fatalf("expected continuous_window_id %s, got %s", windowID, got)
	}

	windowsResp = performJSON(t, router, http.MethodGet, "/api/v1/continuous-profiles/"+profileID+"/windows", nil, http.StatusOK)
	windows = windowsResp["windows"].([]any)
	if got := windows[0].(map[string]any)["status"].(string); got != string(minidrop.TaskStatusDone) {
		t.Fatalf("expected done window, got %s", got)
	}
	summary := windowsResp["summary"].(map[string]any)
	if got := summary["done_windows"].(float64); got != 1 {
		t.Fatalf("expected 1 done window, got %v", got)
	}
	if got := summary["latest_status"].(string); got != string(minidrop.TaskStatusDone) {
		t.Fatalf("expected latest done status, got %s", got)
	}
	if got := summary["done_ratio"].(float64); got != 1 {
		t.Fatalf("expected done ratio 1, got %v", got)
	}
}

func TestContinuousProfileEnableDisableWritesAudit(t *testing.T) {
	svc := newTestService(t)
	router := svc.Router()

	performJSON(t, router, http.MethodPost, "/api/v1/agents/heartbeat", map[string]any{
		"agent_id": "agt_schedule_toggle",
		"hostname": "demo-host",
		"ip":       "127.0.0.1",
		"version":  "0.1.0",
	}, http.StatusOK)

	createResp := performJSON(t, router, http.MethodPost, "/api/v1/continuous-profiles", map[string]any{
		"name":                "toggle profile",
		"target_pid":          4321,
		"target_agent_id":     "agt_schedule_toggle",
		"sample_duration_sec": 15,
		"sample_rate_hz":      99,
		"collector_type":      "mock-perf",
		"interval_sec":        300,
	}, http.StatusCreated)
	profileID := createResp["profile"].(map[string]any)["id"].(string)

	disableResp := performJSON(t, router, http.MethodPost, "/api/v1/continuous-profiles/"+profileID+"/disable", nil, http.StatusOK)
	if got := disableResp["profile"].(map[string]any)["enabled"].(bool); got {
		t.Fatal("expected disabled profile")
	}

	var disabledAudit minidrop.AuditLog
	if err := svc.db.First(&disabledAudit, "entity_type = ? AND entity_id = ? AND action = ?", "continuous_profile", profileID, "continuous_profile_disabled").Error; err != nil {
		t.Fatalf("expected disabled audit log: %v", err)
	}

	enableResp := performJSON(t, router, http.MethodPost, "/api/v1/continuous-profiles/"+profileID+"/enable", nil, http.StatusOK)
	if got := enableResp["profile"].(map[string]any)["enabled"].(bool); !got {
		t.Fatal("expected enabled profile")
	}

	var enabledAudit minidrop.AuditLog
	if err := svc.db.First(&enabledAudit, "entity_type = ? AND entity_id = ? AND action = ?", "continuous_profile", profileID, "continuous_profile_enabled").Error; err != nil {
		t.Fatalf("expected enabled audit log: %v", err)
	}
}

func TestContinuousWindowFiltersScopeTimeline(t *testing.T) {
	svc := newTestService(t)
	router := svc.Router()

	performJSON(t, router, http.MethodPost, "/api/v1/agents/heartbeat", map[string]any{
		"agent_id": "agt_schedule_filter",
		"hostname": "demo-host",
		"ip":       "127.0.0.1",
		"version":  "0.1.0",
	}, http.StatusOK)

	createResp := performJSON(t, router, http.MethodPost, "/api/v1/continuous-profiles", map[string]any{
		"name":                "filtered windows",
		"target_pid":          4321,
		"target_agent_id":     "agt_schedule_filter",
		"sample_duration_sec": 15,
		"sample_rate_hz":      99,
		"collector_type":      "mock-perf",
		"interval_sec":        300,
	}, http.StatusCreated)
	profileID := createResp["profile"].(map[string]any)["id"].(string)

	if err := svc.db.Where("profile_id = ?", profileID).Delete(&minidrop.ContinuousProfileWindow{}).Error; err != nil {
		t.Fatalf("delete initial window: %v", err)
	}

	base := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	testWindows := []minidrop.ContinuousProfileWindow{
		{
			ID:            "cwin_filter_old_done",
			ProfileID:     profileID,
			TaskID:        "tsk_filter_old_done",
			WindowStartAt: base,
			WindowEndAt:   base.Add(5 * time.Minute),
			Status:        minidrop.TaskStatusDone,
			StatusReason:  "old done window",
			CreatedAt:     base,
			UpdatedAt:     base,
		},
		{
			ID:            "cwin_filter_selected_done",
			ProfileID:     profileID,
			TaskID:        "tsk_filter_selected_done",
			WindowStartAt: base.Add(10 * time.Minute),
			WindowEndAt:   base.Add(15 * time.Minute),
			Status:        minidrop.TaskStatusDone,
			StatusReason:  "selected done window",
			CreatedAt:     base,
			UpdatedAt:     base,
		},
		{
			ID:            "cwin_filter_failed",
			ProfileID:     profileID,
			TaskID:        "tsk_filter_failed",
			WindowStartAt: base.Add(20 * time.Minute),
			WindowEndAt:   base.Add(25 * time.Minute),
			Status:        minidrop.TaskStatusFailed,
			StatusReason:  "failed window",
			CreatedAt:     base,
			UpdatedAt:     base,
		},
	}
	if err := svc.db.Create(&testWindows).Error; err != nil {
		t.Fatalf("seed timeline windows: %v", err)
	}

	path := "/api/v1/continuous-profiles/" + profileID + "/windows?status=DONE&from=" +
		base.Add(5*time.Minute).Format(time.RFC3339) + "&to=" +
		base.Add(15*time.Minute).Format(time.RFC3339) + "&limit=10"
	windowsResp := performJSON(t, router, http.MethodGet, path, nil, http.StatusOK)
	windows := windowsResp["windows"].([]any)
	if len(windows) != 1 {
		t.Fatalf("expected 1 filtered window, got %d", len(windows))
	}
	window := windows[0].(map[string]any)
	if got := window["id"].(string); got != "cwin_filter_selected_done" {
		t.Fatalf("expected selected done window, got %s", got)
	}
	summary := windowsResp["summary"].(map[string]any)
	if got := summary["total_windows"].(float64); got != 1 {
		t.Fatalf("expected filtered total 1, got %v", got)
	}
	if got := summary["latest_status"].(string); got != string(minidrop.TaskStatusDone) {
		t.Fatalf("expected latest done status, got %s", got)
	}

	performJSON(t, router, http.MethodGet, "/api/v1/continuous-profiles/"+profileID+"/windows?status=BOGUS", nil, http.StatusBadRequest)
}

func TestContinuousProfileTrendsAggregateTopNWindows(t *testing.T) {
	svc := newTestService(t)
	router := svc.Router()

	profileID := "cprof_trend"
	base := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	profile := minidrop.ContinuousProfile{
		ID:                profileID,
		Name:              "trend profile",
		TargetPID:         4321,
		TargetAgentID:     "agt_trend",
		SampleDurationSec: 15,
		SampleRateHz:      99,
		CollectorType:     "mock-perf",
		WindowDurationSec: minidrop.ContinuousWindowDurationSec,
		IntervalSec:       300,
		Enabled:           true,
		CreatedAt:         base,
		UpdatedAt:         base,
	}
	if err := svc.db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile: %v", err)
	}

	windows := []minidrop.ContinuousProfileWindow{
		{
			ID:            "cwin_trend_new",
			ProfileID:     profileID,
			TaskID:        "tsk_trend_new",
			WindowStartAt: base.Add(5 * time.Minute),
			WindowEndAt:   base.Add(10 * time.Minute),
			Status:        minidrop.TaskStatusDone,
			StatusReason:  "new window complete",
			CreatedAt:     base,
			UpdatedAt:     base,
		},
		{
			ID:            "cwin_trend_old",
			ProfileID:     profileID,
			TaskID:        "tsk_trend_old",
			WindowStartAt: base,
			WindowEndAt:   base.Add(5 * time.Minute),
			Status:        minidrop.TaskStatusDone,
			StatusReason:  "old window complete",
			CreatedAt:     base,
			UpdatedAt:     base,
		},
	}
	if err := svc.db.Create(&windows).Error; err != nil {
		t.Fatalf("create trend windows: %v", err)
	}

	for _, window := range windows {
		task := minidrop.Task{
			ID:                  window.TaskID,
			TargetPID:           4321,
			TargetAgentID:       "agt_trend",
			SampleDurationSec:   15,
			SampleRateHz:        99,
			CollectorType:       "mock-perf",
			ContinuousProfileID: profileID,
			ContinuousWindowID:  window.ID,
			Status:              minidrop.TaskStatusDone,
			StatusReason:        "done",
			CreatedAt:           window.CreatedAt,
			UpdatedAt:           window.UpdatedAt,
		}
		if err := svc.db.Create(&task).Error; err != nil {
			t.Fatalf("create task %s: %v", task.ID, err)
		}
	}

	newTopNPath := writeTestTopN(t, svc, "tsk_trend_new", []hotspotPayload{
		{Function: "main.expensiveLoop", Samples: 90, Percent: 70.0},
		{Function: "runtime.mallocgc", Samples: 20, Percent: 15.0},
	})
	oldTopNPath := writeTestTopN(t, svc, "tsk_trend_old", []hotspotPayload{
		{Function: "main.expensiveLoop", Samples: 40, Percent: 30.0},
		{Function: "storage.writeArtifacts", Samples: 25, Percent: 42.0},
	})

	results := []minidrop.AnalysisResult{
		{
			ID:             "res_trend_new",
			TaskID:         "tsk_trend_new",
			FlamegraphPath: "tsk_trend_new/analysis/flamegraph.svg",
			TopNPath:       newTopNPath,
			Summary:        "new window",
			CreatedAt:      base,
			UpdatedAt:      base,
		},
		{
			ID:             "res_trend_old",
			TaskID:         "tsk_trend_old",
			FlamegraphPath: "tsk_trend_old/analysis/flamegraph.svg",
			TopNPath:       oldTopNPath,
			Summary:        "old window",
			CreatedAt:      base,
			UpdatedAt:      base,
		},
	}
	if err := svc.db.Create(&results).Error; err != nil {
		t.Fatalf("create analysis results: %v", err)
	}

	resp := performJSON(t, router, http.MethodGet, "/api/v1/continuous-profiles/"+profileID+"/trends?limit=2", nil, http.StatusOK)
	trendWindows := resp["windows"].([]any)
	if len(trendWindows) != 2 {
		t.Fatalf("expected 2 trend windows, got %d", len(trendWindows))
	}
	if got := trendWindows[0].(map[string]any)["window_id"].(string); got != "cwin_trend_new" {
		t.Fatalf("expected newest window first, got %s", got)
	}

	series := resp["series"].([]any)
	if len(series) == 0 {
		t.Fatal("expected trend series")
	}
	first := series[0].(map[string]any)
	if got := first["function"].(string); got != "main.expensiveLoop" {
		t.Fatalf("expected expensive loop trend first, got %s", got)
	}
	if got := first["delta"].(float64); got != 40 {
		t.Fatalf("expected delta 40, got %v", got)
	}
	if got := first["label"].(string); got != "持续高位" {
		t.Fatalf("expected high trend label, got %s", got)
	}
	if got := first["severity"].(string); got != "critical" {
		t.Fatalf("expected critical severity, got %s", got)
	}
	points := first["points"].([]any)
	if got := points[0].(map[string]any)["percent"].(float64); got != 70 {
		t.Fatalf("expected newest percent 70, got %v", got)
	}

	var storageSeries map[string]any
	for _, item := range series {
		seriesItem := item.(map[string]any)
		if seriesItem["function"].(string) == "storage.writeArtifacts" {
			storageSeries = seriesItem
			break
		}
	}
	if storageSeries == nil {
		t.Fatalf("expected storage trend series, got %+v", series)
	}
	baseline := storageSeries["baseline"].(map[string]any)
	if got := baseline["status"].(string); got != "above" {
		t.Fatalf("expected storage trend above baseline, got %s", got)
	}
	if got := baseline["expected_percent"].(float64); got != 12 {
		t.Fatalf("expected storage baseline 12, got %v", got)
	}
	if got := baseline["actual_percent"].(float64); got != 42 {
		t.Fatalf("expected storage peak 42, got %v", got)
	}
	if got := baseline["delta_percent"].(float64); got != 30 {
		t.Fatalf("expected storage baseline delta 30, got %v", got)
	}

	performJSON(t, router, http.MethodGet, "/api/v1/continuous-profiles/"+profileID+"/trends?limit=bogus", nil, http.StatusBadRequest)
}

func TestListTasksIncludesDoneTaskResultsForAggregation(t *testing.T) {
	svc := newTestService(t)
	router := svc.Router()
	now := time.Now().UTC()
	task := minidrop.Task{
		ID:                "tsk_list_result",
		TargetPID:         4321,
		TargetAgentID:     "agt_list",
		SampleDurationSec: 15,
		SampleRateHz:      99,
		CollectorType:     "mock-perf",
		Status:            minidrop.TaskStatusDone,
		StatusReason:      "done",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := svc.db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	topNPath := writeTestTopN(t, svc, task.ID, []hotspotPayload{
		{Function: "storage.writeArtifacts", Samples: 42, Percent: 42.0},
	})
	if err := svc.db.Create(&minidrop.AnalysisResult{
		ID:             "res_list_result",
		TaskID:         task.ID,
		FlamegraphPath: "tsk_list_result/analysis/flamegraph.svg",
		TopNPath:       topNPath,
		Summary:        "done",
		CreatedAt:      now,
		UpdatedAt:      now,
	}).Error; err != nil {
		t.Fatalf("create analysis result: %v", err)
	}

	resp := performJSON(t, router, http.MethodGet, "/api/v1/tasks", nil, http.StatusOK)
	tasks := resp["tasks"].([]any)
	if len(tasks) != 1 {
		t.Fatalf("expected one task, got %d", len(tasks))
	}
	result := tasks[0].(map[string]any)["result"].(map[string]any)
	hotspots := result["hotspots"].([]any)
	if got := hotspots[0].(map[string]any)["function"].(string); got != "storage.writeArtifacts" {
		t.Fatalf("expected list result hotspot, got %s", got)
	}
}

func TestClassifyTrendUsesFineGrainedThresholds(t *testing.T) {
	tests := []struct {
		name         string
		peak         float64
		delta        float64
		wantLabel    string
		wantSeverity string
	}{
		{name: "critical high peak", peak: 50, delta: 0, wantLabel: "持续高位", wantSeverity: "critical"},
		{name: "elevated peak", peak: 35, delta: 0, wantLabel: "基线偏高", wantSeverity: "warning"},
		{name: "rising", peak: 20, delta: 15, wantLabel: "明显升高", wantSeverity: "warning"},
		{name: "falling", peak: 20, delta: -15, wantLabel: "明显回落", wantSeverity: "success"},
		{name: "minor change", peak: 20, delta: 8, wantLabel: "小幅波动", wantSeverity: "normal"},
		{name: "stable", peak: 20, delta: 7.9, wantLabel: "平稳", wantSeverity: "normal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			label, severity, reason := classifyTrend(tt.peak, tt.delta)
			if label != tt.wantLabel {
				t.Fatalf("expected label %s, got %s", tt.wantLabel, label)
			}
			if severity != tt.wantSeverity {
				t.Fatalf("expected severity %s, got %s", tt.wantSeverity, severity)
			}
			if reason == "" {
				t.Fatal("expected non-empty reason")
			}
		})
	}
}

func writeTestTopN(t *testing.T, svc *Service, taskID string, hotspots []hotspotPayload) string {
	t.Helper()
	topNRelPath := filepath.ToSlash(filepath.Join(taskID, "analysis", "topn.json"))
	topNAbsPath := filepath.Join(svc.cfg.ArtifactDir, filepath.FromSlash(topNRelPath))
	if err := os.MkdirAll(filepath.Dir(topNAbsPath), 0o755); err != nil {
		t.Fatalf("create topn dir: %v", err)
	}
	data, err := json.Marshal(hotspots)
	if err != nil {
		t.Fatalf("marshal topn: %v", err)
	}
	if err := os.WriteFile(topNAbsPath, data, 0o644); err != nil {
		t.Fatalf("write topn: %v", err)
	}
	return topNRelPath
}

func performJSON(t *testing.T, router http.Handler, method, path string, body any, expected int) map[string]any {
	t.Helper()

	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != expected {
		t.Fatalf("expected status %d, got %d: %s", expected, rec.Code, rec.Body.String())
	}

	if rec.Body.Len() == 0 {
		return map[string]any{}
	}

	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return decoded
}
