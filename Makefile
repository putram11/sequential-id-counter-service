.PHONY: build test clean run docker proto migrate help

# Variables
BINARY_NAME=sequential-id-service
WORKER_BINARY=worker
BUILD_DIR=bin
PROTO_DIR=proto
PKG_DIR=pkg/proto
MIGRATION_DIR=migrations

# Build configuration
GO_VERSION=1.21
LDFLAGS=-ldflags="-w -s"

# Development database URL
DEV_DB_URL=postgres://sequser:seqpass@localhost:5432/seqdb?sslmode=disable

# Help target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build targets
build: ## Build the API service binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) cmd/api/main.go

build-worker: ## Build the worker binary
	@echo "Building $(WORKER_BINARY)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(WORKER_BINARY) cmd/worker/main.go

build-all: build build-worker ## Build all binaries

# Development targets
run: ## Run the API service locally
	@echo "Running $(BINARY_NAME)..."
	go run cmd/api/main.go

run-worker: ## Run the worker locally
	@echo "Running $(WORKER_BINARY)..."
	go run cmd/worker/main.go

dev: ## Start development environment with Docker Compose
	@echo "Starting development environment..."
	docker-compose --profile dev up -d

dev-down: ## Stop development environment
	@echo "Stopping development environment..."
	docker-compose --profile dev down

dev-logs: ## Show development environment logs
	docker-compose logs -f

# Testing targets
test: ## Run unit tests
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	go test -v -tags=integration ./tests/integration/...

test-load: ## Run load tests
	@echo "Running load tests..."
	go test -v -tags=load ./tests/load/...

coverage: test ## Generate test coverage report
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Code quality targets
lint: ## Run linter
	@echo "Running linter..."
	golangci-lint run

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

# Protocol Buffers targets
proto: ## Generate Go code from protobuf files
	@echo "Generating protobuf code..."
	@mkdir -p $(PKG_DIR)
	protoc --go_out=$(PKG_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(PKG_DIR) --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/*.proto

proto-clean: ## Clean generated protobuf files
	@echo "Cleaning protobuf generated files..."
	rm -rf $(PKG_DIR)/*.pb.go

# Database migration targets
migrate-up: ## Run database migrations up
	@echo "Running database migrations up..."
	migrate -path $(MIGRATION_DIR) -database "$(DEV_DB_URL)" up

migrate-down: ## Run database migrations down
	@echo "Running database migrations down..."
	migrate -path $(MIGRATION_DIR) -database "$(DEV_DB_URL)" down

migrate-reset: ## Reset database (down and up)
	@echo "Resetting database..."
	migrate -path $(MIGRATION_DIR) -database "$(DEV_DB_URL)" down -all
	migrate -path $(MIGRATION_DIR) -database "$(DEV_DB_URL)" up

migrate-create: ## Create new migration file (usage: make migrate-create NAME=migration_name)
	@echo "Creating migration file..."
	migrate create -ext sql -dir $(MIGRATION_DIR) $(NAME)

# Docker targets
docker-build: ## Build Docker image for API service
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):latest .

docker-build-worker: ## Build Docker image for worker
	@echo "Building Docker worker image..."
	docker build -f Dockerfile.worker -t $(WORKER_BINARY):latest .

docker-build-all: docker-build docker-build-worker ## Build all Docker images

docker-run: ## Run API service in Docker
	@echo "Running $(BINARY_NAME) in Docker..."
	docker run -p 8080:8080 -p 9090:9090 $(BINARY_NAME):latest

docker-push: ## Push Docker images to registry
	@echo "Pushing Docker images..."
	docker push $(BINARY_NAME):latest
	docker push $(WORKER_BINARY):latest

# Kubernetes targets
k8s-deploy: ## Deploy to Kubernetes
	@echo "Deploying to Kubernetes..."
	kubectl apply -f k8s/

k8s-delete: ## Delete from Kubernetes
	@echo "Deleting from Kubernetes..."
	kubectl delete -f k8s/

k8s-status: ## Check Kubernetes deployment status
	@echo "Checking Kubernetes status..."
	kubectl get pods -l app=$(BINARY_NAME)
	kubectl get services -l app=$(BINARY_NAME)

# Dependency management
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download

deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

deps-vendor: ## Vendor dependencies
	@echo "Vendoring dependencies..."
	go mod vendor

# Documentation targets
docs: ## Generate API documentation
	@echo "Generating API documentation..."
	swag init -g cmd/api/main.go -o docs/

docs-serve: ## Serve documentation locally
	@echo "Serving documentation on http://localhost:8080/swagger/"
	@echo "Make sure the service is running..."

# Security targets
security-scan: ## Run security scan
	@echo "Running security scan..."
	gosec ./...

vuln-check: ## Check for vulnerabilities
	@echo "Checking for vulnerabilities..."
	govulncheck ./...

# Performance targets
pprof-cpu: ## Run CPU profiling
	@echo "Running CPU profiling..."
	go test -cpuprofile=cpu.prof -bench=. ./...

pprof-mem: ## Run memory profiling
	@echo "Running memory profiling..."
	go test -memprofile=mem.prof -bench=. ./...

# Utility targets
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	rm -f *.prof
	docker system prune -f

tools: ## Install development tools
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install github.com/securecodewarrior/govulncheck@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

version: ## Show version information
	@echo "Go version: $(shell go version)"
	@echo "Build target: $(BINARY_NAME)"
	@echo "Git commit: $(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
	@echo "Build time: $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')"

# CI/CD targets
ci-test: deps lint test ## Run CI tests
	@echo "Running CI tests..."

ci-build: clean build-all docker-build-all ## Run CI build
	@echo "Running CI build..."

ci-deploy: ## Run CI deployment
	@echo "Running CI deployment..."
	@echo "This should be configured with your CI/CD pipeline"

# Monitoring targets
metrics: ## Show metrics endpoint
	@echo "Metrics available at: http://localhost:2112/metrics"
	@echo "Use 'curl http://localhost:2112/metrics' to view"

health: ## Check service health
	@echo "Checking service health..."
	@curl -f http://localhost:8081/health || echo "Service not healthy"

status: ## Show service status
	@echo "Checking service status..."
	@curl -f http://localhost:8080/api/v1/status/SG || echo "Service not responding"

# Default target
all: clean deps proto build-all test ## Run clean, deps, proto, build-all, test
