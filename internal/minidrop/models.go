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
