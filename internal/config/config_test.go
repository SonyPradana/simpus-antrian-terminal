package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func clearEnv(t *testing.T) {
	t.Helper()
	keys := []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "HTTP_ADDR", "API_TOKEN"}
	for _, key := range keys {
		old, existed := os.LookupEnv(key)
		_ = os.Unsetenv(key)
		t.Cleanup(func() {
			if existed {
				_ = os.Setenv(key, old)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
}

func TestMissingDatabaseSettingsRejected(t *testing.T) {
	clearEnv(t)
	t.Setenv("API_TOKEN", "secret")

	_, err := Load("")
	if err == nil || !strings.Contains(err.Error(), "DB_HOST") {
		t.Fatalf("expected DB_HOST error, got %v", err)
	}
}

func TestNonSensitiveDefaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("DB_HOST", "db.local")
	t.Setenv("DB_USER", "dbuser")
	t.Setenv("DB_PASSWORD", "dbpass")
	t.Setenv("DB_NAME", "simpus")
	t.Setenv("API_TOKEN", "secret")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DBPort != 3306 || cfg.HTTPAddr != ":8080" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.DBHost != "db.local" || cfg.DBPassword != "dbpass" {
		t.Fatalf("environment values were not applied: %+v", cfg)
	}
}

func TestEnvironmentOverridesFile(t *testing.T) {
	clearEnv(t)
	t.Setenv("DB_HOST", "envhost")
	t.Setenv("DB_PORT", "3307")
	t.Setenv("DB_USER", "envuser")
	t.Setenv("DB_PASSWORD", "envpass")
	t.Setenv("DB_NAME", "envdb")
	t.Setenv("HTTP_ADDR", ":9000")
	t.Setenv("API_TOKEN", "envtoken")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{"db_host":"filehost","db_port":3306,"db_user":"fileuser","db_password":"filepass","db_name":"filedb","http_addr":":8080","api_token":"filetoken"}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DBHost != "envhost" || cfg.DBPort != 3307 || cfg.APIToken != "envtoken" {
		t.Fatalf("environment did not override file: %+v", cfg)
	}
}

func TestFileConfigUsedWithoutEnvironment(t *testing.T) {
	clearEnv(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{"db_host":"filehost","db_port":3307,"db_user":"fileuser","db_password":"filepass","db_name":"filedb","http_addr":":9000","api_token":"filetoken"}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DBHost != "filehost" || cfg.DBPort != 3307 || cfg.APIToken != "filetoken" {
		t.Fatalf("file config was not used: %+v", cfg)
	}
}

func TestInvalidPortRejected(t *testing.T) {
	clearEnv(t)
	t.Setenv("DB_HOST", "db.local")
	t.Setenv("DB_USER", "dbuser")
	t.Setenv("DB_PASSWORD", "dbpass")
	t.Setenv("DB_NAME", "simpus")
	t.Setenv("DB_PORT", "not-a-number")
	t.Setenv("API_TOKEN", "secret")

	_, err := Load("")
	if err == nil || !strings.Contains(err.Error(), "DB_PORT") {
		t.Fatalf("expected DB_PORT error, got %v", err)
	}
}

func TestTokenRequired(t *testing.T) {
	clearEnv(t)
	t.Setenv("DB_HOST", "db.local")
	t.Setenv("DB_USER", "dbuser")
	t.Setenv("DB_PASSWORD", "dbpass")
	t.Setenv("DB_NAME", "simpus")

	_, err := Load("")
	if err == nil || !strings.Contains(err.Error(), "API_TOKEN") {
		t.Fatalf("expected API_TOKEN error, got %v", err)
	}
}
