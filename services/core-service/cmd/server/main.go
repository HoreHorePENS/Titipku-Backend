package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/titipku/titipku-backend/internal/platform/config"
	"github.com/titipku/titipku-backend/internal/platform/database"
	"github.com/titipku/titipku-backend/internal/platform/httpserver"
	"github.com/titipku/titipku-backend/internal/platform/identity"
	"github.com/titipku/titipku-backend/internal/platform/logger"
	"github.com/titipku/titipku-backend/services/core-service/internal/adapter/httpapi"
	"github.com/titipku/titipku-backend/services/core-service/internal/adapter/repository/postgres"
	"github.com/titipku/titipku-backend/services/core-service/internal/migrations"
	"github.com/titipku/titipku-backend/services/core-service/internal/usecase"
)

func main() {
	if err := run(); err != nil {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadBase("core-service", 8080)
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

	store := postgres.NewStore(db.Pool)
	app := usecase.New(store, identity.NewPasswordHasher(), tokenManager, time.Now)
	handler := httpapi.NewHandler(app, config.String("PUBLIC_WEB_URL", "http://localhost:3000"))

	publicRateLimit, err := config.Int("PUBLIC_RATE_LIMIT_PER_MINUTE", 60)
	if err != nil {
		return err
	}
	if publicRateLimit < 0 || publicRateLimit > 10000 {
		return fmt.Errorf("PUBLIC_RATE_LIMIT_PER_MINUTE must be between 0 and 10000")
	}

	corsOrigins := strings.Split(config.String("CORS_ALLOWED_ORIGINS", "http://localhost:3000"), ",")
	router := httpapi.NewRouter(handler, log, tokenManager, db, httpapi.RouterOptions{
		AllowedOrigins:           corsOrigins,
		PublicRateLimitPerMinute: publicRateLimit,
	})

	server := httpserver.New(cfg.HTTPAddress, router, cfg.ShutdownTimeout)
	log.Info("http server starting", "address", cfg.HTTPAddress)

	return server.Run(ctx)
}
