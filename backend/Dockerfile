# Build command: podman build -t cross-doc-tool-dev .
# Run command: podman run -p 8080:8080 --read-only --tmpfs /tmp cross-doc-tool-dev

# ---- Build stage ----
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG CGO_CFLAGS="-Wno-discarded-qualifiers"
ENV CGO_CFLAGS=${CGO_CFLAGS}

# GIT_BRANCH is passed in via --build-arg (deploy.sh/package.sh compute it with
# `git rev-parse --abbrev-ref HEAD` on the host) since .dockerignore excludes
# .git, so the branch can't be read from inside the build context itself.
ARG GIT_BRANCH=unknown
RUN go build -trimpath -ldflags="-s -w -X main.GitBranch=${GIT_BRANCH}" -o app-binary

# ---- Run stage ----
FROM alpine:3.21

# bash and curl are required by the BITs test suite; mysql-client provides
# mysqldump, which saveBackup() (utils.go) shells out to on every daemon
# tick (issue #391 — it was missing, so the backup daemon failed silently
# before ever reaching the store/ zip step)
RUN apk add --no-cache bash curl mysql-client \
    && addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

RUN mkdir -p cache store backup logs plugins \
    && chown -R appuser:appgroup /app

COPY --from=builder --chown=appuser:appgroup /app/app-binary .
COPY --from=builder --chown=appuser:appgroup /app/config.json .
COPY --from=builder --chown=appuser:appgroup /app/plugins ./plugins
COPY --chown=appuser:appgroup testing_tooling ./testing_tooling

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO /dev/null http://localhost:8080/ || exit 1

CMD ["./app-binary"]
