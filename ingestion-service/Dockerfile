# ---------- BUILD STAGE ----------
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy the entire workspace to ensure go.work resolution works correctly
# .dockerignore should be used to exclude .git, node_modules etc.
COPY . .

# Build the specific service
# We need to be in the root for go.work to function, but point to the main.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/ingestion-service ./ingestion-service/main.go

# ---------- RUNTIME STAGE ----------
FROM gcr.io/distroless/static-debian12

WORKDIR /app
COPY --from=builder /bin/ingestion-service /app/ingestion-service

EXPOSE 8081

# Must use shell form or explicit /app path if not in path (distroless has no shell)
ENTRYPOINT ["/app/ingestion-service"]
