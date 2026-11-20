#!/usr/bin/env bash
# Project-local Postgres + Redis for KD Helpdesk (Libredesk) development.
# Everything lives in ./local (git-ignored); nothing is installed as a system service.
#
#   dev/services.sh start | stop | status
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LOCAL="$ROOT/local"
PG_BIN="${PG_BIN:-/opt/homebrew/opt/postgresql@16/bin}"
PGDATA="$LOCAL/postgres"
PG_PORT=5434
REDIS_PORT=6380
SECRETS="$LOCAL/secrets.env"
# macOS: Postgres refuses to start without a valid locale in the environment
export LC_ALL="${LC_ALL:-en_US.UTF-8}"

mkdir -p "$LOCAL"

ensure_secrets() {
  [ -f "$SECRETS" ] && return
  {
    echo "DB_PASSWORD=$(openssl rand -hex 16)"
    echo "ENCRYPTION_KEY=$(openssl rand -hex 16)"
    # Meets Libredesk's rule: upper, lower, number, special char
    echo "SYSTEM_USER_PASSWORD=Kd-$(openssl rand -hex 10)x!A9"
  } > "$SECRETS"
  chmod 600 "$SECRETS"
}

pg_running() { "$PG_BIN/pg_ctl" -D "$PGDATA" status >/dev/null 2>&1; }
redis_running() { redis-cli -p "$REDIS_PORT" ping >/dev/null 2>&1; }

start() {
  ensure_secrets
  source "$SECRETS"
  if [ ! -f "$PGDATA/PG_VERSION" ]; then
    echo "Creating project-local Postgres..."
    pwfile="$(mktemp)"; echo "$DB_PASSWORD" > "$pwfile"
    "$PG_BIN/initdb" -D "$PGDATA" -U libredesk --pwfile="$pwfile" \
      --auth=scram-sha-256 --encoding=UTF8 >/dev/null
    rm -f "$pwfile"
  fi
  if ! pg_running; then
    "$PG_BIN/pg_ctl" -D "$PGDATA" -l "$LOCAL/postgres.log" -w \
      -o "-p $PG_PORT -k '$LOCAL' -c listen_addresses=127.0.0.1" start >/dev/null
  fi
  PGPASSWORD="$DB_PASSWORD" "$PG_BIN/psql" -h 127.0.0.1 -p "$PG_PORT" -U libredesk -d postgres -tAc \
    "SELECT 1 FROM pg_database WHERE datname='libredesk'" | grep -q 1 \
    || PGPASSWORD="$DB_PASSWORD" "$PG_BIN/createdb" -h 127.0.0.1 -p "$PG_PORT" -U libredesk libredesk
  echo "Postgres running on 127.0.0.1:$PG_PORT"

  if ! redis_running; then
    redis-server --port "$REDIS_PORT" --bind 127.0.0.1 --dir "$LOCAL" \
      --pidfile "$LOCAL/redis.pid" --logfile "$LOCAL/redis.log" \
      --save "" --appendonly no --daemonize yes
  fi
  echo "Redis running on 127.0.0.1:$REDIS_PORT"
}

stop() {
  pg_running && "$PG_BIN/pg_ctl" -D "$PGDATA" -m fast stop >/dev/null && echo "Postgres stopped" || echo "Postgres not running"
  redis_running && redis-cli -p "$REDIS_PORT" shutdown nosave >/dev/null 2>&1; echo "Redis stopped"
}

status() {
  pg_running && echo "Postgres: running (127.0.0.1:$PG_PORT)" || echo "Postgres: stopped"
  redis_running && echo "Redis: running (127.0.0.1:$REDIS_PORT)" || echo "Redis: stopped"
}

case "${1:-}" in
  start) start ;;
  stop) stop ;;
  status) status ;;
  *) echo "usage: dev/services.sh start|stop|status"; exit 1 ;;
esac
