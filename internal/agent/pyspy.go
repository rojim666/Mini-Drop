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
	"time"
)

func (s *Service) runPySpyCollector(ctx context.Context, task apiTask) (string, string, string, error) {
	pyspyPath, err := exec.LookPath("py-spy")
	if err != nil {
		return "", "", "", errors.New("py-spy command not found; install py-spy to use the py-spy collector")
	}

	artifactDir := filepath.Join(s.cfg.ArtifactDir, task.ID, "raw")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return "", "", "", fmt.Errorf("create py-spy artifact dir: %w", err)
	}

	rawAbsPath := filepath.Join(artifactDir, "pyspy.raw.txt")
	timeout := time.Duration(task.SampleDurationSec+10) * time.Second
	collectCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{
		"record",
		"--pid", strconv.Itoa(task.TargetPID),
		"--duration", strconv.Itoa(task.SampleDurationSec),
		"--rate", strconv.Itoa(task.SampleRateHz),
		"--format", "raw",
		"--output", rawAbsPath,
	}
	cmd := exec.CommandContext(collectCtx, pyspyPath, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(collectCtx.Err(), context.DeadlineExceeded) {
			return "", "", "", fmt.Errorf("py-spy timeout after %ds", int(timeout.Seconds()))
		}
		details := strings.TrimSpace(stderr.String())
		if details == "" {
			details = err.Error()
		}
		return "", "", "", fmt.Errorf("py-spy failed: %s", details)
	}

	if info, err := os.Stat(rawAbsPath); err != nil {
		return "", "", "", fmt.Errorf("py-spy raw artifact not created: %w", err)
	} else if info.Size() == 0 {
		return "", "", "", errors.New("py-spy raw artifact is empty")
	}

	rawRelPath := filepath.ToSlash(filepath.Join(task.ID, "raw", "pyspy.raw.txt"))
	return rawRelPath, rawAbsPath, "py-spy raw stacks completed", nil
}
