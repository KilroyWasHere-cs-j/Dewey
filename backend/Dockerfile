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
RUN go build -trimpath -ldflags="-s -w" -o app-binary

# ---- Run stage ----
FROM alpine:3.21

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

RUN mkdir -p cache store backup logs plugins \
    && chown -R appuser:appgroup /app

COPY --from=builder --chown=appuser:appgroup /app/app-binary .
COPY --from=builder --chown=appuser:appgroup /app/plugins ./plugins

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO /dev/null http://localhost:8080/ || exit 1

CMD ["./app-binary"]
