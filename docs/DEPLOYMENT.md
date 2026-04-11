# Production deployment guide

This document describes how to run the **Rewrite Commerce Engine** (v1.0.0+) safely in production. For day-to-day API usage, see [USAGE.md](./USAGE.md).

## 1. What you are deploying

- **Artifact:** a single static binary (`cmd/server`) or the provided **Docker** image (distroless runtime).
- **Database:** **PostgreSQL 16+** (15+ may work; 16 matches CI and `docker-compose.yml`).
- **Shape:** stateless HTTP API; **no** on-disk state beyond logs. Long-running work uses **in-process workers** (webhook outbox dispatcher, payment reconciliation loop) inside the same process.

## 2. Requirements

| Component | Notes |
| --- | --- |
| Postgres | Accessible via `DATABASE_URL`; use TLS in production (`sslmode=require` or equivalent). |
| CPU / RAM | Start with **2 vCPU / 1–2 GiB** for small catalogs; scale horizontally behind a load balancer (all instances share the same DB). |
| Network | Place the service behind a reverse proxy (TLS termination, rate limits, WAF). |

## 3. Version and release

- Release **semver** is stored in the repository root file **`VERSION`** (e.g. `1.0.0`).
- The binary logs `rewrite commerce engine v<version> listening on :<port>` at startup.
- **Docker** builds inject the version with `-ldflags "-X main.version=..."` (see `Dockerfile`).

## 4. Environment configuration

Copy `.env.example` and set values appropriate for production. Below is the full reference.

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `APP_ENV` | No | `development` | Set to `production` for live systems. `DATABASE_URL` may be omitted only for `test` / `testing`. |
| `PORT` | No | `8080` | HTTP listen port (inside container usually `8080`). |
| `DATABASE_URL` | **Yes** (prod) | — | Postgres DSN, e.g. `postgres://user:pass@host:5432/dbname?sslmode=require`. |
| `API_AUTH_TOKEN` | **Yes** (prod) | — | Shared secret for service-to-service calls. Clients send `Authorization: Bearer <token>` or `X-API-Token: <token>`. |
| `AUTH_JWT_SECRET` | Recommended | — | Legacy single signing secret for user JWTs (used when `AUTH_JWT_KEYSET` is empty). |
| `AUTH_JWT_KEYSET` | Optional | — | `kid:secret` pairs, comma-separated, for **JWT key rotation**; all kids verify, first signs. |
| `AUTH_JWT_TTL_MINUTES` | No | `60` | Access token lifetime. |
| `AUTH_REFRESH_TTL_MINUTES` | No | `10080` | Refresh session window (minutes). |
| `ALLOW_LEGACY_ROLE_BYPASS` | No | `false` | If `true`, allows legacy `X-Role` header bypass on guarded paths (avoid in production). |
| `DEFAULT_TENANT_ID` | No | `public` | Used when `X-Tenant-ID` is absent and host does not imply a tenant subdomain. |
| `DEFAULT_REGION_ID` | No | `global` | Used when `X-Region-ID` is absent. |
| `WEBHOOK_TIMEOUT_MS` | No | `3000` | Outbound webhook HTTP client timeout. |
| `WEBHOOK_PAYMENT_SECRET` | If using payment webhooks | — | Verifies provider callbacks under `/webhooks/...`. |
| `PAYMENT_RECONCILE_INTERVAL_SECONDS` | No | `300` | Background reconciliation tick (per default tenant/region in code). |
| `HTTP_TIMEOUT_MS` | No | `10000` | Per-request server timeout middleware. |
| `HTTP_MAX_BODY_BYTES` | No | `1048576` | Max request body size (1 MiB default). |
| `LOG_LEVEL` | No | `info` | Reserved for future structured logging; process still uses standard `log` today. |

### Secrets hygiene

- Generate `API_AUTH_TOKEN` with a CSPRNG (length ≥ 32 bytes, URL-safe encoding).
- Rotate `AUTH_JWT_KEYset` by adding a new `kid` first, then retiring old secrets after clients refresh.
- Never commit real `.env` files; inject secrets via your platform (Kubernetes Secrets, AWS SSM, Vault, etc.).

## 5. Database provisioning

### 5.1 Fresh database (recommended for v1.0.0)

Apply the **baseline schema** and then **every migration** in lexical order:

```bash
export DATABASE_URL="postgres://..."
chmod +x scripts/apply-migrations.sh   # Unix
./scripts/apply-migrations.sh
```

From the `rewrite/` directory, **Make** invokes the same flow:

```bash
make migrate
```

**Windows (PowerShell),** from `rewrite/`:

```powershell
$env:DATABASE_URL = "postgres://..."
powershell -ExecutionPolicy Bypass -File scripts/apply-migrations.ps1
```

This runs:

1. `internal/shared/db/schema.sql` — catalog, payments, webhooks, and related core tables.
2. `internal/shared/db/migrations/*.sql` — checkout, identity, shipping, gift cards, invoices, permissions, etc.

### 5.2 Upgrades

- Ship **forward-only** SQL migrations in `internal/shared/db/migrations/` with the next release; re-run only **new** files on existing databases (your release process should track applied revision).
- Prefer idempotent DDL (`IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`) where practical.
- Take a backup before applying migrations in production.

## 6. Container deployment

### 6.1 Build image

```bash
docker build -t rewrite:1.0.0 .
```

The image entrypoint is `/app/server` (distroless; no shell). The binary embeds the version from `VERSION`.

### 6.2 Local compose (development)

```bash
cp .env.example .env   # edit secrets
docker compose up -d --build
```

Compose wires Postgres and sets `DATABASE_URL` for the app. **Run migrations** against the DB (from host or a job container) using `scripts/apply-migrations.sh`.

### 6.3 Kubernetes (outline)

- **Deployment:** `replicas ≥ 2` for availability; **Pod** exposes container port `8080`.
- **Env:** inject all required variables from Secrets.
- **Probes:** `GET /healthz` (liveness), `GET /readyz` (readiness; checks DB ping).
- **Ingress:** TLS, set trusted headers if you terminate TLS upstream.
- **Migrations:** run as a **Job** before rolling out a new version, or use an init container with `psql` + migration files.

## 7. Networking and TLS

- Terminate TLS at the load balancer or ingress; speak HTTP to the app on a private network.
- If the proxy strips `Host`, configure `X-Tenant-ID` / `X-Region-ID` explicitly (see [USAGE.md](./USAGE.md)).
- Subdomain-based tenant inference uses the **first** label of a 3+ label hostname (e.g. `acme.api.example.com` → tenant `acme`).

## 8. Health checks

| Path | Meaning |
| --- | --- |
| `GET /healthz` | Process is up. |
| `GET /readyz` | Process can reach Postgres (`PingContext`). |

Return **503** from load balancers if `readyz` fails.

## 9. Background work

The same binary runs:

- **Webhook outbox worker** — delivers subscribed events; configure `WEBHOOK_TIMEOUT_MS`.
- **Payment reconciliation** — periodic pass over payments for the configured default tenant/region (`PAYMENT_RECONCILE_INTERVAL_SECONDS`).

These are **not** separate workers in v1.0.0; scaling replicas runs **one copy of each loop per replica** (acceptable for light load; for heavy webhook volume, consider deduplication at the subscription layer or a future dedicated worker release).

## 10. Observability

- Use **structured access logs** from your reverse proxy.
- Correlate with **Postgres** metrics (connections, slow queries).
- Application logs today are **standard library** `log` lines (startup + fatals); extend with OpenTelemetry or zap in a future release if needed.

## 11. Security checklist (production)

- [ ] `API_AUTH_TOKEN` set to a strong random value.
- [ ] `DATABASE_URL` uses TLS and least-privilege DB user.
- [ ] `APP_ENV=production` (or non-test) so empty `DATABASE_URL` is rejected.
- [ ] `ALLOW_LEGACY_ROLE_BYPASS=false`.
- [ ] Permissioned routes (`/payments`, `/shipping`, `/channels`, `/gift-cards`, `/invoices`, `/apps`, `/webhooks/...`, `/identity/users`, `/metadata`) only reachable with valid **`X-User-JWT`** and DB permissions (or admin role in JWT).
- [ ] `WEBHOOK_PAYMENT_SECRET` set if payment webhooks are exposed.
- [ ] Rate limiting and IP allowlists on `/identity/auth/*` as appropriate.

## 12. Rollback

- **Application:** redeploy the previous image/binary.
- **Database:** avoid destructive down migrations; keep forward-compatible DDL and restore from backup if needed.

## 13. Support files

| File | Role |
| --- | --- |
| `api/openapi.yaml` | HTTP contract (operations, schemas, idempotency expectations). |
| `docs/parity_gap_matrix.md` | Functional comparison vs Saleor (reference). |
| `VERSION` | Current release semver for builds and docs. |
