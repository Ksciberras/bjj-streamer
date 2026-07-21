package logging

import (
	"io"
	"log/slog"
)

func New(output io.Writer, level slog.Level, service, environment string) *slog.Logger {
	return slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{Level: level})).With("service", service, "environment", environment)
}
