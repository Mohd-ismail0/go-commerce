SHELL := /bin/sh

.PHONY: sqlc-generate sqlc-check openapi-check test build lint vet vuln docker-build up down migrate

sqlc-generate:
	sh scripts/sqlc-generate.sh

sqlc-check:
	sh scripts/sqlc-generate.sh
	git diff --exit-code -- internal/shared/db/sqlc

openapi-check:
	sh scripts/openapi-validate.sh

test:
	go test ./...

build:
	go build ./...

lint:
	golangci-lint run

vet:
	go vet ./...

vuln:
	govulncheck ./...

docker-build:
	docker build -t rewrite:dev .

up:
	docker compose up -d --build

down:
	docker compose down

migrate:
	psql "$$DATABASE_URL" -f internal/shared/db/schema.sql
