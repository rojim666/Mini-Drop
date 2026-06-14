//go:build linux

package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func (s *Service) runPerfCollector(ctx context.Context, task apiTask) (string, string, string, error) {
	perfPath, err := exec.LookPath("perf")
	if err != nil {
		return "", "", "", errors.New("perf command not found; install linux-tools or linux-perf")
	}

	if err := checkPerfEventParanoid(); err != nil {
		return "", "", "", err
	}

	artifactDir := filepath.Join(s.cfg.ArtifactDir, task.ID, "raw")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return "", "", "", fmt.Errorf("create perf artifact dir: %w", err)
	}

	rawAbsPath := filepath.Join(artifactDir, "perf.data")
	timeout := time.Duration(task.SampleDurationSec+10) * time.Second
	collectCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{
		"record",
		"-F", strconv.Itoa(task.SampleRateHz),
		"-g",
		"-p", strconv.Itoa(task.TargetPID),
		"-o", rawAbsPath,
		"--",
		"sleep", strconv.Itoa(task.SampleDurationSec),
	}
	cmd := exec.CommandContext(collectCtx, perfPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", "", "", fmt.Errorf("start perf record: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var runErr error
	select {
	case <-collectCtx.Done():
		killProcessGroup(cmd.Process.Pid, syscall.SIGTERM)
		select {
		case runErr = <-done:
		case <-time.After(5 * time.Second):
			killProcessGroup(cmd.Process.Pid, syscall.SIGKILL)
			runErr = <-done
		}
		if errors.Is(collectCtx.Err(), context.DeadlineExceeded) {
			return "", "", "", fmt.Errorf("perf record timeout after %ds", int(timeout.Seconds()))
		}
		return "", "", "", collectCtx.Err()
	case runErr = <-done:
	}

	if runErr != nil {
		details := strings.TrimSpace(stderr.String())
		if details == "" {
			details = runErr.Error()
		}
		return "", "", "", fmt.Errorf("perf record failed: %s", details)
	}

	if info, err := os.Stat(rawAbsPath); err != nil {
		return "", "", "", fmt.Errorf("perf.data not created: %w", err)
	} else if info.Size() == 0 {
		return "", "", "", errors.New("perf.data is empty")
	}

	rawRelPath := filepath.ToSlash(filepath.Join(task.ID, "raw", "perf.data"))
	return rawRelPath, rawAbsPath, "perf record completed", nil
}

func checkPerfEventParanoid() error {
	payload, err := os.ReadFile("/proc/sys/kernel/perf_event_paranoid")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read perf_event_paranoid: %w", err)
	}

	value, err := strconv.Atoi(strings.TrimSpace(string(payload)))
	if err != nil {
		return fmt.Errorf("parse perf_event_paranoid: %w", err)
	}
	if value > 1 {
		return fmt.Errorf("perf_event_paranoid=%d blocks process profiling; set it to 1 or lower, or run with CAP_PERFMON/CAP_SYS_ADMIN", value)
	}
	return nil
}

func killProcessGroup(pid int, signal syscall.Signal) {
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(-pid, signal)
}
