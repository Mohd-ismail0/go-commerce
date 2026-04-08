#!/usr/bin/env bash
set -euo pipefail

SQLC_VERSION="${SQLC_VERSION:-1.30.0}"

if command -v sqlc >/dev/null 2>&1; then
  sqlc generate
  exit 0
fi

docker run --rm \
  -v "$(pwd):/src" \
  -w /src \
  "sqlc/sqlc:${SQLC_VERSION}" \
  generate
