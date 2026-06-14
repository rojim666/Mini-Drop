package apiserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"mini-drop/internal/minidrop"
)

type heartbeatRequest struct {
	AgentID  string `json:"agent_id"`
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
	Version  string `json:"version"`
}

type taskMutationRequest struct {
	Reason          string `json:"reason"`
	RawArtifactPath string `json:"raw_artifact_path"`
	FlamegraphPath  string `json:"flamegraph_path"`
	TopNPath        string `json:"topn_path"`
	Summary         string `json:"summary"`
}

type apiError struct {
	Error string `json:"error"`
}

type hotspotPayload struct {
	Function string  `json:"function"`
	Samples  int     `json:"samples"`
	Percent  float64 `json:"percent"`
}

type agentPayload struct {
	ID              string    `json:"id"`
	Hostname        string    `json:"hostname"`
	IP              string    `json:"ip"`
	Version         string    `json:"version"`
	Status          string    `json:"status"`
	LastHeartbeatAt time.Time `json:"last_heartbeat_at"`
}

type taskEventPayload struct {
	ID         string    `json:"id"`
	FromStatus string    `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	Reason     string    `json:"reason"`
	CreatedAt  time.Time `json:"created_at"`
}

type taskResultPayload struct {
	FlamegraphURL string           `json:"flamegraph_url"`
	TopNURL       string           `json:"topn_url"`
	Summary       string           `json:"summary"`
	Hotspots      []hotspotPayload `json:"hotspots"`
}

type taskPayload struct {
	ID                string             `json:"id"`
	TargetPID         int                `json:"target_pid"`
	TargetAgentID     string             `json:"target_agent_id"`
	SampleDurationSec int                `json:"sample_duration_sec"`
	SampleRateHz      int                `json:"sample_rate_hz"`
	CollectorType     string             `json:"collector_type"`
	Status            string             `json:"status"`
	StatusReason      string             `json:"status_reason"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
	StartedAt         *time.Time         `json:"started_at,omitempty"`
	FinishedAt        *time.Time         `json:"finished_at,omitempty"`
	RawArtifactURL    string             `json:"raw_artifact_url,omitempty"`
	AnalysisURL       string             `json:"analysis_artifact_url,omitempty"`
	Events            []taskEventPayload `json:"events,omitempty"`
	Result            *taskResultPayload `json:"result,omitempty"`
}

type auditPayload struct {
	ID         string    `json:"id"`
	EntityType string    `json:"entity_type"`
	EntityID   string    `json:"entity_id"`
	Action     string    `json:"action"`
	Reason     string    `json:"reason"`
	CreatedAt  time.Time `json:"created_at"`
}

func (s *Service) newRouter() *gin.Engine {
	router := gin.New()
	router.Use(s.corsMiddleware(), s.requestLogMiddleware(), gin.Recovery())
	router.StaticFS("/artifacts", http.Dir(s.cfg.ArtifactDir))

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := router.Group("/api/v1")
	{
		v1.GET("/agents", s.listAgents)
		v1.POST("/agents/heartbeat", s.heartbeatAgent)
		v1.GET("/tasks", s.listTasks)
		v1.POST("/tasks", s.createTask)
		v1.GET("/tasks/:id", s.getTask)
		v1.GET("/tasks/:id/results", s.getTaskResults)
		v1.GET("/audit-logs", s.listAuditLogs)

		internal := v1.Group("/internal")
		internal.GET("/tasks/claim", s.claimTask)
		internal.POST("/tasks/:id/uploading", s.markTaskUploading)
		internal.POST("/tasks/:id/complete", s.completeTask)
		internal.POST("/tasks/:id/fail", s.failTask)
	}

	return router
}

func (s *Service) corsMiddleware() gin.HandlerFunc {
	allowedOrigins := splitOrigins(s.cfg.AllowedOrigin)
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" && originAllowed(origin, allowedOrigins) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		} else if len(allowedOrigins) > 0 {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowedOrigins[0])
		}
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func splitOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			origins = append(origins, value)
		}
	}
	return origins
}

func originAllowed(origin string, allowed []string) bool {
	for _, candidate := range allowed {
		if candidate == origin {
			return true
		}
	}
	return false
}

func (s *Service) requestLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		s.log.Info(
			"http_request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(started).Milliseconds(),
		)
	}
}

func (s *Service) listAgents(c *gin.Context) {
	var agents []minidrop.Agent
	if err := s.db.Order("status desc, last_heartbeat_at desc").Find(&agents).Error; err != nil {
		s.writeError(c, http.StatusInternalServerError, err)
		return
	}

	payload := make([]agentPayload, 0, len(agents))
	for _, agent := range agents {
		payload = append(payload, s.toAgentPayload(agent))
	}

	c.JSON(http.StatusOK, gin.H{"agents": payload})
}

func (s *Service) heartbeatAgent(c *gin.Context) {
	var req heartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.writeError(c, http.StatusBadRequest, err)
		return
	}

	req.AgentID = strings.TrimSpace(req.AgentID)
	req.Hostname = strings.TrimSpace(req.Hostname)
	req.IP = strings.TrimSpace(req.IP)
	req.Version = strings.TrimSpace(req.Version)

	if req.AgentID == "" || req.Hostname == "" || req.IP == "" || req.Version == "" {
		s.writeError(c, http.StatusBadRequest, errors.New("agent_id, hostname, ip, and version are required"))
		return
	}

	now := time.Now().UTC()
	var agent minidrop.Agent

	err := s.db.Transaction(func(tx *gorm.DB) error {
		err := tx.First(&agent, "id = ?", req.AgentID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			agent = minidrop.Agent{
				ID:              req.AgentID,
				Hostname:        req.Hostname,
				IP:              req.IP,
				Version:         req.Version,
				Status:          minidrop.AgentStatusOnline,
				LastHeartbeatAt: now,
				CreatedAt:       now,
				UpdatedAt:       now,
			}
			return tx.Create(&agent).Error
		}
		if err != nil {
			return err
		}

		previousStatus := agent.Status
		agent.Hostname = req.Hostname
		agent.IP = req.IP
		agent.Version = req.Version
		agent.Status = minidrop.AgentStatusOnline
		agent.LastHeartbeatAt = now
		agent.UpdatedAt = now
		if err := tx.Save(&agent).Error; err != nil {
			return err
		}

		if previousStatus == minidrop.AgentStatusOffline {
			return tx.Create(&minidrop.AuditLog{
				ID:         minidrop.GenerateID("aud"),
				EntityType: "agent",
				EntityID:   agent.ID,
				Action:     "online",
				Reason:     "agent heartbeat restored",
				CreatedAt:  now,
			}).Error
		}

		return nil
	})
	if err != nil {
		s.writeError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"agent": s.toAgentPayload(agent)})
}

func (s *Service) createTask(c *gin.Context) {
	var input minidrop.CreateTaskInput
	if err := c.ShouldBindJSON(&input); err != nil {
		s.writeError(c, http.StatusBadRequest, err)
		return
	}
	input.Normalize()
	if err := minidrop.ValidateCreateTaskInput(input); err != nil {
		s.writeError(c, http.StatusBadRequest, err)
		return
	}

	now := time.Now().UTC()
	task := minidrop.Task{
		ID:                minidrop.GenerateID("tsk"),
		TargetPID:         input.TargetPID,
		TargetAgentID:     input.TargetAgentID,
		SampleDurationSec: input.SampleDurationSec,
		SampleRateHz:      input.SampleRateHz,
		CollectorType:     input.CollectorType,
		Status:            minidrop.TaskStatusPending,
		StatusReason:      "task created",
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if task.TargetAgentID == "" {
			var agent minidrop.Agent
			if err := tx.Where("status = ?", minidrop.AgentStatusOnline).Order("last_heartbeat_at desc").First(&agent).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errors.New("no online agent available")
				}
				return err
			}
			task.TargetAgentID = agent.ID
		} else {
			var agent minidrop.Agent
			if err := tx.First(&agent, "id = ?", task.TargetAgentID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errors.New("target agent not found")
				}
				return err
			}
			if agent.Status != minidrop.AgentStatusOnline {
				return errors.New("target agent is offline")
			}
		}

		if err := tx.Create(&task).Error; err != nil {
			return err
		}

		return tx.Create(&minidrop.TaskStatusEvent{
			ID:         minidrop.GenerateID("evt"),
			TaskID:     task.ID,
			FromStatus: "",
			ToStatus:   minidrop.TaskStatusPending,
			Reason:     task.StatusReason,
			CreatedAt:  now,
		}).Error
	})
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "no online agent available" || err.Error() == "target agent not found" || err.Error() == "target agent is offline" {
			status = http.StatusConflict
		}
		s.writeError(c, status, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"task": s.toTaskPayload(c, task, nil, nil)})
}

func (s *Service) listTasks(c *gin.Context) {
	var tasks []minidrop.Task
	if err := s.db.Order("created_at desc").Find(&tasks).Error; err != nil {
		s.writeError(c, http.StatusInternalServerError, err)
		return
	}

	payload := make([]taskPayload, 0, len(tasks))
	for _, task := range tasks {
		payload = append(payload, s.toTaskPayload(c, task, nil, nil))
	}

	c.JSON(http.StatusOK, gin.H{"tasks": payload})
}

func (s *Service) getTask(c *gin.Context) {
	task, events, result, err := s.loadTaskBundle(c.Param("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.writeError(c, http.StatusNotFound, err)
			return
		}
		s.writeError(c, http.StatusInternalServerError, err)
		return
	}

	payload := s.toTaskPayload(c, task, events, result)
	c.JSON(http.StatusOK, gin.H{"task": payload})
}

func (s *Service) getTaskResults(c *gin.Context) {
	task, _, result, err := s.loadTaskBundle(c.Param("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.writeError(c, http.StatusNotFound, err)
			return
		}
		s.writeError(c, http.StatusInternalServerError, err)
		return
	}

	if result == nil {
		c.JSON(http.StatusOK, gin.H{"task_id": task.ID, "result": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"task_id": task.ID, "result": s.toTaskResultPayload(c, *result)})
}

func (s *Service) listAuditLogs(c *gin.Context) {
	var logs []minidrop.AuditLog
	if err := s.db.Order("created_at desc").Limit(50).Find(&logs).Error; err != nil {
		s.writeError(c, http.StatusInternalServerError, err)
		return
	}

	payload := make([]auditPayload, 0, len(logs))
	for _, item := range logs {
		payload = append(payload, auditPayload{
			ID:         item.ID,
			EntityType: item.EntityType,
			EntityID:   item.EntityID,
			Action:     item.Action,
			Reason:     item.Reason,
			CreatedAt:  item.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"audit_logs": payload})
}

func (s *Service) claimTask(c *gin.Context) {
	agentID := strings.TrimSpace(c.Query("agent_id"))
	if agentID == "" {
		s.writeError(c, http.StatusBadRequest, errors.New("agent_id query parameter is required"))
		return
	}

	now := time.Now().UTC()
	var task minidrop.Task
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var agent minidrop.Agent
		if err := tx.First(&agent, "id = ?", agentID).Error; err != nil {
			return err
		}
		if agent.Status != minidrop.AgentStatusOnline {
			return errors.New("agent is offline")
		}

		err := tx.Where("target_agent_id = ? AND status = ?", agentID, minidrop.TaskStatusPending).
			Order("created_at asc").
			First(&task).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}

		return s.transitionTask(tx, &task, minidrop.TaskStatusRunning, "agent accepted task", now)
	})
	if err != nil {
		if err.Error() == "agent is offline" {
			s.writeError(c, http.StatusConflict, err)
			return
		}
		s.writeError(c, http.StatusInternalServerError, err)
		return
	}

	if task.ID == "" {
		c.Status(http.StatusNoContent)
		return
	}

	c.JSON(http.StatusOK, gin.H{"task": s.toTaskPayload(c, task, nil, nil)})
}

func (s *Service) markTaskUploading(c *gin.Context) {
	s.mutateTask(c, func(tx *gorm.DB, task *minidrop.Task, req taskMutationRequest, now time.Time) error {
		if strings.TrimSpace(req.RawArtifactPath) == "" {
			return errors.New("raw_artifact_path is required")
		}
		task.RawArtifactPath = filepath.ToSlash(strings.TrimSpace(req.RawArtifactPath))
		return s.transitionTask(tx, task, minidrop.TaskStatusUploading, nonEmptyReason(req.Reason, "mock artifact ready"), now)
	})
}

func (s *Service) completeTask(c *gin.Context) {
	s.mutateTask(c, func(tx *gorm.DB, task *minidrop.Task, req taskMutationRequest, now time.Time) error {
		if strings.TrimSpace(req.FlamegraphPath) == "" || strings.TrimSpace(req.TopNPath) == "" {
			return errors.New("flamegraph_path and topn_path are required")
		}
		task.RawArtifactPath = filepath.ToSlash(strings.TrimSpace(req.RawArtifactPath))
		task.AnalysisArtifactPath = filepath.ToSlash(strings.TrimSpace(req.FlamegraphPath))
		if err := s.transitionTask(tx, task, minidrop.TaskStatusDone, nonEmptyReason(req.Reason, "artifact uploaded and flamegraph generated"), now); err != nil {
			return err
		}

		var result minidrop.AnalysisResult
		err := tx.First(&result, "task_id = ?", task.ID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result = minidrop.AnalysisResult{
				ID:             minidrop.GenerateID("res"),
				TaskID:         task.ID,
				FlamegraphPath: filepath.ToSlash(strings.TrimSpace(req.FlamegraphPath)),
				TopNPath:       filepath.ToSlash(strings.TrimSpace(req.TopNPath)),
				Summary:        strings.TrimSpace(req.Summary),
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			return tx.Create(&result).Error
		}
		if err != nil {
			return err
		}

		result.FlamegraphPath = filepath.ToSlash(strings.TrimSpace(req.FlamegraphPath))
		result.TopNPath = filepath.ToSlash(strings.TrimSpace(req.TopNPath))
		result.Summary = strings.TrimSpace(req.Summary)
		result.UpdatedAt = now
		return tx.Save(&result).Error
	})
}

func (s *Service) failTask(c *gin.Context) {
	s.mutateTask(c, func(tx *gorm.DB, task *minidrop.Task, req taskMutationRequest, now time.Time) error {
		if strings.TrimSpace(req.RawArtifactPath) != "" {
			task.RawArtifactPath = filepath.ToSlash(strings.TrimSpace(req.RawArtifactPath))
		}
		return s.transitionTask(tx, task, minidrop.TaskStatusFailed, nonEmptyReason(req.Reason, "task failed"), now)
	})
}

func (s *Service) mutateTask(c *gin.Context, apply func(tx *gorm.DB, task *minidrop.Task, req taskMutationRequest, now time.Time) error) {
	var req taskMutationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.writeError(c, http.StatusBadRequest, err)
		return
	}

	now := time.Now().UTC()
	var task minidrop.Task
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&task, "id = ?", c.Param("id")).Error; err != nil {
			return err
		}
		return apply(tx, &task, req, now)
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "invalid task transition") {
			status = http.StatusConflict
		} else if strings.Contains(err.Error(), "required") {
			status = http.StatusBadRequest
		}
		s.writeError(c, status, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"task": s.toTaskPayload(c, task, nil, nil)})
}

func (s *Service) transitionTask(tx *gorm.DB, task *minidrop.Task, next minidrop.TaskStatus, reason string, now time.Time) error {
	if err := minidrop.ValidateTaskTransition(task.Status, next); err != nil {
		return err
	}

	from := task.Status
	task.Status = next
	task.StatusReason = reason
	task.UpdatedAt = now

	if next == minidrop.TaskStatusRunning && task.StartedAt == nil {
		task.StartedAt = &now
	}
	if minidrop.IsTerminalTaskStatus(next) {
		task.FinishedAt = &now
	}

	if err := tx.Save(task).Error; err != nil {
		return err
	}

	return tx.Create(&minidrop.TaskStatusEvent{
		ID:         minidrop.GenerateID("evt"),
		TaskID:     task.ID,
		FromStatus: from,
		ToStatus:   next,
		Reason:     reason,
		CreatedAt:  now,
	}).Error
}

func (s *Service) loadTaskBundle(taskID string) (minidrop.Task, []minidrop.TaskStatusEvent, *minidrop.AnalysisResult, error) {
	var task minidrop.Task
	if err := s.db.First(&task, "id = ?", taskID).Error; err != nil {
		return minidrop.Task{}, nil, nil, err
	}

	var events []minidrop.TaskStatusEvent
	if err := s.db.Where("task_id = ?", taskID).Order("created_at asc").Find(&events).Error; err != nil {
		return minidrop.Task{}, nil, nil, err
	}

	var result minidrop.AnalysisResult
	if err := s.db.First(&result, "task_id = ?", taskID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return task, events, nil, nil
		}
		return minidrop.Task{}, nil, nil, err
	}

	return task, events, &result, nil
}

func (s *Service) toAgentPayload(agent minidrop.Agent) agentPayload {
	return agentPayload{
		ID:              agent.ID,
		Hostname:        agent.Hostname,
		IP:              agent.IP,
		Version:         agent.Version,
		Status:          string(agent.Status),
		LastHeartbeatAt: agent.LastHeartbeatAt,
	}
}

func (s *Service) toTaskPayload(c *gin.Context, task minidrop.Task, events []minidrop.TaskStatusEvent, result *minidrop.AnalysisResult) taskPayload {
	payload := taskPayload{
		ID:                task.ID,
		TargetPID:         task.TargetPID,
		TargetAgentID:     task.TargetAgentID,
		SampleDurationSec: task.SampleDurationSec,
		SampleRateHz:      task.SampleRateHz,
		CollectorType:     task.CollectorType,
		Status:            string(task.Status),
		StatusReason:      task.StatusReason,
		CreatedAt:         task.CreatedAt,
		UpdatedAt:         task.UpdatedAt,
		StartedAt:         task.StartedAt,
		FinishedAt:        task.FinishedAt,
	}

	if task.RawArtifactPath != "" {
		payload.RawArtifactURL = s.artifactURL(c, task.RawArtifactPath)
	}
	if task.AnalysisArtifactPath != "" {
		payload.AnalysisURL = s.artifactURL(c, task.AnalysisArtifactPath)
	}

	if len(events) > 0 {
		payload.Events = make([]taskEventPayload, 0, len(events))
		for _, event := range events {
			payload.Events = append(payload.Events, taskEventPayload{
				ID:         event.ID,
				FromStatus: string(event.FromStatus),
				ToStatus:   string(event.ToStatus),
				Reason:     event.Reason,
				CreatedAt:  event.CreatedAt,
			})
		}
	}

	if result != nil {
		taskResult := s.toTaskResultPayload(c, *result)
		payload.Result = &taskResult
	}

	return payload
}

func (s *Service) toTaskResultPayload(c *gin.Context, result minidrop.AnalysisResult) taskResultPayload {
	payload := taskResultPayload{
		FlamegraphURL: s.artifactURL(c, result.FlamegraphPath),
		TopNURL:       s.artifactURL(c, result.TopNPath),
		Summary:       result.Summary,
	}

	absPath := s.artifactAbsPath(result.TopNPath)
	hotspots, err := mustReadTopN(absPath)
	if err == nil {
		payload.Hotspots = hotspots
	}

	return payload
}

func (s *Service) writeError(c *gin.Context, status int, err error) {
	c.JSON(status, apiError{Error: err.Error()})
}

func nonEmptyReason(reason, fallback string) string {
	if strings.TrimSpace(reason) == "" {
		return fallback
	}
	return strings.TrimSpace(reason)
}

func decodeJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func jsonBody(v any) (io.Reader, error) {
	payload, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(payload), nil
}

func relativeArtifactPath(root, candidate string) string {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return filepath.ToSlash(candidate)
	}
	return filepath.ToSlash(rel)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func mustJSONString(v any) string {
	payload, _ := json.Marshal(v)
	return string(payload)
}

func debugValue(label string, v any) string {
	return fmt.Sprintf("%s=%s", label, mustJSONString(v))
}
