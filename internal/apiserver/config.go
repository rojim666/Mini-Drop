package apiserver

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr                   string
	DataDir                string
	DBDriver               string
	DBPath                 string
	PostgresDSN            string
	ArtifactDir            string
	StorageBackend         string
	MinIOEndpoint          string
	MinIOPublicEndpoint    string
	MinIOAccessKey         string
	MinIOSecretKey         string
	MinIOBucket            string
	MinIORegion            string
	MinIOUseSSL            bool
	MinIOPresignTTL        time.Duration
	AllowedOrigin          string
	AgentAllowlist         []string
	OfflineAfter           time.Duration
	OfflineCheckInterval   time.Duration
	ContinuousScanInterval time.Duration
}

func LoadConfigFromEnv() (Config, error) {
	root, err := os.Getwd()
	if err != nil {
		return Config{}, fmt.Errorf("get working directory: %w", err)
	}

	dataDir := getenv("MINIDROP_DATA_DIR", filepath.Join(root, "data"))
	artifactDir := getenv("MINIDROP_ARTIFACT_DIR", filepath.Join(root, "artifacts"))

	return Config{
		Addr:                   getenv("MINIDROP_API_ADDR", "127.0.0.1:8080"),
		DataDir:                dataDir,
		DBDriver:               getenv("MINIDROP_DB_DRIVER", "sqlite"),
		DBPath:                 getenv("MINIDROP_DB_PATH", filepath.Join(dataDir, "mini-drop.db")),
		PostgresDSN:            getenv("MINIDROP_POSTGRES_DSN", ""),
		ArtifactDir:            artifactDir,
		StorageBackend:         getenv("MINIDROP_STORAGE_BACKEND", "local"),
		MinIOEndpoint:          getenv("MINIDROP_MINIO_ENDPOINT", "127.0.0.1:9000"),
		MinIOPublicEndpoint:    getenv("MINIDROP_MINIO_PUBLIC_ENDPOINT", ""),
		MinIOAccessKey:         getenv("MINIDROP_MINIO_ACCESS_KEY", "minidrop"),
		MinIOSecretKey:         getenv("MINIDROP_MINIO_SECRET_KEY", "minidrop123"),
		MinIOBucket:            getenv("MINIDROP_MINIO_BUCKET", "mini-drop-artifacts"),
		MinIORegion:            getenv("MINIDROP_MINIO_REGION", "us-east-1"),
		MinIOUseSSL:            boolFromEnv("MINIDROP_MINIO_USE_SSL", false),
		MinIOPresignTTL:        durationFromEnv("MINIDROP_MINIO_PRESIGN_TTL_SEC", 900),
		AllowedOrigin:          getenv("MINIDROP_ALLOWED_ORIGIN", "http://127.0.0.1:5173"),
		AgentAllowlist:         splitCSV(getenv("MINIDROP_AGENT_ALLOWLIST", "")),
		OfflineAfter:           durationFromEnv("MINIDROP_OFFLINE_AFTER_SEC", 30),
		OfflineCheckInterval:   durationFromEnv("MINIDROP_OFFLINE_SCAN_SEC", 10),
		ContinuousScanInterval: durationFromEnv("MINIDROP_CONTINUOUS_SCAN_SEC", 60),
	}, nil
}

func (cfg Config) withDefaults() Config {
	if cfg.DBDriver == "" {
		cfg.DBDriver = "sqlite"
	}
	if cfg.StorageBackend == "" {
		cfg.StorageBackend = "local"
	}
	if cfg.MinIOEndpoint == "" {
		cfg.MinIOEndpoint = "127.0.0.1:9000"
	}
	if cfg.MinIOAccessKey == "" {
		cfg.MinIOAccessKey = "minidrop"
	}
	if cfg.MinIOSecretKey == "" {
		cfg.MinIOSecretKey = "minidrop123"
	}
	if cfg.MinIOBucket == "" {
		cfg.MinIOBucket = "mini-drop-artifacts"
	}
	if cfg.MinIORegion == "" {
		cfg.MinIORegion = "us-east-1"
	}
	if cfg.MinIOPresignTTL <= 0 {
		cfg.MinIOPresignTTL = 15 * time.Minute
	}
	if cfg.OfflineAfter <= 0 {
		cfg.OfflineAfter = 30 * time.Second
	}
	if cfg.OfflineCheckInterval <= 0 {
		cfg.OfflineCheckInterval = 10 * time.Second
	}
	if cfg.ContinuousScanInterval <= 0 {
		cfg.ContinuousScanInterval = 60 * time.Second
	}
	return cfg
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func boolFromEnv(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(getenv(key, "")))
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func splitCSV(raw string) []string {
	values := []string{}
	for _, part := range strings.Split(raw, ",") {
		value := strings.TrimSpace(part)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
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
