SERVICE ?= auth-service
BIN_DIR ?= bin

.PHONY: help tidy fmt vet test test-cover build run lint

help:
	@echo "Usage:"
	@echo "  make tidy                  Download and tidy dependencies"
	@echo "  make fmt                   Format all Go packages"
	@echo "  make vet                   Run go vet"
	@echo "  make test                  Run all tests"
	@echo "  make test-cover            Run tests with coverage"
	@echo "  make build                 Build all service binaries into $(BIN_DIR)"
	@echo "  make run SERVICE=<name>    Run a service locally"
	@echo "  make lint                  Run golangci-lint"

tidy:
	go mod tidy

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

test-cover:
	go test -cover ./...

build:
	go build -trimpath -o $(BIN_DIR)/auth-service ./services/auth-service/cmd/server
	go build -trimpath -o $(BIN_DIR)/user-service ./services/user-service/cmd/server
	go build -trimpath -o $(BIN_DIR)/order-service ./services/order-service/cmd/server
	go build -trimpath -o $(BIN_DIR)/core-service ./services/core-service/cmd/server

run:
	go run ./services/$(SERVICE)/cmd/server

lint:
	golangci-lint run ./...

