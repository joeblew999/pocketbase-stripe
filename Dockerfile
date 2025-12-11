# Multi-stage build for smaller production image
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o pocketbase-stripe main.go

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
ENV PB_HOST=0.0.0.0
ENV PB_PORT=8090

# Stripe (set via docker run -e or .env file)
ENV STRIPE_SECRET_KEY=""
ENV STRIPE_WHSEC=""
ENV STRIPE_SUCCESS_URL=""
ENV STRIPE_CANCEL_URL=""
ENV STRIPE_BILLING_RETURN_URL=""

# Development mode
ENV DEVELOPMENT=""

# Expose the PocketBase port
EXPOSE ${PB_PORT}

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:${PB_PORT}/api/health || exit 1

# Run PocketBase
CMD ["/app/pocketbase-stripe", "serve", "--http", "0.0.0.0:8090"]
