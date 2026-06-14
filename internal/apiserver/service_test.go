package apiserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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

func TestTaskTransitionValidation(t *testing.T) {
	if err := minidrop.ValidateTaskTransition(minidrop.TaskStatusPending, minidrop.TaskStatusRunning); err != nil {
		t.Fatalf("expected valid transition: %v", err)
	}
	if err := minidrop.ValidateTaskTransition(minidrop.TaskStatusPending, minidrop.TaskStatusDone); err == nil {
		t.Fatal("expected invalid transition")
	}
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
