# Variables
APP_NAME=boilerplate-api
GO=go
GOFLAGS=-v
BUILD_DIR=build
DOCKER_IMAGE=boilerplate-api:latest

# Colors
GREEN=\033[0;32m
RED=\033[0;31m
NC=\033[0m

.PHONY: all build clean test coverage run dev migrate proto swagger docker help

## help: Display this help message
help:
	@echo "Available commands:"
	@awk 'BEGIN {FS = ":.*##"; printf "\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  ${GREEN}%-15s${NC} %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

## all: Build the application
all: clean build

## build: Build the application
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(APP_NAME) ./api/main.go
	@$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-grpc ./grpc/main.go
	@echo "${GREEN}Build complete!${NC}"

## build-prod: Build for production
build-prod:
	@echo "Building $(APP_NAME) for production..."
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 GOOS=linux $(GO) build -ldflags="-w -s" -o $(BUILD_DIR)/$(APP_NAME) ./api/main.go
	@CGO_ENABLED=0 GOOS=linux $(GO) build -ldflags="-w -s" -o $(BUILD_DIR)/$(APP_NAME)-grpc ./grpc/main.go
	@echo "${GREEN}Production build complete!${NC}"

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -rf ./docs
	@echo "${GREEN}Clean complete!${NC}"

## test: Run tests
test:
	@echo "Running tests..."
	@$(GO) test -v ./...
	@echo "${GREEN}Tests complete!${NC}"

## test-coverage: Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@$(GO) test -v -coverprofile=coverage.out ./...
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "${GREEN}Coverage report generated: coverage.html${NC}"

## run: Run the application
run:
	@echo "Starting $(APP_NAME)..."
	@$(GO) run ./api/main.go

## run-grpc: Run gRPC server
run-grpc:
	@echo "Starting gRPC server..."
	@$(GO) run ./grpc/main.go

## run-all: Run both REST and gRPC servers
run-all:
	@echo "Starting all servers..."
	@$(GO) run ./main.go

## dev: Run with hot reload
dev:
	@echo "Starting $(APP_NAME) in development mode..."
	@air -c .air.toml

## migrate: Run database migrations
migrate:
	@echo "Running migrations..."
	@$(GO) run ./cmd/migrate/main.go up

## migrate-down: Rollback database migrations
migrate-down:
	@echo "Rolling back migrations..."
	@$(GO) run ./cmd/migrate/main.go down

## migrate-create: Create a new migration
migrate-create:
	@read -p "Enter migration name: " name; \
	$(GO) run ./cmd/migrate/main.go create $$name

## proto: Generate gRPC code
proto:
	@echo "Generating gRPC code..."
	@protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		grpc/proto/*.proto
	@echo "${GREEN}gRPC code generation complete!${NC}"

## swagger: Generate Swagger documentation
swagger:
	@echo "Generating Swagger documentation..."
	@swag init -g api/main.go -o docs
	@echo "${GREEN}Swagger documentation generated!${NC}"

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	@$(GO) mod download
	@$(GO) mod tidy
	@echo "${GREEN}Dependencies downloaded!${NC}"

## docker: Build Docker image
docker:
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE) .
	@echo "${GREEN}Docker image built: $(DOCKER_IMAGE)${NC}"

## docker-run: Run with Docker Compose
docker-run:
	@echo "Starting with Docker Compose..."
	@docker-compose up -d
	@echo "${GREEN}Services started!${NC}"

## docker-stop: Stop Docker Compose services
docker-stop:
	@echo "Stopping Docker Compose services..."
	@docker-compose down
	@echo "${GREEN}Services stopped!${NC}"

## docker-logs: View Docker logs
docker-logs:
	@docker-compose logs -f

## lint: Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run ./...
	@echo "${GREEN}Linting complete!${NC}"

## fmt: Format code
fmt:
	@echo "Formatting code..."
	@$(GO) fmt ./...
	@echo "${GREEN}Code formatted!${NC}"

## vet: Run go vet
vet:
	@echo "Running go vet..."
	@$(GO) vet ./...
	@echo "${GREEN}Vet complete!${NC}"

## mod-update: Update dependencies
mod-update:
	@echo "Updating dependencies..."
	@$(GO) get -u ./...
	@$(GO) mod tidy
	@echo "${GREEN}Dependencies updated!${NC}"

## install-tools: Install development tools
install-tools:
	@echo "Installing development tools..."
	@$(GO) install github.com/cosmtrek/air@latest
	@$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@$(GO) install github.com/swaggo/swag/cmd/swag@latest
	@$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "${GREEN}Tools installed!${NC}"

## seed: Seed the database
seed:
	@echo "Seeding database..."
	@$(GO) run ./cmd/seed/main.go
	@echo "${GREEN}Database seeded!${NC}"

## benchmark: Run benchmarks
benchmark:
	@echo "Running benchmarks..."
	@$(GO) test -bench=. -benchmem ./...
	@echo "${GREEN}Benchmarks complete!${NC}"