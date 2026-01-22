# =============================================================================
# TerraCost Multi-Stage Dockerfile
# =============================================================================
# Stages:
#   1. deps     - Download Go dependencies
#   2. build    - Build the CLI binary
#   3. cli      - Minimal CLI image
#   4. api      - API server image (for Decision Plane)
# =============================================================================

# -----------------------------------------------------------------------------
# Stage 1: Dependencies
# -----------------------------------------------------------------------------
FROM golang:1.22-alpine AS deps

WORKDIR /app

# Install git for private modules (if needed)
RUN apk add --no-cache git ca-certificates

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# -----------------------------------------------------------------------------
# Stage 2: Build
# -----------------------------------------------------------------------------
FROM deps AS build

# Copy source code
COPY . .

# Build arguments for versioning
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# Build CLI binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE}" \
    -o /terracost \
    ./cmd/terracost

# -----------------------------------------------------------------------------
# Stage 3: CLI Image (Default)
# -----------------------------------------------------------------------------
FROM alpine:3.19 AS cli

# Install CA certificates and tzdata for HTTPS and timezone
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -u 1000 terracost
USER terracost

# Copy binary
COPY --from=build /terracost /usr/local/bin/terracost

# Working directory for plans and backups
WORKDIR /workspace

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD terracost --version || exit 1

ENTRYPOINT ["terracost"]
CMD ["--help"]

# -----------------------------------------------------------------------------
# Stage 4: API Server Image
# -----------------------------------------------------------------------------
FROM alpine:3.19 AS api

# Install CA certificates and tzdata
RUN apk add --no-cache ca-certificates tzdata curl

# Create non-root user
RUN adduser -D -u 1000 terracost
USER terracost

# Copy binary
COPY --from=build /terracost /usr/local/bin/terracost

# Expose API port
EXPOSE 8080

# Working directory
WORKDIR /app

# Health check for API
HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Default to API server mode (placeholder - API not yet implemented)
ENTRYPOINT ["terracost"]
CMD ["api", "serve", "--port", "8080"]
