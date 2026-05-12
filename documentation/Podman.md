# Podman

### What is Podman?
[Podman](https://podman.io/) is a container runtime that provides a Docker-compatible interface. Frontend, backend, Prometheus, as well as database tools.

### deploy.sh script
To simplify deployment, a `deploy.sh` script is provided that automates the build and deployment process. This script will deploy all the necessary containers for the application as one Pod.

## Items that are deployed
**Golang backend service**
**Svelte frontend service**
**Metrics tracking Prometheus**
**Database tools**


### deploy.sh

- 1 Remove any existing pod
```bash
podman pod rm -f dewey-pod
```

- 2 Create a new pod called `dewey-pod`
```bash
podman pod create -p 8080:8080 -p 3000:3000 dewey-pod
```

- 3 Pull the latest Prometheus image, verify it, and run it in detached mode on port 9090
```bash
podman pull docker.io/prom/prometheus:latest
podman images | grep prometheus
podman run -d --pod dewey-pod\
  --name dewey-prometheus \
  -p 9090:9090 \
  prom/prometheus:latest
```

- 4 List running containers and verify Prometheus is healthy
```bash
podman ps
curl -s http://localhost:9090/-/healthy
```

- 5 Build and run the Golang backend service in the `dewey-pod` Pod
```bash
podman build \
  --build-arg CGO_CFLAGS="-Wno-discarded-qualifiers" \
  -t cross-doc-tool-dev ./backend
podman run -d --pod dewey-pod --name cross-doc-tool-dev cross-doc-tool-dev
```

- 6
```bash
podman build -t cross-doc-tool-frontend ./frontend
podman run -d --pod dewey-pod --name cross-doc-tool-frontend cross-doc-tool-frontend
```
