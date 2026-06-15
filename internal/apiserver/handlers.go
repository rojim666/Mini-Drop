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
	"strconv"
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
	Reason               string `json:"reason"`
	RawArtifactPath      string `json:"raw_artifact_path"`
	FlamegraphPath       string `json:"flamegraph_path"`
	TopNPath             string `json:"topn_path"`
	ResourceTimelinePath string `json:"resource_timeline_path"`
	Summary              string `json:"summary"`
}

type continuousProfileRequest struct {
	Name              string `json:"name"`
	TargetPID         int    `json:"target_pid"`
	TargetAgentID     string `json:"target_agent_id"`
	SampleDurationSec int    `json:"sample_duration_sec"`
	SampleRateHz      int    `json:"sample_rate_hz"`
	CollectorType     string `json:"collector_type"`
	IntervalSec       int    `json:"interval_sec"`
	ScheduleMode      string `json:"schedule_mode"`
	CronExpression    string `json:"cron_expression"`
	StaggerSec        int    `json:"stagger_sec"`
}

type continuousProfilePayload struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	TargetPID         int        `json:"target_pid"`
	TargetAgentID     string     `json:"target_agent_id"`
	SampleDurationSec int        `json:"sample_duration_sec"`
	SampleRateHz      int        `json:"sample_rate_hz"`
	CollectorType     string     `json:"collector_type"`
	WindowDurationSec int        `json:"window_duration_sec"`
	IntervalSec       int        `json:"interval_sec"`
	ScheduleMode      string     `json:"schedule_mode"`
	CronExpression    string     `json:"cron_expression"`
	StaggerSec        int        `json:"stagger_sec"`
	Enabled           bool       `json:"enabled"`
	LastWindowStartAt *time.Time `json:"last_window_start_at,omitempty"`
	LastScheduledAt   *time.Time `json:"last_scheduled_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type continuousWindowPayload struct {
	ID            string    `json:"id"`
	ProfileID     string    `json:"profile_id"`
	TaskID        string    `json:"task_id"`
	WindowStartAt time.Time `json:"window_start_at"`
	WindowEndAt   time.Time `json:"window_end_at"`
	Status        string    `json:"status"`
	StatusReason  string    `json:"status_reason"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type continuousWindowSummaryPayload struct {
	TotalWindows        int        `json:"total_windows"`
	DoneWindows         int        `json:"done_windows"`
	FailedWindows       int        `json:"failed_windows"`
	RunningWindows      int        `json:"running_windows"`
	PendingWindows      int        `json:"pending_windows"`
	LatestStatus        string     `json:"latest_status"`
	LatestStatusReason  string     `json:"latest_status_reason"`
	LatestWindowStartAt *time.Time `json:"latest_window_start_at,omitempty"`
	LatestWindowEndAt   *time.Time `json:"latest_window_end_at,omitempty"`
	DoneRatio           float64    `json:"done_ratio"`
}

type continuousWindowFilters struct {
	Status    minidrop.TaskStatus
	HasStatus bool
	From      *time.Time
	To        *time.Time
	Limit     int
}

type continuousTrendWindowPayload struct {
	WindowID      string    `json:"window_id"`
	TaskID        string    `json:"task_id"`
	WindowStartAt time.Time `json:"window_start_at"`
	WindowEndAt   time.Time `json:"window_end_at"`
	Status        string    `json:"status"`
}

type continuousTrendPointPayload struct {
	WindowID string  `json:"window_id"`
	TaskID   string  `json:"task_id"`
	Percent  float64 `json:"percent"`
	Samples  int     `json:"samples"`
}

type continuousTrendBaselinePayload struct {
	Status          string  `json:"status"`
	Description     string  `json:"description"`
	ExpectedPercent float64 `json:"expected_percent"`
	ActualPercent   float64 `json:"actual_percent"`
	DeltaPercent    float64 `json:"delta_percent"`
	Reason          string  `json:"reason"`
}

type continuousTrendSeriesPayload struct {
	Function string                          `json:"function"`
	Average  float64                         `json:"average"`
	Peak     float64                         `json:"peak"`
	Delta    float64                         `json:"delta"`
	Label    string                          `json:"label"`
	Severity string                          `json:"severity"`
	Reason   string                          `json:"reason"`
	Baseline *continuousTrendBaselinePayload `json:"baseline,omitempty"`
	Points   []continuousTrendPointPayload   `json:"points"`
}

type continuousTrendPayload struct {
	Windows []continuousTrendWindowPayload `json:"windows"`
	Series  []continuousTrendSeriesPayload `json:"series"`
}

type apiError struct {
	Error string `json:"error"`
}

type hotspotPayload struct {
	Function string  `json:"function"`
	Samples  int     `json:"samples"`
	Percent  float64 `json:"percent"`
}

type attributionEvidencePayload struct {
	Kind             string                   `json:"kind"`
	Detail           string                   `json:"detail"`
	Function         string                   `json:"function,omitempty"`
	Samples          int                      `json:"samples,omitempty"`
	Percent          float64                  `json:"percent,omitempty"`
	ResourceTimeline *resourceTimelinePayload `json:"resource_timeline,omitempty"`
}

type attributionSourcePayload struct {
	TaskID               string `json:"task_id"`
	CollectorType        string `json:"collector_type"`
	SampleDurationSec    int    `json:"sample_duration_sec"`
	SampleRateHz         int    `json:"sample_rate_hz"`
	TopNPath             string `json:"topn_path"`
	ResourceTimelinePath string `json:"resource_timeline_path,omitempty"`
}

type resourceTimelinePointPayload struct {
	OffsetSec float64 `json:"offset_sec"`
	Value     float64 `json:"value"`
	Samples   int     `json:"samples"`
}

type resourceTimelinePayload struct {
	Source      string                         `json:"source"`
	Signal      string                         `json:"signal"`
	Alignment   string                         `json:"alignment"`
	Summary     string                         `json:"summary"`
	WindowSec   int                            `json:"window_sec"`
	TopFunction string                         `json:"top_function"`
	PeakPercent float64                        `json:"peak_percent"`
	Points      []resourceTimelinePointPayload `json:"points"`
}

type attributionPayload struct {
	Conclusion       string                       `json:"conclusion"`
	Confidence       float64                      `json:"confidence"`
	Evidence         []attributionEvidencePayload `json:"evidence"`
	Recommendations  []string                     `json:"recommendations"`
	Source           attributionSourcePayload     `json:"source"`
	ResourceTimeline *resourceTimelinePayload     `json:"resource_timeline,omitempty"`
	ToolTrace        []attributionToolCallPayload `json:"tool_trace,omitempty"`
	Prompt           string                       `json:"prompt,omitempty"`
	PersistedAt      *time.Time                   `json:"persisted_at,omitempty"`
}

type attributionToolCallPayload struct {
	Name   string `json:"name"`
	Input  string `json:"input"`
	Output string `json:"output"`
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
	FlamegraphURL string              `json:"flamegraph_url"`
	TopNURL       string              `json:"topn_url"`
	Summary       string              `json:"summary"`
	Hotspots      []hotspotPayload    `json:"hotspots"`
	Attribution   *attributionPayload `json:"attribution,omitempty"`
}

type taskPayload struct {
	ID                  string             `json:"id"`
	TargetPID           int                `json:"target_pid"`
	TargetAgentID       string             `json:"target_agent_id"`
	SampleDurationSec   int                `json:"sample_duration_sec"`
	SampleRateHz        int                `json:"sample_rate_hz"`
	CollectorType       string             `json:"collector_type"`
	ContinuousProfileID string             `json:"continuous_profile_id,omitempty"`
	ContinuousWindowID  string             `json:"continuous_window_id,omitempty"`
	Status              string             `json:"status"`
	StatusReason        string             `json:"status_reason"`
	CreatedAt           time.Time          `json:"created_at"`
	UpdatedAt           time.Time          `json:"updated_at"`
	StartedAt           *time.Time         `json:"started_at,omitempty"`
	FinishedAt          *time.Time         `json:"finished_at,omitempty"`
	RawArtifactURL      string             `json:"raw_artifact_url,omitempty"`
	AnalysisURL         string             `json:"analysis_artifact_url,omitempty"`
	Events              []taskEventPayload `json:"events,omitempty"`
	Result              *taskResultPayload `json:"result,omitempty"`
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
		v1.GET("/continuous-profiles", s.listContinuousProfiles)
		v1.POST("/continuous-profiles", s.createContinuousProfile)
		v1.GET("/continuous-profiles/:id", s.getContinuousProfile)
		v1.POST("/continuous-profiles/:id/enable", s.enableContinuousProfile)
		v1.POST("/continuous-profiles/:id/disable", s.disableContinuousProfile)
		v1.GET("/continuous-profiles/:id/windows", s.listContinuousProfileWindows)
		v1.GET("/continuous-profiles/:id/trends", s.getContinuousProfileTrends)
		v1.GET("/audit-logs", s.listAuditLogs)

		internal := v1.Group("/internal")
		internal.GET("/tasks/claim", s.claimTask)
		internal.POST("/tasks/:id/uploading", s.markTaskUploading)
		internal.POST("/tasks/:id/complete", s.completeTask)
		internal.POST("/tasks/:id/fail", s.failTask)
		internal.GET("/continuous-profiles/claim", s.claimContinuousProfileWindow)
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
			agent, err := s.pickAutoTargetAgent(tx)
			if err != nil {
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
		var result *minidrop.AnalysisResult
		if task.Status == minidrop.TaskStatusDone {
			loadedResult, err := s.loadAnalysisResult(task.ID)
			if err != nil {
				s.log.Warn("load task list result failed", "task_id", task.ID, "error", err)
			}
			result = loadedResult
		}
		payload = append(payload, s.toTaskPayload(c, task, nil, result))
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

	c.JSON(http.StatusOK, gin.H{"task_id": task.ID, "result": s.toTaskResultPayload(c, task, *result)})
}

func (s *Service) listContinuousProfiles(c *gin.Context) {
	var profiles []minidrop.ContinuousProfile
	if err := s.db.Order("created_at desc").Find(&profiles).Error; err != nil {
		s.writeError(c, http.StatusInternalServerError, err)
		return
	}

	payload := make([]continuousProfilePayload, 0, len(profiles))
	for _, profile := range profiles {
		payload = append(payload, s.toContinuousProfilePayload(profile))
	}

	c.JSON(http.StatusOK, gin.H{"profiles": payload})
}

func (s *Service) createContinuousProfile(c *gin.Context) {
	var req continuousProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.writeError(c, http.StatusBadRequest, err)
		return
	}

	input := minidrop.CreateContinuousProfileInput{
		Name:              req.Name,
		TargetPID:         req.TargetPID,
		TargetAgentID:     req.TargetAgentID,
		SampleDurationSec: req.SampleDurationSec,
		SampleRateHz:      req.SampleRateHz,
		CollectorType:     req.CollectorType,
		IntervalSec:       req.IntervalSec,
		ScheduleMode:      req.ScheduleMode,
		CronExpression:    req.CronExpression,
		StaggerSec:        req.StaggerSec,
	}
	input.Normalize()
	if err := minidrop.ValidateCreateContinuousProfileInput(input); err != nil {
		s.writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := validateContinuousSchedule(input); err != nil {
		s.writeError(c, http.StatusBadRequest, err)
		return
	}

	now := time.Now().UTC()
	profile := minidrop.ContinuousProfile{
		ID:                minidrop.GenerateID("cprof"),
		Name:              nonEmptyReason(input.Name, fmt.Sprintf("PID %d continuous profile", input.TargetPID)),
		TargetPID:         input.TargetPID,
		TargetAgentID:     input.TargetAgentID,
		SampleDurationSec: input.SampleDurationSec,
		SampleRateHz:      input.SampleRateHz,
		CollectorType:     input.CollectorType,
		WindowDurationSec: minidrop.ContinuousWindowDurationSec,
		IntervalSec:       input.IntervalSec,
		ScheduleMode:      input.ScheduleMode,
		CronExpression:    input.CronExpression,
		StaggerSec:        input.StaggerSec,
		Enabled:           true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if profile.TargetAgentID == "" {
			agent, err := s.pickAutoTargetAgent(tx)
			if err != nil {
				return err
			}
			profile.TargetAgentID = agent.ID
		} else {
			var agent minidrop.Agent
			if err := tx.First(&agent, "id = ?", profile.TargetAgentID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errors.New("target agent not found")
				}
				return err
			}
			if agent.Status != minidrop.AgentStatusOnline {
				return errors.New("target agent is offline")
			}
		}

		if err := tx.Create(&profile).Error; err != nil {
			return err
		}
		_, _, err := s.materializeContinuousWindow(tx, &profile, now, "initial continuous profiling window")
		return err
	})
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "no online agent available" || err.Error() == "target agent not found" || err.Error() == "target agent is offline" {
			status = http.StatusConflict
		}
		s.writeError(c, status, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"profile": s.toContinuousProfilePayload(profile)})
}

func (s *Service) pickAutoTargetAgent(tx *gorm.DB) (minidrop.Agent, error) {
	var agents []minidrop.Agent
	if err := tx.Where("status = ?", minidrop.AgentStatusOnline).
		Order("last_heartbeat_at desc, id asc").
		Find(&agents).Error; err != nil {
		return minidrop.Agent{}, err
	}
	if len(agents) == 0 {
		return minidrop.Agent{}, errors.New("no online agent available")
	}

	activeStatuses := []minidrop.TaskStatus{
		minidrop.TaskStatusPending,
		minidrop.TaskStatusRunning,
		minidrop.TaskStatusUploading,
	}

	var (
		bestAgent minidrop.Agent
		bestLoad  int64 = -1
	)
	for _, agent := range agents {
		var load int64
		if err := tx.Model(&minidrop.Task{}).
			Where("target_agent_id = ? AND status IN ?", agent.ID, activeStatuses).
			Count(&load).Error; err != nil {
			return minidrop.Agent{}, err
		}

		if bestLoad == -1 || load < bestLoad || (load == bestLoad && agent.LastHeartbeatAt.After(bestAgent.LastHeartbeatAt)) || (load == bestLoad && agent.LastHeartbeatAt.Equal(bestAgent.LastHeartbeatAt) && agent.ID < bestAgent.ID) {
			bestAgent = agent
			bestLoad = load
		}
	}

	return bestAgent, nil
}

func (s *Service) getContinuousProfile(c *gin.Context) {
	var profile minidrop.ContinuousProfile
	if err := s.db.First(&profile, "id = ?", c.Param("id")).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.writeError(c, http.StatusNotFound, err)
			return
		}
		s.writeError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"profile": s.toContinuousProfilePayload(profile)})
}

func (s *Service) enableContinuousProfile(c *gin.Context) {
	s.setContinuousProfileEnabled(c, true)
}

func (s *Service) disableContinuousProfile(c *gin.Context) {
	s.setContinuousProfileEnabled(c, false)
}

func (s *Service) setContinuousProfileEnabled(c *gin.Context, enabled bool) {
	now := time.Now().UTC()
	action := "continuous_profile_disabled"
	reason := "continuous profiling paused by operator"
	if enabled {
		action = "continuous_profile_enabled"
		reason = "continuous profiling resumed by operator"
	}

	var profile minidrop.ContinuousProfile
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&profile, "id = ?", c.Param("id")).Error; err != nil {
			return err
		}
		if profile.Enabled == enabled {
			return nil
		}
		profile.Enabled = enabled
		profile.UpdatedAt = now
		if err := tx.Save(&profile).Error; err != nil {
			return err
		}
		return tx.Create(&minidrop.AuditLog{
			ID:         minidrop.GenerateID("aud"),
			EntityType: "continuous_profile",
			EntityID:   profile.ID,
			Action:     action,
			Reason:     reason,
			CreatedAt:  now,
		}).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.writeError(c, http.StatusNotFound, err)
			return
		}
		s.writeError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"profile": s.toContinuousProfilePayload(profile)})
}

func (s *Service) listContinuousProfileWindows(c *gin.Context) {
	filters, err := parseContinuousWindowFilters(c)
	if err != nil {
		s.writeError(c, http.StatusBadRequest, err)
		return
	}

	var windows []minidrop.ContinuousProfileWindow
	query := s.db.Where("profile_id = ?", c.Param("id"))
	if filters.HasStatus {
		query = query.Where("status = ?", filters.Status)
	}
	if filters.From != nil {
		query = query.Where("window_start_at >= ?", *filters.From)
	}
	if filters.To != nil {
		query = query.Where("window_start_at <= ?", *filters.To)
	}

	if err := query.Order("window_start_at desc").Limit(filters.Limit).Find(&windows).Error; err != nil {
		s.writeError(c, http.StatusInternalServerError, err)
		return
	}

	payload := make([]continuousWindowPayload, 0, len(windows))
	for _, window := range windows {
		payload = append(payload, s.toContinuousWindowPayload(window))
	}

	c.JSON(http.StatusOK, gin.H{
		"windows": payload,
		"summary": summarizeContinuousWindows(windows),
	})
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

		err := tx.Model(&minidrop.Task{}).
			Joins("LEFT JOIN continuous_profile_windows ON continuous_profile_windows.id = tasks.continuous_window_id").
			Where("tasks.target_agent_id = ? AND tasks.status = ?", agentID, minidrop.TaskStatusPending).
			Where("tasks.continuous_window_id = '' OR tasks.continuous_window_id IS NULL OR continuous_profile_windows.window_start_at <= ?", now).
			Order("tasks.created_at asc").
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

func (s *Service) claimContinuousProfileWindow(c *gin.Context) {
	agentID := strings.TrimSpace(c.Query("agent_id"))
	if agentID == "" {
		s.writeError(c, http.StatusBadRequest, errors.New("agent_id query parameter is required"))
		return
	}

	now := time.Now().UTC()
	var window *minidrop.ContinuousProfileWindow
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var agent minidrop.Agent
		if err := tx.First(&agent, "id = ?", agentID).Error; err != nil {
			return err
		}
		if agent.Status != minidrop.AgentStatusOnline {
			return errors.New("agent is offline")
		}

		var err error
		window, err = s.materializeDueContinuousWindowForAgent(tx, agentID, now)
		return err
	})
	if err != nil {
		if err.Error() == "agent is offline" {
			s.writeError(c, http.StatusConflict, err)
			return
		}
		s.writeError(c, http.StatusInternalServerError, err)
		return
	}

	if window == nil {
		c.Status(http.StatusNoContent)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"window": s.toContinuousWindowPayload(*window)})
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
				ID:                   minidrop.GenerateID("res"),
				TaskID:               task.ID,
				FlamegraphPath:       filepath.ToSlash(strings.TrimSpace(req.FlamegraphPath)),
				TopNPath:             filepath.ToSlash(strings.TrimSpace(req.TopNPath)),
				ResourceTimelinePath: filepath.ToSlash(strings.TrimSpace(req.ResourceTimelinePath)),
				Summary:              strings.TrimSpace(req.Summary),
				CreatedAt:            now,
				UpdatedAt:            now,
			}
			return tx.Create(&result).Error
		}
		if err != nil {
			return err
		}

		result.FlamegraphPath = filepath.ToSlash(strings.TrimSpace(req.FlamegraphPath))
		result.TopNPath = filepath.ToSlash(strings.TrimSpace(req.TopNPath))
		result.ResourceTimelinePath = filepath.ToSlash(strings.TrimSpace(req.ResourceTimelinePath))
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

	if err := tx.Create(&minidrop.TaskStatusEvent{
		ID:         minidrop.GenerateID("evt"),
		TaskID:     task.ID,
		FromStatus: from,
		ToStatus:   next,
		Reason:     reason,
		CreatedAt:  now,
	}).Error; err != nil {
		return err
	}

	s.log.Info(
		"task status transitioned",
		"task_id", task.ID,
		"target_agent_id", task.TargetAgentID,
		"collector_type", task.CollectorType,
		"from_status", from,
		"to_status", next,
		"reason", reason,
	)

	if task.ContinuousWindowID != "" {
		return tx.Model(&minidrop.ContinuousProfileWindow{}).
			Where("id = ?", task.ContinuousWindowID).
			Updates(map[string]any{
				"status":        next,
				"status_reason": reason,
				"updated_at":    now,
			}).Error
	}

	return nil
}

func (s *Service) materializeDueContinuousWindowForAgent(tx *gorm.DB, agentID string, now time.Time) (*minidrop.ContinuousProfileWindow, error) {
	var profiles []minidrop.ContinuousProfile
	if err := tx.Where("enabled = ? AND target_agent_id = ?", true, agentID).
		Order("last_window_start_at asc, created_at asc").
		Find(&profiles).Error; err != nil {
		return nil, err
	}

	for i := range profiles {
		if !continuousProfileDue(profiles[i], now) {
			continue
		}
		_, window, err := s.materializeContinuousWindow(tx, &profiles[i], now, "continuous profiling window scheduled")
		if err != nil {
			return nil, err
		}
		if window != nil {
			return window, nil
		}
	}

	return nil, nil
}

func (s *Service) materializeContinuousWindow(tx *gorm.DB, profile *minidrop.ContinuousProfile, now time.Time, reason string) (*minidrop.Task, *minidrop.ContinuousProfileWindow, error) {
	windowStart := nextContinuousWindowStart(*profile, now)
	windowEnd := windowStart.Add(time.Duration(profile.WindowDurationSec) * time.Second)

	var existing minidrop.ContinuousProfileWindow
	err := tx.Where("profile_id = ? AND window_start_at = ?", profile.ID, windowStart).First(&existing).Error
	if err == nil {
		return nil, &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}

	windowID := minidrop.GenerateID("cwin")
	task := minidrop.Task{
		ID:                  minidrop.GenerateID("tsk"),
		TargetPID:           profile.TargetPID,
		TargetAgentID:       profile.TargetAgentID,
		SampleDurationSec:   profile.SampleDurationSec,
		SampleRateHz:        profile.SampleRateHz,
		CollectorType:       profile.CollectorType,
		ContinuousProfileID: profile.ID,
		ContinuousWindowID:  windowID,
		Status:              minidrop.TaskStatusPending,
		StatusReason:        reason,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	window := minidrop.ContinuousProfileWindow{
		ID:            windowID,
		ProfileID:     profile.ID,
		TaskID:        task.ID,
		WindowStartAt: windowStart,
		WindowEndAt:   windowEnd,
		Status:        minidrop.TaskStatusPending,
		StatusReason:  reason,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := tx.Create(&task).Error; err != nil {
		return nil, nil, err
	}
	if err := tx.Create(&minidrop.TaskStatusEvent{
		ID:         minidrop.GenerateID("evt"),
		TaskID:     task.ID,
		FromStatus: "",
		ToStatus:   minidrop.TaskStatusPending,
		Reason:     reason,
		CreatedAt:  now,
	}).Error; err != nil {
		return nil, nil, err
	}
	if err := tx.Create(&window).Error; err != nil {
		return nil, nil, err
	}

	profile.LastWindowStartAt = &windowStart
	profile.LastScheduledAt = &now
	profile.UpdatedAt = now
	if err := tx.Save(profile).Error; err != nil {
		return nil, nil, err
	}

	return &task, &window, nil
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

func (s *Service) loadAnalysisResult(taskID string) (*minidrop.AnalysisResult, error) {
	var result minidrop.AnalysisResult
	if err := s.db.First(&result, "task_id = ?", taskID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &result, nil
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
		ID:                  task.ID,
		TargetPID:           task.TargetPID,
		TargetAgentID:       task.TargetAgentID,
		SampleDurationSec:   task.SampleDurationSec,
		SampleRateHz:        task.SampleRateHz,
		CollectorType:       task.CollectorType,
		ContinuousProfileID: task.ContinuousProfileID,
		ContinuousWindowID:  task.ContinuousWindowID,
		Status:              string(task.Status),
		StatusReason:        task.StatusReason,
		CreatedAt:           task.CreatedAt,
		UpdatedAt:           task.UpdatedAt,
		StartedAt:           task.StartedAt,
		FinishedAt:          task.FinishedAt,
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
		taskResult := s.toTaskResultPayload(c, task, *result)
		payload.Result = &taskResult
	}

	return payload
}

func (s *Service) toContinuousProfilePayload(profile minidrop.ContinuousProfile) continuousProfilePayload {
	return continuousProfilePayload{
		ID:                profile.ID,
		Name:              profile.Name,
		TargetPID:         profile.TargetPID,
		TargetAgentID:     profile.TargetAgentID,
		SampleDurationSec: profile.SampleDurationSec,
		SampleRateHz:      profile.SampleRateHz,
		CollectorType:     profile.CollectorType,
		WindowDurationSec: profile.WindowDurationSec,
		IntervalSec:       profile.IntervalSec,
		ScheduleMode:      normalizedScheduleMode(profile.ScheduleMode),
		CronExpression:    profile.CronExpression,
		StaggerSec:        profile.StaggerSec,
		Enabled:           profile.Enabled,
		LastWindowStartAt: profile.LastWindowStartAt,
		LastScheduledAt:   profile.LastScheduledAt,
		CreatedAt:         profile.CreatedAt,
		UpdatedAt:         profile.UpdatedAt,
	}
}

func (s *Service) toContinuousWindowPayload(window minidrop.ContinuousProfileWindow) continuousWindowPayload {
	return continuousWindowPayload{
		ID:            window.ID,
		ProfileID:     window.ProfileID,
		TaskID:        window.TaskID,
		WindowStartAt: window.WindowStartAt,
		WindowEndAt:   window.WindowEndAt,
		Status:        string(window.Status),
		StatusReason:  window.StatusReason,
		CreatedAt:     window.CreatedAt,
		UpdatedAt:     window.UpdatedAt,
	}
}

func summarizeContinuousWindows(windows []minidrop.ContinuousProfileWindow) continuousWindowSummaryPayload {
	summary := continuousWindowSummaryPayload{
		TotalWindows: len(windows),
		LatestStatus: "NONE",
	}
	if len(windows) == 0 {
		return summary
	}

	latest := windows[0]
	summary.LatestStatus = string(latest.Status)
	summary.LatestStatusReason = latest.StatusReason
	summary.LatestWindowStartAt = &latest.WindowStartAt
	summary.LatestWindowEndAt = &latest.WindowEndAt

	for _, window := range windows {
		switch window.Status {
		case minidrop.TaskStatusDone:
			summary.DoneWindows++
		case minidrop.TaskStatusFailed:
			summary.FailedWindows++
		case minidrop.TaskStatusRunning, minidrop.TaskStatusUploading:
			summary.RunningWindows++
		case minidrop.TaskStatusPending:
			summary.PendingWindows++
		}
	}

	summary.DoneRatio = float64(summary.DoneWindows) / float64(summary.TotalWindows)
	return summary
}

func parseContinuousWindowFilters(c *gin.Context) (continuousWindowFilters, error) {
	filters := continuousWindowFilters{Limit: 24}

	status := strings.ToUpper(strings.TrimSpace(c.Query("status")))
	if status != "" && status != "ALL" {
		switch minidrop.TaskStatus(status) {
		case minidrop.TaskStatusPending, minidrop.TaskStatusRunning, minidrop.TaskStatusUploading, minidrop.TaskStatusDone, minidrop.TaskStatusFailed:
			filters.Status = minidrop.TaskStatus(status)
			filters.HasStatus = true
		default:
			return filters, errors.New("status must be one of: PENDING, RUNNING, UPLOADING, DONE, FAILED")
		}
	}

	from, err := parseOptionalRFC3339Query(c, "from")
	if err != nil {
		return filters, err
	}
	to, err := parseOptionalRFC3339Query(c, "to")
	if err != nil {
		return filters, err
	}
	if from != nil && to != nil && from.After(*to) {
		return filters, errors.New("from must be before to")
	}
	filters.From = from
	filters.To = to

	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil || limit < 1 || limit > 100 {
			return filters, errors.New("limit must be between 1 and 100")
		}
		filters.Limit = limit
	}

	return filters, nil
}

func parseOptionalRFC3339Query(c *gin.Context, name string) (*time.Time, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, nil
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be RFC3339 time", name)
	}
	value = value.UTC()
	return &value, nil
}

func (s *Service) getContinuousProfileTrends(c *gin.Context) {
	windowLimit := 12
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil || limit < 1 || limit > 24 {
			s.writeError(c, http.StatusBadRequest, errors.New("limit must be between 1 and 24"))
			return
		}
		windowLimit = limit
	}

	var windows []minidrop.ContinuousProfileWindow
	if err := s.db.Where("profile_id = ? AND status = ?", c.Param("id"), minidrop.TaskStatusDone).
		Order("window_start_at desc").
		Limit(windowLimit).
		Find(&windows).Error; err != nil {
		s.writeError(c, http.StatusInternalServerError, err)
		return
	}
	if len(windows) == 0 {
		c.JSON(http.StatusOK, gin.H{"windows": []continuousTrendWindowPayload{}, "series": []continuousTrendSeriesPayload{}})
		return
	}

	// Load top hotspots for each completed window and build a compact cross-window trend.
	type windowHotspots struct {
		window        minidrop.ContinuousProfileWindow
		collectorType string
		hotspots      []hotspotPayload
	}
	records := make([]windowHotspots, 0, len(windows))
	functionOrder := make([]string, 0, 8)
	functionSeen := map[string]bool{}

	for _, window := range windows {
		var task minidrop.Task
		if err := s.db.First(&task, "id = ?", window.TaskID).Error; err != nil {
			s.log.Warn("load trend task failed", "window_id", window.ID, "task_id", window.TaskID, "error", err)
			continue
		}
		var result minidrop.AnalysisResult
		if err := s.db.First(&result, "task_id = ?", window.TaskID).Error; err != nil {
			s.log.Warn("load trend result failed", "window_id", window.ID, "task_id", window.TaskID, "error", err)
			continue
		}

		hotspots, err := mustReadTopN(s.artifactAbsPath(result.TopNPath))
		if err != nil {
			s.log.Warn("read trend topn failed", "window_id", window.ID, "task_id", window.TaskID, "error", err)
			continue
		}

		records = append(records, windowHotspots{window: window, collectorType: task.CollectorType, hotspots: hotspots})
		for _, hotspot := range hotspots {
			if !functionSeen[hotspot.Function] {
				functionSeen[hotspot.Function] = true
				functionOrder = append(functionOrder, hotspot.Function)
			}
		}
	}

	if len(records) == 0 {
		c.JSON(http.StatusOK, gin.H{"windows": []continuousTrendWindowPayload{}, "series": []continuousTrendSeriesPayload{}})
		return
	}
	if len(functionOrder) > 5 {
		functionOrder = functionOrder[:5]
	}

	var baselines []minidrop.AttributionBaseline
	if err := s.db.Order("collector_type asc, function_pattern asc").Find(&baselines).Error; err != nil {
		s.log.Warn("load trend baselines failed", "profile_id", c.Param("id"), "error", err)
	}

	windowsPayload := make([]continuousTrendWindowPayload, 0, len(records))
	seriesMap := make(map[string][]continuousTrendPointPayload, len(functionOrder))
	collectorByFunction := make(map[string]string, len(functionOrder))
	for _, function := range functionOrder {
		seriesMap[function] = []continuousTrendPointPayload{}
	}

	for _, record := range records {
		windowsPayload = append(windowsPayload, continuousTrendWindowPayload{
			WindowID:      record.window.ID,
			TaskID:        record.window.TaskID,
			WindowStartAt: record.window.WindowStartAt,
			WindowEndAt:   record.window.WindowEndAt,
			Status:        string(record.window.Status),
		})

		hotspotMap := make(map[string]hotspotPayload, len(record.hotspots))
		for _, hotspot := range record.hotspots {
			hotspotMap[hotspot.Function] = hotspot
		}

		for _, function := range functionOrder {
			hotspot := hotspotMap[function]
			if hotspot.Function != "" && collectorByFunction[function] == "" {
				collectorByFunction[function] = record.collectorType
			}
			seriesMap[function] = append(seriesMap[function], continuousTrendPointPayload{
				WindowID: record.window.ID,
				TaskID:   record.window.TaskID,
				Percent:  hotspot.Percent,
				Samples:  hotspot.Samples,
			})
		}
	}

	seriesPayload := make([]continuousTrendSeriesPayload, 0, len(functionOrder))
	for _, function := range functionOrder {
		points := seriesMap[function]
		if len(points) == 0 {
			continue
		}
		var total float64
		peak := 0.0
		for _, point := range points {
			total += point.Percent
			if point.Percent > peak {
				peak = point.Percent
			}
		}
		average := total / float64(len(points))
		delta := 0.0
		if len(points) > 1 {
			delta = points[0].Percent - points[len(points)-1].Percent
		}
		label, severity, reason := classifyTrend(peak, delta)
		seriesPayload = append(seriesPayload, continuousTrendSeriesPayload{
			Function: function,
			Average:  roundFloat(average, 2),
			Peak:     roundFloat(peak, 2),
			Delta:    roundFloat(delta, 2),
			Label:    label,
			Severity: severity,
			Reason:   reason,
			Baseline: trendBaselineForFunction(collectorByFunction[function], function, peak, baselines),
			Points:   points,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"windows": windowsPayload,
		"series":  seriesPayload,
	})
}

func trendBaselineForFunction(collectorType string, function string, peakPercent float64, baselines []minidrop.AttributionBaseline) *continuousTrendBaselinePayload {
	for _, baseline := range baselines {
		if baseline.CollectorType != "" && baseline.CollectorType != collectorType {
			continue
		}
		if !strings.Contains(strings.ToLower(function), strings.ToLower(baseline.FunctionPattern)) {
			continue
		}

		delta := roundFloat(peakPercent-baseline.ExpectedPercent, 1)
		status := "within"
		reason := "峰值与历史 baseline 偏差在 10 个百分点以内"
		switch {
		case delta >= 10:
			status = "above"
			reason = "峰值高于历史 baseline 10 个百分点以上"
		case delta <= -10:
			status = "below"
			reason = "峰值低于历史 baseline 10 个百分点以上"
		}

		return &continuousTrendBaselinePayload{
			Status:          status,
			Description:     baseline.Description,
			ExpectedPercent: roundFloat(baseline.ExpectedPercent, 1),
			ActualPercent:   roundFloat(peakPercent, 1),
			DeltaPercent:    delta,
			Reason:          reason,
		}
	}

	return nil
}

func classifyTrend(peak, delta float64) (string, string, string) {
	switch {
	case peak >= 50:
		return "持续高位", "critical", "峰值占比达到 50% 以上，建议优先查看对应窗口火焰图"
	case peak >= 35:
		return "基线偏高", "warning", "峰值占比达到 35% 以上，建议结合 baseline 偏差判断是否需要排查"
	case delta >= 15:
		return "明显升高", "warning", "最新窗口相对最早窗口上升 15 个百分点以上"
	case delta <= -15:
		return "明显回落", "success", "最新窗口相对最早窗口下降 15 个百分点以上"
	case delta >= 8 || delta <= -8:
		return "小幅波动", "normal", "最近窗口间占比变化达到 8 个百分点以上，但未触发明显趋势阈值"
	default:
		return "平稳", "normal", "最近窗口间占比变化未超过 8 个百分点"
	}
}

func (s *Service) toTaskResultPayload(c *gin.Context, task minidrop.Task, result minidrop.AnalysisResult) taskResultPayload {
	payload := taskResultPayload{
		FlamegraphURL: s.artifactURL(c, result.FlamegraphPath),
		TopNURL:       s.artifactURL(c, result.TopNPath),
		Summary:       result.Summary,
	}

	absPath := s.artifactAbsPath(result.TopNPath)
	hotspots, err := mustReadTopN(absPath)
	if err == nil {
		payload.Hotspots = hotspots
		payload.Attribution = s.loadOrBuildAttribution(task, result.TopNPath, result.ResourceTimelinePath, hotspots)
	}

	return payload
}

func (s *Service) loadOrBuildAttribution(task minidrop.Task, topNPath string, resourceTimelinePath string, hotspots []hotspotPayload) *attributionPayload {
	var existing minidrop.AttributionResult
	var hasRecord bool
	if err := s.db.First(&existing, "task_id = ?", task.ID).Error; err == nil {
		hasRecord = true
		payload, decodeErr := attributionPayloadFromRecord(existing)
		if decodeErr == nil && payload.ResourceTimeline != nil {
			return payload
		}
		if decodeErr != nil {
			s.log.Warn("decode persisted attribution failed", "task_id", task.ID, "error", decodeErr)
		} else {
			s.log.Info("rebuilding attribution with resource timeline", "task_id", task.ID)
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		s.log.Warn("load persisted attribution failed", "task_id", task.ID, "error", err)
	}

	var baselines []minidrop.AttributionBaseline
	if err := s.db.Order("collector_type asc, function_pattern asc").Find(&baselines).Error; err != nil {
		s.log.Warn("load attribution baselines failed", "task_id", task.ID, "error", err)
	}

	var timeline *resourceTimelinePayload
	if strings.TrimSpace(resourceTimelinePath) != "" {
		loadedTimeline, err := mustReadResourceTimeline(s.artifactAbsPath(resourceTimelinePath))
		if err != nil {
			s.log.Warn("read resource timeline failed", "task_id", task.ID, "path", resourceTimelinePath, "error", err)
		} else {
			timeline = loadedTimeline
		}
	}

	payload := buildAttributionWithBaselinesAndTimeline(task, topNPath, hotspots, baselines, timeline, resourceTimelinePath)
	now := time.Now().UTC()
	record, buildErr := attributionRecordFromPayload(task.ID, payload, now)
	if buildErr != nil {
		s.log.Warn("build attribution record failed", "task_id", task.ID, "error", buildErr)
		return payload
	}

	if hasRecord {
		record.ID = existing.ID
		record.CreatedAt = existing.CreatedAt
		record.UpdatedAt = now
		if err := s.db.Save(&record).Error; err != nil {
			s.log.Warn("persist rebuilt attribution failed", "task_id", task.ID, "error", err)
			return payload
		}
		payload.PersistedAt = &record.UpdatedAt
		return payload
	}

	if err := s.db.Create(&record).Error; err != nil {
		s.log.Warn("persist attribution failed", "task_id", task.ID, "error", err)
		return payload
	}
	payload.PersistedAt = &record.CreatedAt
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
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
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
