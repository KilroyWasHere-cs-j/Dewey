#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

# --- LOGGING ---
# Mirror everything printed to the terminal into a timestamped log file so a
# run can be reviewed or attached to a bug report after the fact.
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
LOG_DIR="$SCRIPT_DIR/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/package-$(date +%Y%m%d-%H%M%S).log"
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
  echo -e "  ${DIM}${CYAN}\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80${NC}"
}

# --- STARTUP BANNER ---
echo ""
echo -e "  ${CYAN}${BOLD}\xe2\x94\x8c\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x90${NC}"
echo -e "  ${CYAN}${BOLD}\xe2\x94\x82${NC}  ${WHITE}${BOLD}DEWEY ${NC}${DIM}Server Package Builder${NC}  ${CYAN}${BOLD}\xe2\x94\x82${NC}"
echo -e "  ${CYAN}${BOLD}\xe2\x94\x94\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x98${NC}"

# Requires dewey-pod to be running (run deploy.sh first) so podman generate kube has something to inspect
if ! podman pod exists dewey-pod 2>/dev/null; then
  log "error" "dewey-pod is not running. Run ./deploy.sh first."
  exit 1
fi

# ---------------- BUILD IMAGES ----------------
section "Build"
log "info" "Building backend image ${DIM}(cross-doc-tool-dev)${NC}..."
# Stamp the binary with the branch it's being packaged from (issue #66) so
# it's visible via GET /version without needing to shell into the container.
GIT_BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
podman build \
  --build-arg CGO_CFLAGS="-Wno-discarded-qualifiers" \
  --build-arg GIT_BRANCH="$GIT_BRANCH" \
  -t cross-doc-tool-dev ./backend
log "success" "Backend image built"

log "info" "Building frontend image ${DIM}(admin-portal)${NC}..."
podman build -t admin-portal ./frontend/doctooladmin
log "success" "Frontend image built"

log "info" "Staging docs for dewey-cli's docs command (issue #379)..."
# go:embed (cli/docs_viewer.go) can only reach files inside the cli/ module,
# so the real source docs get staged here as plain copies right before the
# build picks them up.
mkdir -p cli/embedded_docs
cp README.md cli/embedded_docs/readme.md
cp frontend/doctooladmin/README.md cli/embedded_docs/admin_readme.md
log "success" "Docs staged"

log "info" "Building dewey-cli..."
# The deployment target is always a Linux server (podman pods, bash run.sh
# below) regardless of what OS/arch this script itself runs on, so the
# build target is fixed rather than inferred from the packaging host
# (issue #280).
(cd cli && GOOS=linux GOARCH=amd64 go build -o dewey-cli .)
log "success" "dewey-cli built"

log "info" "Building dewey-mcp..."
# Same fixed linux/amd64 target as dewey-cli above (issue #280) — dewey-mcp
# is a stdio MCP server, not a pod container, so it's bundled as a plain
# binary the same way dewey-cli is rather than built into a podman image.
(cd dewey-mcp && GOOS=linux GOARCH=amd64 go build -o dewey-mcp .)
log "success" "dewey-mcp built"

# ---------------- PACKAGE ----------------
# Exports the full pod as a self-contained bundle: images + pod spec + run script.
# The resulting .tar.gz can be transferred to any server and deployed with ./run.sh
section "Package"

SAVE_DIR="./build-images"
BUNDLE_DIR="${SAVE_DIR}/dewey-bundle"
BUNDLE_FILE="${SAVE_DIR}/dewey-bundle-$(date +%Y%m%d-%H%M%S).tar.gz"
mkdir -p "$BUNDLE_DIR"

log "info" "Generating pod spec..."
podman generate kube dewey-pod > "${BUNDLE_DIR}/dewey-pod.yaml"

log "info" "Saving backend image..."
podman save localhost/cross-doc-tool-dev:latest -o "${BUNDLE_DIR}/cross-doc-tool-dev.tar"

log "info" "Saving frontend image..."
podman save localhost/admin-portal:latest -o "${BUNDLE_DIR}/admin-portal.tar"

# mysql/prometheus aren't built by this repo, so podman generate kube (above)
# only captures a reference to whatever tag is running, not the image itself.
# Without saving+bundling them too, a target server with no network access to
# docker.io can't deploy at all (issue #207). Read the tag from the running
# containers themselves rather than hardcoding it a second time here, so this
# always matches whatever deploy.sh actually pinned and started.
MYSQL_IMAGE="$(podman inspect dewey-mysql --format '{{.ImageName}}')"
PROMETHEUS_IMAGE="$(podman inspect dewey-prometheus --format '{{.ImageName}}')"

log "info" "Saving MySQL image (${MYSQL_IMAGE})..."
podman save "$MYSQL_IMAGE" -o "${BUNDLE_DIR}/mysql.tar"

log "info" "Saving Prometheus image (${PROMETHEUS_IMAGE})..."
podman save "$PROMETHEUS_IMAGE" -o "${BUNDLE_DIR}/prometheus.tar"

log "info" "Bundling dewey-cli..."
cp cli/dewey-cli "${BUNDLE_DIR}/dewey-cli"
chmod +x "${BUNDLE_DIR}/dewey-cli"
rm -f cli/dewey-cli

log "info" "Bundling dewey-mcp..."
cp dewey-mcp/dewey-mcp "${BUNDLE_DIR}/dewey-mcp"
chmod +x "${BUNDLE_DIR}/dewey-mcp"
rm -f dewey-mcp/dewey-mcp

log "info" "Writing run script..."
cat > "${BUNDLE_DIR}/run.sh" <<'EOF'
#!/bin/bash

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
  echo -e "  ${DIM}${CYAN}\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80${NC}"
  echo -e "  ${MAGENTA}${BOLD}\xe2\x97\x86 ${label}${NC}"
  echo -e "  ${DIM}${CYAN}\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80\xe2\x94\x80${NC}"
}

podman load -i cross-doc-tool-dev.tar
podman load -i admin-portal.tar
podman load -i mysql.tar
podman load -i prometheus.tar

# By default, wipe the backend data volumes (dewey-store, dewey-cache,
# dewey-backup, dewey-logs) so each run starts fresh. Metrics like
# app_files_in_store read these directories live, so leftover files from a
# previous run made the dashboard look like nothing had reset between runs.
# Pass --keep-data to preserve them instead (e.g. re-running against real
# data you don't want to lose), or --wipe-data to explicitly confirm a wipe
# when running non-interactively (see the TTY check below). mysql-data is
# never touched by this script.
KEEP_DATA=false
KEEP_DATA_SET=false
WIPE_DATA_SET=false
for arg in "$@"; do
  case "$arg" in
    --keep-data) KEEP_DATA=true; KEEP_DATA_SET=true ;;
    --wipe-data) WIPE_DATA_SET=true ;;
  esac
done

# Remove any existing pod first so its containers release the volumes
# before we try to remove them — a volume in use by a running container
# can't be removed.
podman pod rm -f dewey-pod 2>/dev/null || true

# Ask interactively unless --keep-data/--wipe-data already answered the
# question, or there's no TTY to ask on (e.g. running from CI/automation).
# Previously this always defaulted to wipe regardless of TTY, so a
# non-interactive run (cron, CI, SSH without -t) silently wiped the store
# with only a log line to show for it — since mysql-data isn't touched by
# this script, that left MySQL's file-metadata pointing at documents no
# longer on disk (issue #206).
if [ "$KEEP_DATA_SET" = false ] && [ "$WIPE_DATA_SET" = false ] && [ -t 0 ]; then
  read -r -p "  Wipe store/cache/backup/logs volumes before this run? [y/N] " wipe_answer
  case "$wipe_answer" in
    [yY]*) KEEP_DATA=false ;;
    *) KEEP_DATA=true ;;
  esac
elif [ "$KEEP_DATA_SET" = false ] && [ "$WIPE_DATA_SET" = false ]; then
  log "warn" "No TTY and no --keep-data/--wipe-data flag; defaulting to --keep-data. Pass --wipe-data to wipe non-interactively."
  KEEP_DATA=true
fi

if [ "$KEEP_DATA" = true ]; then
  echo "Keeping existing store/cache/backup/logs volumes (--keep-data passed)"
else
  echo "Resetting store/cache/backup/logs volumes (default; pass --keep-data to preserve)..."
  for vol in dewey-store dewey-cache dewey-backup dewey-logs; do
    podman volume inspect "$vol" &>/dev/null && podman volume rm "$vol"
  done
fi

# Prefer the machine's primary LAN IP so the frontend is reachable from other
# devices on the network, not just this host — falls back to localhost if
# none is found. Computed before podman play kube (rather than just before
# the summary, as before) so it can patch dewey-pod.yaml's ORIGIN below.
HOST_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
[ -z "$HOST_IP" ] && HOST_IP="localhost"

# dewey-pod.yaml was captured via `podman generate kube` on whatever machine
# ran package.sh, so its ORIGIN env value is that machine's IP — wrong here.
# SvelteKit's CSRF checkOrigin needs ORIGIN to match the address clients
# actually use to reach this deployment, so patch it in place before playing
# the pod (podman play kube has no per-env override flag).
sed -i "/name: ORIGIN/{n;s|value: .*|value: http://${HOST_IP}:3000|}" dewey-pod.yaml

# --replace lets this be re-run against an already-deployed pod without
# manually tearing it down first (the pod removal above already handles
# that for us, but --replace keeps this safe to re-run either way).
podman play kube --replace dewey-pod.yaml
log "success" "Pod deployed"

# ---------------- POST-DEPLOYMENT INFO ----------------
section "Post-Deployment Info"

# Give the pod's containers a moment to start before checking on them —
# podman play kube returns as soon as it's created them, not once they're
# actually serving traffic.
sleep 5

echo ""
log "info" "Host IP: ${BOLD}${HOST_IP}${NC}"
echo ""
echo -e "  ${DIM}\xe2\x94\x82${NC} Frontend      http://${HOST_IP}:3000"
echo -e "  ${DIM}\xe2\x94\x82${NC} Backend API   http://${HOST_IP}:8080"
echo -e "  ${DIM}\xe2\x94\x82${NC} Prometheus    not published to the LAN (issue #204); ssh -L 9090:localhost:9090 to reach it"
echo -e "  ${DIM}\xe2\x94\x82${NC} MySQL         not published to the LAN (issue #200); reachable inside the pod only"
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

# Prometheus isn't published to the LAN (issue #204), so it's not reachable
# via localhost from the host either — check the container's running state
# via podman instead. podman play kube prefixes container names with the
# pod name, so this is dewey-pod-dewey-prometheus rather than
# deploy.sh's bare dewey-prometheus.
if [ "$(podman inspect -f '{{.State.Running}}' dewey-pod-dewey-prometheus 2>/dev/null)" = "true" ]; then
  log "success" "Prometheus is responding (container running)"
else
  log "warn" "Prometheus did not respond"
fi

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
echo -e "  ${DIM}\xe2\x94\x82${NC} Watch backend logs (BITs):    ${CYAN}podman logs -f dewey-pod-cross-doc-tool-dev${NC}"
echo -e "  ${DIM}\xe2\x94\x82${NC} Attach to backend container:  ${CYAN}podman attach dewey-pod-cross-doc-tool-dev${NC}"
echo -e "  ${DIM}\xe2\x94\x82${NC} Use the CLI:                  ${CYAN}./dewey-cli --help${NC}"
echo -e "  ${DIM}\xe2\x94\x82${NC} Connect an MCP client:        ${CYAN}./dewey-mcp${NC} (stdio; point your MCP client's command at this binary)"
echo ""
EOF
chmod +x "${BUNDLE_DIR}/run.sh"

log "info" "Compressing bundle..."
tar -czf "$BUNDLE_FILE" -C "$SAVE_DIR" dewey-bundle
rm -rf "$BUNDLE_DIR"

log "success" "Bundle saved to ${BUNDLE_FILE}"
echo ""
