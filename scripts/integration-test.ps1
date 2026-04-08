$ErrorActionPreference = "Stop"

$pgImage = if ($env:PG_IMAGE) { $env:PG_IMAGE } else { "postgres:16" }
$containerName = if ($env:PG_CONTAINER_NAME) { $env:PG_CONTAINER_NAME } else { "pg-ci" }
$pgPort = if ($env:PG_PORT) { $env:PG_PORT } else { "55432" }
$pgUser = if ($env:PG_USER) { $env:PG_USER } else { "postgres" }
$pgPassword = if ($env:PG_PASSWORD) { $env:PG_PASSWORD } else { "postgres" }
$pgDb = if ($env:PG_DB) { $env:PG_DB } else { "rewrite" }

try {
  docker run -d --name $containerName -p "${pgPort}:5432" -e "POSTGRES_USER=$pgUser" -e "POSTGRES_PASSWORD=$pgPassword" -e "POSTGRES_DB=$pgDb" $pgImage | Out-Null

  for ($i = 1; $i -le 30; $i++) {
    docker exec $containerName pg_isready -U $pgUser -d $pgDb | Out-Null
    if ($LASTEXITCODE -eq 0) {
      break
    }

    if ($i -eq 30) {
      throw "postgres did not become ready in time"
    }

    Start-Sleep -Seconds 1
  }

  $env:DATABASE_URL = "postgres://$($pgUser):$($pgPassword)@127.0.0.1:$($pgPort)/$($pgDb)?sslmode=disable"
  $internalDatabaseUrl = "postgres://$($pgUser):$($pgPassword)@127.0.0.1:5432/$($pgDb)?sslmode=disable"
  Get-Content "internal/shared/db/schema.sql" | docker exec -i $containerName psql $internalDatabaseUrl -v ON_ERROR_STOP=1
  if ($LASTEXITCODE -ne 0) {
    throw "failed to apply schema.sql"
  }

  $env:RUN_INTEGRATION = "1"
  go test ./internal/integration/... -count=1
  if ($LASTEXITCODE -ne 0) {
    throw "integration tests failed"
  }
}
finally {
  docker rm -f $containerName | Out-Null
}
