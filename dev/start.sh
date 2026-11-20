#!/usr/bin/env bash
# Start KD Helpdesk locally at http://localhost:9000 (Ctrl+C to stop).
# Postgres/Redis keep running in the background; stop them with dev/services.sh stop.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

if lsof -nP -iTCP:9000 -sTCP:LISTEN >/dev/null 2>&1; then
  echo "Port 9000 is already in use - KD Helpdesk (or something else) is already running."
  exit 1
fi

"$ROOT/dev/services.sh" start
source "$ROOT/dev/env.sh"
cd "$LIBREDESK_DIR"
./libredesk --upgrade --yes --config "$LIBREDESK_CONFIG"
exec ./libredesk --config "$LIBREDESK_CONFIG"
