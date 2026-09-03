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
# Debian instead of Alpine (issue #394): mysqldump's --set-gtid-purged=OFF
# flag (utils.go, issue #391) isn't recognized by MariaDB's mariadb-dump,
# and both Alpine's "mysql-client" and Debian's own default-mysql-client
# are MariaDB, not genuine Oracle MySQL tooling. Adding MySQL's official
# APT repo below pulls the real client, version-matched to the mysql:9.7
# server image deploy.sh runs.
FROM debian:bookworm-slim

# bash and curl are required by the BITs test suite.
RUN apt-get update && apt-get install -y --no-install-recommends \
        bash curl ca-certificates gnupg \
    && curl -fsSL https://repo.mysql.com/RPM-GPG-KEY-mysql-2025 \
        | gpg --dearmor -o /usr/share/keyrings/mysql-archive-keyring.gpg \
    && echo "deb [signed-by=/usr/share/keyrings/mysql-archive-keyring.gpg] http://repo.mysql.com/apt/debian bookworm mysql-9.7-lts" \
        > /etc/apt/sources.list.d/mysql.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends mysql-client \
    && apt-get purge -y --auto-remove gnupg \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system appgroup \
    && useradd --system --gid appgroup --no-create-home --shell /usr/sbin/nologin appuser

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
    CMD curl -fsS http://localhost:8080/ >/dev/null || exit 1

CMD ["./app-binary"]
