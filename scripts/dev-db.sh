#!/usr/bin/env bash
# Start (or stop) a throwaway MySQL instance for trying NavMini.
# Requires mysqld/mysql/mysqladmin on PATH (Homebrew: brew install mysql).
set -euo pipefail
cd "$(dirname "$0")/.."

DATA_DIR="${NAVMINI_DB_DIR:-/tmp/navmini-mysql}"
PORT="${NAVMINI_DB_PORT:-13306}"
SOCKET="$DATA_DIR/mysql.sock"
BASE_DIR="$(dirname "$(command -v mysqld)")/.."
CMD="${1:-start}"

mysql_admin() { mysqladmin -uroot -h127.0.0.1 -P"$PORT" "$@"; }
mysql_root()  { mysql -uroot -h127.0.0.1 -P"$PORT" "$@"; }

case "$CMD" in
  start)
    if mysql_admin ping >/dev/null 2>&1; then
      echo "MySQL already running on 127.0.0.1:$PORT"
    else
      mkdir -p "$DATA_DIR"
      if [ ! -d "$DATA_DIR/data" ]; then
        echo "Initializing $DATA_DIR/data ..."
        mysqld --initialize-insecure --datadir="$DATA_DIR/data" --basedir="$BASE_DIR" \
          --log-error="$DATA_DIR/init.log"
      fi
      echo "Starting mysqld on port $PORT ..."
      nohup mysqld --datadir="$DATA_DIR/data" --basedir="$BASE_DIR" --port="$PORT" \
        --socket="$SOCKET" --pid-file="$DATA_DIR/mysql.pid" \
        --log-error="$DATA_DIR/server.log" --bind-address=127.0.0.1 --mysqlx=OFF \
        >"$DATA_DIR/stdout.log" 2>&1 &
      for _ in $(seq 1 40); do
        mysql_admin ping >/dev/null 2>&1 && break
        sleep 0.5
      done
      if ! mysql_admin ping >/dev/null 2>&1; then
        echo "mysqld failed to start; see $DATA_DIR/server.log" >&2
        exit 1
      fi
    fi
    if ! mysql_root -N -B -e "SHOW DATABASES LIKE 'demo_shop'" | grep -q demo_shop; then
      echo "Seeding demo databases ..."
      mysql_root < scripts/seed.sql
    fi
    cat <<INFO

Demo database ready:
  host     127.0.0.1
  port     $PORT
  user     navmini
  password navmini_pw
  database demo_shop / analytics
(also available as root without a password, for local development only)

Start NavMini:
  ./bin/navmini             # browser UI
  open dist/NavMini.app     # native window (after ./scripts/build-app.sh)
INFO
    ;;
  stop)
    mysql_admin shutdown 2>/dev/null || true
    echo "stopped"
    ;;
  reset)
    mysql_admin shutdown 2>/dev/null || true
    sleep 1
    rm -rf "$DATA_DIR"
    echo "removed $DATA_DIR"
    ;;
  *)
    echo "usage: $0 {start|stop|reset}" >&2
    exit 2
    ;;
esac
