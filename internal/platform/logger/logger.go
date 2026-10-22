package logger

import (
	"log/slog"
	"os"
)

func New(serviceName, environment string) *slog.Logger {
	level := slog.LevelInfo
	if environment == "development" {
		level = slog.LevelDebug
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})

	return slog.New(handler).With(
		slog.String("service", serviceName),
		slog.String("env", environment),
	)
}
