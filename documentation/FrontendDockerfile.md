# Frontend Dockerfile

**Lives** `./frontend/doctooladmin/Dockerfile`

## Dockerfile Flow til to Deployment

# Build stage
- 1 Set environment OS and working directory as well as copy package files
```Dockerfile
FROM node:20-alpine AS build
WORKDIR /app
COPY package*.json ./
```

- 2 Initate build and run of npm packages
```Dockerfile
RUN npm ci
COPY . .
RUN npm run build
```

# Production stage

- 1 Set environment OS and working directory as well as copy package files
```Dockerfile
FROM node:20-alpine
WORKDIR /app
```

- 2 Copy build output and package files
```Dockerfile
COPY --from=build /app/build ./build
COPY --from=build /app/package*.json ./
```

- 3 Install production dependencies and set up the entrypoint
```Dockerfile
RUN npm ci --omit=dev
```

- 4 Expose port and set up the entrypoint
```Dockerfile
EXPOSE 3000
CMD ["node", "build"]
```
