# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] — 2026-04-11

### Added

- First stable release of the **Rewrite Commerce Engine**: OpenAPI-first, multi-tenant Go monolith.
- REST APIs for catalog (products, variants, attributes, channels, listings), checkout (sessions, lines, recalculate, complete, validation, gift cards), orders, payments, fulfillments, shipping, customers, identity, webhooks, apps, pricing, promotions, inventory (including warehouses), shop settings, gift card issuance, order invoices, and more.
- PostgreSQL schema baseline (`internal/shared/db/schema.sql`) plus ordered migrations under `internal/shared/db/migrations/`.
- Database bootstrap script: `scripts/apply-migrations.sh` (and PowerShell twin `scripts/apply-migrations.ps1`).
- Production and usage documentation: `docs/DEPLOYMENT.md`, `docs/USAGE.md`.
- Binary version reporting via `VERSION` file and `go build -ldflags "-X main.version=..."` (see `Dockerfile`).

### Notes

- Configure `API_AUTH_TOKEN`, `DATABASE_URL`, and JWT secrets before exposing any environment beyond local development.
- Sensitive route families require `X-User-JWT` and database-backed permissions (see `docs/USAGE.md`).

