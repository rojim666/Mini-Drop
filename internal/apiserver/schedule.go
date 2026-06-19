package apiserver

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"mini-drop/internal/minidrop"
)

func validateContinuousSchedule(input minidrop.CreateContinuousProfileInput) error {
	if normalizedScheduleMode(input.ScheduleMode) != minidrop.ContinuousScheduleCron {
		return nil
	}
	_, err := nextCronSchedule(input.CronExpression, time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC))
	if err != nil {
		return fmt.Errorf("cron_expression is invalid: %w", err)
	}
	return nil
}

func normalizedScheduleMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return minidrop.ContinuousScheduleInterval
	}
	return mode
}

func continuousProfileDue(profile minidrop.ContinuousProfile, now time.Time) bool {
	if !profile.Enabled {
		return false
	}
	if profile.LastWindowStartAt == nil {
		return true
	}
	return !now.Before(nextContinuousWindowStart(profile, now))
}

func nextContinuousWindowStart(profile minidrop.ContinuousProfile, now time.Time) time.Time {
	switch normalizedScheduleMode(profile.ScheduleMode) {
	case minidrop.ContinuousScheduleCron:
		return nextCronWindowStart(profile, now)
	default:
		return nextIntervalWindowStart(profile, now)
	}
}

func nextIntervalWindowStart(profile minidrop.ContinuousProfile, now time.Time) time.Time {
	stagger := time.Duration(profile.StaggerSec) * time.Second
	if profile.LastWindowStartAt != nil {
		return profile.LastWindowStartAt.Add(continuousInterval(profile))
	}

	windowDuration := time.Duration(profile.WindowDurationSec) * time.Second
	if windowDuration <= 0 {
		windowDuration = time.Duration(minidrop.ContinuousWindowDurationSec) * time.Second
	}
	return now.Add(-stagger).Truncate(windowDuration).Add(stagger)
}

func nextCronWindowStart(profile minidrop.ContinuousProfile, now time.Time) time.Time {
	anchor := now
	if profile.LastWindowStartAt != nil {
		anchor = profile.LastWindowStartAt.Add(-time.Duration(profile.StaggerSec) * time.Second)
	}

	next, err := nextCronSchedule(profile.CronExpression, anchor)
	if err != nil {
		return nextIntervalWindowStart(profile, now)
	}
	return next.Add(time.Duration(profile.StaggerSec) * time.Second)
}

func continuousInterval(profile minidrop.ContinuousProfile) time.Duration {
	interval := time.Duration(profile.IntervalSec) * time.Second
	if interval <= 0 {
		interval = time.Duration(minidrop.ContinuousWindowDurationSec) * time.Second
	}
	return interval
}

func nextCronSchedule(expression string, after time.Time) (time.Time, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return time.Time{}, errors.New("empty expression")
	}
	if strings.HasPrefix(expression, "@every ") {
		rawDuration := strings.TrimSpace(strings.TrimPrefix(expression, "@every "))
		duration, err := time.ParseDuration(rawDuration)
		if err != nil {
			return time.Time{}, err
		}
		if duration < 30*time.Second || duration > time.Hour {
			return time.Time{}, errors.New("@every duration must be between 30s and 1h")
		}
		return after.Truncate(time.Second).Add(duration), nil
	}

	spec, err := parseCronExpression(expression)
	if err != nil {
		return time.Time{}, err
	}

	candidate := after.Truncate(time.Minute).Add(time.Minute)
	deadline := after.Add(366 * 24 * time.Hour)
	for !candidate.After(deadline) {
		if spec.matches(candidate) {
			return candidate, nil
		}
		candidate = candidate.Add(time.Minute)
	}

	return time.Time{}, errors.New("no matching time within one year")
}

type cronSpec struct {
	minutes     map[int]bool
	hours       map[int]bool
	daysOfMonth map[int]bool
	months      map[int]bool
	weekdays    map[int]bool
}

func parseCronExpression(expression string) (cronSpec, error) {
	fields := strings.Fields(expression)
	if len(fields) != 5 {
		return cronSpec{}, errors.New("expected five fields: minute hour day-of-month month day-of-week")
	}

	minutes, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return cronSpec{}, fmt.Errorf("minute: %w", err)
	}
	hours, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return cronSpec{}, fmt.Errorf("hour: %w", err)
	}
	daysOfMonth, err := parseCronField(fields[2], 1, 31)
	if err != nil {
		return cronSpec{}, fmt.Errorf("day-of-month: %w", err)
	}
	months, err := parseCronField(fields[3], 1, 12)
	if err != nil {
		return cronSpec{}, fmt.Errorf("month: %w", err)
	}
	weekdays, err := parseCronField(fields[4], 0, 7)
	if err != nil {
		return cronSpec{}, fmt.Errorf("day-of-week: %w", err)
	}
	if weekdays[7] {
		weekdays[0] = true
		delete(weekdays, 7)
	}

	return cronSpec{
		minutes:     minutes,
		hours:       hours,
		daysOfMonth: daysOfMonth,
		months:      months,
		weekdays:    weekdays,
	}, nil
}

func parseCronField(field string, minValue int, maxValue int) (map[int]bool, error) {
	values := map[int]bool{}
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, errors.New("empty list item")
		}

		step := 1
		base := part
		if strings.Contains(part, "/") {
			pieces := strings.Split(part, "/")
			if len(pieces) != 2 || pieces[1] == "" {
				return nil, fmt.Errorf("invalid step %q", part)
			}
			parsedStep, err := strconv.Atoi(pieces[1])
			if err != nil || parsedStep <= 0 {
				return nil, fmt.Errorf("invalid step %q", part)
			}
			step = parsedStep
			base = pieces[0]
		}

		start, end, err := cronFieldRange(base, minValue, maxValue)
		if err != nil {
			return nil, err
		}
		for value := start; value <= end; value += step {
			values[value] = true
		}
	}
	return values, nil
}

func cronFieldRange(field string, minValue int, maxValue int) (int, int, error) {
	if field == "*" || field == "" {
		return minValue, maxValue, nil
	}

	if strings.Contains(field, "-") {
		pieces := strings.Split(field, "-")
		if len(pieces) != 2 {
			return 0, 0, fmt.Errorf("invalid range %q", field)
		}
		start, err := parseCronNumber(pieces[0], minValue, maxValue)
		if err != nil {
			return 0, 0, err
		}
		end, err := parseCronNumber(pieces[1], minValue, maxValue)
		if err != nil {
			return 0, 0, err
		}
		if start > end {
			return 0, 0, fmt.Errorf("range start exceeds end %q", field)
		}
		return start, end, nil
	}

	value, err := parseCronNumber(field, minValue, maxValue)
	if err != nil {
		return 0, 0, err
	}
	return value, value, nil
}

func parseCronNumber(raw string, minValue int, maxValue int) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q", raw)
	}
	if value < minValue || value > maxValue {
		return 0, fmt.Errorf("%d outside allowed range %d-%d", value, minValue, maxValue)
	}
	return value, nil
}

func (spec cronSpec) matches(candidate time.Time) bool {
	weekday := int(candidate.Weekday())
	return spec.minutes[candidate.Minute()] &&
		spec.hours[candidate.Hour()] &&
		spec.daysOfMonth[candidate.Day()] &&
		spec.months[int(candidate.Month())] &&
		spec.weekdays[weekday]
}
