package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type JWT struct {
	Secret string
	Issuer string
	TTL    time.Duration
}

type Base struct {
	ServiceName     string
	Environment     string
	HTTPAddress     string
	ShutdownTimeout time.Duration
	DatabaseURL     string
	JWT             JWT
}

func LoadBase(serviceName string, defaultPort int) (Base, error) {
	if err := loadDotEnv(serviceName); err != nil {
		return Base{}, err
	}

	port, err := Int("HTTP_PORT", defaultPort)
	if err != nil {
		return Base{}, err
	}
	if port < 1 || port > 65535 {
		return Base{}, fmt.Errorf("HTTP_PORT must be between 1 and 65535")
	}

	databaseURL, err := Required("DATABASE_URL")
	if err != nil {
		return Base{}, err
	}

	jwtSecret, err := Required("JWT_SECRET")
	if err != nil {
		return Base{}, err
	}
	if len(jwtSecret) < 32 {
		return Base{}, fmt.Errorf("JWT_SECRET must contain at least 32 characters")
	}

	jwtTTL, err := Duration("JWT_TTL", 24*time.Hour)
	if err != nil {
		return Base{}, err
	}
	if jwtTTL <= 0 {
		return Base{}, fmt.Errorf("JWT_TTL must be positive")
	}

	shutdownTimeout, err := Duration("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Base{}, err
	}

	return Base{
		ServiceName:     serviceName,
		Environment:     String("APP_ENV", "development"),
		HTTPAddress:     ":" + strconv.Itoa(port),
		ShutdownTimeout: shutdownTimeout,
		DatabaseURL:     databaseURL,
		JWT: JWT{
			Secret: jwtSecret,
			Issuer: String("JWT_ISSUER", "titipku"),
			TTL:    jwtTTL,
		},
	}, nil
}

func String(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func Required(key string) (string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func Int(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return parsed, nil
}

func Duration(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", key, err)
	}
	return parsed, nil
}

func loadDotEnv(serviceName string) error {
	candidates := []string{
		".env",
		filepath.Join("services", serviceName, ".env"),
	}

	for _, candidate := range candidates {
		err := godotenv.Load(candidate)
		if err == nil {
			return nil
		}
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		return fmt.Errorf("load %s: %w", candidate, err)
	}

	return nil
}
