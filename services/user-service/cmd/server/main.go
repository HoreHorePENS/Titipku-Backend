package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/titipku/titipku-backend/internal/platform/config"
	"github.com/titipku/titipku-backend/internal/platform/database"
	"github.com/titipku/titipku-backend/internal/platform/httpserver"
	"github.com/titipku/titipku-backend/internal/platform/identity"
	"github.com/titipku/titipku-backend/internal/platform/logger"
	"github.com/titipku/titipku-backend/services/user-service/internal/adapter/httpapi"
	"github.com/titipku/titipku-backend/services/user-service/internal/adapter/repository/postgres"
	"github.com/titipku/titipku-backend/services/user-service/internal/migrations"
	"github.com/titipku/titipku-backend/services/user-service/internal/usecase"
)

func main() {
	if err := run(); err != nil {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadBase("user-service", 8082)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := logger.New(cfg.ServiceName, cfg.Environment)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer db.Close()

	if err := database.Migrate(ctx, db.Pool, migrations.FS, "."); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	log.Info("database migrations applied")

	tokenManager, err := identity.NewTokenManager(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.TTL)
	if err != nil {
		return fmt.Errorf("create token manager: %w", err)
	}

	profileRepository := postgres.NewProfileRepository(db.Pool)
	profiles := usecase.NewService(profileRepository, time.Now)
	handler := httpapi.NewHandler(profiles)
	router := httpapi.NewRouter(handler, log, tokenManager, db)

	server := httpserver.New(cfg.HTTPAddress, router, cfg.ShutdownTimeout)
	log.Info("http server starting", "address", cfg.HTTPAddress)

	return server.Run(ctx)
}
