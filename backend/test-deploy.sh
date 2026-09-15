#!/bin/bash
# Spins up a throwaway, local-only MySQL container and runs the backend
# against it — for fast Go iteration (go build/run speed, not a
# container rebuild) without touching the real deploy.sh-based stack.
# Runs from its own scratch directory with its own config.json on
# BACKEND_PORT (default 18080, override by exporting it yourself) rather
# than the tracked backend/config.json, so it can listen on a different
# port than deploy.sh's real backend and run alongside it. Data doesn't
# persist across runs. Safe to run repeatedly — reuses the MySQL
# container if it's already there.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

CONTAINER_NAME="dewey-test-mysql"
MYSQL_PORT=13306
MYSQL_ROOT_PASSWORD="testpass"
MYSQL_DATABASE="dewey_test"
BACKEND_PORT="${BACKEND_PORT:-18080}"

if podman ps --filter "name=^${CONTAINER_NAME}\$" --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}\$"; then
  echo "${CONTAINER_NAME} already running"
elif podman ps -a --filter "name=^${CONTAINER_NAME}\$" --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}\$"; then
  echo "Starting existing ${CONTAINER_NAME} container..."
  podman start "$CONTAINER_NAME"
else
  echo "Creating ${CONTAINER_NAME} container..."
  podman run -d --name "$CONTAINER_NAME" \
    -e MYSQL_ROOT_PASSWORD="$MYSQL_ROOT_PASSWORD" \
    -e MYSQL_DATABASE="$MYSQL_DATABASE" \
    -p "${MYSQL_PORT}:3306" \
    docker.io/library/mysql:8
fi

echo "Waiting for MySQL to be ready..."
# The official mysql image briefly starts an internal bootstrap server (to
# run its init scripts) that answers mysqladmin ping, then shuts it down
# and starts the real one — a single successful ping can catch that
# bootstrap instance and report ready right before the connection resets.
# Requiring two consecutive successful pings skips over that gap: the
# bootstrap's shutdown causes at least one failed ping in between, so two
# in a row only happens once the real server is actually up and staying up.
consecutive_ok=0
for i in $(seq 1 60); do
  if podman exec "$CONTAINER_NAME" mysqladmin ping -uroot -p"$MYSQL_ROOT_PASSWORD" --silent 2>/dev/null; then
    consecutive_ok=$((consecutive_ok + 1))
    if [ "$consecutive_ok" -ge 2 ]; then
      echo "MySQL ready"
      break
    fi
  else
    consecutive_ok=0
  fi
  sleep 1
done

# Same backend/.env convention as dev.sh, for FILES_PASSWORD/
# MACHINES_PASSWORD overrides — DB_DSN below always points at the
# throwaway container above regardless of what .env says, since that's
# the whole point of this script.
ENV_FILE="$SCRIPT_DIR/.env"
if [ -f "$ENV_FILE" ]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

export DB_DSN="root:${MYSQL_ROOT_PASSWORD}@tcp(127.0.0.1:${MYSQL_PORT})/${MYSQL_DATABASE}?parseTime=true"
export FILES_PASSWORD="${FILES_PASSWORD:-testpw}"
export MACHINES_PASSWORD="${MACHINES_PASSWORD:-testpw}"

SCRATCH_DIR="$(mktemp -d)"
trap 'rm -rf "$SCRATCH_DIR"' EXIT
mkdir -p "$SCRATCH_DIR"/{cache,store,backup,plugins,plugin-scratch}
cp plugins/*.lua "$SCRATCH_DIR/plugins/" 2>/dev/null || true

# BITs (main.go) now passes its own port explicitly, so it's safe to run
# here too — it'll test this scratch instance, not the real deploy.sh
# backend on :8080.
cp -r testing_tooling "$SCRATCH_DIR/testing_tooling"

cat > "$SCRATCH_DIR/config.json" <<EOF
{
	"file_system": {
		"upload_dir": "./cache",
		"file_system_base_dir": "./store",
		"backup_dir": "./backup",
		"daemon_tick_time_minutes": 60,
		"alpha": 1,
		"beta": 1,
		"t_base": 5,
		"tick_max": 500,
		"tick_min": 1,
		"backup_interval_minutes": 6000
	},
	"server": {
		"max_file_size_bytes": 52428800,
		"port_number": "${BACKEND_PORT}",
		"app_version": "test"
	},
	"rate_limit": {
		"requests_per_second": 80,
		"burst": 120
	},
	"database": {
		"max_open_connections": 10,
		"max_idle_connections": 10,
		"connection_timeout_multiplier_minutes": 2
	},
	"plugin": {
		"plugin_dir": "./plugins",
		"plugin_scratch_dir": "./plugin-scratch",
		"http_timeout_multiplier_seconds": 5,
		"hook_timeout_multiplier_seconds": 30,
		"max_plugin_file_size_bytes": 52428800,
		"max_plugin_download_size_bytes": 52428800
	}
}
EOF

echo "Building backend..."
go build -o "$SCRATCH_DIR/dewey-backend" .

echo "Starting backend on :${BACKEND_PORT} ..."
cd "$SCRATCH_DIR"
exec ./dewey-backend
