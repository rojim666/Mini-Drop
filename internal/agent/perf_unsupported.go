//go:build !linux

package agent

import (
	"context"
	"errors"
)

func (s *Service) runPerfCollector(ctx context.Context, task apiTask) (string, string, string, error) {
	return "", "", "", errors.New("perf collector requires linux")
}
