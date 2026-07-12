.DEFAULT_GOAL := help

.PHONY: help setup api web build up down logs restart check clean

help:
	@printf '%s\n' 'setup      Create .env from .env.example' 'api        Run the Go API locally' 'web        Run the Vite website locally' 'build      Build Docker images' 'up         Start DataDock with Docker Compose' 'down       Stop DataDock containers' 'logs       Follow container logs' 'restart    Restart DataDock containers' 'check      Build API and website' 'clean      Stop containers and remove metadata volume'

setup:
	@test -f .env || cp .env.example .env

api:
	cd api && go run ./cmd/server

web:
	cd website && npm run dev -- --host 0.0.0.0

build:
	docker compose build

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

restart:
	docker compose up -d --build

check:
	cd api && go build ./...
	cd website && npm run typecheck && npm run build

clean:
	docker compose down --volumes --remove-orphans
