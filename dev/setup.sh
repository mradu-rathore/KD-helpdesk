#!/usr/bin/env bash
# One-time local setup for KD Helpdesk on macOS (no Docker).
# Everything it creates lives in ./local (git-ignored) plus two git-ignored symlinks.
#
# Needs: Homebrew, python3.11, node 18+, yarn 1.x, redis-server, uv
#   brew install mariadb@10.11 redis uv
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LOCAL="$ROOT/local"
BENCH="$LOCAL/frappe-bench"
SITE="helpdesk.localhost"
mkdir -p "$LOCAL"

# 1. Project-local MariaDB on 127.0.0.1:3307
"$ROOT/dev/db.sh" start
source "$LOCAL/db.env"

# 2. bench CLI in a project-local virtualenv
if [ ! -x "$LOCAL/bench-venv/bin/bench" ]; then
  uv venv -q --python 3.11 "$LOCAL/bench-venv"
  VIRTUAL_ENV="$LOCAL/bench-venv" uv pip install -q frappe-bench
fi
export PATH="$LOCAL/bench-venv/bin:$PATH"

# 3. Frappe Framework v15 (downloaded, not committed)
if [ ! -d "$BENCH/apps/frappe" ]; then
  (cd "$LOCAL" && bench init frappe-bench --frappe-branch version-15 \
    --python "$(command -v python3.11)" --skip-assets --no-backups)
fi
cd "$BENCH"
bench set-config -g db_host 127.0.0.1
bench set-config -g -p db_port 3307
[ -d apps/telephony ] || bench get-app telephony   # required by Helpdesk

# 4. Use the Helpdesk code from this repo (apps/helpdesk) instead of a download
ln -sfn ../../../apps/helpdesk apps/helpdesk
./env/bin/python -m pip install -q -e apps/helpdesk
printf "frappe\ntelephony\nhelpdesk\n" > sites/apps.txt
# Helpdesk's UI build expects to sit inside a bench: ../frappe/ui and ../../sites
ln -sfn ../local/frappe-bench/apps/frappe "$ROOT/apps/frappe"
ln -sfn local/frappe-bench/sites "$ROOT/sites"
(cd apps/frappe/ui && yarn install --silent)
(cd apps/helpdesk && yarn install --silent)

# 5. Create the site (random local admin password saved in local/site.env)
if [ ! -d "sites/$SITE" ]; then
  redis-server config/redis_cache.conf --daemonize yes
  redis-server config/redis_queue.conf --daemonize yes
  ADMIN_PW="$(openssl rand -hex 8)"
  echo "ADMIN_PASSWORD=$ADMIN_PW" > "$LOCAL/site.env"; chmod 600 "$LOCAL/site.env"
  bench new-site "$SITE" --db-root-username root --db-root-password "$DB_ROOT_PASSWORD" \
    --admin-password "$ADMIN_PW" --install-app telephony --install-app helpdesk
  for p in 11000 13000; do redis-cli -p $p shutdown nosave >/dev/null 2>&1 || true; done
fi
bench use "$SITE"
bench --site "$SITE" set-config developer_mode 1
bench --site "$SITE" set-config mute_emails 1
bench --site "$SITE" set-config server_script_enabled 1

# 6. Build frontend assets
bench build

# Undo files that installs/builds regenerate inside the vendored Helpdesk code
git -C "$ROOT" checkout -- apps/helpdesk/desk/auto-imports.d.ts apps/helpdesk/desk/yarn.lock 2>/dev/null || true

echo
echo "Done. Start with: dev/start.sh"
echo "Open http://$SITE:8000/helpdesk  (user: Administrator, password in local/site.env)"
