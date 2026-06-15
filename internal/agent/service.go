package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type Service struct {
	cfg        Config
	client     *http.Client
	log        *slog.Logger
	processing atomic.Bool
}

type apiTask struct {
	ID                string `json:"id"`
	TargetPID         int    `json:"target_pid"`
	TargetAgentID     string `json:"target_agent_id"`
	SampleDurationSec int    `json:"sample_duration_sec"`
	SampleRateHz      int    `json:"sample_rate_hz"`
	CollectorType     string `json:"collector_type"`
	Status            string `json:"status"`
}

type taskEnvelope struct {
	Task apiTask `json:"task"`
}

type analyzerResult struct {
	FlamegraphPath string `json:"flamegraph_path"`
	TopNPath       string `json:"topn_path"`
	Summary        string `json:"summary"`
}

func New(cfg Config) (*Service, error) {
	if err := os.MkdirAll(cfg.ArtifactDir, 0o755); err != nil {
		return nil, fmt.Errorf("create artifact dir: %w", err)
	}

	return &Service{
		cfg:    cfg,
		client: &http.Client{Timeout: 15 * time.Second},
		log:    slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}, nil
}

func (s *Service) Run(ctx context.Context) error {
	if err := s.sendHeartbeat(ctx); err != nil {
		s.log.Warn("initial heartbeat failed", "error", err)
	}

	heartbeatTicker := time.NewTicker(s.cfg.HeartbeatInterval)
	defer heartbeatTicker.Stop()

	pollTicker := time.NewTicker(s.cfg.PollInterval)
	defer pollTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-heartbeatTicker.C:
			if err := s.sendHeartbeat(ctx); err != nil {
				s.log.Warn("heartbeat failed", "error", err)
			}
		case <-pollTicker.C:
			if s.processing.Load() {
				continue
			}
			if err := s.claimContinuousProfileWindow(ctx); err != nil {
				s.log.Warn("claim continuous profile window failed", "error", err)
			}
			task, ok, err := s.claimTask(ctx)
			if err != nil {
				s.log.Warn("claim task failed", "error", err)
				continue
			}
			if !ok {
				continue
			}

			s.processing.Store(true)
			go func(task apiTask) {
				defer s.processing.Store(false)
				if err := s.processTask(ctx, task); err != nil {
					s.log.Error("process task failed", "task_id", task.ID, "error", err)
				}
			}(task)
		}
	}
}

func (s *Service) sendHeartbeat(ctx context.Context) error {
	body := map[string]string{
		"agent_id": s.cfg.AgentID,
		"hostname": s.cfg.Hostname,
		"ip":       s.cfg.IP,
		"version":  s.cfg.Version,
	}
	return s.postJSON(ctx, "/api/v1/agents/heartbeat", body, nil)
}

func (s *Service) claimTask(ctx context.Context) (apiTask, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.APIBaseURL+"/api/v1/internal/tasks/claim?agent_id="+s.cfg.AgentID, nil)
	if err != nil {
		return apiTask{}, false, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return apiTask{}, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return apiTask{}, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(resp.Body)
		return apiTask{}, false, fmt.Errorf("claim task: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	var envelope taskEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return apiTask{}, false, err
	}

	return envelope.Task, true, nil
}

func (s *Service) claimContinuousProfileWindow(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.APIBaseURL+"/api/v1/internal/continuous-profiles/claim?agent_id="+s.cfg.AgentID, nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusCreated {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("claim continuous profile window: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	return nil
}

func (s *Service) processTask(ctx context.Context, task apiTask) error {
	started := time.Now()
	s.log.Info(
		"task processing started",
		"task_id", task.ID,
		"agent_id", s.cfg.AgentID,
		"collector_type", task.CollectorType,
		"target_pid", task.TargetPID,
		"sample_rate_hz", task.SampleRateHz,
		"sample_duration_sec", task.SampleDurationSec,
	)
	if ok, err := processExists(task.TargetPID); err != nil {
		reason := fmt.Sprintf("process existence check failed: %v", err)
		s.log.Error("task failed before collection", "task_id", task.ID, "reason", reason)
		return s.failTask(ctx, task.ID, reason, "")
	} else if !ok {
		s.log.Error("task failed before collection", "task_id", task.ID, "reason", "target pid not found")
		return s.failTask(ctx, task.ID, "target pid not found", "")
	}

	rawRelPath, rawAbsPath, uploadReason, err := s.collectTask(ctx, task)
	if err != nil {
		s.log.Error("task collection failed", "task_id", task.ID, "collector_type", task.CollectorType, "error", err)
		return s.failTask(ctx, task.ID, err.Error(), "")
	}
	s.log.Info(
		"task collection completed",
		"task_id", task.ID,
		"collector_type", task.CollectorType,
		"raw_artifact_path", rawRelPath,
		"duration_ms", time.Since(started).Milliseconds(),
	)

	if err := s.postJSON(ctx, "/api/v1/internal/tasks/"+task.ID+"/uploading", map[string]string{
		"reason":            uploadReason,
		"raw_artifact_path": rawRelPath,
	}, nil); err != nil {
		return err
	}
	s.log.Info("task marked uploading", "task_id", task.ID, "reason", uploadReason, "raw_artifact_path", rawRelPath)

	result, err := s.runAnalyzer(ctx, task, rawAbsPath)
	if err != nil {
		s.log.Error("task analysis failed", "task_id", task.ID, "raw_artifact_path", rawRelPath, "error", err)
		return s.failTask(ctx, task.ID, fmt.Sprintf("analyzer failed: %v", err), rawRelPath)
	}
	s.log.Info(
		"task analysis completed",
		"task_id", task.ID,
		"flamegraph_path", result.FlamegraphPath,
		"topn_path", result.TopNPath,
	)

	if err := s.postJSON(ctx, "/api/v1/internal/tasks/"+task.ID+"/complete", map[string]string{
		"reason":            "artifact uploaded and flamegraph generated",
		"raw_artifact_path": rawRelPath,
		"flamegraph_path":   result.FlamegraphPath,
		"topn_path":         result.TopNPath,
		"summary":           result.Summary,
	}, nil); err != nil {
		return err
	}

	s.log.Info("task processing completed", "task_id", task.ID, "duration_ms", time.Since(started).Milliseconds())
	return nil
}

func (s *Service) collectTask(ctx context.Context, task apiTask) (string, string, string, error) {
	switch strings.ToLower(strings.TrimSpace(task.CollectorType)) {
	case "", "mock-perf":
		time.Sleep(s.cfg.MockCollectDelay)
		rawRelPath, rawAbsPath, err := s.writeMockArtifact(task)
		if err != nil {
			return "", "", "", fmt.Errorf("write mock artifact: %w", err)
		}
		return rawRelPath, rawAbsPath, "mock collector finished", nil
	case "perf":
		return s.runPerfCollector(ctx, task)
	case "ebpf-syscall":
		return s.runEBPFSyscallCollector(ctx, task)
	case "py-spy":
		return s.runPySpyCollector(ctx, task)
	default:
		return "", "", "", fmt.Errorf("unsupported collector_type %q", task.CollectorType)
	}
}

func (s *Service) writeMockArtifact(task apiTask) (string, string, error) {
	artifactDir := filepath.Join(s.cfg.ArtifactDir, task.ID, "raw")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return "", "", err
	}

	rawAbsPath := filepath.Join(artifactDir, "mock.perf.data.json")
	payload := map[string]any{
		"task_id":             task.ID,
		"target_pid":          task.TargetPID,
		"collector_type":      task.CollectorType,
		"sample_duration_sec": task.SampleDurationSec,
		"sample_rate_hz":      task.SampleRateHz,
		"captured_at":         time.Now().UTC().Format(time.RFC3339),
		"frames": []map[string]any{
			{"stack": []string{"root", "server.main", "http.serve", "runtime.pollWork"}, "samples": 44},
			{"stack": []string{"root", "server.main", "http.serve", "handler.profileTask"}, "samples": 31},
			{"stack": []string{"root", "server.main", "storage.writeArtifacts"}, "samples": 19},
			{"stack": []string{"root", "runtime.schedule", "runtime.findrunnable"}, "samples": 11},
			{"stack": []string{"root", "server.main", "db.persistStatus"}, "samples": 9},
		},
	}

	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(rawAbsPath, content, 0o644); err != nil {
		return "", "", err
	}

	rawRelPath := filepath.ToSlash(filepath.Join(task.ID, "raw", "mock.perf.data.json"))
	return rawRelPath, rawAbsPath, nil
}

func (s *Service) runAnalyzer(ctx context.Context, task apiTask, rawAbsPath string) (analyzerResult, error) {
	outputDir := filepath.Join(s.cfg.ArtifactDir, task.ID, "analysis")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return analyzerResult{}, err
	}

	cmd := exec.CommandContext(
		ctx,
		s.cfg.PythonBin,
		s.cfg.AnalyzerScript,
		"--task-id", task.ID,
		"--raw-path", rawAbsPath,
		"--output-dir", outputDir,
		"--target-pid", strconv.Itoa(task.TargetPID),
		"--sample-rate", strconv.Itoa(task.SampleRateHz),
		"--sample-duration", strconv.Itoa(task.SampleDurationSec),
	)
	cmd.Dir = filepath.Dir(filepath.Dir(s.cfg.AnalyzerScript))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		details := strings.TrimSpace(stderr.String())
		if details == "" {
			details = strings.TrimSpace(stdout.String())
		}
		if details == "" {
			details = err.Error()
		}
		return analyzerResult{}, errors.New(details)
	}

	var result analyzerResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return analyzerResult{}, fmt.Errorf("decode analyzer output: %w", err)
	}

	return result, nil
}

func (s *Service) failTask(ctx context.Context, taskID, reason, rawArtifactPath string) error {
	body := map[string]string{
		"reason": reason,
	}
	if rawArtifactPath != "" {
		body["raw_artifact_path"] = rawArtifactPath
	}
	return s.postJSON(ctx, "/api/v1/internal/tasks/"+taskID+"/fail", body, nil)
}

func (s *Service) postJSON(ctx context.Context, endpoint string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.APIBaseURL+endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		content, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s returned %d: %s", endpoint, resp.StatusCode, strings.TrimSpace(string(content)))
	}

	if out == nil || resp.ContentLength == 0 {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func processExists(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}

	if runtime.GOOS == "windows" {
		cmd := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/FO", "CSV", "/NH")
		output, err := cmd.Output()
		if err != nil {
			return false, err
		}
		text := strings.TrimSpace(string(output))
		if text == "" || strings.Contains(text, "No tasks are running") || strings.Contains(text, "INFO:") {
			return false, nil
		}
		return true, nil
	}

	_, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid)))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
