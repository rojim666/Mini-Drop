package apiserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mini-drop/internal/minidrop"
)

func TestAIAttributionClientUsesOpenAICompatibleResponse(t *testing.T) {
	var observedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedPath = r.URL.Path
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("expected bearer key, got %q", got)
		}
		var req aiAttributionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "test-model" {
			t.Fatalf("expected model test-model, got %s", req.Model)
		}
		if len(req.Messages) != 2 || !strings.Contains(req.Messages[1].Content, "runtime.schedule") {
			t.Fatalf("expected structured evidence prompt, got %+v", req.Messages)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": `{"conclusion":"AI 判断调度等待是首要瓶颈","confidence":0.87,"recommendations":["检查 goroutine 阻塞点","对齐 run queue 与 CPU 时间线","复核锁竞争"]}`,
					},
				},
			},
		})
	}))
	defer server.Close()

	client := newAIAttributionClient(Config{
		AIEnabled:   true,
		AIAPIKey:    "test-key",
		AIBaseURL:   server.URL + "/v1",
		AIModel:     "test-model",
		AITimeout:   time.Second,
		AIMaxTokens: 256,
	}.withDefaults())
	task := minidrop.Task{ID: "tsk_ai", CollectorType: minidrop.CollectorMockPerf, SampleDurationSec: 15, SampleRateHz: 99}
	hotspots := []hotspotPayload{{Function: "runtime.schedule", Samples: 35, Percent: 35}}
	rulePayload := buildAttribution(task, "tsk_ai/analysis/topn.json", hotspots)

	got, err := client.Analyze(context.Background(), task, rulePayload, hotspots)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if observedPath != "/v1/chat/completions" {
		t.Fatalf("expected chat completions path, got %s", observedPath)
	}
	if got.AnalysisEngine != "ai" || got.Model != "test-model" {
		t.Fatalf("expected AI engine and model, got %+v", got)
	}
	if got.Conclusion != "AI 判断调度等待是首要瓶颈" {
		t.Fatalf("expected AI conclusion, got %q", got.Conclusion)
	}
	if got.Confidence != 0.87 {
		t.Fatalf("expected confidence 0.87, got %v", got.Confidence)
	}
	if len(got.ToolTrace) == 0 || got.ToolTrace[len(got.ToolTrace)-1].Name != "call_ai_model" {
		t.Fatalf("expected AI tool trace, got %+v", got.ToolTrace)
	}
}

func TestBuildAIAttributionFallsBackToRules(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "quota exceeded", http.StatusTooManyRequests)
	}))
	defer server.Close()

	svc := &Service{
		cfg: Config{
			AIEnabled: true,
			AIBaseURL: server.URL,
			AIAPIKey:  "test-key",
			AIModel:   "test-model",
			AITimeout: time.Second,
		}.withDefaults(),
		ai: newAIAttributionClient(Config{
			AIEnabled: true,
			AIBaseURL: server.URL,
			AIAPIKey:  "test-key",
			AIModel:   "test-model",
			AITimeout: time.Second,
		}.withDefaults()),
		log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	task := minidrop.Task{ID: "tsk_fallback", CollectorType: minidrop.CollectorMockPerf, SampleDurationSec: 15, SampleRateHz: 99}
	hotspots := []hotspotPayload{{Function: "runtime.schedule", Samples: 35, Percent: 35}}
	rulePayload := buildAttribution(task, "tsk_fallback/analysis/topn.json", hotspots)

	got := svc.buildAIAttribution(task, rulePayload, hotspots)
	if got.AnalysisEngine != "rule" {
		t.Fatalf("expected rule fallback engine, got %s", got.AnalysisEngine)
	}
	if got.FallbackReason == "" || !strings.Contains(got.FallbackReason, "HTTP 429") {
		t.Fatalf("expected fallback reason with HTTP 429, got %q", got.FallbackReason)
	}
	if len(got.ToolTrace) == 0 || !strings.Contains(got.ToolTrace[len(got.ToolTrace)-1].Output, "fallback_to_rule") {
		t.Fatalf("expected fallback tool trace, got %+v", got.ToolTrace)
	}
}
