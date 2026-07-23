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

	"github.com/kyransciberras/bjj-streaming/internal/audit"
	"github.com/kyransciberras/bjj-streaming/internal/auth"
	"github.com/kyransciberras/bjj-streaming/internal/config"
	"github.com/kyransciberras/bjj-streaming/internal/courses"
	"github.com/kyransciberras/bjj-streaming/internal/database"
	"github.com/kyransciberras/bjj-streaming/internal/httpserver"
	"github.com/kyransciberras/bjj-streaming/internal/learning"
	"github.com/kyransciberras/bjj-streaming/internal/libraries"
	"github.com/kyransciberras/bjj-streaming/internal/logging"
	"github.com/kyransciberras/bjj-streaming/internal/objectstorage"
	"github.com/kyransciberras/bjj-streaming/internal/users"
	"github.com/kyransciberras/bjj-streaming/internal/videos"
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
		CookieSecure:   cfg.AuthCookieSecure,
		SessionIdleTTL: cfg.SessionIdleTTL, SessionAbsoluteTTL: cfg.SessionAbsoluteTTL,
	}, cfg.LoginRateLimit)
	if err != nil {
		return err
	}
	userStore := users.NewStore(db)
	userHandler := users.NewHandler(userStore, authHandler)
	libraryHandler := libraries.NewHandler(libraries.NewStore(db), userStore, authHandler)
	auditHandler := audit.NewHandler(audit.NewStore(db), authHandler)
	objects, err := objectstorage.New(context.Background(), cfg.ObjectEndpoint, cfg.ObjectPublicEndpoint, cfg.ObjectRegion, cfg.ObjectBucket, cfg.ObjectAccessKey, cfg.ObjectSecretKey, cfg.ObjectPathStyle, cfg.UploadURLTTL)
	if err != nil {
		return err
	}
	videoStore := videos.NewStore(db)
	videoHandler := videos.NewHandler(videoStore, objects, authHandler)
	learningHandler := learning.NewHandler(learning.NewStore(db), videoStore, objects, authHandler)
	courseHandler := courses.NewHandler(courses.NewStore(db), videoStore, authHandler)

	server := &http.Server{
		Addr: cfg.HTTPAddr, Handler: httpserver.New(logger, db, authHandler, userHandler, libraryHandler, auditHandler, videoHandler, learningHandler, courseHandler),
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
