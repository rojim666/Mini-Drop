//go:build !linux

package agent

import (
	"context"
	"errors"
)

func (s *Service) runEBPFSyscallCollector(ctx context.Context, task apiTask) (string, string, string, error) {
	return "", "", "", errors.New("ebpf-syscall collector requires linux")
}
