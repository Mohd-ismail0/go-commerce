$ErrorActionPreference = "Stop"

# Comma-separated import paths to skip for local policy-constrained environments.
$excludeRaw = if ($env:GO_TEST_EXCLUDE) { $env:GO_TEST_EXCLUDE } else { "rewrite/internal/modules/webhooks" }
$exclude = $excludeRaw.Split(",") | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne "" }

$allPackages = go list ./...
if ($LASTEXITCODE -ne 0) {
  throw "failed to list packages"
}

$filtered = @()
foreach ($pkg in $allPackages) {
  if ($exclude -notcontains $pkg) {
    $filtered += $pkg
  }
}

if ($filtered.Count -eq 0) {
  throw "no packages left to test after applying GO_TEST_EXCLUDE"
}

go test $filtered
if ($LASTEXITCODE -ne 0) {
  throw "local test run failed"
}
