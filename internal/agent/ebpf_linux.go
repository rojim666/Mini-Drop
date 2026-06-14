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

func (s *Service) runEBPFSyscallCollector(ctx context.Context, task apiTask) (string, string, string, error) {
	bpftracePath, err := exec.LookPath("bpftrace")
	if err != nil {
		return "", "", "", errors.New("bpftrace command not found; install bpftrace to use ebpf-syscall")
	}

	artifactDir := filepath.Join(s.cfg.ArtifactDir, task.ID, "raw")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return "", "", "", fmt.Errorf("create ebpf artifact dir: %w", err)
	}

	rawAbsPath := filepath.Join(artifactDir, "ebpf.syscalls.txt")
	timeout := time.Duration(task.SampleDurationSec+10) * time.Second
	collectCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	program := fmt.Sprintf(
		"tracepoint:syscalls:sys_enter_* /pid == %d/ { @[probe] = count(); } interval:s:%d { exit(); }",
		task.TargetPID,
		task.SampleDurationSec,
	)
	cmd := exec.CommandContext(collectCtx, bpftracePath, "-e", program)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", "", "", fmt.Errorf("start bpftrace: %w", err)
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
			return "", "", "", fmt.Errorf("bpftrace timeout after %ds", int(timeout.Seconds()))
		}
		return "", "", "", collectCtx.Err()
	case runErr = <-done:
	}

	output := strings.TrimSpace(stdout.String())
	if runErr != nil {
		details := strings.TrimSpace(stderr.String())
		if details == "" {
			details = output
		}
		if details == "" {
			details = runErr.Error()
		}
		return "", "", "", fmt.Errorf("bpftrace failed: %s", details)
	}
	if output == "" {
		details := strings.TrimSpace(stderr.String())
		if strings.Contains(strings.ToLower(details), "permission") {
			return "", "", "", fmt.Errorf("bpftrace permission denied: %s", details)
		}
		return "", "", "", errors.New("bpftrace produced no syscall samples")
	}

	content := "# Mini-Drop ebpf-syscall raw artifact\n" +
		"# collector_type=ebpf-syscall\n" +
		"# target_pid=" + strconv.Itoa(task.TargetPID) + "\n" +
		"# sample_duration_sec=" + strconv.Itoa(task.SampleDurationSec) + "\n" +
		output + "\n"
	if err := os.WriteFile(rawAbsPath, []byte(content), 0o644); err != nil {
		return "", "", "", fmt.Errorf("write ebpf artifact: %w", err)
	}

	rawRelPath := filepath.ToSlash(filepath.Join(task.ID, "raw", "ebpf.syscalls.txt"))
	return rawRelPath, rawAbsPath, "bpftrace syscall histogram completed", nil
}
