#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

# --- LOGGING ---
# Mirror everything printed to the terminal into a timestamped log file so a
# run can be reviewed or attached to a bug report after the fact.
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
LOG_DIR="$SCRIPT_DIR/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/cleanup-$(date +%Y%m%d-%H%M%S).log"
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
section() {
  local label="$1"
  echo ""
  echo -e "  ${DIM}${CYAN}\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80${NC}"
  echo -e "  ${MAGENTA}${BOLD}\xe2\x97\x86 ${label}${NC}"
  echo -e "  ${DIM}${CYAN}\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80${NC}"
}

# --- STARTUP BANNER ---
echo ""
echo -e "  ${CYAN}${BOLD}\xe2\x94\x8c\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x90${NC}"
echo -e "  ${CYAN}${BOLD}\xe2\x94\x82${NC}  ${WHITE}${BOLD}DEWEY ${NC}${DIM}Workspace Cleanup${NC}  ${CYAN}${BOLD}\xe2\x94\x82${NC}"
echo -e "  ${CYAN}${BOLD}\xe2\x94\x94\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x98${NC}"

# Removes a path if it exists, logging either way instead of failing silently
# or throwing on a missing target — every path this script touches is
# optional (a fresh checkout won't have any of them yet).
remove_if_exists() {
  local path="$1"
  if [ -e "$path" ]; then
    rm -rf "$path"
    log "success" "Removed $path"
  else
    log "info" "$path not present, skipping"
  fi
}

# ---------------- REPO BUILD ARTIFACTS ----------------
section "Repo Build Artifacts"

# Compiled binaries (all gitignored — see .gitignore)
remove_if_exists "$SCRIPT_DIR/backend/Dewey"
remove_if_exists "$SCRIPT_DIR/cli/dewey-cli"
remove_if_exists "$SCRIPT_DIR/dewey-mcp/dewey-mcp"

# SvelteKit's adapter-node build output
remove_if_exists "$SCRIPT_DIR/frontend/doctooladmin/build"

# Go test/coverage artifacts can land anywhere under backend/ depending on
# which package a test was run from, so these are swept by pattern rather
# than a single fixed path (mirrors the patterns in .gitignore).
log "info" "Sweeping Go test/coverage artifacts under backend/..."
find "$SCRIPT_DIR/backend" \
  \( -name "*.test" -o -name "*.out" -o -name "coverage.*" -o -name "*.coverprofile" -o -name "profile.cov" \) \
  -type f -print -delete | sed 's/^/  /'
log "success" "Go test/coverage artifacts swept"

# ---------------- LOCAL TEST/DEV LEFTOVERS ----------------
section "Local Test/Dev Leftovers"

# These only exist when the backend has been run directly on the host
# (e.g. `go run .` from backend/, or test_suite.sh run manually per its own
# header comment) rather than through deploy.sh's podman volumes — real
# uploaded/backed-up files and run logs can accumulate here.
remove_if_exists "$SCRIPT_DIR/backend/store"
remove_if_exists "$SCRIPT_DIR/backend/cache"
remove_if_exists "$SCRIPT_DIR/backend/backup"
remove_if_exists "$SCRIPT_DIR/backend/logs"
remove_if_exists "$SCRIPT_DIR/backend/testing_tooling/logs"

# deploy.sh/package.sh/this script's own run logs (repo root logs/). The
# currently-open log file for *this* run is excluded so its own output
# survives to be reviewed after the script exits.
if [ -d "$LOG_DIR" ]; then
  log "info" "Clearing old run logs in $LOG_DIR (keeping this run's log)..."
  find "$LOG_DIR" -type f ! -name "$(basename "$LOG_FILE")" -print -delete | sed 's/^/  /'
  log "success" "Old run logs cleared"
fi

# ---------------- PODMAN CLEANUP ----------------
section "Podman Cleanup"

# Unused/dangling images accumulate fast from repeated deploy.sh runs (every
# rebuild of admin-portal/cross-doc-tool-dev leaves the previous layers
# behind as untagged images) and can eat enough disk to break future builds.
# -a also removes unused images that were never referenced by a container,
# not just dangling ones.
log "info" "Pruning unused podman images..."
podman image prune -a -f | sed 's/^/  /'
log "success" "Unused images pruned"

# Stopped containers not part of a running pod (e.g. left over from a failed
# or manually-run container outside deploy.sh).
log "info" "Pruning stopped containers..."
podman container prune -f | sed 's/^/  /'
log "success" "Stopped containers pruned"

# Only removes volumes not referenced by any container — dewey-store,
# dewey-cache, dewey-backup, dewey-logs, dewey-plugin-scratch, mysql-data,
# and prometheus-data are all actively mounted by dewey-pod's containers
# while it's running, so this is safe to run without tearing the pod down
# first and won't touch real application data.
log "info" "Pruning unused podman volumes..."
podman volume prune -f | sed 's/^/  /'
log "success" "Unused volumes pruned"

# ---------------- SUMMARY ----------------
section "Summary"
echo ""
echo -e "  ${GREEN}${BOLD}\xe2\x96\x88\xe2\x96\x88\xe2\x96\x88 Cleanup complete ${GREEN}\xe2\x96\x88\xe2\x96\x88\xe2\x96\x88${NC}"
echo ""
log "info" "Disk usage now:"
df -h / | sed 's/^/  /'
echo ""
log "info" "dewey-pod was left running — this cleans up leftovers, it doesn't tear down the deployment."
echo ""
