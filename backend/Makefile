SQLC_VERSION := v1.31.1

# Local PostGIS. The official image is amd64-only, so Apple Silicon runs it
# under emulation; that matches what production runs.
PG_CONTAINER := shield-pg
PG_PORT      := 55440
export DATABASE_URL      ?= postgres://shield:shield@127.0.0.1:$(PG_PORT)/shield?sslmode=disable
export TEST_DATABASE_URL ?= postgres://shield:shield@127.0.0.1:$(PG_PORT)/postgres?sslmode=disable

.PHONY: db-up db-down run worker budget migrate migrate-status seed-countries routes build test lint generate sqlc-check docker

db-up: ## start local PostGIS on $(PG_PORT)
	@docker start $(PG_CONTAINER) 2>/dev/null || docker run -d --restart unless-stopped --platform linux/amd64 --name $(PG_CONTAINER) \
		-e POSTGRES_USER=shield -e POSTGRES_PASSWORD=shield -e POSTGRES_DB=shield \
		-p $(PG_PORT):5432 postgis/postgis:18-3.6
	@until docker exec $(PG_CONTAINER) pg_isready -U shield -h 127.0.0.1 >/dev/null 2>&1; do sleep 1; done
	@echo "PostGIS ready on $(PG_PORT)"

db-down:
	docker rm -f $(PG_CONTAINER)

run:
	go run ./cmd/shield serve

worker:
	go run ./cmd/shield worker

# S2.7 budget (p95 <= 200 ms, <= 300 KB gzipped) against a running server
# with data: start it with `RATE_LIMIT_RPS=0 make run`, or point
# SHIELD_BUDGET_URL elsewhere.
budget: ## check the performance budget against a running server
	SHIELD_BUDGET_URL=$${SHIELD_BUDGET_URL:-http://localhost:8080} go test ./test/budget -count=1 -v

migrate:
	go run ./cmd/shield migrate --yes

migrate-status:
	go run ./cmd/shield migrate status

# Natural Earth borders. migrate seeds them on an empty table; this reloads.
# Uses the local copy in data/ when present (offline), else downloads; the
# SHA-256 is verified either way.
COUNTRIES_FILE := data/ne_50m_admin_0_countries.geojson
seed-countries:
	go run ./cmd/shield seed-countries $(if $(wildcard $(COUNTRIES_FILE)),--file $(COUNTRIES_FILE))

routes:
	go run ./cmd/shield routes

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/shield ./cmd/shield

test:
	go test ./... -count=1 -race

lint:
	golangci-lint run

generate: ## regenerate sqlc code from db/migrations and db/queries/<domain>/
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

sqlc-check: ## fail when committed sqlc output is stale
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) diff

docker:
	docker build -t shield:dev .
