# Saleor to Rewrite Mapping

For a **parity status matrix** (full / partial / gap / REST divergence), see [`parity_gap_matrix.md`](parity_gap_matrix.md).

This document maps Saleor domain behavior to the Go rewrite modules and REST surface.
Saleor references were taken from the Python/Django codebase under `saleor/saleor`.

## Domain Mapping

| Saleor area | Saleor files | Rewrite module | Primary REST surface |
| --- | --- | --- | --- |
| Product/Catalog | `product/models.py`, `graphql/product/schema.py`, `graphql/product/mutations` | `internal/modules/catalog` | `/products` |
| Checkout/Orders | `checkout/models.py`, `checkout/complete_checkout.py`, `order/models.py`, `order/actions.py` | `internal/modules/orders` | `/orders` |
| Customers/Accounts | `account/models.py`, `graphql/account/schema.py` | `internal/modules/customers` | `/customers` |
| Inventory/Warehousing | `warehouse/models.py`, `warehouse/management.py`, `warehouse/availability.py` | `internal/modules/inventory` | `/inventory` |
| Pricing/Tax/Channels | `tax/models.py`, `tax/utils.py`, `channel/models.py` | `internal/modules/pricing` | `/pricing` |
| Promotions/Discounts | `discount/models.py`, `discount/tasks.py`, `graphql/discount/schema.py` | `internal/modules/promotions` | `/promotions` |
| Localization/Regions | `graphql/translations/schema.py`, `shipping/models.py`, `graphql/shop/types.py` | `internal/modules/regions` | `/regions` |
| Multi-brand controls | `channel/models.py`, `graphql/channel/schema.py` | `internal/modules/brands` | `/brands` |
| Payments (next milestone) | `payment/models.py`, `payment/utils.py`, `graphql/payment/schema.py` | future `internal/modules/payments` | `/payments` |

## Model Mapping

| Saleor model/entity | Rewrite model/table target |
| --- | --- |
| `Product`, `ProductVariant`, `ProductChannelListing` | `products` + `product_variants` + regional/tenant price rows |
| `Checkout`, `CheckoutLine`, `Order`, `OrderLine` | `orders` + `order_lines` |
| `User`, `Address`, customer metadata | `customers` + `customer_addresses` |
| `Warehouse`, `Stock`, `Allocation`, `Reservation` | `warehouses` + `stock_items` + `inventory_reservations` |
| `TaxClass`, tax configuration | `tax_classes` + `tax_rates` |
| `Promotion`, `PromotionRule`, `Voucher` | `promotions` + `promotion_rules` + `coupons` |
| Language/country/currency config | `regions` + `brands` and locale/currency attributes |

All core tables in rewrite include:
- `tenant_id`
- `region_id`
- `created_at`
- `updated_at`

## Flow Mapping (GraphQL to REST + Services)

| Saleor behavior | Saleor entry points | Rewrite endpoint | Rewrite service intent |
| --- | --- | --- | --- |
| Product create/update/list | `product_create`, `product_update` mutations | `POST /products`, `GET /products` | `catalog.Service.Save/List` |
| Checkout completion to order | `complete_checkout`, `create_order_from_checkout` | `POST /orders` | `orders.Service.Create` |
| Order status transition | order actions/mutations | `PATCH /orders` | `orders.Service.UpdateStatus` |
| Customer profile management | account mutations | `POST /customers`, `GET /customers` | `customers.Service.Save/List` |
| Inventory adjustments | stock mutations/management | `POST /inventory`, `GET /inventory` | `inventory.Service.Save/List` |
| Price book updates | channel/tax/pricing calculations | `POST /pricing`, `GET /pricing` | `pricing.Service.Save/List` |
| Promotion lifecycle | promotion/voucher mutations | `POST /promotions`, `GET /promotions` | `promotions.Service.Save/List` |

## Event Mapping

Saleor plugin/app event behaviors are mapped to internal events plus async webhooks:

- `order.created`
- `order.completed`
- `product.updated`
- `inventory.changed`

The rewrite bus lives in `internal/shared/events` and decouples modules from webhook delivery.
