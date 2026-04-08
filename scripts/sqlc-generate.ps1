$ErrorActionPreference = "Stop"

$sqlcVersion = if ($env:SQLC_VERSION) { $env:SQLC_VERSION } else { "1.30.0" }

if (Get-Command sqlc -ErrorAction SilentlyContinue) {
  sqlc generate
  exit $LASTEXITCODE
}

docker run --rm `
  -v "${PWD}:/src" `
  -w /src `
  "sqlc/sqlc:$sqlcVersion" `
  generate
