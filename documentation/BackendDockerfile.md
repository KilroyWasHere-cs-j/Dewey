# Podman build file

**Lives** `./backend/Dockerfile`

### Purpose
The backend Dockerfile is used to build the backend into a container; for deployment. Dockerfile is located at `./backend/Dockerfile`. 


## Dockerfile Flow til to Deployment

# Build stage

- 1 Set environment OS and working directory as well as copy package files.
```Dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git # TODO: Check if we need this installed

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build binary
ARG CGO_CFLAGS="-Wno-discarded-qualifiers"
ENV CGO_CFLAGS=${CGO_CFLAGS}
RUN go build -o app-binary
```


# Production stage

- 1 Set environment OS and working directory as well as copy package files.
```Dockerfile
FROM alpine:latest
WORKDIR /app
```

- 2 Install system utilities.
```Dockerfile
# Install nano here
RUN apk add --no-cache nano
RUN apk add --no-cache curl
RUN apk add --no-cache htop
```

- 3 Copy only the binary from builder.
```Dockerfile
# Copy only the binary from builder
COPY --from=builder /app/app-binary .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/rules ./rules
```

- 4 Expose port and run the binary.
```Dockerfile
EXPOSE 8080
CMD ["./app-binary"]
```
