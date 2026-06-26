

# Build stage
FROM node:20-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

# Production stage
FROM node:20-alpine
WORKDIR /app
COPY --from=build /app/build ./build
COPY --from=build /app/package*.json ./
RUN npm ci --omit=dev

EXPOSE 3000
# Use absolute path — podman kube play doesn't always inherit the image ENV
# (including PATH) when no ENTRYPOINT is set, so 'node' alone won't resolve.
CMD ["/usr/local/bin/node", "build"]
