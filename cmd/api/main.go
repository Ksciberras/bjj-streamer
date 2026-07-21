package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kyransciberras/bjj-streaming/internal/auth"
	"github.com/kyransciberras/bjj-streaming/internal/config"
	"github.com/kyransciberras/bjj-streaming/internal/database"
	"github.com/kyransciberras/bjj-streaming/internal/httpserver"
	"github.com/kyransciberras/bjj-streaming/internal/logging"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := logging.New(os.Stdout, cfg.LogLevel, "api", cfg.Environment)
	slog.SetDefault(logger)

	connectCtx, cancelConnect := context.WithTimeout(context.Background(), cfg.DBConnectTimeout)
	defer cancelConnect()
	db, err := database.Open(connectCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	authHandler, err := auth.NewHandler(auth.NewStore(db), auth.Settings{
		CookieSecure: cfg.AuthCookieSecure, InvitationTTL: cfg.InvitationTTL,
		SessionIdleTTL: cfg.SessionIdleTTL, SessionAbsoluteTTL: cfg.SessionAbsoluteTTL,
	}, cfg.LoginRateLimit, cfg.InvitationRateLimit)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr: cfg.HTTPAddr, Handler: httpserver.New(logger, db, authHandler),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("api listening", "address", cfg.HTTPAddr)
		serverErrors <- server.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		logger.Info("api shutting down")
		return server.Shutdown(shutdownCtx)
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
