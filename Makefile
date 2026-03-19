# Uptime Monitor Makefile

.PHONY: help build run test clean docker-build docker-run migrate-up migrate-down dev-setup build-css

# Default target
help:
	@echo "Available targets:"
	@echo "  build-css    - Build CSS with Tailwind"
	@echo "  run          - Run the application"
	@echo "  test         - Run all tests"
	@echo "  clean        - Clean build artifacts"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run with Docker Compose"
	@echo "  migrate-up   - Run database migrations"
	@echo "  dev-setup    - Set up development environment"
	@echo "  verify       - Verify deployment setup"
	@echo "  templ        - Generate templ templates"

# Build the application
build: templ build-css
	CGO_ENABLED=0 go build -ldflags='-w -s' -o main ./cmd/main.go

# Build CSS with Tailwind
build-css:
	npm run build:css

# Run the application
run: build
	./main

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f main
	rm -f scripts/migrate
	go clean

# Generate templ templates
templ:
	@if command -v templ >/dev/null 2>&1; then \
		templ generate; \
	else \
		echo "Installing templ..."; \
		go install github.com/a-h/templ/cmd/templ@latest; \
		templ generate; \
	fi

# Build Docker image
docker-build:
	docker build -t uptime-monitor .

# Run with Docker Compose (development)
docker-run:
	docker compose up -d

# Run with Docker Compose (production)
docker-run-prod:
	docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d

# Stop Docker Compose
docker-stop:
	docker compose down

# Build migration tool
migrate-tool:
	go build -o scripts/migrate ./scripts/migrate.go

# Run database migrations (requires running database)
migrate-up: migrate-tool
	./scripts/migrate -up

# Run migrations with custom database connection
migrate-up-custom: migrate-tool
	./scripts/migrate -host $(DB_HOST) -port $(DB_PORT) -database $(DB_NAME) -user $(DB_USER) -password $(DB_PASSWORD) -up

# Set up development environment
dev-setup:
	@echo "Setting up development environment..."
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "Created .env file from template. Please customize it."; \
	fi
	@echo "Installing dependencies..."
	go mod download
	@echo "Installing templ..."
	go install github.com/a-h/templ/cmd/templ@latest
	@echo "Generating templates..."
	templ generate
	@echo "Development environment ready!"

# Verify deployment setup
verify:
	@chmod +x scripts/verify-deployment.sh
	@./scripts/verify-deployment.sh

# Database operations
db-start:
	docker compose up -d postgres redis

db-stop:
	docker compose stop postgres redis

db-reset: db-stop
	docker compose rm -f postgres redis
	docker volume rm uptime-monitor_postgres_data uptime-monitor_redis_data || true
	docker compose up -d postgres redis
	sleep 5
	$(MAKE) migrate-up

# Linting and formatting
fmt:
	go fmt ./...
	goimports -w .

lint:
	golangci-lint run

# Build for different platforms
build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags='-w -s' -o main-linux ./cmd/main.go

build-windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags='-w -s' -o main.exe ./cmd/main.go

build-mac:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags='-w -s' -o main-mac ./cmd/main.go
