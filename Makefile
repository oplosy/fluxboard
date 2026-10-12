# Fluxboard developer tasks (docs/10-INFRA-DEVOPS.md §3). Run from repo root.
# Requires: docker + docker compose. Go tooling is invoked via containers where
# possible so a bare checkout needs no local installs beyond Docker + make.

SHELL := /bin/sh
COMPOSE := docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.override.yml
NETWORK := fluxboard_default
MIGRATIONS := $(CURDIR)/backend/migrations

# Migration DSN targets the owner role on the compose network (see .env).
DATABASE_URL_MIGRATE ?= postgres://fluxboard_owner:owner_pw@postgres:5432/fluxboard?sslmode=disable

.DEFAULT_GOAL := help

.PHONY: help up down logs migrate migrate-down sqlc openapi gen-client seed stripe-seed \
        api worker web test test-integration lint audit

help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

up: ## Start the full local stack (builds + hot reload)
	$(COMPOSE) up --build -d

down: ## Stop the stack and remove volumes
	$(COMPOSE) down -v

logs: ## Tail all service logs
	$(COMPOSE) logs -f

migrate: ## Apply all migrations (owner role)
	docker run --rm --network $(NETWORK) -v "$(MIGRATIONS)":/migrations \
		migrate/migrate -path=/migrations -database "$(DATABASE_URL_MIGRATE)" up

migrate-down: ## Roll back the last migration
	docker run --rm --network $(NETWORK) -v "$(MIGRATIONS)":/migrations \
		migrate/migrate -path=/migrations -database "$(DATABASE_URL_MIGRATE)" down 1

sqlc: ## Regenerate query code from SQL (pinned: CI verifies with the same version)
	MSYS_NO_PATHCONV=1 docker run --rm -v "$(CURDIR)/backend":/src -w /src sqlc/sqlc:1.27.0 generate

openapi: ## Regenerate the OpenAPI 3.1 spec from swaggo annotations (needs swag v2)
	cd backend && swag init -g cmd/api/main.go -o docs --parseInternal --parseDepth 2 --v3.1 --outputTypes json
	cp backend/docs/swagger.json backend/internal/interface/http/handlers/openapi.json

gen-client: ## Generate the TS API client from the served openapi.json (Phase 7)
	@echo "gen-client: point openapi-typescript at http://localhost:8080/api/v1/openapi.json (Phase 7 frontend)."

# Go commands that need the compose DB run in a golang container on the compose
# network with the repo .env (DSNs use the `postgres` service name).
# MSYS_NO_PATHCONV stops Git Bash on Windows rewriting /src; no-op elsewhere.
GO_RUN = MSYS_NO_PATHCONV=1 docker run --rm --network $(NETWORK) --env-file .env \
	-v "$(CURDIR)/backend":/src -v fluxboard_gomod:/go/pkg/mod -w /src golang:1.26 go run

seed: stripe-seed ## Load the demo fixture (docs/12-TESTING.md §5); re-run resets it. SEED_ARGS="-clock YYYY-MM-DD"
	$(GO_RUN) ./cmd/seed $(SEED_ARGS)

stripe-seed: ## Upsert the plan catalog into plans (stub mode, no Stripe calls)
	$(GO_RUN) ./cmd/stripeseed

api: ## Run the API with hot reload (in-stack)
	$(COMPOSE) up api

worker: ## Run the worker (in-stack)
	$(COMPOSE) up worker

web: ## Run the Next.js dev server (http://localhost:3000)
	cd web && pnpm install && pnpm dev

test: ## Run unit tests
	cd backend && go test ./...

# Integration DSN targets the app role on the HOST-exposed postgres port
# (compose stack up + migrated). Override for CI.
TEST_DATABASE_URL ?= postgres://fluxboard_app:app_pw@localhost:5432/fluxboard?sslmode=disable
# Object-store suite targets the compose RustFS on its host port (ADR-027).
TEST_S3_ENDPOINT ?= localhost:9000
TEST_S3_ACCESS_KEY ?= minioadmin
TEST_S3_SECRET_KEY ?= minioadmin

test-integration: ## Run integration suites against the compose stack (make up + migrate first)
	cd backend && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" TEST_S3_ENDPOINT="$(TEST_S3_ENDPOINT)" \
		TEST_S3_ACCESS_KEY="$(TEST_S3_ACCESS_KEY)" TEST_S3_SECRET_KEY="$(TEST_S3_SECRET_KEY)" \
		go test -tags=integration ./...

lint: ## Run golangci-lint + go-arch-lint dependency check
	cd backend && golangci-lint run ./...
	cd backend && go-arch-lint check

audit: ## Vulnerability scan (Go + npm)
	cd backend && govulncheck ./...
