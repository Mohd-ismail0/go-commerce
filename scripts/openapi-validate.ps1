$ErrorActionPreference = "Stop"

$redoclyVersion = if ($env:REDOCLY_CLI_VERSION) { $env:REDOCLY_CLI_VERSION } else { "2.25.3" }

if (Get-Command redocly -ErrorAction SilentlyContinue) {
  redocly lint api/openapi.yaml
  exit $LASTEXITCODE
}

docker run --rm `
  -v "${PWD}:/work" `
  -w /work `
  "ghcr.io/redocly/cli:$redoclyVersion" `
  lint api/openapi.yaml
