# Build command: podman build -t cross-doc-tool-dev .
# Run command: podman run -p 8080:8080 cross-doc-tool-dev

# ---- Build stage ----
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install git (needed for private modules sometimes)
RUN apk add --no-cache git # TODO: Check if we need this installed

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the full project (not just *.go)
COPY . .

# Build binary
ARG CGO_CFLAGS="-Wno-discarded-qualifiers"
ENV CGO_CFLAGS=${CGO_CFLAGS}
RUN go build -o app-binary

# ---- Run stage ----
FROM alpine:latest

WORKDIR /app

# Install nano here
RUN apk add --no-cache nano
RUN apk add --no-cache curl
RUN apk add --no-cache htop

# Copy only the binary from builder
COPY --from=builder /app/app-binary .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/rules ./rules


EXPOSE 8080

CMD ["./app-binary"]
