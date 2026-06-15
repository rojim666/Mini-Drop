package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProcessTaskFailsWhenPIDDoesNotExist(t *testing.T) {
	var failedReason string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/fail") {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode fail payload: %v", err)
		}
		failedReason = payload["reason"]
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"task":{"status":"FAILED"}}`))
	}))
	defer server.Close()

	svc, err := New(Config{
		APIBaseURL:        server.URL,
		AgentID:           "agt_test",
		Hostname:          "test-host",
		IP:                "127.0.0.1",
		Version:           "test",
		PythonBin:         "python",
		AnalyzerScript:    "main.py",
		ArtifactDir:       t.TempDir(),
		HeartbeatInterval: time.Hour,
		PollInterval:      time.Hour,
		MockCollectDelay:  time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	err = svc.processTask(context.Background(), apiTask{
		ID:                "tsk_missing_pid",
		TargetPID:         99999999,
		TargetAgentID:     "agt_test",
		SampleDurationSec: 1,
		SampleRateHz:      99,
		CollectorType:     "perf",
		Status:            "RUNNING",
	})
	if err != nil {
		t.Fatalf("process task should report failure to API: %v", err)
	}
	if failedReason != "target pid not found" {
		t.Fatalf("expected target pid failure reason, got %q", failedReason)
	}
}
