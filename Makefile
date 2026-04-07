SHELL := /bin/sh

.PHONY: sqlc-generate sqlc-check test build

sqlc-generate:
	sh scripts/sqlc-generate.sh

sqlc-check:
	sh scripts/sqlc-generate.sh
	git diff --exit-code -- internal/shared/db/sqlc

test:
	go test ./...

build:
	go build ./...
