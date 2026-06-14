package apiserver

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	Addr                 string
	DataDir              string
	DBPath               string
	ArtifactDir          string
	AllowedOrigin        string
	OfflineAfter         time.Duration
	OfflineCheckInterval time.Duration
}

func LoadConfigFromEnv() (Config, error) {
	root, err := os.Getwd()
	if err != nil {
		return Config{}, fmt.Errorf("get working directory: %w", err)
	}

	dataDir := getenv("MINIDROP_DATA_DIR", filepath.Join(root, "data"))
	artifactDir := getenv("MINIDROP_ARTIFACT_DIR", filepath.Join(root, "artifacts"))

	return Config{
		Addr:                 getenv("MINIDROP_API_ADDR", "127.0.0.1:8080"),
		DataDir:              dataDir,
		DBPath:               getenv("MINIDROP_DB_PATH", filepath.Join(dataDir, "mini-drop.db")),
		ArtifactDir:          artifactDir,
		AllowedOrigin:        getenv("MINIDROP_ALLOWED_ORIGIN", "http://127.0.0.1:5173"),
		OfflineAfter:         durationFromEnv("MINIDROP_OFFLINE_AFTER_SEC", 30),
		OfflineCheckInterval: durationFromEnv("MINIDROP_OFFLINE_SCAN_SEC", 10),
	}, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func durationFromEnv(key string, fallbackSeconds int) time.Duration {
	value := getenv(key, "")
	if value == "" {
		return time.Duration(fallbackSeconds) * time.Second
	}

	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return time.Duration(fallbackSeconds) * time.Second
	}

	return time.Duration(seconds) * time.Second
}
