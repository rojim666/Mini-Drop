package minidrop

import "time"

type Agent struct {
	ID              string      `gorm:"primaryKey;size:64"`
	Hostname        string      `gorm:"size:255;not null"`
	IP              string      `gorm:"size:255;not null"`
	Version         string      `gorm:"size:64;not null"`
	Status          AgentStatus `gorm:"size:32;not null;index"`
	LastHeartbeatAt time.Time   `gorm:"not null;index"`
	CreatedAt       time.Time   `gorm:"not null"`
	UpdatedAt       time.Time   `gorm:"not null"`
}

type Task struct {
	ID                   string     `gorm:"primaryKey;size:64"`
	TargetPID            int        `gorm:"not null"`
	TargetAgentID        string     `gorm:"size:64;not null;index"`
	SampleDurationSec    int        `gorm:"not null"`
	SampleRateHz         int        `gorm:"not null"`
	CollectorType        string     `gorm:"size:64;not null"`
	ContinuousProfileID  string     `gorm:"size:64;index"`
	ContinuousWindowID   string     `gorm:"size:64;index"`
	Status               TaskStatus `gorm:"size:32;not null;index"`
	StatusReason         string     `gorm:"size:1024;not null"`
	RawArtifactPath      string     `gorm:"size:1024"`
	AnalysisArtifactPath string     `gorm:"size:1024"`
	CreatedAt            time.Time  `gorm:"not null;index"`
	UpdatedAt            time.Time  `gorm:"not null"`
	StartedAt            *time.Time
	FinishedAt           *time.Time
}

type TaskStatusEvent struct {
	ID         string     `gorm:"primaryKey;size:64"`
	TaskID     string     `gorm:"size:64;not null;index"`
	FromStatus TaskStatus `gorm:"size:32"`
	ToStatus   TaskStatus `gorm:"size:32;not null"`
	Reason     string     `gorm:"size:1024;not null"`
	CreatedAt  time.Time  `gorm:"not null;index"`
}

type AuditLog struct {
	ID         string    `gorm:"primaryKey;size:64"`
	EntityType string    `gorm:"size:64;not null;index"`
	EntityID   string    `gorm:"size:64;not null;index"`
	Action     string    `gorm:"size:128;not null"`
	Reason     string    `gorm:"size:1024;not null"`
	CreatedAt  time.Time `gorm:"not null;index"`
}

type AnalysisResult struct {
	ID             string    `gorm:"primaryKey;size:64"`
	TaskID         string    `gorm:"size:64;not null;uniqueIndex"`
	FlamegraphPath string    `gorm:"size:1024;not null"`
	TopNPath       string    `gorm:"size:1024;not null"`
	Summary        string    `gorm:"size:2048;not null"`
	CreatedAt      time.Time `gorm:"not null"`
	UpdatedAt      time.Time `gorm:"not null"`
}

type AttributionBaseline struct {
	ID              string    `gorm:"primaryKey;size:64"`
	CollectorType   string    `gorm:"size:64;not null;index"`
	FunctionPattern string    `gorm:"size:255;not null;index"`
	ExpectedPercent float64   `gorm:"not null"`
	Description     string    `gorm:"size:1024;not null"`
	CreatedAt       time.Time `gorm:"not null"`
	UpdatedAt       time.Time `gorm:"not null"`
}

type AttributionResult struct {
	ID                  string    `gorm:"primaryKey;size:64"`
	TaskID              string    `gorm:"size:64;not null;uniqueIndex"`
	Conclusion          string    `gorm:"size:2048;not null"`
	Confidence          float64   `gorm:"not null"`
	EvidenceJSON        string    `gorm:"type:text;not null"`
	RecommendationsJSON string    `gorm:"type:text;not null"`
	SourceJSON          string    `gorm:"type:text;not null"`
	ToolTraceJSON       string    `gorm:"type:text;not null"`
	Prompt              string    `gorm:"type:text;not null"`
	CreatedAt           time.Time `gorm:"not null"`
	UpdatedAt           time.Time `gorm:"not null"`
}

type ContinuousProfile struct {
	ID                string `gorm:"primaryKey;size:64"`
	Name              string `gorm:"size:255;not null"`
	TargetPID         int    `gorm:"not null"`
	TargetAgentID     string `gorm:"size:64;not null;index"`
	SampleDurationSec int    `gorm:"not null"`
	SampleRateHz      int    `gorm:"not null"`
	CollectorType     string `gorm:"size:64;not null"`
	WindowDurationSec int    `gorm:"not null"`
	IntervalSec       int    `gorm:"not null"`
	Enabled           bool   `gorm:"not null;index"`
	LastWindowStartAt *time.Time
	LastScheduledAt   *time.Time
	CreatedAt         time.Time `gorm:"not null;index"`
	UpdatedAt         time.Time `gorm:"not null"`
}

type ContinuousProfileWindow struct {
	ID            string     `gorm:"primaryKey;size:64"`
	ProfileID     string     `gorm:"size:64;not null;index;uniqueIndex:idx_profile_window_start"`
	TaskID        string     `gorm:"size:64;not null;uniqueIndex"`
	WindowStartAt time.Time  `gorm:"not null;index;uniqueIndex:idx_profile_window_start"`
	WindowEndAt   time.Time  `gorm:"not null;index"`
	Status        TaskStatus `gorm:"size:32;not null;index"`
	StatusReason  string     `gorm:"size:1024;not null"`
	CreatedAt     time.Time  `gorm:"not null;index"`
	UpdatedAt     time.Time  `gorm:"not null"`
}
