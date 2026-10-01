# check=skip=SecretsUsedInArgOrEnv
# ==============================================================================
# Stage 1: Build the Go Microservice Static Binary
# ==============================================================================
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
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/short-auth main.go

# ==============================================================================
# Stage 2: All-in-One Container (Keycloak Identity Provider + Go Auth Gateway)
# ==============================================================================
FROM keycloak/keycloak:latest

USER root
WORKDIR /app

# Copy compiled static Go binary from builder stage
COPY --from=builder /app/short-auth /app/short-auth

# Copy realm export file for automatic realm initialization
COPY realm-export.json /opt/keycloak/data/import/realm-export.json

# Copy and configure startup entrypoint script
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/short-auth /app/entrypoint.sh && \
    chown -R 1000:0 /app /opt/keycloak/data/import

# Set production environment defaults (ensures Gin runs in ReleaseMode)
ENV GIN_MODE=release \
    ENV=production \
    PORT=:3000 \
    DB_VENDOR=postgres \
    DB_SCHEMA=keycloak \
    DB_USERNAME=postgres \
    DB_URL=""

# Port 3000: Go Auth REST Gateway (Primary)
# Port 8080: Keycloak OIDC Provider / Admin Console
EXPOSE 3000 8080

USER 1000

ENTRYPOINT ["/app/entrypoint.sh"]
