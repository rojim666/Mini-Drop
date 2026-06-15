package apiserver

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"mini-drop/internal/minidrop"

	"gorm.io/gorm"
)

type attributionToolRunner struct {
	baselines []minidrop.AttributionBaseline
	trace     []attributionToolCallPayload
}

type ruleMatch struct {
	Classification string
	Detail         string
}

type baselineComparison struct {
	Matched         bool
	Description     string
	ExpectedPercent float64
	ActualPercent   float64
	DeltaPercent    float64
}

type resourceTimelineComparison struct {
	Source      string
	Signal      string
	Summary     string
	Alignment   string
	WindowSec   int
	TopFunction string
	PeakPercent float64
	Points      []resourceTimelinePointPayload
}

func buildAttribution(task minidrop.Task, topNPath string, hotspots []hotspotPayload) *attributionPayload {
	return buildAttributionWithBaselines(task, topNPath, hotspots, nil)
}

func buildAttributionWithBaselines(task minidrop.Task, topNPath string, hotspots []hotspotPayload, baselines []minidrop.AttributionBaseline) *attributionPayload {
	runner := attributionToolRunner{baselines: baselines}
	return runner.build(task, topNPath, hotspots, nil, "")
}

func buildAttributionWithBaselinesAndTimeline(task minidrop.Task, topNPath string, hotspots []hotspotPayload, baselines []minidrop.AttributionBaseline, timeline *resourceTimelinePayload, timelinePath string) *attributionPayload {
	runner := attributionToolRunner{baselines: baselines}
	return runner.build(task, topNPath, hotspots, timeline, timelinePath)
}

func (runner *attributionToolRunner) build(task minidrop.Task, topNPath string, hotspots []hotspotPayload, externalTimeline *resourceTimelinePayload, timelinePath string) *attributionPayload {
	hotspots = normalizeHotspots(hotspots)
	source := attributionSourcePayload{
		TaskID:               task.ID,
		CollectorType:        task.CollectorType,
		SampleDurationSec:    task.SampleDurationSec,
		SampleRateHz:         task.SampleRateHz,
		TopNPath:             topNPath,
		ResourceTimelinePath: strings.TrimSpace(timelinePath),
	}
	runner.recordTool("get_top_hotspots", fmt.Sprintf("task_id=%s topn_path=%s", task.ID, topNPath), fmt.Sprintf("%d hotspots", len(hotspots)))

	if len(hotspots) == 0 {
		return &attributionPayload{
			Conclusion:      "没有足够的 TopN 样本用于归因",
			Confidence:      0.2,
			AnalysisEngine:  "rule",
			Evidence:        []attributionEvidencePayload{{Kind: "topn", Detail: "topn.json 为空或没有有效样本"}},
			Recommendations: []string{"重新采集更长的采样窗口，或确认目标进程在采样期间有 CPU 活动。"},
			Source:          source,
			ToolTrace:       runner.trace,
			Prompt:          attributionPrompt(task, hotspots, nil, nil),
		}
	}

	top := hotspots[0]
	evidence := []attributionEvidencePayload{{
		Kind:     "top_hotspot",
		Detail:   fmt.Sprintf("Top1 热点占 %.1f%%，样本数 %d", top.Percent, top.Samples),
		Function: top.Function,
		Samples:  top.Samples,
		Percent:  top.Percent,
	}}
	for i := 1; i < len(hotspots) && i < 3; i++ {
		item := hotspots[i]
		evidence = append(evidence, attributionEvidencePayload{
			Kind:     "supporting_hotspot",
			Detail:   fmt.Sprintf("Top%d 热点占 %.1f%%，样本数 %d", i+1, item.Percent, item.Samples),
			Function: item.Function,
			Samples:  item.Samples,
			Percent:  item.Percent,
		})
	}

	rule := runner.matchRules(hotspots)
	baseline := runner.compareBaseline(task, hotspots)
	timeline := runner.getResourceTimeline(task, hotspots, rule.Classification, externalTimeline, timelinePath)
	timelinePayload := timeline.toPayload()
	classification := rule.Classification
	conclusion := "CPU 热点较分散，需要结合火焰图继续定位调用路径"
	recommendations := []string{
		"从火焰图中定位 TopN 函数的上游调用链，优先排查重复调用或循环放大。",
		"对比相同业务路径的历史 baseline，确认热点是否为本次变更引入。",
	}
	confidence := 0.55

	switch classification {
	case "single":
		conclusion = fmt.Sprintf("单个热点 %s 主导本次 CPU 样本", top.Function)
		recommendations = []string{
			"优先审查该函数的算法复杂度、循环次数和锁等待路径。",
			"针对该函数补充压测或基准测试，验证优化前后的样本占比变化。",
		}
		confidence = confidenceFor(top.Percent, 0.82)
	case "runtime":
		conclusion = "热点集中在 runtime / scheduler 路径，疑似调度、协程或空闲等待开销"
		recommendations = []string{
			"检查线程或协程数量、阻塞等待、锁竞争和定时器使用情况。",
			"结合运行时指标或 goroutine/thread dump 复核是否存在调度放大。",
		}
		confidence = confidenceFor(top.Percent, 0.72)
	case "io":
		conclusion = "热点指向 IO / storage / persistence 路径，疑似数据读写链路放大"
		recommendations = []string{
			"检查文件、网络、数据库调用频率，确认是否存在批量不足或重复读写。",
			"在相同任务上补充 IO 观测数据，验证延迟和吞吐是否与 CPU 热点一致。",
		}
		confidence = confidenceFor(top.Percent, 0.7)
	case "alloc":
		conclusion = "热点包含分配或内存管理符号，疑似对象创建、复制或 GC 压力"
		recommendations = []string{
			"检查热点调用链中的临时对象、字符串拼接和大块内存复制。",
			"补充 heap/alloc profile，确认内存分配热点是否与 CPU 热点重合。",
		}
		confidence = confidenceFor(top.Percent, 0.68)
	}

	if baseline.Matched {
		evidence = append(evidence, attributionEvidencePayload{
			Kind:     "baseline",
			Detail:   fmt.Sprintf("%s: 当前 %.1f%%，baseline %.1f%%，偏差 %.1f%%", baseline.Description, baseline.ActualPercent, baseline.ExpectedPercent, baseline.DeltaPercent),
			Function: top.Function,
			Percent:  baseline.DeltaPercent,
		})
		if baseline.DeltaPercent >= 20 {
			recommendations = append(recommendations, "当前热点显著高于 baseline，优先排查近期变更、流量模式或配置差异。")
			confidence = math.Min(0.94, confidence+0.04)
		}
	}

	evidence = append(evidence, attributionEvidencePayload{
		Kind:             "resource_timeline",
		Detail:           timeline.Summary,
		Function:         top.Function,
		Percent:          timeline.PeakPercent,
		ResourceTimeline: &timelinePayload,
	})
	recommendations = append(recommendations, timelineRecommendation(timeline.Signal))

	if classification != "single" {
		evidence = append(evidence, attributionEvidencePayload{
			Kind:   "rule_match",
			Detail: rule.Detail,
		})
	}
	evidence = append(evidence, attributionEvidencePayload{
		Kind:   "sampling",
		Detail: fmt.Sprintf("采样参数为 %dHz / %ds，采集器为 %s", task.SampleRateHz, task.SampleDurationSec, task.CollectorType),
	})

	return &attributionPayload{
		Conclusion:       conclusion,
		Confidence:       roundFloat(confidence, 2),
		AnalysisEngine:   "rule",
		Evidence:         evidence,
		Recommendations:  recommendations,
		Source:           source,
		ResourceTimeline: &timelinePayload,
		ToolTrace:        runner.trace,
		Prompt:           attributionPrompt(task, hotspots, &baseline, &timeline),
	}
}

func (runner *attributionToolRunner) matchRules(hotspots []hotspotPayload) ruleMatch {
	classification := classifyHotspots(hotspots)
	detail := ruleDetail(classification)
	runner.recordTool("match_hotspot_rules", fmt.Sprintf("top=%s", hotspots[0].Function), fmt.Sprintf("%s: %s", classification, detail))
	return ruleMatch{Classification: classification, Detail: detail}
}

func (runner *attributionToolRunner) compareBaseline(task minidrop.Task, hotspots []hotspotPayload) baselineComparison {
	top := hotspots[0]
	for _, baseline := range runner.baselines {
		if baseline.CollectorType != "" && baseline.CollectorType != task.CollectorType {
			continue
		}
		if !strings.Contains(strings.ToLower(top.Function), strings.ToLower(baseline.FunctionPattern)) {
			continue
		}
		comparison := baselineComparison{
			Matched:         true,
			Description:     baseline.Description,
			ExpectedPercent: roundFloat(baseline.ExpectedPercent, 1),
			ActualPercent:   roundFloat(top.Percent, 1),
			DeltaPercent:    roundFloat(top.Percent-baseline.ExpectedPercent, 1),
		}
		runner.recordTool(
			"compare_with_baseline",
			fmt.Sprintf("collector=%s function=%s", task.CollectorType, top.Function),
			fmt.Sprintf("matched expected=%.1f actual=%.1f delta=%.1f", comparison.ExpectedPercent, comparison.ActualPercent, comparison.DeltaPercent),
		)
		return comparison
	}

	runner.recordTool(
		"compare_with_baseline",
		fmt.Sprintf("collector=%s function=%s", task.CollectorType, top.Function),
		"no matching baseline",
	)
	return baselineComparison{}
}

func (runner *attributionToolRunner) getResourceTimeline(task minidrop.Task, hotspots []hotspotPayload, classification string, externalTimeline *resourceTimelinePayload, timelinePath string) resourceTimelineComparison {
	top := hotspots[0]
	duration := task.SampleDurationSec
	if duration <= 0 {
		duration = 1
	}

	if externalTimeline != nil {
		timeline := resourceTimelineComparison{
			Source:      nonEmptyString(externalTimeline.Source, "analyzer_resource_timeline"),
			Signal:      nonEmptyString(externalTimeline.Signal, signalForClassification(classification)),
			Alignment:   nonEmptyString(externalTimeline.Alignment, alignmentForClassification(classification)),
			Summary:     nonEmptyString(externalTimeline.Summary, fmt.Sprintf("%ds 采样窗口读取 analyzer 资源时间线，Top1 为 %s", duration, top.Function)),
			WindowSec:   nonZeroInt(externalTimeline.WindowSec, duration),
			TopFunction: nonEmptyString(externalTimeline.TopFunction, top.Function),
			PeakPercent: roundFloat(nonZeroFloat(externalTimeline.PeakPercent, top.Percent), 1),
			Points:      normalizeTimelinePoints(externalTimeline.Points, duration, top),
		}
		runner.recordTool(
			"get_resource_timeline",
			fmt.Sprintf("task_id=%s source_path=%s source=%s", task.ID, strings.TrimSpace(timelinePath), timeline.Source),
			fmt.Sprintf("%s alignment=%s points=%d peak=%.1f", timeline.Signal, timeline.Alignment, len(timeline.Points), timeline.PeakPercent),
		)
		return timeline
	}

	timeline := resourceTimelineComparison{
		Source:      "derived_from_profile",
		Signal:      "cpu_hotspot",
		Alignment:   "cpu",
		WindowSec:   duration,
		TopFunction: top.Function,
		PeakPercent: roundFloat(top.Percent, 1),
		Summary: fmt.Sprintf(
			"%ds 采样窗口内 CPU 热点与 %s 对齐，Top1 占 %.1f%%",
			duration,
			top.Function,
			top.Percent,
		),
	}

	switch classification {
	case "runtime":
		timeline.Signal = "scheduler_wait"
		timeline.Alignment = "runtime/scheduler"
		timeline.Summary = fmt.Sprintf(
			"%ds 采样窗口内热点集中在 runtime/scheduler，疑似等待或调度时间线与 TopN 对齐",
			duration,
		)
	case "io":
		timeline.Signal = "io_pressure"
		timeline.Alignment = "io/storage"
		timeline.Summary = fmt.Sprintf(
			"%ds 采样窗口内 IO/storage 热点占 %.1f%%，建议对齐磁盘、网络或数据库时间线",
			duration,
			top.Percent,
		)
	case "alloc":
		timeline.Signal = "memory_pressure"
		timeline.Alignment = "allocation/gc"
		timeline.Summary = fmt.Sprintf(
			"%ds 采样窗口内分配/内存管理热点占 %.1f%%，建议对齐 alloc、GC 或 RSS 时间线",
			duration,
			top.Percent,
		)
	case "mixed":
		timeline.Signal = "mixed_cpu"
		timeline.Alignment = "mixed"
		timeline.Summary = fmt.Sprintf(
			"%ds 采样窗口内热点分散，资源时间线需要同时对齐 CPU、IO 和等待指标",
			duration,
		)
	}

	if task.CollectorType == minidrop.CollectorEBPFSyscall {
		timeline.Signal = "syscall_pressure"
		timeline.Alignment = "syscall"
		timeline.Summary = fmt.Sprintf(
			"%ds eBPF syscall 窗口内 %s 占 %.1f%%，优先对齐系统调用频率时间线",
			duration,
			top.Function,
			top.Percent,
		)
	}
	timeline.Points = derivedTimelinePoints(duration, top.Percent, top.Samples)

	runner.recordTool(
		"get_resource_timeline",
		fmt.Sprintf("task_id=%s collector=%s window=%ds classification=%s source=%s", task.ID, task.CollectorType, duration, classification, timeline.Source),
		fmt.Sprintf("%s alignment=%s points=%d peak=%.1f", timeline.Signal, timeline.Alignment, len(timeline.Points), timeline.PeakPercent),
	)
	return timeline
}

func (timeline resourceTimelineComparison) toPayload() resourceTimelinePayload {
	return resourceTimelinePayload{
		Source:      timeline.Source,
		Signal:      timeline.Signal,
		Alignment:   timeline.Alignment,
		Summary:     timeline.Summary,
		WindowSec:   timeline.WindowSec,
		TopFunction: timeline.TopFunction,
		PeakPercent: roundFloat(timeline.PeakPercent, 1),
		Points:      timeline.Points,
	}
}

func timelineRecommendation(signal string) string {
	switch signal {
	case "scheduler_wait":
		return "把 runtime/scheduler 热点与线程、协程、锁等待或 run queue 时间线对齐，确认是否存在等待放大。"
	case "io_pressure":
		return "把 IO/storage 热点与磁盘、网络、数据库请求量和延迟时间线对齐，确认瓶颈是否来自外部资源。"
	case "memory_pressure":
		return "把分配热点与 alloc、GC、RSS 或对象数量时间线对齐，确认是否存在分配速率突增。"
	case "syscall_pressure":
		return "把 syscall 分布与系统调用频率时间线对齐，确认是否存在读写、同步或轮询调用放大。"
	case "mixed_cpu":
		return "资源时间线应同时检查 CPU、IO 和等待指标，避免只按单个热点过早归因。"
	default:
		return "把 CPU 利用率时间线与 TopN 热点窗口对齐，确认热点是否稳定出现在采样峰值附近。"
	}
}

func signalForClassification(classification string) string {
	switch classification {
	case "runtime":
		return "scheduler_wait"
	case "io":
		return "io_pressure"
	case "alloc":
		return "memory_pressure"
	case "mixed":
		return "mixed_cpu"
	default:
		return "cpu_hotspot"
	}
}

func alignmentForClassification(classification string) string {
	switch classification {
	case "runtime":
		return "runtime/scheduler"
	case "io":
		return "io/storage"
	case "alloc":
		return "allocation/gc"
	case "mixed":
		return "mixed"
	default:
		return "cpu"
	}
}

func normalizeTimelinePoints(points []resourceTimelinePointPayload, duration int, top hotspotPayload) []resourceTimelinePointPayload {
	if len(points) == 0 {
		return derivedTimelinePoints(duration, top.Percent, top.Samples)
	}
	normalized := make([]resourceTimelinePointPayload, 0, len(points))
	for _, point := range points {
		if point.OffsetSec < 0 {
			point.OffsetSec = 0
		}
		if point.Value < 0 {
			point.Value = 0
		}
		point.OffsetSec = roundFloat(point.OffsetSec, 1)
		point.Value = roundFloat(point.Value, 1)
		normalized = append(normalized, point)
	}
	return normalized
}

func derivedTimelinePoints(duration int, peakPercent float64, samples int) []resourceTimelinePointPayload {
	if duration <= 0 {
		duration = 1
	}
	buckets := minInt(duration, 12)
	if buckets <= 0 {
		buckets = 1
	}
	width := float64(duration) / float64(buckets)
	center := float64(buckets-1) / 2
	points := make([]resourceTimelinePointPayload, 0, buckets)
	for i := 0; i < buckets; i++ {
		distance := 0.0
		if center > 0 {
			distance = math.Abs(float64(i)-center) / center
		}
		value := peakPercent * (1 - distance*0.45)
		if value < math.Min(peakPercent, 8) {
			value = math.Min(peakPercent, 8)
		}
		pointSamples := 0
		if samples > 0 {
			pointSamples = int(math.Max(1, math.Round(float64(samples)/float64(buckets))))
		}
		points = append(points, resourceTimelinePointPayload{
			OffsetSec: roundFloat(float64(i)*width, 1),
			Value:     roundFloat(value, 1),
			Samples:   pointSamples,
		})
	}
	return points
}

func nonEmptyString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func nonZeroInt(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func nonZeroFloat(value, fallback float64) float64 {
	if value == 0 {
		return fallback
	}
	return value
}

func (runner *attributionToolRunner) recordTool(name, input, output string) {
	runner.trace = append(runner.trace, attributionToolCallPayload{Name: name, Input: input, Output: output})
}

func attributionPrompt(task minidrop.Task, hotspots []hotspotPayload, baseline *baselineComparison, timeline *resourceTimelineComparison) string {
	top := "none"
	if len(hotspots) > 0 {
		top = fmt.Sprintf("%s %.1f%%", hotspots[0].Function, hotspots[0].Percent)
	}
	baselineText := "none"
	if baseline != nil && baseline.Matched {
		baselineText = fmt.Sprintf("%s actual=%.1f expected=%.1f delta=%.1f", baseline.Description, baseline.ActualPercent, baseline.ExpectedPercent, baseline.DeltaPercent)
	}
	timelineText := "none"
	if timeline != nil {
		timelineText = fmt.Sprintf("%s alignment=%s peak=%.1f", timeline.Signal, timeline.Alignment, timeline.PeakPercent)
	}
	return fmt.Sprintf(
		"Use only tool evidence to attribute task %s. collector=%s sample=%dHz/%ds top=%s baseline=%s timeline=%s",
		task.ID,
		task.CollectorType,
		task.SampleRateHz,
		task.SampleDurationSec,
		top,
		baselineText,
		timelineText,
	)
}

func seedAttributionBaselines(db *gorm.DB) error {
	now := time.Now().UTC()
	baselines := []minidrop.AttributionBaseline{
		{
			ID:              "base_runtime_schedule",
			CollectorType:   minidrop.CollectorMockPerf,
			FunctionPattern: "runtime",
			ExpectedPercent: 18,
			Description:     "mock runtime scheduling baseline",
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "base_storage_write",
			CollectorType:   minidrop.CollectorMockPerf,
			FunctionPattern: "storage",
			ExpectedPercent: 12,
			Description:     "mock storage write baseline",
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "base_malloc",
			CollectorType:   minidrop.CollectorPerf,
			FunctionPattern: "malloc",
			ExpectedPercent: 10,
			Description:     "native allocation hotspot baseline",
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}

	for _, baseline := range baselines {
		var existing minidrop.AttributionBaseline
		err := db.First(&existing, "id = ?", baseline.ID).Error
		if err == nil {
			continue
		}
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if err := db.Create(&baseline).Error; err != nil {
			return err
		}
	}
	return nil
}

func attributionPayloadFromRecord(record minidrop.AttributionResult) (*attributionPayload, error) {
	var evidence []attributionEvidencePayload
	if err := decodeJSON([]byte(record.EvidenceJSON), &evidence); err != nil {
		return nil, err
	}
	var recommendations []string
	if err := decodeJSON([]byte(record.RecommendationsJSON), &recommendations); err != nil {
		return nil, err
	}
	var source attributionSourcePayload
	if err := decodeJSON([]byte(record.SourceJSON), &source); err != nil {
		return nil, err
	}
	var trace []attributionToolCallPayload
	if err := decodeJSON([]byte(record.ToolTraceJSON), &trace); err != nil {
		return nil, err
	}
	var timeline *resourceTimelinePayload
	if strings.TrimSpace(record.ResourceTimelineJSON) != "" {
		var decoded resourceTimelinePayload
		if err := decodeJSON([]byte(record.ResourceTimelineJSON), &decoded); err != nil {
			return nil, err
		}
		timeline = &decoded
	}

	persistedAt := record.UpdatedAt
	return &attributionPayload{
		Conclusion:       record.Conclusion,
		Confidence:       record.Confidence,
		AnalysisEngine:   nonEmptyString(record.AnalysisEngine, "rule"),
		Model:            record.Model,
		FallbackReason:   record.FallbackReason,
		Evidence:         evidence,
		Recommendations:  recommendations,
		Source:           source,
		ResourceTimeline: timeline,
		ToolTrace:        trace,
		Prompt:           record.Prompt,
		PersistedAt:      &persistedAt,
	}, nil
}

func attributionRecordFromPayload(taskID string, payload *attributionPayload, now time.Time) (minidrop.AttributionResult, error) {
	evidenceJSON, err := json.Marshal(payload.Evidence)
	if err != nil {
		return minidrop.AttributionResult{}, err
	}
	recommendationsJSON, err := json.Marshal(payload.Recommendations)
	if err != nil {
		return minidrop.AttributionResult{}, err
	}
	sourceJSON, err := json.Marshal(payload.Source)
	if err != nil {
		return minidrop.AttributionResult{}, err
	}
	traceJSON, err := json.Marshal(payload.ToolTrace)
	if err != nil {
		return minidrop.AttributionResult{}, err
	}
	timelineJSON := []byte("{}")
	if payload.ResourceTimeline != nil {
		timelineJSON, err = json.Marshal(payload.ResourceTimeline)
		if err != nil {
			return minidrop.AttributionResult{}, err
		}
	}

	return minidrop.AttributionResult{
		ID:                   minidrop.GenerateID("atr"),
		TaskID:               taskID,
		Conclusion:           payload.Conclusion,
		Confidence:           payload.Confidence,
		AnalysisEngine:       nonEmptyString(payload.AnalysisEngine, "rule"),
		Model:                payload.Model,
		FallbackReason:       payload.FallbackReason,
		EvidenceJSON:         string(evidenceJSON),
		RecommendationsJSON:  string(recommendationsJSON),
		SourceJSON:           string(sourceJSON),
		ResourceTimelineJSON: string(timelineJSON),
		ToolTraceJSON:        string(traceJSON),
		Prompt:               payload.Prompt,
		CreatedAt:            now,
		UpdatedAt:            now,
	}, nil
}

func normalizeHotspots(hotspots []hotspotPayload) []hotspotPayload {
	normalized := make([]hotspotPayload, 0, len(hotspots))
	for _, item := range hotspots {
		if strings.TrimSpace(item.Function) == "" || item.Samples <= 0 {
			continue
		}
		item.Function = strings.TrimSpace(item.Function)
		item.Percent = roundFloat(item.Percent, 1)
		normalized = append(normalized, item)
	}
	return normalized
}

func classifyHotspots(hotspots []hotspotPayload) string {
	if len(hotspots) == 0 {
		return "empty"
	}
	if hotspots[0].Percent >= 50 {
		return "single"
	}

	for _, item := range hotspots[:minInt(len(hotspots), 5)] {
		name := strings.ToLower(item.Function)
		switch {
		case containsAny(name, "runtime.", "scheduler", "schedule", "futex", "pthread", "park", "epoll_wait", "poll"):
			return "runtime"
		case containsAny(name, "read", "write", "recv", "send", "fsync", "sqlite", "mysql", "postgres", "rocksdb", "leveldb", "storage", "disk", "net/http"):
			return "io"
		case containsAny(name, "malloc", "free", "memcpy", "memmove", "newobject", "gc", "alloc", "heap"):
			return "alloc"
		}
	}

	return "mixed"
}

func ruleDetail(classification string) string {
	switch classification {
	case "runtime":
		return "TopN 命中 runtime/scheduler/futex/poll 等调度或等待相关符号"
	case "io":
		return "TopN 命中 read/write/recv/send/db/storage 等 IO 或持久化相关符号"
	case "alloc":
		return "TopN 命中 malloc/free/memcpy/gc/alloc 等内存管理相关符号"
	default:
		return "TopN 未命中专门规则，按混合 CPU 热点处理"
	}
}

func confidenceFor(topPercent float64, base float64) float64 {
	return math.Min(0.9, base+(topPercent/100)*0.12)
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func roundFloat(value float64, places int) float64 {
	factor := math.Pow(10, float64(places))
	return math.Round(value*factor) / factor
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
