# Multi-stage build for smaller production image
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary (CGO disabled for static binary, TARGETOS/TARGETARCH set by buildx)
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o pocketbase-stripe main.go

# === Production image ===
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /build/pocketbase-stripe /app/pocketbase-stripe

# Copy hooks and bootstrap files
COPY --from=builder /build/hooks /app/hooks
COPY --from=builder /build/pb_bootstrap /app/pb_bootstrap
COPY --from=builder /build/stripe_bootstrap /app/stripe_bootstrap

# === Environment Variables ===
# PocketBase server (same as Taskfile/process-compose)
# Secrets (STRIPE_*) should be passed via docker run --env-file .env
ENV PB_HOST=0.0.0.0
ENV PB_PORT=8090

# Expose the PocketBase port
EXPOSE ${PB_PORT}

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:${PB_PORT}/api/health || exit 1

# Run PocketBase (uses PB_HOST and PB_PORT env vars)
CMD /app/pocketbase-stripe serve --http "${PB_HOST}:${PB_PORT}"
