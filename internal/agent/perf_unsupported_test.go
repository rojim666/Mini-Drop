//go:build !linux

package agent

import (
	"context"
	"strings"
	"testing"
)

func TestRunPerfCollectorRequiresLinux(t *testing.T) {
	svc := &Service{}
	_, _, _, err := svc.runPerfCollector(context.Background(), apiTask{CollectorType: "perf"})
	if err == nil {
		t.Fatal("expected perf collector to fail on non-linux")
	}
	if !strings.Contains(err.Error(), "perf collector requires linux") {
		t.Fatalf("expected linux requirement error, got %q", err.Error())
	}
}

func TestRunEBPFSyscallCollectorRequiresLinux(t *testing.T) {
	svc := &Service{}
	_, _, _, err := svc.runEBPFSyscallCollector(context.Background(), apiTask{CollectorType: "ebpf-syscall"})
	if err == nil {
		t.Fatal("expected ebpf-syscall collector to fail on non-linux")
	}
	if !strings.Contains(err.Error(), "ebpf-syscall collector requires linux") {
		t.Fatalf("expected linux requirement error, got %q", err.Error())
	}
}
