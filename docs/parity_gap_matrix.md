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

_Last updated: 2026-04-11 — **gift cards** (`/gift-cards`, checkout `POST`/`DELETE .../gift-card`, balance apply on recalculate, deduct on complete), **order invoices** (`/invoices`), payment coverage when `total_cents` is 0 after gift card._

### Parity scope and gap rate (full Saleor ecosystem view)

This matrix compares the rewrite to **Saleor’s broad platform** (all Django apps and major storefront/dashboard capabilities), not a trimmed storefront-only subset.

**Counted units (denominator):**

1. Each row in **Saleor Django apps vs rewrite** (high-level app coverage).
2. Each capability row from **API contract vs router** through **Fulfillments**, plus **Localization, metadata, search** and **Regions & brands** (detailed parity).

**Gap rate:** `(# of rows whose status is **Gap**) / (denominator)`.

**REST** rows count toward the denominator but are scored as **REST**, not **Gap** (deliberate surface mismatch).

**Current snapshot (approx.):** ~**76** counted rows; **6** rows remain **Gap** → **~8%** ecosystem gap: **CMS (pages/menus)** and **CSV jobs** in the app table, plus **multi-shipment checkout**, **draft-order admin depth**, **per-warehouse stock allocation**, and **fulfillment-line/refund depth** in the detailed sections. **Payment “App Store”** and **plugins** are scored **Partial** (compiled integrations + `apps` + webhooks), not full Saleor marketplace parity.

---

## API contract vs router

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| OpenAPI describes HTTP routes | **Partial** | Major routes are documented (products, checkouts, orders, fulfillments, payments, channels, shipping, customers, identity, apps, webhooks, catalog attributes/types, pricing, inventory, promotions, regions, brands, translations, metadata, search). Re-audit when adding handlers; optional query/body edge cases may be lighter than implementation. |
| Idempotency on writes | **Partial** | `Idempotency-Key` is **required** (per OpenAPI) on: `POST /products` (scope `products.upsert`), checkout `POST /checkouts/sessions`, **`PATCH /checkouts/sessions/{checkout_id}`** (`checkouts.session.patch:{checkout_id}`), **`PATCH /shop/settings`** (`shop.settings:{tenant}:{region}`), **`POST /gift-cards`** (`gift_cards.create`), **`POST /invoices`** (`invoices.create`), checkout **`POST .../gift-card`** (`checkouts.gift_card.apply:{checkout_id}`) and **`DELETE .../gift-card`** (`checkouts.gift_card.remove:{checkout_id}`), `POST .../apply-customer-addresses`, `PUT .../lines` (per line id in body), `POST .../recalculate`, `POST .../complete`, `POST /orders`, `POST /fulfillments`, `POST /customers`, `POST /customers/{id}/addresses`, and several payment mutation routes. **Not** required on many other writes (e.g. `POST /product-types`, `POST /attributes`, `PUT .../attribute-values`, variants, media, categories, collections) — clients must treat those as non-idempotent unless extended. |
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
| `warehouse` | `inventory` + checkout stock logic | **Partial** — `GET`/`POST /inventory/warehouses`; stock quantities still on `stock_items` (no Saleor-style per-warehouse allocation API). |
| `webhook` | `webhooks`, `events` | **Partial** |
| `translations` | `localization` | **Partial** |
| `metadata` | `metadata` | **Partial** |
| `permission` | policy middleware, DB permissions | **Partial** |
| `giftcard` | `giftcard` | **Partial** — `GET`/`POST /gift-cards`; checkout apply/remove + balance on recalculate/complete; no full Saleor gift-card product/catalog graph. |
| `invoice` | `invoice` | **Partial** — `GET`/`POST /invoices`, `GET /invoices/{id}`; totals from locked order row; no PDF generation pipeline. |
| `page`, `menu` | — | **Gap** (no CMS). |
| `site` | `shop` | **Partial** — `GET`/`PATCH /shop/settings` (tenant/region display, domain, email, address, metadata); not Saleor’s full site graph. |
| `plugins` | `apps`, `webhooks`, workers | **Partial** — no plugin marketplace; behavior is bespoke modules + webhooks + app registry. |
| `csv` | — | **Gap** (no import/export jobs). |
| `thumbnail`, `seo` | product SEO fields, media URLs only | **Partial** — catalog SEO fields and media URLs; no Saleor-style thumbnail generation service. |
| `schedulers` | workers (payments, webhooks) | **Partial** — not Saleor’s scheduler model. |

---

## Checkout & cart

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Checkout session lifecycle | **Partial** | **Create:** `POST /checkouts/sessions` + `Idempotency-Key`, scope `checkouts.sessions.create`, transactional + advisory lock. **Read:** `GET` session. **Validation:** `GET .../validation` read-only problems (shipping, payment, channel listings, stock). **Patch context:** `PATCH` session + `Idempotency-Key`, scope `checkouts.session.patch:{checkout_id}`, advisory lock + idempotency in same tx as `UPDATE checkout_sessions`. **Lines:** `GET .../lines` (404 if session id unknown for tenant; known session with no lines returns `items: []`). **Line upsert:** `PUT .../lines` + `Idempotency-Key`, scope `checkouts.line.upsert:{checkout_id}:{line_id}`, advisory lock + idempotency in same tx as reservation. **Recalculate:** `POST .../recalculate` + `Idempotency-Key`, scope `checkouts.recalculate:{checkout_id}`. **Complete:** `POST .../complete` + `Idempotency-Key`, scope `checkouts.complete:{checkout_id}`. **Immutability:** non-open sessions reject mutating writes as applicable. |
| Atomic totals refresh | **Full** | `Recalculate` path: `FOR UPDATE` on open session; subtotal, shipping, tax, total in **one transaction**. Internal recalcs after `PATCH` or before complete reuse the same calculation/repo logic; not every internal path persists an idempotency key. |
| Shipping method on checkout | **Partial** | Shipping method id + country/postal on session; eligibility via `shipping_methods` rules (channels, postal prefixes, min/max order value). |
| Stock reservation per line | **Partial** | Idempotent line upsert acquires **soft reservation** (`stock_reservations` keyed by `checkout_line_id`, TTL). Salable quantity uses `stock_items.quantity` minus other active reservations. **Complete** deducts stock, clears this checkout’s reservations, and enforces `on_hand` against other checkouts’ reservations + line demand. **No** multi-warehouse optimizer beyond `resolveStockItemID`-style resolution. |
| Multiple shipping / split deliveries | **Gap** | Single shipping context on checkout session. |
| Gift cards | **Partial** | `POST/DELETE /checkouts/sessions/{id}/gift-card` + `Idempotency-Key`; one open checkout per card (`ux_checkout_open_unique_gift_card`); `gift_card_applied_cents` and reduced `total_cents` after tax during **recalculate** (`FOR UPDATE` on card); balance debited on **complete**; `GET/POST /gift-cards` for issuance (`gift_cards.manage`). No gift-card products or multi-code stacking like Saleor. |
| Checkout “problems” / errors collection | **Partial** | `GET /checkouts/sessions/{checkout_id}/validation` returns codes such as `empty_cart`, `shipping_method_required`, `payment_coverage_required`, `variant_listing_mismatch`, `insufficient_stock`. Narrower than Saleor’s full `checkoutProblems` graph. |
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
| Sites / storefront config | **Partial** | `GET`/`PATCH /shop/settings` with defaults per tenant/region (`shop_settings`); `PATCH` + `Idempotency-Key`, advisory lock + idempotency in tx. Not Saleor’s full multi-site model. |

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
| Warehouse registry | **Partial** | `GET/POST /inventory/warehouses` lists/saves `Warehouse` rows (id, name, code, `is_active`). |
| Multi-warehouse allocation | **Gap** | Stock remains on flat `stock_items`; no per-warehouse quantities or Saleor-style allocations. |

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
| Weight / item-based rates | **Partial** | Methods keep flat `price_cents`; optional `weight_surcharge_per_kg_cents` and `POST /shipping/resolve` with `total_weight_grams` returns `quoted_price_cents`. No full rate tables or item-count tiers. |
| Delivery time / metadata | **Partial** | `delivery_days_min`, `delivery_days_max`, `description` on `shipping_methods`. |

---

## Payments

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Payment CRUD, transactions | **Partial** | Create/list/update payments; capture, refund, void; transaction history; provider webhook endpoint documented. |
| Reconciliation worker | **Partial** | Background reconciliation in `app.go` (interval from config). |
| Payment apps / plugins | **Partial** | No installable **App Store**; payments and providers are compiled services. **`apps`** + webhook outbox approximate extension points. |

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
3. After a tranche ships, adjust the relevant rows, the **Saleor apps** summary if modules were added, recompute the **full ecosystem** gap snapshot in **Parity scope and gap rate**, and the **_Last updated_** line at the top.
