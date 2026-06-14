package minidrop

import "fmt"

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "PENDING"
	TaskStatusRunning   TaskStatus = "RUNNING"
	TaskStatusUploading TaskStatus = "UPLOADING"
	TaskStatusDone      TaskStatus = "DONE"
	TaskStatusFailed    TaskStatus = "FAILED"
)

type AgentStatus string

const (
	AgentStatusUnknown AgentStatus = "UNKNOWN"
	AgentStatusOnline  AgentStatus = "ONLINE"
	AgentStatusOffline AgentStatus = "OFFLINE"
)

var allowedTaskTransitions = map[TaskStatus]map[TaskStatus]bool{
	TaskStatusPending: {
		TaskStatusRunning: true,
		TaskStatusFailed:  true,
	},
	TaskStatusRunning: {
		TaskStatusUploading: true,
		TaskStatusFailed:    true,
	},
	TaskStatusUploading: {
		TaskStatusDone:   true,
		TaskStatusFailed: true,
	},
}

func ValidateTaskTransition(from, to TaskStatus) error {
	if from == "" && to == TaskStatusPending {
		return nil
	}

	if next, ok := allowedTaskTransitions[from]; ok && next[to] {
		return nil
	}

	return fmt.Errorf("invalid task transition %s -> %s", from, to)
}

func IsTerminalTaskStatus(status TaskStatus) bool {
	return status == TaskStatusDone || status == TaskStatusFailed
}
