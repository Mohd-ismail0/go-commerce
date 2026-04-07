#!/usr/bin/env sh
set -eu

docker run --rm -v "$(pwd):/work" -w /work ghcr.io/redocly/cli:latest lint api/openapi.yaml
