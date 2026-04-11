#!/usr/bin/env bash
# Apply baseline schema then incremental migrations (v1.0.0+ production bootstrap).
# Requires: psql client, DATABASE_URL (postgres://...), run from repository root.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "error: DATABASE_URL is not set" >&2
  exit 1
fi

echo "Applying internal/shared/db/schema.sql ..."
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f internal/shared/db/schema.sql

for f in internal/shared/db/migrations/*.sql; do
  [[ -f "$f" ]] || continue
  echo "Applying $f ..."
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$f"
done

echo "Database provisioning complete."
