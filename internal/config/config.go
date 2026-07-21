package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"
)

type Config struct {
	Environment      string
	HTTPAddr         string
	DatabaseURL      string
	DBConnectTimeout time.Duration
	ShutdownTimeout  time.Duration
	LogLevel         slog.Level
}

func Load() (Config, error) {
	return load(os.LookupEnv)
}

func load(lookup func(string) (string, bool)) (Config, error) {
	var cfg Config
	var errs []error

	cfg.Environment = value(lookup, "APP_ENV", "development")
	if cfg.Environment != "development" && cfg.Environment != "test" && cfg.Environment != "production" {
		errs = append(errs, fmt.Errorf("APP_ENV must be development, test, or production"))
	}
	cfg.HTTPAddr = value(lookup, "HTTP_ADDR", ":8080")
	if !strings.Contains(cfg.HTTPAddr, ":") {
		errs = append(errs, fmt.Errorf("HTTP_ADDR must include a port"))
	}
	cfg.DatabaseURL = value(lookup, "DATABASE_URL", "")
	if cfg.DatabaseURL == "" {
		errs = append(errs, fmt.Errorf("DATABASE_URL is required"))
	} else if parsed, err := url.Parse(cfg.DatabaseURL); err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Host == "" || parsed.Path == "" {
		errs = append(errs, fmt.Errorf("DATABASE_URL must be a valid postgres URL"))
	}
	cfg.DBConnectTimeout = duration(lookup, "DB_CONNECT_TIMEOUT", 5*time.Second, &errs)
	cfg.ShutdownTimeout = duration(lookup, "SHUTDOWN_TIMEOUT", 10*time.Second, &errs)

	switch strings.ToLower(value(lookup, "LOG_LEVEL", "info")) {
	case "debug":
		cfg.LogLevel = slog.LevelDebug
	case "info":
		cfg.LogLevel = slog.LevelInfo
	case "warn":
		cfg.LogLevel = slog.LevelWarn
	case "error":
		cfg.LogLevel = slog.LevelError
	default:
		errs = append(errs, fmt.Errorf("LOG_LEVEL must be debug, info, warn, or error"))
	}

	return cfg, errors.Join(errs...)
}

func value(lookup func(string) (string, bool), key, fallback string) string {
	if v, ok := lookup(key); ok {
		return strings.TrimSpace(v)
	}
	return fallback
}

func duration(lookup func(string) (string, bool), key string, fallback time.Duration, errs *[]error) time.Duration {
	raw := value(lookup, key, fallback.String())
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		*errs = append(*errs, fmt.Errorf("%s must be a positive duration", key))
		return fallback
	}
	return parsed
}
