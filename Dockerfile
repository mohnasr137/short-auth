# ==========================================
# Stage 1: Build the Go Microservice Binary
# ==========================================
FROM golang:alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Cache dependencies layer
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Compile static, stripped binary (CGO disabled for zero external libc dependencies)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/auth-server main.go

# ==========================================
# Stage 2: Minimal Production Runtime
# ==========================================
FROM alpine:latest

WORKDIR /app

# Install CA certificates for secure TLS and wget for container health checks
RUN apk --no-cache add ca-certificates tzdata wget

# Security: Create and run under a non-privileged system user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy compiled binary from builder
COPY --from=builder /app/auth-server /app/auth-server

# Assign ownership to non-root user
RUN chown -R appuser:appgroup /app

USER appuser

# Expose HTTP port
EXPOSE 3000

# Container health probe
HEALTHCHECK --interval=10s --timeout=3s --start-period=30s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:3000/health || exit 1

ENTRYPOINT ["/app/auth-server"]
