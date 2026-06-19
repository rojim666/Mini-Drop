package minidrop

import "testing"

func TestCreateTaskInputNormalizeAndValidate(t *testing.T) {
	input := CreateTaskInput{
		TargetPID:         1234,
		SampleDurationSec: 15,
		SampleRateHz:      99,
	}
	input.Normalize()

	if input.CollectorType != CollectorMockPerf {
		t.Fatalf("expected default collector %s, got %s", CollectorMockPerf, input.CollectorType)
	}
	if err := ValidateCreateTaskInput(input); err != nil {
		t.Fatalf("expected normalized task input to validate: %v", err)
	}
}

func TestCreateTaskInputValidationRejectsBadValues(t *testing.T) {
	tests := []struct {
		name  string
		input CreateTaskInput
	}{
		{
			name: "missing pid",
			input: CreateTaskInput{
				SampleDurationSec: 15,
				SampleRateHz:      99,
				CollectorType:     CollectorMockPerf,
			},
		},
		{
			name: "duration too short",
			input: CreateTaskInput{
				TargetPID:         1234,
				SampleDurationSec: 0,
				SampleRateHz:      99,
				CollectorType:     CollectorMockPerf,
			},
		},
		{
			name: "sample rate too high",
			input: CreateTaskInput{
				TargetPID:         1234,
				SampleDurationSec: 15,
				SampleRateHz:      10001,
				CollectorType:     CollectorMockPerf,
			},
		},
		{
			name: "unsupported collector",
			input: CreateTaskInput{
				TargetPID:         1234,
				SampleDurationSec: 15,
				SampleRateHz:      99,
				CollectorType:     "unknown",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateCreateTaskInput(tt.input); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestContinuousProfileInputNormalizeAndValidate(t *testing.T) {
	input := CreateContinuousProfileInput{
		TargetPID:         1234,
		SampleDurationSec: 15,
		SampleRateHz:      99,
		Name:              "demo profile",
		StaggerSec:        15,
	}
	input.Normalize()

	if input.CollectorType != CollectorMockPerf {
		t.Fatalf("expected default collector %s, got %s", CollectorMockPerf, input.CollectorType)
	}
	if input.IntervalSec != ContinuousWindowDurationSec {
		t.Fatalf("expected default interval %d, got %d", ContinuousWindowDurationSec, input.IntervalSec)
	}
	if input.ScheduleMode != ContinuousScheduleInterval {
		t.Fatalf("expected interval schedule, got %s", input.ScheduleMode)
	}
	if err := ValidateCreateContinuousProfileInput(input); err != nil {
		t.Fatalf("expected normalized profile input to validate: %v", err)
	}
}

func TestContinuousProfileValidationAllowsCron(t *testing.T) {
	input := CreateContinuousProfileInput{
		TargetPID:         1234,
		SampleDurationSec: 15,
		SampleRateHz:      99,
		CollectorType:     CollectorMockPerf,
		ScheduleMode:      ContinuousScheduleCron,
		CronExpression:    "*/5 * * * *",
		IntervalSec:       ContinuousWindowDurationSec,
		StaggerSec:        30,
	}

	if err := ValidateCreateContinuousProfileInput(input); err != nil {
		t.Fatalf("expected cron profile input to validate: %v", err)
	}
}

func TestContinuousProfileValidationRejectsBadValues(t *testing.T) {
	base := CreateContinuousProfileInput{
		TargetPID:         1234,
		SampleDurationSec: 15,
		SampleRateHz:      99,
		CollectorType:     CollectorMockPerf,
		IntervalSec:       ContinuousWindowDurationSec,
		ScheduleMode:      ContinuousScheduleInterval,
	}

	tests := []struct {
		name   string
		mutate func(*CreateContinuousProfileInput)
	}{
		{
			name: "interval too short",
			mutate: func(input *CreateContinuousProfileInput) {
				input.IntervalSec = 29
			},
		},
		{
			name: "sample duration exceeds window",
			mutate: func(input *CreateContinuousProfileInput) {
				input.SampleDurationSec = ContinuousWindowDurationSec + 1
			},
		},
		{
			name: "unsupported schedule",
			mutate: func(input *CreateContinuousProfileInput) {
				input.ScheduleMode = "calendar"
			},
		},
		{
			name: "cron expression required",
			mutate: func(input *CreateContinuousProfileInput) {
				input.ScheduleMode = ContinuousScheduleCron
				input.CronExpression = ""
			},
		},
		{
			name: "stagger too large",
			mutate: func(input *CreateContinuousProfileInput) {
				input.StaggerSec = 301
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := base
			tt.mutate(&input)
			if err := ValidateCreateContinuousProfileInput(input); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
