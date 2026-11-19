#!/usr/bin/env bash
# Project-local MariaDB for KD Helpdesk development.
# Data lives in ./local/mariadb (git-ignored); nothing is installed as a system service.
#
#   dev/db.sh start    start MariaDB on 127.0.0.1:3307 (creates it on first run)
#   dev/db.sh stop     stop it
#   dev/db.sh status   show whether it is running
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MARIADB_BIN="${MARIADB_BIN:-/opt/homebrew/opt/mariadb@10.11/bin}"
CNF="$ROOT/dev/my.cnf"
DATA="$ROOT/local/mariadb"
PIDFILE="$ROOT/local/mariadb.pid"
LOG="$ROOT/local/mariadb.log"
ENVFILE="$ROOT/local/db.env"
SOCK="/tmp/kd-helpdesk-mariadb.sock"

running() { [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; }

init_db() {
  echo "Creating project-local MariaDB data directory..."
  mkdir -p "$DATA"
  "$MARIADB_BIN/mariadb-install-db" --defaults-file="$CNF" --datadir="$DATA" \
    --auth-root-authentication-method=normal --skip-test-db >/dev/null
  # Random local-only root password, stored in a git-ignored file
  echo "DB_ROOT_PASSWORD=$(openssl rand -hex 16)" > "$ENVFILE"
  chmod 600 "$ENVFILE"
}

start() {
  if running; then echo "MariaDB already running (pid $(cat "$PIDFILE"))"; return; fi
  local fresh=0
  [ -d "$DATA/mysql" ] || { init_db; fresh=1; }
  "$MARIADB_BIN/mariadbd" --defaults-file="$CNF" --datadir="$DATA" \
    --pid-file="$PIDFILE" --log-error="$LOG" >/dev/null 2>&1 &
  for _ in $(seq 1 30); do
    # ping exits 0 whenever the server is up, even if auth is refused
    "$MARIADB_BIN/mariadb-admin" --socket="$SOCK" -uroot ping >/dev/null 2>&1 && break
    sleep 1
  done
  if [ "$fresh" = 1 ]; then
    "$MARIADB_BIN/mariadb" --socket="$SOCK" -uroot \
      -e "ALTER USER 'root'@'localhost' IDENTIFIED BY '$(pw)'; CREATE USER IF NOT EXISTS 'root'@'127.0.0.1' IDENTIFIED BY '$(pw)'; GRANT ALL PRIVILEGES ON *.* TO 'root'@'127.0.0.1' WITH GRANT OPTION; FLUSH PRIVILEGES;"
  fi
  echo "MariaDB running on 127.0.0.1:3307 (pid $(cat "$PIDFILE"))"
}

pw() { grep DB_ROOT_PASSWORD "$ENVFILE" | cut -d= -f2; }

stop() {
  if running; then kill "$(cat "$PIDFILE")"; echo "MariaDB stopped"; else echo "MariaDB not running"; fi
}

case "${1:-}" in
  start) start ;;
  stop) stop ;;
  status) running && echo "running (pid $(cat "$PIDFILE"))" || echo "stopped" ;;
  *) echo "usage: dev/db.sh start|stop|status"; exit 1 ;;
esac
