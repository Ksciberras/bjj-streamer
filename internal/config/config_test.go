package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadValidConfiguration(t *testing.T) {
	values := map[string]string{
		"APP_ENV": "test", "HTTP_ADDR": "127.0.0.1:9000",
		"DATABASE_URL":       "postgres://user:pass@db:5432/app?sslmode=disable",
		"DB_CONNECT_TIMEOUT": "2s", "SHUTDOWN_TIMEOUT": "3s", "LOG_LEVEL": "debug",
	}
	cfg, err := load(mapLookup(values))
	if err != nil {
		t.Fatalf("load returned error: %v", err)
	}
	if cfg.Environment != "test" || cfg.DBConnectTimeout != 2*time.Second || cfg.ShutdownTimeout != 3*time.Second {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadReportsAllInvalidValues(t *testing.T) {
	values := map[string]string{
		"APP_ENV": "staging", "HTTP_ADDR": "8080", "DATABASE_URL": "mysql://db/app",
		"DB_CONNECT_TIMEOUT": "never", "SHUTDOWN_TIMEOUT": "0s", "LOG_LEVEL": "verbose",
	}
	_, err := load(mapLookup(values))
	if err == nil {
		t.Fatal("expected validation error")
	}
	for _, field := range []string{"APP_ENV", "HTTP_ADDR", "DATABASE_URL", "DB_CONNECT_TIMEOUT", "SHUTDOWN_TIMEOUT", "LOG_LEVEL"} {
		if !strings.Contains(err.Error(), field) {
			t.Errorf("error %q does not mention %s", err, field)
		}
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	_, err := load(mapLookup(map[string]string{}))
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected DATABASE_URL error, got %v", err)
	}
}

func TestProductionRequiresSecureAuthCookie(t *testing.T) {
	_, err := load(mapLookup(map[string]string{"APP_ENV": "production", "DATABASE_URL": "postgres://user:pass@db/app", "AUTH_COOKIE_SECURE": "false"}))
	if err == nil || !strings.Contains(err.Error(), "AUTH_COOKIE_SECURE") {
		t.Fatalf("expected secure cookie error, got %v", err)
	}
}

func TestSessionIdleCannotExceedAbsoluteLifetime(t *testing.T) {
	_, err := load(mapLookup(map[string]string{"DATABASE_URL": "postgres://user:pass@db/app", "SESSION_IDLE_TTL": "2h", "SESSION_ABSOLUTE_TTL": "1h"}))
	if err == nil || !strings.Contains(err.Error(), "SESSION_IDLE_TTL") {
		t.Fatalf("expected session lifetime error, got %v", err)
	}
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) { value, ok := values[key]; return value, ok }
}
