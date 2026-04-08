SHELL := /usr/bin/env bash

SQLC_VERSION ?= 1.30.0
REDOCLY_CLI_VERSION ?= 2.25.3

.PHONY: sqlc-generate sqlc-check openapi-check test build lint vet vuln docker-build up down migrate

sqlc-generate:
	SQLC_VERSION=$(SQLC_VERSION) bash scripts/sqlc-generate.sh

sqlc-check:
	SQLC_VERSION=$(SQLC_VERSION) bash scripts/sqlc-generate.sh
	git diff --exit-code -- internal/shared/db/sqlc

openapi-check:
	REDOCLY_CLI_VERSION=$(REDOCLY_CLI_VERSION) bash scripts/openapi-validate.sh

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
