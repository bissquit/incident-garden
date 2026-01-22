.PHONY: help dev test test-unit test-integration test-all lint migrate-up migrate-down migrate-create migrate-force build docker-build docker-up docker-down generate openapi-validate

help:
	@echo "Available commands:"
	@echo "  make help            - Show this help message"
	@echo "  make dev             - Run locally with hot-reload (air)"
	@echo "  make test            - Run all tests"
	@echo "  make test-unit       - Run only unit tests"
	@echo "  make test-integration- Run only integration tests"
	@echo "  make test-all        - Run unit and integration tests"
	@echo "  make lint            - Run linters"
	@echo "  make migrate-up      - Apply migrations"
	@echo "  make migrate-down    - Rollback last migration"
	@echo "  make migrate-create  - Create new migration (usage: make migrate-create NAME=migration_name)"
	@echo "  make migrate-force   - Force migration version (usage: make migrate-force VERSION=version_number)"
	@echo "  make build           - Build binary"
	@echo "  make docker-build    - Build Docker image"
	@echo "  make docker-up       - Start docker-compose"
	@echo "  make docker-down     - Stop docker-compose"
	@echo "  make generate        - Generate code (sqlc, mocks)"
	@echo "  make openapi-validate - Validate OpenAPI spec"

dev:
	@command -v air > /dev/null 2>&1 || { echo "air not installed. Run: go install github.com/air-verse/air@latest"; exit 1; }
	@test -f .env || { echo ".env file not found. Copy from .env.example first: cp .env.example .env"; exit 1; }
	@set -a; . ./.env; set +a; air

test:
	go test -v -race -coverprofile=coverage.out ./...

test-unit:
	go test -v -race ./internal/...

test-integration:
	go test -v -race -count=1 -tags=integration ./tests/integration/...

test-all: test-unit test-integration

lint:
	@command -v golangci-lint > /dev/null 2>&1 || { echo "golangci-lint not installed. See: https://golangci-lint.run/welcome/install/"; exit 1; }
	golangci-lint run

migrate-up:
	@command -v migrate > /dev/null 2>&1 || { echo "migrate not installed. Run: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"; exit 1; }
	@test -f .env && . .env || . .env.example; migrate -path migrations -database "$$DATABASE_URL" up

migrate-down:
	@command -v migrate > /dev/null 2>&1 || { echo "migrate not installed. Run: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"; exit 1; }
	@test -f .env && . .env || . .env.example; migrate -path migrations -database "$$DATABASE_URL" down 1

migrate-create:
	@if [ -z "$(NAME)" ]; then echo "Usage: make migrate-create NAME=migration_name"; exit 1; fi
	@command -v migrate > /dev/null 2>&1 || { echo "migrate not installed. Run: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"; exit 1; }
	migrate create -ext sql -dir migrations -seq $(NAME)

migrate-force:
	@if [ -z "$(VERSION)" ]; then echo "Usage: make migrate-force VERSION=version_number"; exit 1; fi
	@command -v migrate > /dev/null 2>&1 || { echo "migrate not installed. Run: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"; exit 1; }
	@test -f .env && . .env || . .env.example; migrate -path migrations -database "$$DATABASE_URL" force $(VERSION)

build:
	CGO_ENABLED=0 go build -o bin/statuspage ./cmd/statuspage

docker-build:
	docker build -t statuspage:latest -f deployments/docker/Dockerfile .

docker-up:
	docker-compose -f deployments/docker/docker-compose.yml up -d

docker-down:
	docker-compose -f deployments/docker/docker-compose.yml down

generate:
	@echo "Code generation not yet configured"

openapi-validate:
	@if command -v swagger-cli > /dev/null 2>&1; then \
		swagger-cli validate api/openapi/openapi.yaml; \
	elif command -v oapi-codegen > /dev/null 2>&1; then \
		oapi-codegen -generate types api/openapi/openapi.yaml > /dev/null; \
	else \
		echo "No OpenAPI validator found. Install one of:"; \
		echo "  npm install -g @apidevtools/swagger-cli"; \
		echo "  go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest"; \
		exit 1; \
	fi
