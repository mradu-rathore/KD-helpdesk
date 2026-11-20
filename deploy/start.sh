#!/bin/sh
# Container entrypoint: set up / upgrade the database, then run Libredesk.
set -e

# Render (and most PaaS) tell the app which port to listen on via $PORT
export LIBREDESK_APP__SERVER__ADDRESS="0.0.0.0:${PORT:-9000}"

for var in LIBREDESK_APP__ENCRYPTION_KEY LIBREDESK_DB__HOST; do
  eval "val=\${$var:-}"
  if [ -z "$val" ]; then echo "Missing required env var: $var" >&2; exit 1; fi
done

# First boot creates the schema and the System admin user
# (password from LIBREDESK_SYSTEM_USER_PASSWORD); later boots skip it.
./libredesk --install --idempotent-install --yes --config config.toml
./libredesk --upgrade --yes --config config.toml

# KD: load Zendesk-style views + sample data once (free hosting has no shell to do it by hand)
if [ "${KD_LOAD_SAMPLE_DATA:-false}" = "true" ]; then
  export PGHOST="$LIBREDESK_DB__HOST" PGPORT="${LIBREDESK_DB__PORT:-5432}" \
    PGUSER="$LIBREDESK_DB__USER" PGPASSWORD="$LIBREDESK_DB__PASSWORD" PGDATABASE="$LIBREDESK_DB__DATABASE"
  loaded=$(psql -tAc "SELECT count(*) FROM views WHERE name = 'All tickets' AND visibility = 'all'" || echo 0)
  if [ "$loaded" = "0" ]; then
    echo "Loading KD views and sample data..."
    psql -v ON_ERROR_STOP=1 -q -f seed.sql
  fi
  # Earlier sample data left the Support team without a timezone, which breaks
  # Libredesk's team loading; give it KD's
  psql -q -c "UPDATE teams SET timezone = 'Europe/Dublin' WHERE timezone IS NULL" || true
  # Sample customers (added later than the tickets, so checked on their own)
  customers=$(psql -tAc "SELECT count(*) FROM kd_customers" || echo 1)
  if [ "$customers" = "0" ]; then
    echo "Loading KD sample customers..."
    psql -v ON_ERROR_STOP=1 -q -f seed-customers.sql
  fi
fi

exec ./libredesk --config config.toml
