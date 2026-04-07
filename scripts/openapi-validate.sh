#!/usr/bin/env sh
set -eu

docker run --rm -v "$(pwd):/work" -w /work ghcr.io/redocly/cli:v1.24.0 lint api/openapi.yaml
