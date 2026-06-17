package apiserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"mini-drop/internal/minidrop"
)

type aiAttributionClient struct {
	baseURL   string
	apiKey    string
	model     string
	timeout   time.Duration
	maxTokens int
	http      *http.Client
}

type aiAttributionRequest struct {
	Model          string            `json:"model"`
	Messages       []aiChatMessage   `json:"messages"`
	Temperature    float64           `json:"temperature"`
	MaxTokens      int               `json:"max_tokens,omitempty"`
	ResponseFormat *aiResponseFormat `json:"response_format,omitempty"`
}

type aiChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type aiResponseFormat struct {
	Type string `json:"type"`
}

type aiAttributionResponse struct {
	Choices []struct {
		Message aiChatMessage `json:"message"`
	} `json:"choices"`
}

type aiAttributionOutput struct {
	Conclusion      string   `json:"conclusion"`
	Confidence      float64  `json:"confidence"`
	Recommendations []string `json:"recommendations"`
}

func newAIAttributionClient(cfg Config) *aiAttributionClient {
	if !cfg.AIEnabled || strings.TrimSpace(cfg.AIAPIKey) == "" {
		return nil
	}
	timeout := cfg.AITimeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &aiAttributionClient{
		baseURL:   strings.TrimSpace(cfg.AIBaseURL),
		apiKey:    strings.TrimSpace(cfg.AIAPIKey),
		model:     strings.TrimSpace(cfg.AIModel),
		timeout:   timeout,
		maxTokens: cfg.AIMaxTokens,
		http:      &http.Client{Timeout: timeout},
	}
}

func (client *aiAttributionClient) Analyze(ctx context.Context, task minidrop.Task, rulePayload *attributionPayload, hotspots []hotspotPayload) (*attributionPayload, error) {
	if client == nil {
		return nil, errors.New("AI attribution is not configured")
	}
	if rulePayload == nil {
		return nil, errors.New("rule attribution payload is required")
	}

	requestPayload := aiAttributionRequest{
		Model:       client.model,
		Temperature: 0.2,
		MaxTokens:   client.maxTokens,
		ResponseFormat: &aiResponseFormat{
			Type: "json_object",
		},
		Messages: []aiChatMessage{
			{
				Role: "system",
				Content: strings.Join([]string{
					"You are Mini-Drop's performance analysis assistant.",
					"Use only the provided structured evidence.",
					"Do not invent metrics, files, source code, or business context.",
					"Return strict JSON with conclusion, confidence, and recommendations.",
					"Conclusion and recommendations must be concise Chinese suitable for an operations console.",
				}, " "),
			},
			{
				Role:    "user",
				Content: aiAttributionPrompt(task, rulePayload, hotspots),
			},
		},
	}

	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal AI attribution request: %w", err)
	}

	endpoint, err := chatCompletionsURL(client.baseURL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("create AI attribution request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+client.apiKey)

	resp, err := client.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call AI attribution model: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("read AI attribution response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("AI attribution model returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var decoded aiAttributionResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return nil, fmt.Errorf("decode AI attribution response: %w", err)
	}
	if len(decoded.Choices) == 0 || strings.TrimSpace(decoded.Choices[0].Message.Content) == "" {
		return nil, errors.New("AI attribution response has no message content")
	}

	var output aiAttributionOutput
	if err := json.Unmarshal([]byte(decoded.Choices[0].Message.Content), &output); err != nil {
		return nil, fmt.Errorf("decode AI attribution JSON content: %w", err)
	}
	if strings.TrimSpace(output.Conclusion) == "" || len(output.Recommendations) == 0 {
		return nil, errors.New("AI attribution JSON must include conclusion and recommendations")
	}

	payload := cloneAttributionPayload(rulePayload)
	payload.Conclusion = trimConsoleText(output.Conclusion, 240)
	payload.Confidence = clampConfidence(output.Confidence, rulePayload.Confidence)
	payload.Recommendations = normalizeRecommendations(output.Recommendations, rulePayload.Recommendations)
	payload.AnalysisEngine = "ai"
	payload.Model = client.model
	payload.FallbackReason = ""
	payload.Prompt = requestPayload.Messages[1].Content
	payload.ToolTrace = append(payload.ToolTrace, attributionToolCallPayload{
		Name:   "call_ai_model",
		Input:  fmt.Sprintf("model=%s evidence_items=%d hotspots=%d", client.model, len(rulePayload.Evidence), len(hotspots)),
		Output: "AI attribution JSON accepted",
	})
	return payload, nil
}

func chatCompletionsURL(base string) (string, error) {
	value := strings.TrimSpace(base)
	if value == "" {
		value = "https://api.openai.com/v1"
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("parse MINIDROP_AI_BASE_URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("MINIDROP_AI_BASE_URL must be an absolute URL, got %q", base)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(parsed.Path, "/chat/completions") {
		parsed.Path = path.Join(parsed.Path, "chat/completions")
	}
	return parsed.String(), nil
}

func aiProviderLabel(base string) string {
	lower := strings.ToLower(strings.TrimSpace(base))
	switch {
	case strings.Contains(lower, "openai.com"):
		return "OpenAI compatible"
	case strings.Contains(lower, "azure.com"):
		return "Azure OpenAI compatible"
	case strings.Contains(lower, "localhost"), strings.Contains(lower, "127.0.0.1"):
		return "Local OpenAI compatible"
	case lower == "":
		return "OpenAI compatible"
	default:
		return "Custom OpenAI compatible"
	}
}

func (s *Service) aiEndpointLabel(base string) string {
	endpoint, err := chatCompletionsURL(base)
	if err != nil {
		return err.Error()
	}
	return endpoint
}

func aiAttributionPrompt(task minidrop.Task, rulePayload *attributionPayload, hotspots []hotspotPayload) string {
	type promptEvidence struct {
		Task             minidrop.Task                `json:"task"`
		Hotspots         []hotspotPayload             `json:"hotspots"`
		RuleConclusion   string                       `json:"rule_conclusion"`
		RuleConfidence   float64                      `json:"rule_confidence"`
		Evidence         []attributionEvidencePayload `json:"evidence"`
		ResourceTimeline *resourceTimelinePayload     `json:"resource_timeline,omitempty"`
		ToolTrace        []attributionToolCallPayload `json:"tool_trace"`
		RequiredJSON     map[string]string            `json:"required_json"`
	}

	evidence := promptEvidence{
		Task:             task,
		Hotspots:         hotspots,
		RuleConclusion:   rulePayload.Conclusion,
		RuleConfidence:   rulePayload.Confidence,
		Evidence:         rulePayload.Evidence,
		ResourceTimeline: rulePayload.ResourceTimeline,
		ToolTrace:        rulePayload.ToolTrace,
		RequiredJSON: map[string]string{
			"conclusion":      "string, Chinese, one sentence",
			"confidence":      "number from 0 to 1",
			"recommendations": "array of 3-5 Chinese action items",
		},
	}
	data, err := json.Marshal(evidence)
	if err != nil {
		return "Return JSON using the rule attribution evidence."
	}
	return string(data)
}

func cloneAttributionPayload(input *attributionPayload) *attributionPayload {
	if input == nil {
		return nil
	}
	clone := *input
	clone.Evidence = append([]attributionEvidencePayload(nil), input.Evidence...)
	clone.Recommendations = append([]string(nil), input.Recommendations...)
	clone.ToolTrace = append([]attributionToolCallPayload(nil), input.ToolTrace...)
	return &clone
}

func trimConsoleText(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}

func clampConfidence(value float64, fallback float64) float64 {
	if value <= 0 || value > 1 {
		value = fallback
	}
	if value < 0.05 {
		value = 0.05
	}
	if value > 0.98 {
		value = 0.98
	}
	return roundFloat(value, 2)
}

func normalizeRecommendations(items []string, fallback []string) []string {
	normalized := make([]string, 0, 5)
	seen := map[string]struct{}{}
	for _, item := range items {
		item = trimConsoleText(item, 180)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
		if len(normalized) == 5 {
			break
		}
	}
	if len(normalized) == 0 {
		return fallback
	}
	return normalized
}
