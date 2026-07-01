#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

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
RESET_DB=false
for arg in "$@"; do
  case "$arg" in
    --reset-db) RESET_DB=true ;;
  esac
done

# --- CLEANUP ---
section "Cleanup"
log "info" "Removing existing pod..."
podman pod rm -f dewey-pod 2>/dev/null || true
log "success" "Clean slate ready"

# --- POD CREATION ---
section "Pod Creation"
log "info" "Creating dewey-pod..."
# Added 9090 here so Prometheus is accessible externally
podman pod create --infra=true \
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

podman run -d --pod dewey-pod \
  --name dewey-mysql \
  -e MYSQL_ROOT_PASSWORD=dewey \
  -e MYSQL_DATABASE=deweyRecords \
  -v mysql-data:/var/lib/mysql:Z \
  docker.io/library/mysql:latest

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
podman pull docker.io/prom/prometheus:latest

log "info" "Starting Prometheus container..."
podman run -d --pod dewey-pod \
  --name dewey-prometheus \
  docker.io/prom/prometheus:latest

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
log "info" "Building backend image ${DIM}(cross-doc-tool-dev)${NC}..."
podman build \
  --build-arg CGO_CFLAGS="-Wno-discarded-qualifiers" \
  -t cross-doc-tool-dev ./backend

log "info" "Starting backend container..."
# Named volumes for store/cache/backup/logs so uploaded files and app logs
# survive a pod recreation, the same way mysql-data does for the database.
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
podman run -d --pod dewey-pod --name svelte-container admin-portal
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
