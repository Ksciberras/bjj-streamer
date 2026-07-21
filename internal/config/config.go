package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment         string
	HTTPAddr            string
	DatabaseURL         string
	DBConnectTimeout    time.Duration
	ShutdownTimeout     time.Duration
	LogLevel            slog.Level
	AuthCookieSecure    bool
	InvitationTTL       time.Duration
	SessionIdleTTL      time.Duration
	SessionAbsoluteTTL  time.Duration
	LoginRateLimit      int
	InvitationRateLimit int
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
	cfg.InvitationTTL = duration(lookup, "INVITATION_TTL", 24*time.Hour, &errs)
	cfg.SessionIdleTTL = duration(lookup, "SESSION_IDLE_TTL", 12*time.Hour, &errs)
	cfg.SessionAbsoluteTTL = duration(lookup, "SESSION_ABSOLUTE_TTL", 7*24*time.Hour, &errs)
	if cfg.SessionIdleTTL > cfg.SessionAbsoluteTTL {
		errs = append(errs, fmt.Errorf("SESSION_IDLE_TTL must not exceed SESSION_ABSOLUTE_TTL"))
	}
	cfg.AuthCookieSecure = boolean(lookup, "AUTH_COOKIE_SECURE", cfg.Environment == "production", &errs)
	if cfg.Environment == "production" && !cfg.AuthCookieSecure {
		errs = append(errs, fmt.Errorf("AUTH_COOKIE_SECURE must be true in production"))
	}
	cfg.LoginRateLimit = positiveInt(lookup, "LOGIN_RATE_LIMIT", 10, &errs)
	cfg.InvitationRateLimit = positiveInt(lookup, "INVITATION_RATE_LIMIT", 10, &errs)

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

func boolean(lookup func(string) (string, bool), key string, fallback bool, errs *[]error) bool {
	raw, ok := lookup(key)
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s must be true or false", key))
		return fallback
	}
	return parsed
}

func positiveInt(lookup func(string) (string, bool), key string, fallback int, errs *[]error) int {
	raw := value(lookup, key, strconv.Itoa(fallback))
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		*errs = append(*errs, fmt.Errorf("%s must be a positive integer", key))
		return fallback
	}
	return parsed
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
