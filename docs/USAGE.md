# API usage guide

How to call the **Rewrite Commerce Engine** v1.0.0 in production-like environments. The **authoritative contract** is `api/openapi.yaml` (import into Postman, Redocly, or your gateway).

## 1. Base URL and contract

- **Prefix:** none at the app level (routes are absolute, e.g. `/products`, `/checkouts/...`).
- **OpenAPI:** `api/openapi.yaml` in this repository.
- **Errors:** JSON objects with `code` and `message` (and optional `details`). HTTP status reflects the class of error (400, 401, 403, 404, 409, 500, etc.).

## 2. Authentication layers

### 2.1 API token (almost all routes)

Every request except **public** paths must include the configured service token:

- `Authorization: Bearer <API_AUTH_TOKEN>` **or**
- `X-API-Token: <API_AUTH_TOKEN>`

**Public paths** (no API token):

- `GET /healthz`, `GET /readyz`
- `POST /identity/auth/login`, `POST /identity/auth/refresh`, `POST /identity/auth/logout`
- `POST /webhooks/*` (provider callbacks; verify provider-specific secrets where documented, e.g. payments)

If `API_AUTH_TOKEN` is unset, the server responds **503** with `auth_unavailable` for protected routes.

### 2.2 User JWT + permissions (sensitive prefixes)

Routes under certain prefixes additionally require a valid **`X-User-JWT`** (Bearer user access token issued by this service’s identity flows). The middleware checks:

- JWT signature (via `AUTH_JWT_SECRET` or `AUTH_JWT_KEYSET`),
- Optional tenant match with `X-Tenant-ID`,
- **`admin` role** in the token **or** the mapped user’s permission row in Postgres.

**Prefixes that enforce permissions** (v1.0.0; see `internal/app/app.go`):

- `/payments`, `/shipping`, `/identity/users`, `/metadata`
- `/apps`, `/apps/webhook-subscriptions`, `/webhooks/deliveries`, `/webhooks/outbox`
- `/channels`, `/gift-cards`, `/invoices`

Missing or insufficient permission → **403** `forbidden`.

### 2.3 Legacy role header (discouraged)

`ALLOW_LEGACY_ROLE_BYPASS=true` can allow a non-empty `X-Role` header to satisfy some legacy checks. **Disable in production.**

## 3. Multi-tenancy

Each request is scoped by:

| Header | When omitted |
| --- | --- |
| `X-Tenant-ID` | Defaults from **subdomain** (first label if host has ≥ 3 labels) or `DEFAULT_TENANT_ID` (`public`). |
| `X-Region-ID` | Defaults to `DEFAULT_REGION_ID` (`global`). |

Identifiers must match `^[a-z0-9][a-z0-9_-]{1,62}$` (lowercase). Invalid values → **400**.

## 4. Idempotency on writes

Many **mutating** endpoints require the header:

```http
Idempotency-Key: <unique string per logical operation>
```

Scopes are documented per operation in OpenAPI (e.g. `products.upsert`, `checkouts.complete:{checkout_id}`, `gift_cards.create`, `invoices.create`). Retries with the **same** key replay the **same** transactional outcome (stored resource id).

**Always** send a fresh key for a **new** business action (new checkout completion, new order, etc.).

## 5. Common request flow (storefront-oriented)

1. **Catalog:** `GET /products`, `GET /products/{id}` (optional `expand=attribute_values`, channel filters per OpenAPI).
2. **Customer / identity:** register users via your onboarding; `POST /identity/auth/login` returns tokens; use **`X-User-JWT`** for permissioned admin routes.
3. **Checkout:**  
   - `POST /checkouts/sessions` + `Idempotency-Key`  
   - `PUT /checkouts/sessions/{id}/lines` per line + `Idempotency-Key`  
   - `PATCH /checkouts/sessions/{id}` for context (shipping, channel, voucher, etc.) + `Idempotency-Key`  
   - `POST /checkouts/sessions/{id}/recalculate` + `Idempotency-Key`  
   - Optional: `POST /checkouts/sessions/{id}/gift-card` / `DELETE .../gift-card` + `Idempotency-Key`  
   - `POST /checkouts/sessions/{id}/complete` + `Idempotency-Key` after authorized payments cover `total_cents` (can be **0** if a gift card covers the full amount).
4. **Payments:** create and capture per `api/openapi.yaml` under `/payments`.
5. **Post-order:** `POST /fulfillments`, `GET /orders/...`, `POST /invoices` for billing documents.

## 6. Gift cards (v1.0.0)

- **Issue:** `POST /gift-cards` with `Idempotency-Key` (requires `gift_cards.manage`).
- **Storefront:** `POST /checkouts/sessions/{checkout_id}/gift-card` with body `{ "code": "..." }` and `Idempotency-Key`.  
- **Remove:** `DELETE /checkouts/sessions/{checkout_id}/gift-card` with `Idempotency-Key`.  
- At most **one open checkout** may hold a given card. Balances are applied during **recalculate** and debited on **complete**.

## 7. Invoices (v1.0.0)

- **Create:** `POST /invoices` + `Idempotency-Key` (requires `invoices.manage`); body includes `order_id` and `status` (`draft` | `issued` | `void`). Totals are copied from the **locked order** row.
- **List:** `GET /invoices?order_id=<id>`
- **Get:** `GET /invoices/{invoice_id}`

No PDF generation in v1.0.0 — persist `metadata` or integrate an external renderer.

## 8. Pagination and filtering

Follow query parameters documented in OpenAPI per resource (`channel_id`, `published_only`, etc.). If a parameter is not listed, assume **no** stable pagination contract for that route unless added in a later release.

## 9. Rate limiting

The application does not enforce global rate limits. Use **API gateway**, **reverse proxy**, or **WAF** limits for public endpoints (`/identity/auth/*`, webhooks, etc.).

## 10. Local smoke test

```bash
export API_AUTH_TOKEN=change-me
curl -sS -H "Authorization: Bearer $API_AUTH_TOKEN" \
  -H "X-Tenant-ID: public" -H "X-Region-ID: global" \
  http://localhost:8080/healthz
```

Then open `api/openapi.yaml` in your OpenAPI UI and execute a `GET /products` with the same headers.
