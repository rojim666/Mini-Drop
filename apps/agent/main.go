package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"mini-drop/internal/agent"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := agent.LoadConfigFromEnv()
	if err != nil {
		log.Fatalf("load agent config: %v", err)
	}

	svc, err := agent.New(cfg)
	if err != nil {
		log.Fatalf("create agent service: %v", err)
	}

	if err := svc.Run(ctx); err != nil {
		log.Fatalf("run agent service: %v", err)
	}
}
