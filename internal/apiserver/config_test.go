package apiserver

import (
	"strings"
	"testing"
)

func TestConfigDefaultsToSQLite(t *testing.T) {
	cfg := Config{DBPath: "mini-drop.db"}.withDefaults()
	if cfg.DBDriver != "sqlite" {
		t.Fatalf("expected sqlite default, got %q", cfg.DBDriver)
	}

	dialector, err := cfg.gormDialector()
	if err != nil {
		t.Fatalf("expected sqlite dialector: %v", err)
	}
	if dialector.Name() != "sqlite" {
		t.Fatalf("expected sqlite dialector, got %q", dialector.Name())
	}
}

func TestPostgresConfigRequiresDSN(t *testing.T) {
	_, err := Config{DBDriver: "postgres"}.gormDialector()
	if err == nil {
		t.Fatal("expected missing postgres DSN error")
	}
	if !strings.Contains(err.Error(), "MINIDROP_POSTGRES_DSN") {
		t.Fatalf("expected postgres DSN error, got %q", err.Error())
	}
}

func TestPostgresConfigAcceptsPostgresqlAlias(t *testing.T) {
	dialector, err := Config{
		DBDriver:    "postgresql",
		PostgresDSN: "host=localhost user=postgres password=dev dbname=mini_drop sslmode=disable",
	}.gormDialector()
	if err != nil {
		t.Fatalf("expected postgres dialector: %v", err)
	}
	if dialector.Name() != "postgres" {
		t.Fatalf("expected postgres dialector, got %q", dialector.Name())
	}
}

func TestConfigRejectsUnsupportedDBDriver(t *testing.T) {
	_, err := Config{DBDriver: "mysql"}.gormDialector()
	if err == nil {
		t.Fatal("expected unsupported driver error")
	}
	if !strings.Contains(err.Error(), "unsupported MINIDROP_DB_DRIVER") {
		t.Fatalf("expected unsupported driver error, got %q", err.Error())
	}
}
