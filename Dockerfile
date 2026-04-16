# ---- Build stage ----
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install git (needed for private modules sometimes)
RUN apk add --no-cache git

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the full project (not just *.go)
COPY . .

# Build binary
RUN go build -o app-binary

# ---- Run stage ----
FROM alpine:latest

WORKDIR /app

# Copy only the binary from builder
COPY --from=builder /app/app-binary .

EXPOSE 8080

CMD ["./app-binary"]
