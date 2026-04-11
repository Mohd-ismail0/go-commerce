# Rewrite Commerce Engine

**v1.0.0** — OpenAPI-first, multi-tenant Go commerce backend (modular monolith). This release is the first stable semver for production deployment.

| Doc | Purpose |
| --- | --- |
| [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) | **Production:** Postgres, env vars, migrations, Docker/Kubernetes, security checklist |
| [docs/USAGE.md](docs/USAGE.md) | **API usage:** auth headers, tenant/region, idempotency, common flows |
| [api/openapi.yaml](api/openapi.yaml) | HTTP contract (machine-readable) |
| [CHANGELOG.md](CHANGELOG.md) | Release notes |

## Quick start (local)

1. Copy `.env.example` to `.env` and set secrets (`API_AUTH_TOKEN`, `AUTH_JWT_*`, etc.).
2. Start Postgres + app:

   ```bash
   docker compose up -d --build
   ```

3. **Provision the database** (baseline schema + all migrations):

   ```bash
   export DATABASE_URL="postgres://postgres:postgres@localhost:5432/rewrite?sslmode=disable"
   make migrate
   ```

   On Windows PowerShell:

   ```powershell
   $env:DATABASE_URL = "postgres://postgres:postgres@localhost:5432/rewrite?sslmode=disable"
   powershell -ExecutionPolicy Bypass -File scripts/apply-migrations.ps1
   ```

4. Run the server locally (without Docker):

   ```bash
   go run ./cmd/server
   ```

   Startup logs include: `rewrite commerce engine v1.0.0 listening on :8080` (version from `VERSION` when built via `Dockerfile`, or `dev` for plain `go run`).

5. Quality checks:

   ```bash
   make check
   make integration-test
   make build
   ```

If `make` is unavailable (e.g. default Windows PowerShell without Git Bash), run equivalents:

```bash
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate
go test ./...
go vet ./...
go build ./...
```

PowerShell helpers:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/sqlc-generate.ps1
powershell -ExecutionPolicy Bypass -File scripts/openapi-validate.ps1
powershell -ExecutionPolicy Bypass -File scripts/integration-test.ps1
powershell -ExecutionPolicy Bypass -File scripts/test-local.ps1
```

## Release artifact

- **Version file:** `VERSION` (single line, e.g. `1.0.0`).
- **Docker:** `docker build -t rewrite:1.0.0 .` embeds the version via `-ldflags` (see `Dockerfile`).
- **Binary:** `go build -ldflags "-s -w -X main.version=1.0.0" -o server ./cmd/server`

## Reliability (summary)

- Stateless HTTP with tenant/region middleware (`X-Tenant-ID`, `X-Region-ID`, defaults from env / host).
- Durable **event outbox** and webhook dispatcher.
- **Optimistic concurrency** on order status updates where implemented.
- **Idempotency-Key** on critical writes (see OpenAPI and [USAGE.md](docs/USAGE.md)).
- Sensitive routes require **`X-User-JWT`** plus DB permissions (or admin role); see [DEPLOYMENT.md](docs/DEPLOYMENT.md).
- `ALLOW_LEGACY_ROLE_BYPASS` defaults **off**; enable only for controlled migration windows.
- Identity: login / refresh / logout, hashed refresh tokens, optional JWT keyset (`AUTH_JWT_KEYSET`), refresh replay detection.

## Database layout

- **`internal/shared/db/schema.sql`** — consolidated baseline used with CI and integration tests.
- **`internal/shared/db/migrations/*.sql`** — ordered migrations (checkout, identity, shipping, gift cards, invoices, …).
- **Production / fresh install:** run `scripts/apply-migrations.sh` (or `make migrate`) so **both** baseline and migrations are applied.

## Toolchain pins

- Go: `1.26.x` (CI), `go 1.26.0` (`go.mod`)
- sqlc: `1.30.0` (`SQLC_VERSION`)
- golangci-lint: `2.11.4` (`GOLANGCI_LINT_VERSION`)
- Redocly CLI: `2.25.3` (`REDOCLY_CLI_VERSION`)

Override per run, e.g. `SQLC_VERSION=1.30.0 make sqlc-generate`.

## Integration tests

- Opt-in; require Postgres.
- `make integration-test` starts a temporary Postgres container, applies **`schema.sql` only**, runs `./internal/integration/...`.  
  **Note:** full application features also rely on **`migrations/`**; use `make migrate` against a dev DB for end-to-end manual testing of checkout, gift cards, invoices, etc.

## Local test wrapper

- `make test-local` or `scripts/test-local.ps1` for locked-down hosts.
- Override excluded packages with `GO_TEST_EXCLUDE` (comma-separated import paths).

## Rollback

- Prefer **forward-fix** migrations.
- Keep schema backward compatible across rolling deploys.
- `docker compose down` stops the local stack without deleting the volume unless you remove it explicitly.
