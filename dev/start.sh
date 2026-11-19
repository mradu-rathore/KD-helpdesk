#!/usr/bin/env bash
# Start KD Helpdesk locally: project MariaDB + Frappe (web, workers, redis, socketio).
# Open http://helpdesk.localhost:8000/helpdesk when it's up. Ctrl+C stops Frappe;
# run `dev/db.sh stop` to stop the database too.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export PATH="$ROOT/local/bench-venv/bin:$PATH"

"$ROOT/dev/db.sh" start
cd "$ROOT/local/frappe-bench"

# Refuse to start a second copy (its Redis/web ports would clash)
for port in 8000 9000 11000 13000; do
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "Port $port is already in use - KD Helpdesk (or something else) is already running."
    echo "Stop the other copy first (Ctrl+C in its terminal), then run dev/start.sh again."
    exit 1
  fi
done

exec bench start
