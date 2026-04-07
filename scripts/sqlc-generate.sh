#!/usr/bin/env sh
set -eu

docker run --rm \
  -v "$(pwd):/src" \
  -w /src \
  sqlc/sqlc:1.27.0 \
  generate
