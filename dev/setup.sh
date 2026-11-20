#!/usr/bin/env bash
# One-time local setup for KD Helpdesk (Libredesk) on macOS.
# Needs: go (see libredesk/go.mod), node + pnpm, redis-server,
#        brew install postgresql@16
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

"$ROOT/dev/services.sh" start
source "$ROOT/dev/env.sh"
mkdir -p "$ROOT/local/uploads"
# Base config; secrets and connection details come from env vars (dev/env.sh)
[ -f "$LIBREDESK_CONFIG" ] || cp "$LIBREDESK_DIR/config.sample.toml" "$LIBREDESK_CONFIG"

"$ROOT/dev/build.sh"

cd "$LIBREDESK_DIR"
./libredesk --install --idempotent-install --yes --config "$LIBREDESK_CONFIG"
./libredesk --upgrade --yes --config "$LIBREDESK_CONFIG"

echo
echo "Done. Start with: dev/start.sh"
echo "Open http://localhost:9000  (email: System, password: SYSTEM_USER_PASSWORD in local/secrets.env)"
