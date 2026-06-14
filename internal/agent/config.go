package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	APIBaseURL        string
	AgentID           string
	Hostname          string
	IP                string
	Version           string
	PythonBin         string
	AnalyzerScript    string
	ArtifactDir       string
	HeartbeatInterval time.Duration
	PollInterval      time.Duration
	MockCollectDelay  time.Duration
}

func LoadConfigFromEnv() (Config, error) {
	root, err := os.Getwd()
	if err != nil {
		return Config{}, fmt.Errorf("get working directory: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "mini-drop-agent"
	}

	return Config{
		APIBaseURL:        getenv("MINIDROP_API_BASE_URL", "http://127.0.0.1:8080"),
		AgentID:           getenv("MINIDROP_AGENT_ID", "agt_local"),
		Hostname:          getenv("MINIDROP_AGENT_HOSTNAME", hostname),
		IP:                getenv("MINIDROP_AGENT_IP", "127.0.0.1"),
		Version:           getenv("MINIDROP_AGENT_VERSION", "0.1.0"),
		PythonBin:         getenv("MINIDROP_PYTHON_BIN", "python"),
		AnalyzerScript:    getenv("MINIDROP_ANALYZER_SCRIPT", filepath.Join(root, "apps", "analyzer", "main.py")),
		ArtifactDir:       getenv("MINIDROP_ARTIFACT_DIR", filepath.Join(root, "artifacts")),
		HeartbeatInterval: durationFromEnv("MINIDROP_AGENT_HEARTBEAT_SEC", 5),
		PollInterval:      durationFromEnv("MINIDROP_AGENT_POLL_SEC", 2),
		MockCollectDelay:  durationFromEnv("MINIDROP_AGENT_COLLECT_SEC", 2),
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
