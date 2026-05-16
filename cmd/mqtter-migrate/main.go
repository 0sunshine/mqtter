package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"mqtter/internal/config"
	"mqtter/internal/storage/postgres"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := config.LoadFromEnv()
	store, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect database", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := store.ApplyMigrations(ctx); err != nil {
		logger.Error("failed to apply migrations", "error", err)
		os.Exit(1)
	}
	if err := store.EnsureMessagePartition(ctx, time.Now().UTC()); err != nil {
		logger.Error("failed to ensure current message partition", "error", err)
		os.Exit(1)
	}
	logger.Info("database migrations applied")
}
