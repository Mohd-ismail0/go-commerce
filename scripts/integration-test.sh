#!/usr/bin/env bash
set -euo pipefail

PG_IMAGE="${PG_IMAGE:-postgres:16}"
PG_CONTAINER_NAME="${PG_CONTAINER_NAME:-pg-ci}"
PG_PORT="${PG_PORT:-55432}"
PG_USER="${PG_USER:-postgres}"
PG_PASSWORD="${PG_PASSWORD:-postgres}"
PG_DB="${PG_DB:-rewrite}"

docker run -d \
  --name "${PG_CONTAINER_NAME}" \
  -p "${PG_PORT}:5432" \
  -e "POSTGRES_USER=${PG_USER}" \
  -e "POSTGRES_PASSWORD=${PG_PASSWORD}" \
  -e "POSTGRES_DB=${PG_DB}" \
  "${PG_IMAGE}" >/dev/null

cleanup() {
  docker rm -f "${PG_CONTAINER_NAME}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

for i in $(seq 1 30); do
  if docker exec "${PG_CONTAINER_NAME}" pg_isready -U "${PG_USER}" -d "${PG_DB}" >/dev/null 2>&1; then
    break
  fi

  if [[ "${i}" -eq 30 ]]; then
    echo "postgres did not become ready in time"
    exit 1
  fi

  sleep 1
done

export DATABASE_URL="postgres://${PG_USER}:${PG_PASSWORD}@127.0.0.1:${PG_PORT}/${PG_DB}?sslmode=disable"
INTERNAL_DATABASE_URL="postgres://${PG_USER}:${PG_PASSWORD}@127.0.0.1:5432/${PG_DB}?sslmode=disable"
docker exec -i "${PG_CONTAINER_NAME}" psql "${INTERNAL_DATABASE_URL}" -v ON_ERROR_STOP=1 < internal/shared/db/schema.sql

export RUN_INTEGRATION=1
go test ./internal/integration/... -count=1
