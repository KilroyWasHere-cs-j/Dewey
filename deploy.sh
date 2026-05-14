echo ""
echo "==> Delete existing pod..."
podman pod rm -f dewey-pod

echo ""
echo "==> Creating pod..."
podman pod create -p 8080:8080 -p 3000:3000 dewey-pod

echo ""
echo "==> Deploying Prometheus..."

podman pull docker.io/prom/prometheus:latest
podman images | grep prometheus

podman run -d --pod dewey-pod\
  --name dewey-prometheus \
  -p 9090:9090 \
  prom/prometheus:latest

podman ps
curl -s http://localhost:9090/-/healthy

# ---------------- BACKEND ----------------
echo ""
echo "==> Building backend image..."
podman build \
  --build-arg CGO_CFLAGS="-Wno-discarded-qualifiers" \
  -t cross-doc-tool-dev ./backend

echo ""
echo "==> Running backend container..."
podman run -d --pod dewey-pod --name cross-doc-tool-dev cross-doc-tool-dev

# ---------------- FRONTEND ----------------
echo ""
echo "==> Deploying Administration tool..."
podman build -t admin-portal ./frontend/doctooladmin
podman run -d --pod dewey-pod --name svelte-container admin-portal
