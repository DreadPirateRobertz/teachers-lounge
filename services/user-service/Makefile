.PHONY: build test lint migrate-up migrate-down migrate-version

MIGRATIONS_PATH ?= ../../infra/db/migrations

build:
	go build ./...

test:
	go test ./... -race -count=1

lint:
	golangci-lint run ./...

# Run migrations against DATABASE_URL
migrate-up:
	DATABASE_URL=$(DATABASE_URL) MIGRATIONS_PATH=$(MIGRATIONS_PATH) \
		go run ./cmd/migrate up

migrate-down:
	DATABASE_URL=$(DATABASE_URL) MIGRATIONS_PATH=$(MIGRATIONS_PATH) \
		go run ./cmd/migrate down

migrate-version:
	DATABASE_URL=$(DATABASE_URL) MIGRATIONS_PATH=$(MIGRATIONS_PATH) \
		go run ./cmd/migrate version

# Build the migrate binary for use in CI/Docker
build-migrate:
	CGO_ENABLED=0 GOOS=linux go build -o /migrate-bin ./cmd/migrate
