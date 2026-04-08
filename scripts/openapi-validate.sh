#!/usr/bin/env bash
set -euo pipefail

REDOCLY_CLI_VERSION="${REDOCLY_CLI_VERSION:-2.25.3}"

if command -v redocly >/dev/null 2>&1; then
  redocly lint api/openapi.yaml
  exit 0
fi

docker run --rm -v "$(pwd):/work" -w /work "ghcr.io/redocly/cli:${REDOCLY_CLI_VERSION}" lint api/openapi.yaml
