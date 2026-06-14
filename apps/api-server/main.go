package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"mini-drop/internal/apiserver"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := apiserver.LoadConfigFromEnv()
	if err != nil {
		log.Fatalf("load api config: %v", err)
	}

	svc, err := apiserver.New(ctx, cfg)
	if err != nil {
		log.Fatalf("create api service: %v", err)
	}
	defer svc.Close()

	if err := svc.Run(ctx); err != nil {
		log.Fatalf("run api service: %v", err)
	}
}
