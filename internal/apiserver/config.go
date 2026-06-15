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
	AuthEnabled            bool
	AuthUsername           string
	AuthPassword           string
	AuthTokenSecret        string
	AuthSessionTTL         time.Duration
	AgentAllowlist         []string
	AIEnabled              bool
	AIBaseURL              string
	AIAPIKey               string
	AIModel                string
	AITimeout              time.Duration
	AIMaxTokens            int
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
		AuthEnabled:            boolFromEnv("MINIDROP_AUTH_ENABLED", true),
		AuthUsername:           getenv("MINIDROP_AUTH_USERNAME", "demo"),
		AuthPassword:           getenv("MINIDROP_AUTH_PASSWORD", "minidrop"),
		AuthTokenSecret:        getenv("MINIDROP_AUTH_TOKEN_SECRET", "mini-drop-demo-secret"),
		AuthSessionTTL:         durationFromEnv("MINIDROP_AUTH_SESSION_TTL_SEC", 8*60*60),
		AgentAllowlist:         splitCSV(getenv("MINIDROP_AGENT_ALLOWLIST", "")),
		AIEnabled:              boolFromEnv("MINIDROP_AI_ENABLED", os.Getenv("MINIDROP_AI_API_KEY") != ""),
		AIBaseURL:              getenv("MINIDROP_AI_BASE_URL", "https://api.openai.com/v1"),
		AIAPIKey:               getenv("MINIDROP_AI_API_KEY", ""),
		AIModel:                getenv("MINIDROP_AI_MODEL", "gpt-4o-mini"),
		AITimeout:              durationFromEnv("MINIDROP_AI_TIMEOUT_SEC", 20),
		AIMaxTokens:            intFromEnv("MINIDROP_AI_MAX_TOKENS", 800),
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
	if cfg.AuthUsername == "" {
		cfg.AuthUsername = "demo"
	}
	if cfg.AuthPassword == "" {
		cfg.AuthPassword = "minidrop"
	}
	if cfg.AuthTokenSecret == "" {
		cfg.AuthTokenSecret = "mini-drop-demo-secret"
	}
	if cfg.AuthSessionTTL <= 0 {
		cfg.AuthSessionTTL = 8 * time.Hour
	}
	if cfg.AIBaseURL == "" {
		cfg.AIBaseURL = "https://api.openai.com/v1"
	}
	if cfg.AIModel == "" {
		cfg.AIModel = "gpt-4o-mini"
	}
	if cfg.AITimeout <= 0 {
		cfg.AITimeout = 20 * time.Second
	}
	if cfg.AIMaxTokens <= 0 {
		cfg.AIMaxTokens = 800
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
	return time.Duration(intFromEnv(key, fallbackSeconds)) * time.Second
}

func intFromEnv(key string, fallback int) int {
	value := getenv(key, "")
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}

	return parsed
}
