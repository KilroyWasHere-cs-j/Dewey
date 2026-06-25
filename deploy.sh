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

podman volume inspect mysql-data &>/dev/null && podman volume rm mysql-data # Remove for prod

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
podman run -d --pod dewey-pod --name cross-doc-tool-dev cross-doc-tool-dev
log "success" "Backend running"

# ---------------- FRONTEND ----------------
section "Frontend"
log "info" "Building frontend image ${DIM}(admin-portal)${NC}..."
podman build -t admin-portal ./frontend/doctooladmin

log "info" "Starting Svelte frontend container..."
podman run -d --pod dewey-pod --name svelte-container admin-portal
log "success" "Frontend running"

# ---------------- SAVE POD TO FILE ----------------
# Optionally export all pod images as a single compressed tarball for offline VM deployment
section "Export"
echo ""
echo -e "  ${DIM}\xe2\x94\x82${NC}  Save the entire pod as a single portable image file?"
echo -e "  ${DIM}\xe2\x94\x82${NC}  ${DIM}This bundles all 4 images into one .tar.gz for VM deployment.${NC}"
echo -e "  ${DIM}\xe2\x94\x82${NC}"
read -rp "$(echo -e "  ${DIM}\xe2\x94\x94\xe2\x94\x80${NC} ${WHITE}Export dewey-pod images? [y/N]:${NC} ")" SAVE_CHOICE

if [[ "${SAVE_CHOICE,,}" == "y" ]]; then
  SAVE_DIR="./build-images"
  SAVE_FILE="${SAVE_DIR}/dewey-pod-all.tar.gz"
  mkdir -p "$SAVE_DIR"

  log "info" "Saving all pod images to ${BOLD}${SAVE_FILE}${NC}..."

  # Bundle every image in the pod into a single compressed archive
  podman save \
    cross-doc-tool-dev \
    admin-portal \
    docker.io/library/mysql:latest \
    docker.io/prom/prometheus:latest \
    | gzip > "$SAVE_FILE" &
  SAVE_PID=$!
  spinner "$SAVE_PID" "Compressing images..."
  wait "$SAVE_PID"

  FILE_SIZE=$(du -h "$SAVE_FILE" | cut -f1)
  log "success" "Saved: ${SAVE_FILE} (${FILE_SIZE})"
  echo ""
  echo -e "  ${DIM}To load on a VM:${NC}"
  echo -e "  ${CYAN}gunzip -c dewey-pod-all.tar.gz | podman load${NC}"
fi

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
