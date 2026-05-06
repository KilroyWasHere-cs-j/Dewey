#!/usr/bin/env bash
set -e

echo ""
echo "==> Deploying CrossDocTool..."

echo ""
echo "==> Destroying existing pod..."
podman pod rm -f cross-doc-pod >/dev/null 2>&1 || true

# ---------------- BACKEND ----------------
echo ""
echo "==> Building backend image..."
podman build \
  --build-arg CGO_CFLAGS="-Wno-discarded-qualifiers" \
  -t cross-doc-tool-dev ./backend

echo ""
echo "==> Running backend container..."
podman rm -f cross-doc-tool-dev >/dev/null 2>&1 || true
podman run -d --name cross-doc-tool-dev -p 8080:8080 cross-doc-tool-dev

# ---------------- FRONTEND ----------------
echo ""
echo "==> Deploying Administration tool..."

echo ""
echo "==> Building frontend image (doctooladmin)..."
podman build -t doctooladmin:latest -f frontend/doctooladmin/Dockerfile frontend/doctooladmin

echo ""
echo "==> Stopping existing frontend container (if any)..."
podman rm -f doctooladmin >/dev/null 2>&1 || true

echo ""
echo "==> Running frontend container..."
container_id=$(podman run -d \
  --name doctooladmin \
  -p 3000:3000 \
  -e NODE_ENV=production \
  -e PORT=3000 \
  --pull=never \
  doctooladmin:latest)

echo "Started frontend container $container_id"

# ---------------- VERIFY ----------------
echo ""
echo "==> Verifying frontend container..."
sleep 2

if podman ps --filter "name=doctooladmin" --format "{{.Names}}" | grep -q '^doctooladmin$'; then
  echo "Frontend container running"

  echo ""
  echo "==> Generating systemd unit..."
  podman generate systemd --new --name doctooladmin -f > doctooladmin.service

  echo "Generated doctooladmin.service in $(pwd)"
  echo "To install system-wide:"
  echo "sudo mv doctooladmin.service /etc/systemd/system/"
  echo "sudo systemctl daemon-reload"
  echo "sudo systemctl enable --now doctooladmin.service"
else
  echo "ERROR: Frontend container failed to start"
  echo ""
  echo "==> Logs:"
  podman logs doctooladmin || true
  exit 1
fi

echo ""
echo "==> Done!"
echo "Backend: http://localhost:8080"
echo "Frontend: http://localhost:3000"
echo ""
