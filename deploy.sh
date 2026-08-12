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
# dewey-backup, dewey-logs, dewey-plugin-scratch) instead of the default
# behavior, which wipes them on every deploy. Default is to wipe: metrics
# like app_files_in_store read these directories live, so leftover files
# from a previous deployment made the dashboard look like nothing had reset
# between deploys.
#
# --wipe-data explicitly opts into that wipe when there's no TTY to prompt
# on. Without it, a non-interactive run (cron, CI, SSH without -t) used to
# silently fall through to the wipe default with just a log line to show
# for it — since mysql-data survives by default but the store doesn't, that
# left MySQL's file-metadata pointing at documents no longer on disk
# (issue #206). See the TTY check below for the safe default this enables.
#
# --clean-slate is the deliberate, all-of-it version of the above: it forces
# both the mysql-data wipe (same as --reset-db) and the store/cache wipe
# (same as the default minus --keep-data), regardless of what those
# individual flags are also passed, since the whole point is an unambiguous
# full reset rather than something that depends on flag order (issue #296).
# Gated behind its own typed confirmation — see below — since the other
# flags' prompts aren't a strong enough gate for permanently destroying
# every stored file and its metadata in one shot.
RESET_DB=false
KEEP_DATA=false
KEEP_DATA_SET=false
WIPE_DATA_SET=false
CLEAN_SLATE=false
for arg in "$@"; do
  case "$arg" in
    --reset-db) RESET_DB=true ;;
    --keep-data) KEEP_DATA=true; KEEP_DATA_SET=true ;;
    --wipe-data) WIPE_DATA_SET=true ;;
    --clean-slate) CLEAN_SLATE=true ;;
  esac
done

if [ "$CLEAN_SLATE" = true ]; then
  # Applied after parsing (not inside the case arm above) so --clean-slate
  # always wins over --keep-data/--wipe-data no matter which order they're
  # passed in.
  RESET_DB=true
  KEEP_DATA=false
  KEEP_DATA_SET=true
  WIPE_DATA_SET=true

  echo ""
  log "warn" "--clean-slate will PERMANENTLY delete the entire database and all stored files/cache. This cannot be undone."
  if [ -t 0 ]; then
    read -r -p "  Type 'yes' to continue: " clean_slate_confirm
    if [ "$clean_slate_confirm" != "yes" ]; then
      log "error" "Aborted: --clean-slate not confirmed."
      exit 1
    fi
  else
    log "error" "--clean-slate requires an interactive terminal to confirm. Refusing to run non-interactively."
    exit 1
  fi
fi

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
# 3306 is intentionally not published here (issue #200) — MySQL only needs
# to be reachable inside the pod's network (backend talks to it over
# 127.0.0.1), not from the LAN. The app's only access control
# (known_machines IP allowlist) guards HTTP routes, not the database itself,
# so publishing 3306 let anyone on the LAN connect straight to MySQL.
#
# 9090 (Prometheus) is intentionally not published either (issue #204) —
# Prometheus has no authentication in front of it, so publishing it exposed
# the full PromQL query API, target list, and Prometheus's own internal
# state to anyone on the LAN. Use an SSH port-forward
# (ssh -L 9090:localhost:9090 <host>, then curl/browse localhost:9090 on
# your machine) when you actually need to reach it.
podman pod create --infra=true \
  --cpus "$POD_CPUS" \
  --memory "$POD_MEMORY" \
  -p 8080:8080 \
  -p 3000:3000 \
  dewey-pod

# Per-container limits (issue #211): the pod-level cap above (issue #168)
# only bounds the aggregate, so a heavy container could still consume the
# whole pod's budget and starve the others (e.g. a backend upload/barcode
# burst OOM-killing MySQL or Prometheus even though the pod stays under its
# cap). These four are sized to sum to the pod defaults (4 CPUs / 4g) so no
# single container can eat the whole allocation alone.

# ---------------- MYSQL ----------------
section "MySQL"
log "info" "Deploying MySQL..."

if [ "$RESET_DB" = true ]; then
  log "warn" "Resetting mysql-data volume (--reset-db passed)..."
  podman volume inspect mysql-data &>/dev/null && podman volume rm mysql-data
else
  log "info" "Keeping existing mysql-data volume (pass --reset-db to wipe)"
fi

# Root password used to be hardcoded (issue #200), meaning "dewey" was the
# permanent root password for every deployment this script produced. Instead,
# generate a random one and persist it in a local, gitignored file so it
# survives re-deploys that keep mysql-data — MySQL only honors
# MYSQL_ROOT_PASSWORD on first init of an empty data dir, so if the volume
# survives but the password doesn't, the backend's new DSN stops matching
# what's actually in the (still-initialized) database. Only regenerate when
# --reset-db has just wiped that volume, since that's the one case where the
# next container init will actually apply a new password.
MYSQL_PASSWORD_FILE=".mysql-root-password"
if [ "$RESET_DB" = true ] || [ ! -f "$MYSQL_PASSWORD_FILE" ]; then
  log "info" "Generating new MySQL root password..."
  MYSQL_ROOT_PASSWORD="$(openssl rand -hex 24)"
  printf '%s' "$MYSQL_ROOT_PASSWORD" > "$MYSQL_PASSWORD_FILE"
  chmod 600 "$MYSQL_PASSWORD_FILE"
else
  MYSQL_ROOT_PASSWORD="$(cat "$MYSQL_PASSWORD_FILE")"
fi

# Pinned rather than :latest (issue #207) — package.sh bundles whatever tag
# is actually running so offline deploys get the exact version this was
# tested against, instead of silently pulling a different one later.
podman run -d --pod dewey-pod \
  --name dewey-mysql \
  --cpus 1 --memory 1g \
  -e MYSQL_ROOT_PASSWORD="$MYSQL_ROOT_PASSWORD" \
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
  --cpus 0.5 --memory 512m \
  docker.io/prom/prometheus:v3.13.1

# Brief pause to let Prometheus spin up internal networking before healthcheck
sleep 2
log "info" "Checking Prometheus health..."
# 9090 isn't published to the LAN (issue #204), so curling an HTTP endpoint
# from the host isn't an option anymore either — check the container's own
# running state via podman instead.
if [ "$(podman inspect -f '{{.State.Running}}' dewey-prometheus 2>/dev/null)" = "true" ]; then
  log "success" "Prometheus is healthy!"
else
  log "warn" "Prometheus container did not start."
fi

# ---------------- BACKEND ----------------
section "Backend"

# Ask interactively unless --keep-data/--wipe-data already answered the
# question, or there's no TTY to ask on (e.g. running from CI/automation).
if [ "$KEEP_DATA_SET" = false ] && [ "$WIPE_DATA_SET" = false ] && [ -t 0 ]; then
  read -r -p "  Wipe store/cache/backup/logs volumes before this deploy? [Y/n] " wipe_answer
  case "$wipe_answer" in
    [nN]*) KEEP_DATA=true ;;
    *) KEEP_DATA=false ;;
  esac
elif [ "$KEEP_DATA_SET" = false ] && [ "$WIPE_DATA_SET" = false ]; then
  # No TTY and no explicit flag (issue #206): default to the safe option
  # instead of silently wiping. Pass --wipe-data to wipe non-interactively.
  log "warn" "No TTY and no --keep-data/--wipe-data flag; defaulting to --keep-data. Pass --wipe-data to wipe non-interactively."
  KEEP_DATA=true
fi

if [ "$KEEP_DATA" = true ]; then
  log "info" "Keeping existing store/cache/backup/logs/plugin-scratch volumes"
else
  log "warn" "Resetting store/cache/backup/logs/plugin-scratch volumes (default; pass --keep-data to preserve)..."
  for vol in dewey-store dewey-cache dewey-backup dewey-logs dewey-plugin-scratch; do
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
# Named volumes for store/cache/backup/logs/plugin-scratch. By default these
# are wiped above on every deploy; pass --keep-data to let them survive pod
# recreation instead, the same way mysql-data does for the database.
# dewey-plugin-scratch backs files.read/files.write (issue #284) — the
# container runs --read-only, so without an actual volume mounted at
# /app/plugin-scratch, fileSystemInit's MkdirAll for that directory fails
# at startup (silently: it warns and moves on rather than failing fast),
# and every files.read/files.write call from a plugin errors out.
# DB_DSN is now required (issue #200) — db.go no longer has a hardcoded
# fallback, so it must be passed the same generated root password MySQL
# was started with above.
# Largest share of the four per-container limits (issue #211): this is the
# burst source (uploads/barcode processing) they exist to contain.
podman run -d --pod dewey-pod --name cross-doc-tool-dev \
  --read-only --tmpfs /tmp \
  --cpus 2 --memory 2g \
  -e DB_DSN="root:${MYSQL_ROOT_PASSWORD}@tcp(127.0.0.1:3306)/deweyRecords" \
  -v dewey-store:/app/store:Z \
  -v dewey-cache:/app/cache:Z \
  -v dewey-backup:/app/backup:Z \
  -v dewey-logs:/app/logs:Z \
  -v dewey-plugin-scratch:/app/plugin-scratch:Z \
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
  --cpus 0.5 --memory 512m \
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
echo -e "  ${DIM}\xe2\x94\x82${NC} Prometheus    not published to the LAN (issue #204); ssh -L 9090:localhost:9090 to reach it"
echo -e "  ${DIM}\xe2\x94\x82${NC} MySQL         not published to the LAN (issue #200); reachable inside the pod only"
echo ""

# --- KNOWN MACHINES ---
# /metrics now sits behind the known_machines IP allowlist like every other
# backend route (issue #203), so register this deploy host the same way the
# backend seeds its own loopback addresses (db.go) — otherwise the health
# check and metrics snapshot below would need a manual addMachine call first.
log "info" "Registering deploy host (${HOST_IP}) in known_machines..."
podman exec dewey-mysql mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" deweyRecords \
  -e "INSERT IGNORE INTO known_machines (ip, label) VALUES ('${HOST_IP}', 'deploy-host');" \
  2>/dev/null && log "success" "Deploy host registered" \
  || log "warn" "Could not register deploy host in known_machines"

# --- HEALTH CHECKS ---
# check_health reports whether a service answered at all rather than
# requiring a 200 — every backend route sits behind the known_machines IP
# allowlist, so a 403 there still proves the service is up.
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

# Backend's /metrics now sits behind the known_machines allowlist (issue
# #203), same as every other route. The registration above should cover
# this curl, but Podman's NAT can present a host-to-published-port
# connection under a different source IP than ${HOST_IP} depending on the
# network backend in use — if this or the metrics snapshot below still 403s,
# check `podman logs cross-doc-tool-dev` for the "unregistered machine" IP
# it actually saw and register that one instead.
check_health "Frontend"   "http://127.0.0.1:3000"
check_health "Backend"    "http://127.0.0.1:8080/metrics"

# Prometheus isn't published to the LAN (issue #204), so it's no longer
# reachable via localhost from the host either — same container-state check
# used right after starting it above.
if [ "$(podman inspect -f '{{.State.Running}}' dewey-prometheus 2>/dev/null)" = "true" ]; then
  log "success" "Prometheus is responding (container running)"
else
  log "warn" "Prometheus did not respond"
fi

# --- KEY METRICS SNAPSHOT ---
echo ""
log "info" "Backend metrics snapshot:"
BACKEND_METRICS="$(curl -s --max-time 5 http://127.0.0.1:8080/metrics 2>/dev/null || true)"
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
