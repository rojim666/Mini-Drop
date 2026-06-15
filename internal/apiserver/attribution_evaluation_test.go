package apiserver

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"mini-drop/internal/minidrop"
)

type attributionEvaluationCriterion struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

type attributionEvaluationOutput struct {
	ID             string                           `json:"id"`
	Scenario       string                           `json:"scenario"`
	CollectorType  string                           `json:"collector_type"`
	Expected       string                           `json:"expected"`
	TopFunction    string                           `json:"top_function"`
	Conclusion     string                           `json:"conclusion"`
	Confidence     float64                          `json:"confidence"`
	TimelineSource string                           `json:"timeline_source"`
	TimelineSignal string                           `json:"timeline_signal"`
	EvidenceKinds  []string                         `json:"evidence_kinds"`
	ToolNames      []string                         `json:"tool_names"`
	Score          int                              `json:"score"`
	MaxScore       int                              `json:"max_score"`
	Passed         bool                             `json:"passed"`
	Criteria       []attributionEvaluationCriterion `json:"criteria"`
}

func TestAttributionEvaluationSamples(t *testing.T) {
	samples := []struct {
		id                 string
		scenario           string
		collectorType      string
		expected           string
		hotspots           []hotspotPayload
		baselines          []minidrop.AttributionBaseline
		timeline           *resourceTimelinePayload
		timelinePath       string
		wantConclusion     []string
		wantEvidenceKinds  []string
		wantTimelineSource string
		wantTimelineSignal string
		wantToolNames      []string
		wantRecommendation []string
		minConfidence      float64
	}{
		{
			id:            "single_cpu_hotspot",
			scenario:      "single CPU hotspot should produce a direct optimization target",
			collectorType: minidrop.CollectorMockPerf,
			expected:      "single-hotspot CPU attribution",
			hotspots: []hotspotPayload{
				{Function: "main.spinCPU", Samples: 80, Percent: 64},
				{Function: "runtime.nanotime", Samples: 20, Percent: 16},
			},
			wantConclusion:     []string{"单个热点"},
			wantEvidenceKinds:  []string{"top_hotspot", "supporting_hotspot", "resource_timeline", "sampling"},
			wantTimelineSource: "derived_from_profile",
			wantTimelineSignal: "cpu_hotspot",
			wantToolNames:      []string{"get_top_hotspots", "match_hotspot_rules", "compare_with_baseline", "get_resource_timeline"},
			wantRecommendation: []string{"基准测试"},
			minConfidence:      0.85,
		},
		{
			id:            "runtime_scheduler_wait",
			scenario:      "runtime scheduler symbols should point to scheduling or wait amplification",
			collectorType: minidrop.CollectorMockPerf,
			expected:      "runtime/scheduler attribution",
			hotspots: []hotspotPayload{
				{Function: "runtime.schedule", Samples: 35, Percent: 35},
				{Function: "runtime.park_m", Samples: 21, Percent: 21},
			},
			wantConclusion:     []string{"runtime / scheduler"},
			wantEvidenceKinds:  []string{"top_hotspot", "supporting_hotspot", "resource_timeline", "rule_match", "sampling"},
			wantTimelineSource: "derived_from_profile",
			wantTimelineSignal: "scheduler_wait",
			wantToolNames:      []string{"get_top_hotspots", "match_hotspot_rules", "compare_with_baseline", "get_resource_timeline"},
			wantRecommendation: []string{"runtime/scheduler"},
			minConfidence:      0.7,
		},
		{
			id:            "io_baseline_regression",
			scenario:      "storage hotspot above seeded baseline should surface baseline evidence",
			collectorType: minidrop.CollectorMockPerf,
			expected:      "IO/storage attribution with baseline delta",
			hotspots: []hotspotPayload{
				{Function: "storage.writeArtifacts", Samples: 42, Percent: 42},
				{Function: "handler.profileTask", Samples: 20, Percent: 20},
			},
			baselines: []minidrop.AttributionBaseline{
				{
					CollectorType:   minidrop.CollectorMockPerf,
					FunctionPattern: "storage",
					ExpectedPercent: 12,
					Description:     "storage write baseline",
				},
			},
			wantConclusion:     []string{"IO / storage"},
			wantEvidenceKinds:  []string{"top_hotspot", "supporting_hotspot", "baseline", "resource_timeline", "rule_match", "sampling"},
			wantTimelineSource: "derived_from_profile",
			wantTimelineSignal: "io_pressure",
			wantToolNames:      []string{"get_top_hotspots", "match_hotspot_rules", "compare_with_baseline", "get_resource_timeline"},
			wantRecommendation: []string{"IO/storage"},
			minConfidence:      0.74,
		},
		{
			id:            "perf_allocation_with_timeline",
			scenario:      "perf allocation hotspot should keep analyzer-backed timeline evidence",
			collectorType: minidrop.CollectorPerf,
			expected:      "allocation attribution with perf-script timeline",
			hotspots: []hotspotPayload{
				{Function: "malloc", Samples: 32, Percent: 32},
				{Function: "memcpy", Samples: 20, Percent: 20},
			},
			baselines: []minidrop.AttributionBaseline{
				{
					CollectorType:   minidrop.CollectorPerf,
					FunctionPattern: "malloc",
					ExpectedPercent: 10,
					Description:     "native allocation baseline",
				},
			},
			timeline: &resourceTimelinePayload{
				Source:      "perf_script_samples",
				Signal:      "cpu_cycles",
				Alignment:   "cpu",
				Summary:     "perf script samples align with malloc CPU cycles",
				WindowSec:   15,
				TopFunction: "malloc",
				PeakPercent: 91,
				Points: []resourceTimelinePointPayload{
					{OffsetSec: 0, Value: 40, Samples: 4},
					{OffsetSec: 5, Value: 91, Samples: 9},
					{OffsetSec: 10, Value: 53, Samples: 5},
				},
			},
			timelinePath:       "tsk_eval/analysis/resource_timeline.json",
			wantConclusion:     []string{"分配"},
			wantEvidenceKinds:  []string{"top_hotspot", "supporting_hotspot", "baseline", "resource_timeline", "rule_match", "sampling"},
			wantTimelineSource: "perf_script_samples",
			wantTimelineSignal: "cpu_cycles",
			wantToolNames:      []string{"get_top_hotspots", "match_hotspot_rules", "compare_with_baseline", "get_resource_timeline"},
			wantRecommendation: []string{"CPU 利用率"},
			minConfidence:      0.7,
		},
		{
			id:            "mixed_cpu_hotspots",
			scenario:      "mixed symbols should avoid overconfident single-cause attribution",
			collectorType: minidrop.CollectorPerf,
			expected:      "mixed CPU attribution",
			hotspots: []hotspotPayload{
				{Function: "business.route", Samples: 24, Percent: 24},
				{Function: "crypto.hash", Samples: 19, Percent: 19},
				{Function: "template.render", Samples: 15, Percent: 15},
			},
			wantConclusion:     []string{"CPU 热点较分散"},
			wantEvidenceKinds:  []string{"top_hotspot", "supporting_hotspot", "resource_timeline", "rule_match", "sampling"},
			wantTimelineSource: "derived_from_profile",
			wantTimelineSignal: "mixed_cpu",
			wantToolNames:      []string{"get_top_hotspots", "match_hotspot_rules", "compare_with_baseline", "get_resource_timeline"},
			wantRecommendation: []string{"同时检查 CPU"},
			minConfidence:      0.5,
		},
		{
			id:            "ebpf_syscall_distribution",
			scenario:      "eBPF syscall collector should keep syscall timeline alignment",
			collectorType: minidrop.CollectorEBPFSyscall,
			expected:      "syscall-pressure attribution",
			hotspots: []hotspotPayload{
				{Function: "read", Samples: 42, Percent: 42},
				{Function: "write", Samples: 30, Percent: 30},
				{Function: "epoll_wait", Samples: 10, Percent: 10},
			},
			wantConclusion:     []string{"IO / storage"},
			wantEvidenceKinds:  []string{"top_hotspot", "supporting_hotspot", "resource_timeline", "rule_match", "sampling"},
			wantTimelineSource: "derived_from_profile",
			wantTimelineSignal: "syscall_pressure",
			wantToolNames:      []string{"get_top_hotspots", "match_hotspot_rules", "compare_with_baseline", "get_resource_timeline"},
			wantRecommendation: []string{"syscall"},
			minConfidence:      0.74,
		},
	}

	for _, sample := range samples {
		t.Run(sample.id, func(t *testing.T) {
			task := minidrop.Task{
				ID:                "tsk_eval_" + sample.id,
				CollectorType:     sample.collectorType,
				SampleDurationSec: 15,
				SampleRateHz:      99,
			}
			got := buildAttributionWithBaselinesAndTimeline(
				task,
				"tsk_eval_"+sample.id+"/analysis/topn.json",
				sample.hotspots,
				sample.baselines,
				sample.timeline,
				sample.timelinePath,
			)
			if got == nil {
				t.Fatal("expected attribution")
			}

			criteria := []attributionEvaluationCriterion{
				{
					Name:   "conclusion",
					Weight: 2,
					Passed: containsAllSubstrings(got.Conclusion, sample.wantConclusion),
					Detail: fmt.Sprintf("want %q in %q", strings.Join(sample.wantConclusion, ", "), got.Conclusion),
				},
				{
					Name:   "evidence",
					Weight: 2,
					Passed: hasEvidenceKinds(got.Evidence, sample.wantEvidenceKinds),
					Detail: fmt.Sprintf("want %v got %v", sample.wantEvidenceKinds, evidenceKinds(got.Evidence)),
				},
				{
					Name:   "timeline",
					Weight: 2,
					Passed: got.ResourceTimeline != nil &&
						got.ResourceTimeline.Source == sample.wantTimelineSource &&
						got.ResourceTimeline.Signal == sample.wantTimelineSignal &&
						len(got.ResourceTimeline.Points) > 0,
					Detail: timelineDetail(got.ResourceTimeline, sample.wantTimelineSource, sample.wantTimelineSignal),
				},
				{
					Name:   "tool_trace",
					Weight: 2,
					Passed: hasToolNames(got.ToolTrace, sample.wantToolNames),
					Detail: fmt.Sprintf("want %v got %v", sample.wantToolNames, toolNames(got.ToolTrace)),
				},
				{
					Name:   "recommendation",
					Weight: 1,
					Passed: hasRecommendation(got.Recommendations, sample.wantRecommendation),
					Detail: fmt.Sprintf("want %q in %v", strings.Join(sample.wantRecommendation, ", "), got.Recommendations),
				},
				{
					Name:   "confidence",
					Weight: 1,
					Passed: got.Confidence >= sample.minConfidence && got.Confidence <= 1,
					Detail: fmt.Sprintf("want >= %.2f got %.2f", sample.minConfidence, got.Confidence),
				},
			}
			output := attributionEvaluationOutput{
				ID:            sample.id,
				Scenario:      sample.scenario,
				CollectorType: sample.collectorType,
				Expected:      sample.expected,
				TopFunction:   sample.hotspots[0].Function,
				Conclusion:    got.Conclusion,
				Confidence:    got.Confidence,
				EvidenceKinds: evidenceKinds(got.Evidence),
				ToolNames:     toolNames(got.ToolTrace),
				Score:         evaluationScore(criteria),
				MaxScore:      evaluationMaxScore(criteria),
				Passed:        evaluationPassed(criteria),
				Criteria:      criteria,
			}
			if got.ResourceTimeline != nil {
				output.TimelineSource = got.ResourceTimeline.Source
				output.TimelineSignal = got.ResourceTimeline.Signal
			}
			payload, err := json.Marshal(output)
			if err != nil {
				t.Fatalf("marshal evaluation output: %v", err)
			}
			t.Logf("ATTRIBUTION_EVAL %s", payload)

			for _, criterion := range criteria {
				if !criterion.Passed {
					t.Errorf("%s failed: %s", criterion.Name, criterion.Detail)
				}
			}
		})
	}
}

func containsAllSubstrings(value string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}

func hasEvidenceKinds(evidence []attributionEvidencePayload, wanted []string) bool {
	got := map[string]bool{}
	for _, item := range evidence {
		got[item.Kind] = true
	}
	for _, kind := range wanted {
		if !got[kind] {
			return false
		}
	}
	return true
}

func evidenceKinds(evidence []attributionEvidencePayload) []string {
	kinds := make([]string, 0, len(evidence))
	for _, item := range evidence {
		kinds = append(kinds, item.Kind)
	}
	return kinds
}

func hasToolNames(trace []attributionToolCallPayload, wanted []string) bool {
	got := map[string]bool{}
	for _, item := range trace {
		got[item.Name] = true
	}
	for _, name := range wanted {
		if !got[name] {
			return false
		}
	}
	return true
}

func toolNames(trace []attributionToolCallPayload) []string {
	names := make([]string, 0, len(trace))
	for _, item := range trace {
		names = append(names, item.Name)
	}
	return names
}

func hasRecommendation(recommendations []string, wanted []string) bool {
	for _, needle := range wanted {
		found := false
		for _, recommendation := range recommendations {
			if strings.Contains(recommendation, needle) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func timelineDetail(timeline *resourceTimelinePayload, wantSource, wantSignal string) string {
	if timeline == nil {
		return "resource timeline missing"
	}
	return fmt.Sprintf(
		"want source=%s signal=%s with points; got source=%s signal=%s points=%d",
		wantSource,
		wantSignal,
		timeline.Source,
		timeline.Signal,
		len(timeline.Points),
	)
}

func evaluationScore(criteria []attributionEvaluationCriterion) int {
	score := 0
	for _, criterion := range criteria {
		if criterion.Passed {
			score += criterion.Weight
		}
	}
	return score
}

func evaluationMaxScore(criteria []attributionEvaluationCriterion) int {
	total := 0
	for _, criterion := range criteria {
		total += criterion.Weight
	}
	return total
}

func evaluationPassed(criteria []attributionEvaluationCriterion) bool {
	for _, criterion := range criteria {
		if !criterion.Passed {
			return false
		}
	}
	return true
}
