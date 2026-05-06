echo ""
echo "==> Creating pod..."
podman pod create -p 8080:8080 -p 3000:3000 cross-doc-pod


echo ""
echo "==> Deploying Prometheus..."
# Pull the latest Prometheus image

podman pull docker.io/prom/prometheus:latest

# Verify the image
podman images | grep prometheus

# Run Prometheus in detached mode on port 9090
podman run -d \
  --name my-prometheus \
  -p 9090:9090 \
  prom/prometheus:latest

# Check the container is running
podman ps

# Verify Prometheus is up
curl -s http://localhost:9090/-/healthy

# ---------------- BACKEND ----------------
echo ""
echo "==> Building backend image..."
podman build \
  --build-arg CGO_CFLAGS="-Wno-discarded-qualifiers" \
  -t cross-doc-tool-dev ./backend

echo ""
echo "==> Running backend container..."
podman run -d --pod cross-doc-pod --name cross-doc-tool-dev cross-doc-tool-dev

# ---------------- FRONTEND ----------------
echo ""
echo "==> Deploying Administration tool..."
podman build -t admin-portal ./frontend/doctooladmin
podman run -d --pod cross-doc-pod --name svelte-container admin-portal
