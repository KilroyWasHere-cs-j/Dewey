echo ""
echo "==> Creating pod..."
podman pod create -p 8080:8080 -p 3000:3000 cross-doc-pod

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
podman build -t my-svelte-app ./frontend/doctooladmin
podman run -d --pod cross-doc-pod --name svelte-container my-svelte-app
