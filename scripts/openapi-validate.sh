#!/usr/bin/env bash
set -euo pipefail

REDOCLY_CLI_VERSION="${REDOCLY_CLI_VERSION:-2.25.3}"

docker run --rm -v "$(pwd):/work" -w /work "ghcr.io/redocly/cli:${REDOCLY_CLI_VERSION}" lint api/openapi.yaml
