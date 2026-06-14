package apiserver

import (
	"testing"
	"time"

	"mini-drop/internal/minidrop"
)

func TestNextContinuousWindowStartAppliesIntervalStagger(t *testing.T) {
	now := time.Date(2026, 1, 2, 10, 7, 0, 0, time.UTC)
	profile := minidrop.ContinuousProfile{
		WindowDurationSec: minidrop.ContinuousWindowDurationSec,
		IntervalSec:       300,
		ScheduleMode:      minidrop.ContinuousScheduleInterval,
		StaggerSec:        45,
	}

	got := nextContinuousWindowStart(profile, now)
	want := time.Date(2026, 1, 2, 10, 5, 45, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected staggered interval start %s, got %s", want, got)
	}
}

func TestNextContinuousWindowStartUsesCronWithStagger(t *testing.T) {
	now := time.Date(2026, 1, 2, 10, 3, 0, 0, time.UTC)
	profile := minidrop.ContinuousProfile{
		WindowDurationSec: minidrop.ContinuousWindowDurationSec,
		IntervalSec:       300,
		ScheduleMode:      minidrop.ContinuousScheduleCron,
		CronExpression:    "*/5 * * * *",
		StaggerSec:        30,
	}

	got := nextContinuousWindowStart(profile, now)
	want := time.Date(2026, 1, 2, 10, 5, 30, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected cron start %s, got %s", want, got)
	}
}

func TestNextCronScheduleSupportsEveryShortcut(t *testing.T) {
	after := time.Date(2026, 1, 2, 10, 3, 4, 0, time.UTC)
	got, err := nextCronSchedule("@every 5m", after)
	if err != nil {
		t.Fatalf("next cron schedule: %v", err)
	}

	want := time.Date(2026, 1, 2, 10, 8, 4, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected @every schedule %s, got %s", want, got)
	}
}

func TestValidateContinuousScheduleRejectsInvalidCron(t *testing.T) {
	input := minidrop.CreateContinuousProfileInput{
		ScheduleMode:   minidrop.ContinuousScheduleCron,
		CronExpression: "61 * * * *",
	}

	if err := validateContinuousSchedule(input); err == nil {
		t.Fatal("expected invalid cron expression")
	}
}
