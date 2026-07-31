#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

# --- LOGGING ---
# Mirror everything printed to the terminal into a timestamped log file so a
# run can be reviewed or attached to a bug report after the fact.
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
LOG_DIR="$SCRIPT_DIR/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/deploy-$(date +%Y%m%d-%H%M%S).log"
exec > >(tee -a "$LOG_FILE") 2>&1
echo "Logging output to $LOG_FILE"

# --- COLOR DEFINITIONS ---
NC='\033[0m'
BOLD='\033[1m'
DIM='\033[2m'
CYAN='\033[0;36m'
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
MAGENTA='\033[0;35m'
WHITE='\033[1;37m'

# --- LOGGING FUNCTION ---
log() {
  local level="$1"
  local message="$2"
  case "$level" in
    "info")    echo -e "  ${CYAN}${BOLD}>${NC} ${BOLD}${message}${NC}" ;;
    "success") echo -e "  ${GREEN}${BOLD}\xE2\x9C\x94${NC} ${GREEN}${BOLD}${message}${NC}" ;;
    "warn")    echo -e "  ${YELLOW}${BOLD}!${NC} ${YELLOW}${message}${NC}" ;;
    "error")   echo -e "  ${RED}${BOLD}\xE2\x9C\x98 ERROR:${NC} ${RED}${BOLD}${message}${NC}" ;;
  esac
}

# --- SECTION BANNER ---
# Prints a styled header for each deployment phase
section() {
  local label="$1"
  echo ""
  echo -e "  ${DIM}${CYAN}\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80${NC}"
  echo -e "  ${MAGENTA}${BOLD}\xe2\x97\x86 ${label}${NC}"
  echo -e "  ${DIM}${CYAN}\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80${NC}"
}

# --- SPINNER ---
# Shows a spinning animation while waiting on a background process
spinner() {
  local pid="$1"
  local message="$2"
  local frames=('\xe2\xa0\x8b' '\xe2\xa0\x99' '\xe2\xa0\xb9' '\xe2\xa0\xb8' '\xe2\xa0\xbc' '\xe2\xa0\xb4' '\xe2\xa0\xa6' '\xe2\xa0\xa7' '\xe2\xa0\x87' '\xe2\xa0\x8f')
  local i=0

  while kill -0 "$pid" 2>/dev/null; do
    printf "\r  ${CYAN}${BOLD}%b${NC} ${DIM}%s${NC}" "${frames[$i]}" "$message"
    i=$(( (i + 1) % ${#frames[@]} ))
    sleep 0.1
  done
  # Clear the spinner line
  printf "\r\033[K"
}

# --- STARTUP BANNER ---
echo ""
echo -e "  ${CYAN}${BOLD}\xe2\x94\x8c\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x90${NC}"
echo -e "  ${CYAN}${BOLD}\xe2\x94\x82${NC}  ${WHITE}${BOLD}DEWEY ${NC}${DIM}Container Deployment${NC}  ${CYAN}${BOLD}\xe2\x94\x82${NC}"
echo -e "  ${CYAN}${BOLD}\xe2\x94\x94\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x98${NC}"

# --- ARGUMENT PARSING ---
# --reset-db wipes the mysql-data volume before this run. Default is to keep
# it, since podman named volumes are meant to survive pod recreation.
#
# --keep-data preserves the backend data volumes (dewey-store, dewey-cache,
# dewey-backup, dewey-logs) instead of the default behavior, which wipes them
# on every deploy. Default is to wipe: metrics like app_files_in_store read
# these directories live, so leftover files from a previous deployment made
# the dashboard look like nothing had reset between deploys.
RESET_DB=false
KEEP_DATA=false
KEEP_DATA_SET=false
for arg in "$@"; do
  case "$arg" in
    --reset-db) RESET_DB=true ;;
    --keep-data) KEEP_DATA=true; KEEP_DATA_SET=true ;;
  esac
done

# Prefer the machine's primary LAN IP so the frontend is reachable from other
# devices on the network, not just this host — falls back to localhost if
# none is found (e.g. an isolated CI runner). Computed early (rather than
# just before the summary, as before) so it can also be passed to the
# frontend container as ORIGIN below.
HOST_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
[ -z "$HOST_IP" ] && HOST_IP="localhost"

# Nothing bounded the pod's CPU/RAM before this (issue #168) — a burst of
# uploads, barcode scans, backup zipping, and Prometheus scrapes all at once
# could consume the whole host or get OOM-killed unpredictably instead of
# failing gracefully. Defaults are conservative; override for the actual
# target host's capacity, e.g.:
#   DEWEY_POD_CPUS=8 DEWEY_POD_MEMORY=8g ./deploy.sh
POD_CPUS="${DEWEY_POD_CPUS:-4}"
POD_MEMORY="${DEWEY_POD_MEMORY:-4g}"

# --- CLEANUP ---
section "Cleanup"
log "info" "Removing existing pod..."
podman pod rm -f dewey-pod 2>/dev/null || true
log "success" "Clean slate ready"

# --- POD CREATION ---
section "Pod Creation"
log "info" "Creating dewey-pod..."
log "info" "Resource limits: ${POD_CPUS} CPUs, ${POD_MEMORY} memory (override via DEWEY_POD_CPUS/DEWEY_POD_MEMORY)"
# Added 9090 here so Prometheus is accessible externally
podman pod create --infra=true \
  --cpus "$POD_CPUS" \
  --memory "$POD_MEMORY" \
  -p 8080:8080 \
  -p 3000:3000 \
  -p 3306:3306 \
  -p 9090:9090 \
  dewey-pod

# ---------------- MYSQL ----------------
section "MySQL"
log "info" "Deploying MySQL..."

if [ "$RESET_DB" = true ]; then
  log "warn" "Resetting mysql-data volume (--reset-db passed)..."
  podman volume inspect mysql-data &>/dev/null && podman volume rm mysql-data
else
  log "info" "Keeping existing mysql-data volume (pass --reset-db to wipe)"
fi

# Pinned rather than :latest (issue #207) — package.sh bundles whatever tag
# is actually running so offline deploys get the exact version this was
# tested against, instead of silently pulling a different one later.
podman run -d --pod dewey-pod \
  --name dewey-mysql \
  -e MYSQL_ROOT_PASSWORD=dewey \
  -e MYSQL_DATABASE=deweyRecords \
  -v mysql-data:/var/lib/mysql:Z \
  docker.io/library/mysql:9.7.0

# Critical: database must be ready before the backend starts since it depends on it
(while ! podman exec dewey-mysql mysqladmin ping -h localhost --silent 2>/dev/null; do
  sleep 3
done) &
WAIT_PID=$!
spinner "$WAIT_PID" "Waiting for database to accept connections..."
log "success" "Database is up!"

# ---------------- PROMETHEUS ----------------
section "Prometheus"
log "info" "Pulling Prometheus image..."
# Pinned rather than :latest — see the mysql image note above (issue #207).
podman pull docker.io/prom/prometheus:v3.13.1

log "info" "Starting Prometheus container..."
podman run -d --pod dewey-pod \
  --name dewey-prometheus \
  docker.io/prom/prometheus:v3.13.1

# Brief pause to let Prometheus spin up internal networking before healthcheck
sleep 2
log "info" "Checking Prometheus health..."
if curl -s --fail http://localhost:9090/-/healthy > /dev/null; then
  log "success" "Prometheus is healthy!"
else
  log "warn" "Prometheus health check did not return 200 OK immediately."
fi

# ---------------- BACKEND ----------------
section "Backend"

# Ask interactively unless --keep-data already answered the question, or
# there's no TTY to ask on (e.g. running from CI/automation) — in which
# case the default (wipe) stands, same as before this prompt existed.
if [ "$KEEP_DATA_SET" = false ] && [ -t 0 ]; then
  read -r -p "  Wipe store/cache/backup/logs volumes before this deploy? [Y/n] " wipe_answer
  case "$wipe_answer" in
    [nN]*) KEEP_DATA=true ;;
    *) KEEP_DATA=false ;;
  esac
fi

if [ "$KEEP_DATA" = true ]; then
  log "info" "Keeping existing store/cache/backup/logs volumes"
else
  log "warn" "Resetting store/cache/backup/logs volumes (default; pass --keep-data to preserve)..."
  for vol in dewey-store dewey-cache dewey-backup dewey-logs; do
    podman volume inspect "$vol" &>/dev/null && podman volume rm "$vol"
  done
fi

log "info" "Building backend image ${DIM}(cross-doc-tool-dev)${NC}..."
# Stamp the binary with the branch it's being deployed from (issue #66) so
# it's visible via GET /version without needing to shell into the container.
GIT_BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
podman build \
  --build-arg CGO_CFLAGS="-Wno-discarded-qualifiers" \
  --build-arg GIT_BRANCH="$GIT_BRANCH" \
  -t cross-doc-tool-dev ./backend

log "info" "Starting backend container..."
# Named volumes for store/cache/backup/logs. By default these are wiped above
# on every deploy; pass --keep-data to let them survive pod recreation instead,
# the same way mysql-data does for the database.
podman run -d --pod dewey-pod --name cross-doc-tool-dev \
  -v dewey-store:/app/store:Z \
  -v dewey-cache:/app/cache:Z \
  -v dewey-backup:/app/backup:Z \
  -v dewey-logs:/app/logs:Z \
  cross-doc-tool-dev
log "success" "Backend running"

# ---------------- FRONTEND ----------------
section "Frontend"
log "info" "Building frontend image ${DIM}(admin-portal)${NC}..."
podman build -t admin-portal ./frontend/doctooladmin

log "info" "Starting Svelte frontend container..."
# adapter-node defaults BODY_SIZE_LIMIT to 512K; raise it to match backend's
# maxFileSize (consts.go) so uploads aren't killed before reaching +server.ts.
# ORIGIN tells SvelteKit's CSRF check (checkOrigin) what host to trust — without
# it, adapter-node rejects multipart uploads whose Origin header doesn't match
# what the server expects, which is what every client hits by default since
# ORIGIN is unset otherwise (issue #197).
podman run -d --pod dewey-pod --name svelte-container \
  -e BODY_SIZE_LIMIT=52428800 \
  -e ORIGIN="http://${HOST_IP}:3000" \
  admin-portal
log "success" "Frontend running"

# ---------------- STATUS SUMMARY ----------------
section "Status"
echo ""
echo -e "  ${GREEN}${BOLD}\xe2\x96\x88\xe2\x96\x88\xe2\x96\x88 All components deployed to dewey-pod ${GREEN}\xe2\x96\x88\xe2\x96\x88\xe2\x96\x88${NC}"
echo ""
# podman --format doesn't interpret ANSI codes, so style the output after the fact
podman ps --format "{{.Names}}\t{{.Status}}" --filter pod=dewey-pod \
  | while IFS=$'\t' read -r name status; do
    echo -e "  ${DIM}\xe2\x94\x82${NC} ${BOLD}${name}${NC}\t${DIM}${status}${NC}"
  done
echo ""

# ---------------- POST-DEPLOYMENT INFO ----------------
section "Post-Deployment Info"

echo ""
log "info" "Host IP: ${BOLD}${HOST_IP}${NC}"
echo ""
echo -e "  ${DIM}\xe2\x94\x82${NC} Frontend      http://${HOST_IP}:3000"
echo -e "  ${DIM}\xe2\x94\x82${NC} Backend API   http://${HOST_IP}:8080"
echo -e "  ${DIM}\xe2\x94\x82${NC} Prometheus    http://${HOST_IP}:9090"
echo -e "  ${DIM}\xe2\x94\x82${NC} MySQL         ${HOST_IP}:3306"
echo ""

# --- HEALTH CHECKS ---
# check_health reports whether a service answered at all rather than
# requiring a 200 — the backend's routes other than /metrics sit behind the
# known_machines IP allowlist, so a 403 there still proves the service is up.
check_health() {
  local label="$1"
  local url="$2"
  local code
  code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 "$url" 2>/dev/null || true)
  if [ -n "$code" ] && [ "$code" != "000" ]; then
    log "success" "${label} is responding (HTTP ${code})"
  else
    log "warn" "${label} did not respond"
  fi
}

# Backend's /metrics is registered directly on the gin engine, outside the
# logConnections IP-allowlist group, so it's reachable without first
# registering this host in known_machines.
check_health "Frontend"   "http://localhost:3000"
check_health "Backend"    "http://localhost:8080/metrics"
check_health "Prometheus" "http://localhost:9090/-/healthy"

# --- KEY METRICS SNAPSHOT ---
echo ""
log "info" "Backend metrics snapshot:"
BACKEND_METRICS="$(curl -s --max-time 5 http://localhost:8080/metrics 2>/dev/null || true)"
if [ -n "$BACKEND_METRICS" ]; then
  for metric in app_uptime_seconds app_ram_usage app_heap_usage app_files_in_store app_files_in_backup app_db_errors; do
    value=$(echo "$BACKEND_METRICS" | awk -v m="$metric" '$1 == m {print $2}')
    [ -n "$value" ] && echo -e "  ${DIM}\xe2\x94\x82${NC} ${BOLD}${metric}${NC}\t${value}"
  done
else
  log "warn" "Could not reach backend /metrics for a snapshot"
fi

# --- NEXT STEPS ---
echo ""
log "info" "Next steps:"
echo -e "  ${DIM}\xe2\x94\x82${NC} List running containers:      ${CYAN}podman ps --pod${NC}"
echo -e "  ${DIM}\xe2\x94\x82${NC} Watch backend logs (BITs):    ${CYAN}podman logs -f cross-doc-tool-dev${NC}"
echo -e "  ${DIM}\xe2\x94\x82${NC} Attach to backend container:  ${CYAN}podman attach cross-doc-tool-dev${NC}"
echo ""
