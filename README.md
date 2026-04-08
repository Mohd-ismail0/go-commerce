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
make openapi-check
make sqlc-check
make test
make build
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

## Rollback guidance

- Prefer forward-fix migrations.
- Keep schema backward compatible during deployment waves.
- Use `docker compose down` to stop local stack safely.
