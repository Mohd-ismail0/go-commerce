# Apply baseline schema then migrations. Requires psql in PATH and DATABASE_URL.
# Run from repository root:  powershell -File scripts/apply-migrations.ps1
$ErrorActionPreference = "Stop"
if (-not $env:DATABASE_URL) {
  Write-Error "DATABASE_URL is not set"
}
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $root

$schema = Join-Path $root "internal/shared/db/schema.sql"
Write-Host "Applying $schema ..."
Get-Content $schema -Raw | psql $env:DATABASE_URL -v ON_ERROR_STOP=1

$migDir = Join-Path $root "internal/shared/db/migrations"
$migrations = Get-ChildItem -Path $migDir -Filter "*.sql" | Sort-Object Name
foreach ($m in $migrations) {
  Write-Host "Applying $($m.FullName) ..."
  Get-Content $m.FullName -Raw | psql $env:DATABASE_URL -v ON_ERROR_STOP=1
}
Write-Host "Database provisioning complete."
