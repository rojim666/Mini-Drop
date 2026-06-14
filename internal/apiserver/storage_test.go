package apiserver

import "testing"

func TestLoadConfigFromEnvIncludesMinIODefaults(t *testing.T) {
	t.Setenv("MINIDROP_STORAGE_BACKEND", "")
	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.StorageBackend != "local" {
		t.Fatalf("expected local storage backend by default, got %q", cfg.StorageBackend)
	}
	if cfg.MinIOBucket == "" || cfg.MinIOAccessKey == "" || cfg.MinIOSecretKey == "" {
		t.Fatalf("expected minio defaults to be populated: %+v", cfg)
	}
}

func TestNormalizePublicEndpoint(t *testing.T) {
	base, err := normalizePublicEndpoint("localhost:9000", false)
	if err != nil {
		t.Fatalf("normalize endpoint: %v", err)
	}
	if base.Scheme != "http" || base.Host != "localhost:9000" {
		t.Fatalf("unexpected base URL: %s", base.String())
	}
}

func TestCleanArtifactRelPath(t *testing.T) {
	got := cleanArtifactRelPath("../tsk/analysis/flamegraph.svg")
	if want := "tsk/analysis/flamegraph.svg"; got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}
