package main

import (
	"context"
	"log/slog"
	"mesa-ads/internal/adapter/usecase"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fmt"

	"mesa-ads/internal/adapter/http"
	"mesa-ads/internal/adapter/postgres"
	"mesa-ads/internal/config"
	"mesa-ads/internal/db"
)

// main is the entry point of the mesa-ads usecase. It loads configuration,
// optionally runs database migrations, initializes the database pool and
// repositories, then starts the HTTP server. On receiving a termination
// signal it gracefully shuts down the server.
func main() {
	exitCode := 1
	defer func() {
		if r := recover(); r != nil {
			panic(r)
		} else {
			os.Exit(exitCode)
		}
	}()

	// Load configuration from environment variables.
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	var logger *slog.Logger
	{
		// Initialise structured logger based on configuration.
		var handler slog.Handler
		level := cfg.Log.SlogLevel()
		switch cfg.Log.SlogFormat() {
		case "json":
			handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
		default:
			handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
		}
		logger = slog.New(handler)
	}

	// Optionally run migrations if configured. We use the Psql sub‑config.
	if cfg.Psql.RunMigrations {
		if err = db.Migrate(cfg.Psql.Addr.String()); err != nil {
			logger.Error("migration error", slog.Any("error", err))
		} else {
			logger.Info("migrations applied successfully")
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := db.NewPostgresPool(ctx, cfg.Psql)
	if err != nil {
		logger.Error("database connection error", slog.Any("error", err))
		os.Exit(1)
	}
	defer pool.Close()

	repo := postgres.NewAdRepository(pool)
	svc := usecase.NewAdService(repo)

	handler := httpadapter.NewHandler(svc, logger)
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler: handler.Router(),
	}

	go func() {
		logger.Info("server listening", slog.Int("port", int(cfg.HTTP.Port)))
		if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.Any("error", err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	value := <-quit
	exitCode = 128 + int(value.(syscall.Signal))

	ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
	if err = srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", slog.Any("error", err))
	} else {
		logger.Info("server gracefully stopped")
	}
}
