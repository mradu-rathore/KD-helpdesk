#!/usr/bin/env bash
# Start KD Helpdesk locally: project MariaDB + Frappe (web, workers, redis, socketio).
# Open http://helpdesk.localhost:8000/helpdesk when it's up. Ctrl+C stops Frappe;
# run `dev/db.sh stop` to stop the database too.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export PATH="$ROOT/local/bench-venv/bin:$PATH"

"$ROOT/dev/db.sh" start
cd "$ROOT/local/frappe-bench"
exec bench start
