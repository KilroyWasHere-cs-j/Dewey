#!/usr/bin/env bash

podman pod rm -f cross-doc-pod
podman build -t cross-doc-tool-dev .
podman run -d -p 8080:8080 cross-doc-tool-dev
podman run -d \
  --name grafana \
  -p 3000:3000 \
  -v grafana-data:/var/lib/grafana \
  -e GF_SECURITY_ADMIN_USER=admin \
  -e GF_SECURITY_ADMIN_PASSWORD=admin \
  docker.io/grafana/grafana:latest
echo "==> Done!"
echo "App:     http://localhost:8080"
echo "Grafana: http://localhost:3000"
echo "Login: admin / admin"
echo ""

