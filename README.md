# Rewrite Commerce Engine

Single-binary Go commerce backend replacing Saleor with an OpenAPI-first modular monolith.

## Quick start

1. Copy `.env.example` to your environment values.
2. Start stack:

```bash
make up
```

3. Apply schema:

```bash
make migrate
```

4. Quality checks:

```bash
make check
make integration-test
make build
```

If `make` is unavailable (for example, default Windows PowerShell), run equivalents directly:

```bash
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate
go test ./...
go vet ./...
go build ./...
```

PowerShell helper scripts are also available:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/sqlc-generate.ps1
powershell -ExecutionPolicy Bypass -File scripts/openapi-validate.ps1
powershell -ExecutionPolicy Bypass -File scripts/integration-test.ps1
powershell -ExecutionPolicy Bypass -File scripts/test-local.ps1
```

## Reliability notes

- Stateless HTTP server with tenant/region middleware.
- Durable event outbox and webhook dispatcher tables.
- Optimistic concurrency on order status updates.
- Idempotency-key support for write APIs.
- Sensitive routes require permission checks from signed user JWT (`X-User-JWT`) plus DB role mappings.
- Legacy role-header bypass is disabled by default and can be toggled with `ALLOW_LEGACY_ROLE_BYPASS=true` for migration windows.
- Identity now supports `login`, `refresh`, and `logout` session flows with hashed refresh tokens stored server-side.
- JWT key rotation is supported via `AUTH_JWT_KEYSET` in `kid:secret` CSV format; the first key is used for signing and all keys are accepted for verification.
- Refresh-token replay detection revokes compromised sessions when a previously-rotated token is reused.
- Identity sessions now support listing/revocation APIs and optional device binding on refresh to reduce token theft blast radius.

## Toolchain pins

- Go: `1.26.x` (CI), `go 1.26.0` (`go.mod`)
- sqlc: `1.30.0` (`SQLC_VERSION`)
- golangci-lint: `2.11.4` (`GOLANGCI_LINT_VERSION`)
- Redocly CLI: `2.25.3` (`REDOCLY_CLI_VERSION`)

You can override script versions per run, for example:

```bash
SQLC_VERSION=1.30.0 make sqlc-generate
REDOCLY_CLI_VERSION=2.25.3 make openapi-check
```

## Integration tests

- Integration tests are opt-in and require Postgres.
- `make integration-test` starts a temporary Postgres container, applies `internal/shared/db/schema.sql`, runs `./internal/integration/...`, and cleans up the container.
- On Windows PowerShell, use `scripts/integration-test.ps1` for the same flow.
- For manual runs, set `RUN_INTEGRATION=1` and a valid `DATABASE_URL`.

## Local test wrapper

- CI should continue using full `go test ./...`.
- For locked-down local Windows hosts where policy may block specific test binaries, use:
  - `make test-local` (bash environments), or
  - `powershell -ExecutionPolicy Bypass -File scripts/test-local.ps1`.
- You can override excluded packages with `GO_TEST_EXCLUDE` (comma-separated import paths).

## Rollback guidance

- Prefer forward-fix migrations.
- Keep schema backward compatible during deployment waves.
- Use `docker compose down` to stop local stack safely.
