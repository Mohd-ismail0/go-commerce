#!/usr/bin/env bash
set -euo pipefail

SQLC_VERSION="${SQLC_VERSION:-1.30.0}"

docker run --rm \
  -v "$(pwd):/src" \
  -w /src \
  "sqlc/sqlc:${SQLC_VERSION}" \
  generate
