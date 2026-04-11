# Saleor parity gap matrix (rewrite)

This matrix tracks **functional parity** between Saleor (GraphQL-first reference) and the Go **rewrite** service (REST-first, intentional deviations allowed). Status meanings:

| Status | Meaning |
| --- | --- |
| **Full** | Core Saleor-equivalent behavior covered for typical storefront/admin flows. |
| **Partial** | Implemented but narrower model, fewer edge cases, or simpler rules than Saleor. |
| **Gap** | Missing or stub-level vs Saleor; work planned or not started. |
| **REST** | Deliberately different surface (REST vs GraphQL); behavior may still be **Partial** or **Full** underneath. |

Primary references: `docs/saleor_mapping.md`, `api/openapi.yaml`, `internal/modules/*`, `internal/app/app.go`.

_Last updated: fulfillments OpenAPI + transactional create with idempotency and `order.completed` emission (non-replay only)._

## API contract vs router

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| OpenAPI describes all HTTP routes | **Partial** | All major routes documented, including `/fulfillments` (see `post_fulfillments`, `get_fulfillments`). Re-audit when adding handlers. |
| Idempotency on writes | **Partial** | Supported where OpenAPI marks `Idempotency-Key` (e.g. orders, payments); not uniform across every POST. |
| GraphQL / subscriptions | **REST** | No GraphQL; webhooks + outbox replace plugin real-time patterns. |

## Checkout & cart

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Checkout session lifecycle | **Partial** | Create/get/update context, lines upsert, `recalculate`, `complete`. Open/non-open immutability enforced. |
| Atomic totals refresh | **Full** | `Recalculate`: `FOR UPDATE` on open session; subtotal, shipping, tax/total in **one transaction**. |
| Shipping method on checkout | **Partial** | Method id + country/postal on session; eligibility via `shipping_methods` rules (channels, postal prefixes, min/max order). |
| Stock reservation per line | **Gap** | Saleor allocates/reserves stock through checkout; rewrite validates listings/prices on upsert but does not mirror full reservation lifecycle. |
| Multiple shipping / split deliveries | **Gap** | Single shipping context on session. |
| Gift cards | **Gap** | Not modeled in rewrite checkout/pricing paths. |
| Checkout “problems” / errors collection | **Gap** | No Saleor-style aggregated checkout error list API. |
| Complete → order | **Partial** | Requires authorized payment coverage vs `checkout_id`, shipping context when lines exist; creates order via checkout completion path. |

## Orders

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Order create (direct) | **Partial** | `POST /orders` with idempotency; pricing calculator can adjust totals; voucher insert path. |
| Status transitions | **Partial** | Optimistic concurrency (`expected_updated_at`); cancel restocks (where implemented in repo). |
| Order lines from checkout snapshot | **Partial** | Depends on completion flow vs standalone order create—narrower than Saleor order editing. |
| Draft orders / unlimited order edits | **Gap** | Saleor draft orders and rich mutations not replicated. |
| Refunds at order level (integrated) | **Partial** | Payments module handles capture/refund; order–payment orchestration depth may lag Saleor. |

## Catalog & merchandising

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Products & variants CRUD | **Partial** | REST CRUD; SKU/name/price-centric vs Saleor attributes/product types/tax classes per variant. |
| Categories & collections | **Partial** | List/save + collection–product link endpoint. |
| Product media | **Partial** | Media CRUD under product. |
| Attributes & product types | **Gap** | No Saleor-style attribute system in API. |
| Publishing / availability rules | **Partial** | Channel listings for product/variant; not full Saleor availability engine. |

## Channels & sites

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Channel CRUD | **Partial** | Channels + product/variant listings; active flag enforced on checkout/catalog paths. |
| Per-channel pricing (listings) | **Partial** | Listing price/currency/publish state; differs from Saleor’s full channel listing matrix. |
| Sites / storefront config | **Gap** | Saleor `Site`/`Shop` settings not mirrored as a first-class module. |

## Customers & identity

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Customers list/save | **Partial** | Basic customer records; no rich account API parity. |
| Addresses | **Gap** | Checkout carries country/postal fields; no full `Address` CRUD like Saleor account. |
| Auth (login / refresh / logout) | **Partial** | Identity module: sessions, revoke, device binding options—overlap with Saleor JWT flows but not identical. |
| Staff / permissions model | **Partial** | Policy middleware + DB permissions; differs from Saleor’s dashboard permission graph. |

## Inventory & warehouses

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Stock items list/save | **Partial** | Simple stock rows; not full warehouse graph, allocations, or reservations UI. |
| Multi-warehouse allocation | **Gap** | No Saleor `Warehouse`/`Allocation` parity in API. |

## Pricing, tax, promotions

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Tax classes & rates | **Partial** | CRUD + calculator input hooks on checkout/order. |
| `POST /pricing/calculate` | **Partial** | Central calculator; plugins (Avalara, etc.) not built-in. |
| Promotions & vouchers | **Partial** | Rules + vouchers modules; breadth of Saleor discount engine not guaranteed. |

## Shipping

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Zones & methods | **Partial** | CRUD + `resolve`; JSONB rules for countries, channels, postal prefixes, order bounds. |
| Weight / item-based rates | **Gap** | Flat `price_cents` on method; no weight tables. |
| Delivery time / metadata | **Gap** | Minimal method fields vs Saleor. |

## Payments

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Payment CRUD, transactions | **Partial** | Authorize/capture/refund/void + webhooks path in OpenAPI. |
| Reconciliation worker | **Partial** | Background reconciliation wired in `app.go`. |
| Payment apps / plugins | **Gap** | Saleor’s plugin ecosystem not replicated—single integration style per implementation. |

## Apps & webhooks

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Apps CRUD + activate/deactivate | **Partial** | Deactivate cascades to subscription delivery behavior (inactive apps excluded). |
| Webhook subscriptions & deliveries | **Partial** | Deliveries list, outbox retry; retry skips already-succeeded targets. |
| Async event fan-out | **Partial** | Outbox + worker; tuning and event catalog may be narrower than Saleor. |

## Localization, metadata, search

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Translations | **Partial** | `translations` module—entity coverage may be limited vs Saleor translatable fields. |
| Metadata (public/private) | **Partial** | Generic metadata module. |
| Search index & query | **Partial** | Index + search endpoints; ranking/features vs Saleor search differ. |

## Regions & brands

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Regions / brands CRUD | **Partial** | Baseline config modules; not full Saleor shop/country configuration. |

## Fulfillments

| Capability | Status | Rewrite notes |
| --- | --- | --- |
| Fulfillment records | **Partial** | `GET/POST /fulfillments` in OpenAPI; create uses **one transaction** (order `FOR UPDATE`, insert fulfillment, status transitions + audit, idempotency save). |
| Idempotent create | **Partial** | `Idempotency-Key` required; scope `fulfillments.create`. Replays return stored fulfillment without re-emitting `order.completed`. |
| Fulfillment lines / refunds tie-in | **Gap** | Narrow model vs Saleor fulfillment lines and integrated refunds. |

---

## How to use this matrix

1. Pick a **Gap** or **Partial** row for the next tranche.
2. Confirm **REST** acceptance (no GraphQL) and update **OpenAPI** when adding routes.
3. After shipping a tranche, update the row status and the “Last updated” note at the top.
