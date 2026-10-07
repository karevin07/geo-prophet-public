#!make
.DEFAULT_GOAL := help
.PHONY: install format lint test build dev-web dev-web-api dev-api docker docker-down migrate import-sales import-rentals import-presales fixtures clean help

##@ Setup
install: ## install frontend and Go dependencies
	npm ci
	cd api && go mod download

format: ## format Go source
	cd api && go fmt ./...

lint: ## typecheck frontend and vet Go
	npm run lint
	cd api && go vet ./...

test: ## run frontend tests and Go race tests
	npm test
	cd api && go test -race ./...

build: ## build frontend and Go executables
	npm run build
	cd api && go build ./cmd/server ./cmd/importer

##@ Demo
dev-web: ## run frontend mock demo on loopback port 3100
	VITE_DATA_SOURCE=mock npm run dev -- --host 127.0.0.1 --port 3100 --strictPort

dev-web-api: ## run frontend with local Go API on loopback port 3100
	VITE_DATA_SOURCE=api npm run dev -- --host 127.0.0.1 --port 3100 --strictPort

dev-api: ## run Go API with synthetic data on loopback port 8080
	cd api && DATABASE_URL= API_ADDR=127.0.0.1:8080 go run ./cmd/server

fixtures: ## regenerate authored synthetic CSV fixtures
	node scripts/generate-fixtures.mjs

##@ Local database (requires .env; no automatic imports)
docker: ## build and start isolated local PostGIS demo
	docker compose up --build -d

docker-down: ## stop local stack without deleting data
	docker compose down

migrate: ## apply pending migrations to the local demo database
	docker compose run --rm migrate

CITY ?= a
import-sales: ## explicitly download sales after reviewing source terms; SEASON=115S2 CITY=a
	@test -n "$(SEASON)" || { echo "SEASON is required"; exit 1; }
	docker compose run --rm importer sales -season $(SEASON) -city $(CITY)

import-rentals: ## explicitly download rentals; SEASON=115S2 CITY=a
	@test -n "$(SEASON)" || { echo "SEASON is required"; exit 1; }
	docker compose run --rm importer rentals -season $(SEASON) -city $(CITY)

import-presales: ## explicitly download presales; SEASON=115S2 CITY=a
	@test -n "$(SEASON)" || { echo "SEASON is required"; exit 1; }
	docker compose run --rm importer presales -season $(SEASON) -city $(CITY)

##@ Cleanup
clean: ## remove generated frontend build output only
	npm run clean

##@ Help
help: ## display available commands
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make <target>\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-20s %s\n", $$1, $$2 } /^##@/ { printf "\n%s\n", substr($$0, 5) }' $(MAKEFILE_LIST)
