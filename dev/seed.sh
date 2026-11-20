#!/usr/bin/env bash
# Load KD sample data (fictional agents, customers and tickets) into the local database.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
source "$ROOT/local/secrets.env"
PG_BIN="${PG_BIN:-/opt/homebrew/opt/postgresql@16/bin}"
PGPASSWORD="$DB_PASSWORD" "$PG_BIN/psql" -h 127.0.0.1 -p 5434 -U libredesk -d libredesk \
  -v ON_ERROR_STOP=1 -q -f "$ROOT/dev/seed.sql" -f "$ROOT/dev/seed-customers.sql"
echo "Sample data loaded."
