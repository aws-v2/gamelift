# ── Stage 1: Builder ──────────────────────────────────────────────────────────
FROM golang:1.25-bookworm AS builder

# CGO is required for go-sqlite3 (fallback DB driver)
ENV CGO_ENABLED=1

WORKDIR /app

# Cache dependency downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy full source tree
COPY . .

# Build the API binary
RUN go build -ldflags="-s -w" -o /app/bin/api ./cmd/api

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM debian:bookworm-slim AS runtime

# Install minimal runtime deps (ca-certs for TLS, sqlite3 lib for CGO fallback)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libsqlite3-0 \
 && rm -rf /var/lib/apt/lists/*

# Non-root user for security
RUN useradd -r -u 1001 -UG 0 appuser

WORKDIR /app

# Copy compiled binary
COPY --from=builder /app/bin/api .

# Copy DB migration files
COPY --from=builder /app/migrations ./migrations

# Copy any static/media assets the app references at startup
COPY --from=builder /app/media ./media
COPY --from=builder /app/docs ./docs


# Writable directories (uploads & sqlite fallback live here)
RUN mkdir -p uploads && chown -R appuser:root /app

USER appuser

# Default env — override at runtime / in docker-compose / CI
ENV APP_ENV=production \
    SERVER_PORT=:8091 \
    SQLITE_PATH=/app/gamelift_fallback.db

EXPOSE 8091

ENTRYPOINT ["/app/api"]
