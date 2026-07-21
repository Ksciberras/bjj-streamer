package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/kyransciberras/bjj-streaming/internal/config"
	"github.com/kyransciberras/bjj-streaming/internal/database"
	"github.com/kyransciberras/bjj-streaming/internal/logging"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	logger := logging.New(os.Stdout, cfg.LogLevel, "worker", cfg.Environment)
	slog.SetDefault(logger)
	connectCtx, cancelConnect := context.WithTimeout(context.Background(), cfg.DBConnectTimeout)
	db, err := database.Open(connectCtx, cfg.DatabaseURL)
	cancelConnect()
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger.Info("worker ready; no milestone 1 jobs are defined")
	<-ctx.Done()
	logger.Info("worker shutting down")
}
