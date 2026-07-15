COMPOSE ?= docker compose

.DEFAULT_GOAL := help

.PHONY: help setup api web build up down logs restart ps check clean purge-data

help:
	@printf '%s\n' \
		'setup       Create .env and generate its encryption key when missing' \
		'api         Run the Go API locally on port 8080' \
		'web         Run the Vite website locally on port 5173' \
		'build       Build the Docker images' \
		'up          Build, start, and health-check DataDock' \
		'down        Stop DataDock and preserve metadata' \
		'logs        Follow API and web logs' \
		'restart     Rebuild and restart DataDock' \
		'ps          Show container and health status' \
		'check       Run service tests, builds, and Compose validation' \
		'clean       Stop containers and preserve metadata' \
		'purge-data  Permanently delete metadata with CONFIRM=delete'

setup:
	@umask 077; test -f .env || cp .env.example .env
	@key="$$(sed -n 's/^DATADOCK_ENCRYPTION_KEY=//p' .env | tail -n 1 | tr -d '[:space:]')"; \
	if [ -z "$$key" ]; then \
		if command -v openssl >/dev/null 2>&1; then \
			key="$$(openssl rand -base64 32 | tr -d '\r\n')"; \
		else \
			key="$$(dd if=/dev/urandom bs=32 count=1 2>/dev/null | base64 | tr -d '\r\n')"; \
		fi; \
		test -n "$$key"; \
		temporary="$$(mktemp .env.tmp.XXXXXX)"; \
		awk -v key="$$key" 'BEGIN { written = 0 } /^DATADOCK_ENCRYPTION_KEY=/ { if (!written) { print "DATADOCK_ENCRYPTION_KEY=" key; written = 1 }; next } { print } END { if (!written) print "DATADOCK_ENCRYPTION_KEY=" key }' .env > "$$temporary"; \
		chmod 600 "$$temporary"; \
		mv "$$temporary" .env; \
		printf '%s\n' 'Generated DATADOCK_ENCRYPTION_KEY in .env'; \
	fi
	@chmod 600 .env

api: setup
	@set -a; . ./.env; set +a; cd api && go run ./cmd/server

web:
	@cd website && VITE_DATA_SOURCE="$${VITE_DATA_SOURCE:-api}" VITE_API_BASE_URL="$${VITE_API_BASE_URL:-http://localhost:8080}" npm run dev -- --host 0.0.0.0

build: setup
	@$(COMPOSE) build

up: setup
	@$(COMPOSE) up --build --detach --wait --wait-timeout 180
	@port="$$(sed -n 's/^DATADOCK_WEB_PORT=//p' .env | tail -n 1)"; printf 'DataDock is ready at http://localhost:%s\n' "$${port:-8088}"

down: setup
	@$(COMPOSE) down --remove-orphans

logs: setup
	@$(COMPOSE) logs --follow --tail=200

restart: setup
	@$(COMPOSE) up --build --detach --wait --wait-timeout 180 --force-recreate

ps: setup
	@$(COMPOSE) ps

check: setup
	@$(COMPOSE) config --quiet
	@cd api && go test ./internal/core/service/... -count=1
	@cd api && go vet ./...
	@cd api && go build ./...
	@cd website && npm ci --no-audit --no-fund
	@cd website && npm run typecheck
	@cd website && npm run build

clean: down
	@printf '%s\n' 'Containers stopped. The datadock-data volume was preserved.'

purge-data: setup
	@test "$(CONFIRM)" = "delete" || (printf '%s\n' 'Refusing to delete metadata. Run make purge-data CONFIRM=delete to continue.'; exit 1)
	@$(COMPOSE) down --volumes --remove-orphans
