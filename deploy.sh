#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

# --- COLOR DEFINITIONS ---
NC='\033*0m' # No Color
BOLD='\033*1m'
CYAN='\033*0;36m'
GREEN='\033*0;32m'
RED='\033*0;31m'
YELLOW='\033*0;33m'

# --- LOGGING FUNCTION ---
log() {
  local level="$1"
  local message="$2"
  case "$level" in
    "info")    echo -e "${CYAN}${BOLD}==>${NC} ${BOLD}${message}${NC}" ;;
    "success") echo -e "${GREEN}${BOLD}==>${NC} ${GREEN}${BOLD}${message}${NC}" ;;
    "warn")    echo -e "${YELLOW}${BOLD}==>${NC} ${YELLOW}${message}${NC}" ;;
    "error")   echo -e "${RED}${BOLD}==> ERROR:${NC} ${RED}${BOLD}${message}${NC}" ;;
  esac
}

# --- CLEANUP ---
log "info" "Deleting existing pod if it exists..."
podman pod rm -f dewey-pod 2>/dev/null || true

# --- POD CREATION ---
log "info" "Creating dewey-pod..."
# Added 9090 here so Prometheus is accessible externally
podman pod create --infra=true \
  -p 8080:8080 \
  -p 3000:3000 \
  -p 3306:3306 \
  -p 9090:9090 \
  dewey-pod

# ---------------- MYSQL ----------------
log "info" "Deploying MySQL..."

podman volume inspect mysql-data &>/dev/null && podman volume rm mysql-data # Remove for prod

podman run -d --pod dewey-pod \
  --name dewey-mysql \
  -e MYSQL_ROOT_PASSWORD=dewey \
  -e MYSQL_DATABASE=deweyRecords \
  -v mysql-data:/var/lib/mysql:Z \
  docker.io/library/mysql:latest

# This is critcal to ensure the database is ready before we start the backend which depends on it
echo "Waiting for database..."
while ! podman exec dewey-mysql mysqladmin ping -h localhost --silent; do
    echo "Still waiting..."
    sleep 3  # Wait 3 seconds before checking again
done
echo "Database is up!"

# ---------------- PROMETHEUS ----------------
log "info" "Deploying Prometheus..."
podman pull docker.io/prom/prometheus:latest

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
log "info" "Building backend image (cross-doc-tool-dev)..."
podman build \
  --build-arg CGO_CFLAGS="-Wno-discarded-qualifiers" \
  -t cross-doc-tool-dev ./backend

log "info" "Running backend container..."
podman run -d --pod dewey-pod --name cross-doc-tool-dev cross-doc-tool-dev

# ---------------- FRONTEND ----------------
log "info" "Building Administration tool frontend (admin-portal)..."
podman build -t admin-portal ./frontend/doctooladmin

log "info" "Running Svelte frontend container..."
podman run -d --pod dewey-pod --name svelte-container admin-portal

# ---------------- STATUS SUMMARY ----------------
echo ""
log "success" "All components deployed to dewey-pod successfully!"
echo -e "${BOLD}Current Pod Status:${NC}"
podman ps --filter pod=dewey-pod
