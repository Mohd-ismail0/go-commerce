# Saleor parity gap matrix (rewrite)

This matrix tracks **functional parity** between [Saleor](https://github.com/saleor/saleor) (GraphQL-first reference, Django apps under `saleor/saleor`) and the Go **rewrite** service (**REST-first**, multi-tenant/region). Intentional differences are called out as **REST** or noted inline.

## Status meanings

| Status | Meaning |
| --- | --- |
| **Full** | Core Saleor-equivalent behavior covered for typical storefront flows; edge cases may still differ. |
| **Partial** | Implemented with a **narrower model**, fewer edge cases, or simpler rules than Saleor. |
| **Gap** | Missing or not first-class vs Saleor; not a thin wrapper around the same concepts. |
| **REST** | Deliberate surface mismatch (e.g. no GraphQL); underlying behavior can still be **Partial** or **Full**. |

## Primary references

- `docs/saleor_mapping.md` — domain mapping notes (some rows may lag the code; this matrix is authoritative for parity status).
- `api/openapi.yaml` — HTTP contract and idempotency markers.
- `internal/app/app.go` — wired handlers.
- `internal/modules/*` — behavior and persistence.

_Last updated: 2026-04-11 — checkout **`PATCH /checkouts/sessions/{checkout_id}`** requires **`Idempotency-Key`** (scope `checkouts.session.patch:{checkout_id}`, advisory lock + idempotency save in the same tx as the session update)._

---

## API contract vs router

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| OpenAPI describes HTTP routes | **Partial** | Major routes are documented (products, checkouts, orders, fulfillments, payments, channels, shipping, customers, identity, apps, webhooks, catalog attributes/types, pricing, inventory, promotions, regions, brands, translations, metadata, search). Re-audit when adding handlers; optional query/body edge cases may be lighter than implementation. |
| Idempotency on writes | **Partial** | `Idempotency-Key` is **required** (per OpenAPI) on: `POST /products` (scope `products.upsert`), checkout `POST /checkouts/sessions`, **`PATCH /checkouts/sessions/{checkout_id}`** (`checkouts.session.patch:{checkout_id}`), `POST .../apply-customer-addresses`, `PUT .../lines` (per line id in body), `POST .../recalculate`, `POST .../complete`, `POST /orders`, `POST /fulfillments`, `POST /customers`, `POST /customers/{id}/addresses`, and several payment mutation routes. **Not** required on many other writes (e.g. `POST /product-types`, `POST /attributes`, `PUT .../attribute-values`, variants, media, categories, collections) — clients must treat those as non-idempotent unless extended. |
| GraphQL / subscriptions | **REST** | No GraphQL schema, dataloaders, or subscriptions. **Webhooks + outbox worker** approximate async/plugin patterns only where events are emitted. |
| Error shape & codes | **Partial** | JSON error payloads with `code` / `message`; semantics and HTTP status choices may differ from Saleor’s GraphQL errors. |

---

## Saleor Django apps vs rewrite (high level)

Saleor ships many Django apps under `saleor/saleor`. The rewrite does **not** mirror each app as a module; the table below summarizes coverage.

| Saleor app (indicative) | Rewrite | Parity (summary) |
| --- | --- | --- |
| `account` | `customers`, `identity` | **Partial** — customers + auth sessions; not full staff/account graph. |
| `app` | `apps` | **Partial** |
| `channel` | `channels` | **Partial** |
| `checkout` | `checkout` | **Partial** |
| `core` | shared middleware, config | **Partial** — different cross-cutting model. |
| `discount` | `promotions` | **Partial** |
| `order` | `orders`, `fulfillments` | **Partial** |
| `payment` | `payments` | **Partial** |
| `product` | `catalog` | **Partial** — includes product types & attributes (see Catalog). |
| `shipping` | `shipping` | **Partial** |
| `tax` | `pricing` (tax classes/rates + calculate) | **Partial** |
| `warehouse` | `inventory` + checkout stock logic | **Partial** / **Gap** on multi-warehouse API. |
| `webhook` | `webhooks`, `events` | **Partial** |
| `translations` | `localization` | **Partial** |
| `metadata` | `metadata` | **Partial** |
| `permission` | policy middleware, DB permissions | **Partial** |
| `giftcard` | — | **Gap** |
| `invoice` | — | **Gap** |
| `page`, `menu` | — | **Gap** (no CMS). |
| `site` | — | **Gap** (no Shop/Site settings module). |
| `plugins` | — | **Gap** |
| `csv` | — | **Gap** (no import/export jobs). |
| `thumbnail`, `seo` | product SEO fields, media URLs only | **Gap** / **Partial** — no dedicated thumbnail/SEO subsystems. |
| `schedulers` | workers (payments, webhooks) | **Partial** — not Saleor’s scheduler model. |

---

## Checkout & cart

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Checkout session lifecycle | **Partial** | **Create:** `POST /checkouts/sessions` + `Idempotency-Key`, scope `checkouts.sessions.create`, transactional + advisory lock. **Read:** `GET` session. **Patch context:** `PATCH` session + `Idempotency-Key`, scope `checkouts.session.patch:{checkout_id}`, advisory lock + idempotency in same tx as `UPDATE checkout_sessions`. **Lines:** `GET .../lines` (404 if session id unknown for tenant; known session with no lines returns `items: []`). **Line upsert:** `PUT .../lines` + `Idempotency-Key`, scope `checkouts.line.upsert:{checkout_id}:{line_id}`, advisory lock + idempotency in same tx as reservation. **Recalculate:** `POST .../recalculate` + `Idempotency-Key`, scope `checkouts.recalculate:{checkout_id}`. **Complete:** `POST .../complete` + `Idempotency-Key`, scope `checkouts.complete:{checkout_id}`. **Immutability:** non-open sessions reject mutating writes as applicable. |
| Atomic totals refresh | **Full** | `Recalculate` path: `FOR UPDATE` on open session; subtotal, shipping, tax, total in **one transaction**. Internal recalcs after `PATCH` or before complete reuse the same calculation/repo logic; not every internal path persists an idempotency key. |
| Shipping method on checkout | **Partial** | Shipping method id + country/postal on session; eligibility via `shipping_methods` rules (channels, postal prefixes, min/max order value). |
| Stock reservation per line | **Partial** | Idempotent line upsert acquires **soft reservation** (`stock_reservations` keyed by `checkout_line_id`, TTL). Salable quantity uses `stock_items.quantity` minus other active reservations. **Complete** deducts stock, clears this checkout’s reservations, and enforces `on_hand` against other checkouts’ reservations + line demand. **No** multi-warehouse optimizer beyond `resolveStockItemID`-style resolution. |
| Multiple shipping / split deliveries | **Gap** | Single shipping context on checkout session. |
| Gift cards | **Gap** | Not modeled in checkout, pricing, or payment totals. |
| Checkout “problems” / errors collection | **Gap** | No Saleor-style aggregated `checkoutProblems` / unified validation list API. |
| Complete → order | **Partial** | Requires authorized payment coverage vs `checkout_id`, shipping when lines exist; creates order via completion path. Replay with same idempotency key returns stored order without re-emitting `order.created`. |
| Checkout from saved addresses | **Partial** | `POST .../apply-customer-addresses` + `Idempotency-Key`, scope `checkouts.apply_customer_addresses:{checkout_id}`, advisory lock + idempotency in tx. Copies country/postal from `customer_addresses` for session `customer_id` with `FOR UPDATE` on `checkout_sessions`. |

---

## Orders

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Order create (direct) | **Partial** | `POST /orders` with idempotency; pricing calculator can adjust totals; voucher path exists. |
| Status transitions | **Partial** | Optimistic concurrency (`expected_updated_at`); cancel may restock where implemented in repository. |
| Order lines from checkout snapshot | **Partial** | Depth depends on checkout completion vs standalone create; not Saleor’s full order line mutation set. |
| Draft orders / unlimited order edits | **Gap** | No draft-order lifecycle or rich admin-style order editing parity. |
| Refunds at order level (integrated) | **Partial** | Payments module supports capture/refund/void; coupling between order state and payment refunds may be shallower than Saleor. |

---

## Catalog & merchandising

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Products & variants CRUD | **Partial** | `GET/POST /products`, `GET/POST/PATCH .../variants`. Core fields: SKU, name, price, currency, slug, description, SEO fields, metadata, `external_reference`. Optional **`product_type_id`** (FK to tenant/region product type). Product upsert runs in a **transaction** and **reconciles attribute values** when the type assignment or type↔attribute schema no longer allows stored values (cleanup deletes stale rows). **Idempotency:** `POST /products` requires `Idempotency-Key` (`products.upsert`). Variants/media/category/collection writes are **not** idempotent in OpenAPI unless added later. |
| Categories & collections | **Partial** | `GET/POST /categories`, `GET/POST /collections`, `POST /collections/{id}/products/{product_id}`. |
| Product media | **Partial** | `GET/POST .../media` under product. |
| Product types & catalog attributes | **Partial** | **Product types:** `GET/POST /product-types`, `GET /product-types/{id}` returns type plus assigned attributes (sort order, `variant_only`, attribute metadata). **Attributes:** `GET/POST /attributes` defines `catalog_attributes` with `input_type` ∈ `text`, `number`, `boolean`, `select` (select requires JSON `allowed_values` array). **Linking:** `POST /product-types/{id}/attributes`, `DELETE .../attributes/{attribute_id}` (removes link and clears stored values for that attribute on products of that type). **Values:** `GET/PUT /products/{id}/attribute-values` for **non-**`variant_only` attributes; `GET/PUT .../variants/{id}/attribute-values` for **variant_only** attributes. Product must have `product_type_id` set to write values; values validated against type assignment and input type. **List:** `GET /products?expand=attribute_values` batches product-level values. **Gaps vs Saleor:** values stored as canonical **strings** (no rich attribute value types, references, files, swatches, or per-locale attribute values); no separate Saleor “AttributeValue” entity graph; tax class per variant / full product type metadata not replicated. |
| Publishing / availability rules | **Partial** | `product_channel_listings` / `variant_channel_listings`; `channel_id` + `published_only` filters on product/variant list APIs; active channel enforcement on checkout/catalog paths. Not Saleor’s full availability/publishing engine. |

---

## Channels & sites

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Channel CRUD | **Partial** | Channels + product/variant listings; `is_active` enforced where applicable. |
| Per-channel pricing (listings) | **Partial** | Listing price, currency, publish flags; differs from Saleor’s full channel listing and pricing matrix. |
| Sites / storefront config | **Gap** | No first-class `Site` / `Shop` settings module (default shop, domain, etc.). |

---

## Customers & identity

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Customers list/save | **Partial** | `GET/POST /customers`; `POST` requires `Idempotency-Key` (scope `customers.save`), transactional + advisory lock. No full “account” API parity (orders history, preferences, etc.). |
| Addresses | **Partial** | `GET/POST /customers/{id}/addresses`, `PUT/DELETE .../addresses/{address_id}`. `POST` requires `Idempotency-Key` (scope `customers.addresses.create:{customer_id}`), advisory lock + idempotency in tx with `SELECT ... FOR UPDATE` on `customers`; default shipping/billing flags cleared on other rows when setting defaults. Checkouts can consume addresses via `apply-customer-addresses`. |
| Auth (login / refresh / logout) | **Partial** | `identity`: login, refresh, logout, session listing, revoke, revoke-others; JWT/keyset options in policy middleware. Semantics differ from Saleor’s dashboard/storefront JWT split. |
| Staff / permissions model | **Partial** | `PolicyAuthorization` + permission codes (e.g. `payments.manage`, `channel.manage`, `app.manage`, `webhook.manage`); not Saleor’s fine-grained dashboard permission graph. |

---

## Inventory & warehouses

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Stock items list/save | **Partial** | `GET/POST /inventory`; checkout uses `stock_reservations` and allocations on complete. No admin-focused reservation APIs. |
| Multi-warehouse allocation | **Gap** | No Saleor-style `Warehouse`, stock per warehouse, and allocation API surfaced like Saleor’s warehouse module. |

---

## Pricing, tax, promotions

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Tax classes & rates | **Partial** | CRUD under `/pricing/tax-*`; wired into checkout/order calculation inputs. |
| `POST /pricing/calculate` | **Partial** | Central calculator entrypoint; external tax plugins (Avalara, etc.) not built-in. |
| Promotions & vouchers | **Partial** | `promotions`, rules, vouchers modules; discount **breadth** (conditions, stacking, schedules) not guaranteed to match Saleor. |

---

## Shipping

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Zones & methods | **Partial** | CRUD + `POST /shipping/resolve`; JSONB rules (countries, channels, postal prefixes, order bounds). |
| Weight / item-based rates | **Gap** | Methods use flat `price_cents`; no weight- or item-based rate tables. |
| Delivery time / metadata | **Gap** | Minimal fields compared to Saleor shipping metadata. |

---

## Payments

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Payment CRUD, transactions | **Partial** | Create/list/update payments; capture, refund, void; transaction history; provider webhook endpoint documented. |
| Reconciliation worker | **Partial** | Background reconciliation in `app.go` (interval from config). |
| Payment apps / plugins | **Gap** | No Saleor App Store / plugin architecture; each integration is bespoke in code. |

---

## Apps & webhooks

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Apps CRUD + activate/deactivate | **Partial** | Inactive apps excluded from subscription-driven delivery behavior. |
| Webhook subscriptions & deliveries | **Partial** | Subscriptions, deliveries list, outbox retry; retry skips targets already succeeded. |
| Async event fan-out | **Partial** | Outbox + worker; **event catalog** likely smaller and differently named than Saleor’s GraphQL subscription events. |

---

## Localization, metadata, search

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Translations | **Partial** | `translations` + overlays on catalog list APIs (`language_code`); entity/field coverage may be narrower than Saleor’s translatable model. |
| Metadata (public/private) | **Partial** | Generic `metadata` module; not guaranteed to cover every Saleor metadata surface. |
| Search index & query | **Partial** | `POST /search/index`, `GET /search`; ranking and filters differ from Saleor search. |

---

## Regions & brands

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Regions / brands CRUD | **Partial** | Baseline `regions` and `brands` modules; not full Saleor shop/country/locale configuration. |

---

## Fulfillments

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Fulfillment records | **Partial** | `GET/POST /fulfillments`; create in **one transaction** (order `FOR UPDATE`, fulfillment insert, status/audit updates, idempotency save). |
| Idempotent create | **Partial** | `Idempotency-Key` required; scope `fulfillments.create`; replay returns stored fulfillment without re-emitting `order.completed`. |
| Fulfillment lines / refunds tie-in | **Gap** | Model is narrower than Saleor fulfillment lines; refund integration depth may lag. |

---

## How to use this matrix

1. Treat **Gap** rows as backlog candidates; treat **Partial** rows as “verify behavior in tests and docs before claiming parity.”
2. For any new HTTP surface, update **`api/openapi.yaml`** and re-check the **OpenAPI** and **Idempotency** rows.
3. After a tranche ships, adjust the relevant rows, the **Saleor apps** summary if modules were added, and the **_Last updated_** line at the top.
