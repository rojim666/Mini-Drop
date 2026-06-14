package minidrop

import (
	"errors"
	"strings"
)

type CreateTaskInput struct {
	TargetPID         int    `json:"target_pid"`
	TargetAgentID     string `json:"target_agent_id"`
	SampleDurationSec int    `json:"sample_duration_sec"`
	SampleRateHz      int    `json:"sample_rate_hz"`
	CollectorType     string `json:"collector_type"`
}

func (in *CreateTaskInput) Normalize() {
	in.TargetAgentID = strings.TrimSpace(in.TargetAgentID)
	in.CollectorType = strings.TrimSpace(in.CollectorType)
	if in.CollectorType == "" {
		in.CollectorType = "mock-perf"
	}
}

func ValidateCreateTaskInput(in CreateTaskInput) error {
	if in.TargetPID <= 0 {
		return errors.New("target_pid must be greater than 0")
	}
	if in.SampleDurationSec < 1 || in.SampleDurationSec > 300 {
		return errors.New("sample_duration_sec must be between 1 and 300")
	}
	if in.SampleRateHz < 1 || in.SampleRateHz > 999 {
		return errors.New("sample_rate_hz must be between 1 and 999")
	}
	if strings.TrimSpace(in.CollectorType) == "" {
		return errors.New("collector_type is required")
	}
	return nil
}
