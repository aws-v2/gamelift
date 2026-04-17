# Stage 1: Build
FROM golang:1.25.0-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go.mod and go.sum
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o gamelift-backend ./cmd/api

# Stage 2: Final
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Copy the binary from the builder stage
COPY --from=builder /app/gamelift-backend .

# Copy migrations
COPY --from=builder /app/migrations ./migrations

# Expose the application port
EXPOSE 8091

# Run the application
CMD ["./gamelift-backend"]
