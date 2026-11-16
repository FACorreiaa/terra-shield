SQLC_VERSION := v1.31.1

# Local PostGIS. The official image is amd64-only, so Apple Silicon runs it
# under emulation; that matches what production runs.
PG_CONTAINER := shield-pg
PG_PORT      := 55440
export DATABASE_URL      ?= postgres://shield:shield@127.0.0.1:$(PG_PORT)/shield?sslmode=disable
export TEST_DATABASE_URL ?= postgres://shield:shield@127.0.0.1:$(PG_PORT)/postgres?sslmode=disable

.PHONY: db-up db-down run worker migrate migrate-status routes build test lint generate sqlc-check docker

db-up: ## start local PostGIS on $(PG_PORT)
	@docker start $(PG_CONTAINER) 2>/dev/null || docker run -d --platform linux/amd64 --name $(PG_CONTAINER) \
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

migrate:
	go run ./cmd/shield migrate --yes

migrate-status:
	go run ./cmd/shield migrate status

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
