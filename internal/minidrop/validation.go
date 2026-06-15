package minidrop

import (
	"errors"
	"strings"
)

const (
	CollectorMockPerf    = "mock-perf"
	CollectorPerf        = "perf"
	CollectorEBPFSyscall = "ebpf-syscall"
	CollectorPySpy       = "py-spy"

	ContinuousWindowDurationSec = 300
)

type CreateTaskInput struct {
	TargetPID         int    `json:"target_pid"`
	TargetAgentID     string `json:"target_agent_id"`
	SampleDurationSec int    `json:"sample_duration_sec"`
	SampleRateHz      int    `json:"sample_rate_hz"`
	CollectorType     string `json:"collector_type"`
}

type CreateContinuousProfileInput struct {
	Name              string `json:"name"`
	TargetPID         int    `json:"target_pid"`
	TargetAgentID     string `json:"target_agent_id"`
	SampleDurationSec int    `json:"sample_duration_sec"`
	SampleRateHz      int    `json:"sample_rate_hz"`
	CollectorType     string `json:"collector_type"`
	IntervalSec       int    `json:"interval_sec"`
}

func (in *CreateTaskInput) Normalize() {
	in.TargetAgentID = strings.TrimSpace(in.TargetAgentID)
	in.CollectorType = strings.ToLower(strings.TrimSpace(in.CollectorType))
	if in.CollectorType == "" {
		in.CollectorType = CollectorMockPerf
	}
}

func (in *CreateContinuousProfileInput) Normalize() {
	in.Name = strings.TrimSpace(in.Name)
	in.TargetAgentID = strings.TrimSpace(in.TargetAgentID)
	in.CollectorType = strings.ToLower(strings.TrimSpace(in.CollectorType))
	if in.CollectorType == "" {
		in.CollectorType = CollectorMockPerf
	}
	if in.IntervalSec == 0 {
		in.IntervalSec = ContinuousWindowDurationSec
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
	if in.CollectorType != CollectorMockPerf && in.CollectorType != CollectorPerf && in.CollectorType != CollectorEBPFSyscall && in.CollectorType != CollectorPySpy {
		return errors.New("collector_type must be one of: mock-perf, perf, ebpf-syscall, py-spy")
	}
	return nil
}

func ValidateCreateContinuousProfileInput(in CreateContinuousProfileInput) error {
	taskInput := CreateTaskInput{
		TargetPID:         in.TargetPID,
		TargetAgentID:     in.TargetAgentID,
		SampleDurationSec: in.SampleDurationSec,
		SampleRateHz:      in.SampleRateHz,
		CollectorType:     in.CollectorType,
	}
	if err := ValidateCreateTaskInput(taskInput); err != nil {
		return err
	}
	if in.SampleDurationSec > ContinuousWindowDurationSec {
		return errors.New("sample_duration_sec must not exceed the 300 second profiling window")
	}
	if in.IntervalSec < 30 || in.IntervalSec > 3600 {
		return errors.New("interval_sec must be between 30 and 3600")
	}
	return nil
}
