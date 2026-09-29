#!/bin/bash
set -e

# ==============================================================================
# 1. Validate All Required Database Configuration
# ==============================================================================
MISSING_VARS=()

if [ -z "$DB_URL" ]; then
  MISSING_VARS+=("❌ DB_URL      -> Missing! (e.g. jdbc:postgresql://host.docker.internal:5432/my_database)")
fi

if [ -z "$DB_USERNAME" ]; then
  MISSING_VARS+=("❌ DB_USERNAME -> Missing! (e.g. postgres)")
fi

if [ -z "$DB_PASSWORD" ]; then
  MISSING_VARS+=("❌ DB_PASSWORD -> Missing! (your database password)")
fi

if [ ${#MISSING_VARS[@]} -ne 0 ]; then
  echo "================================================================================"
  echo "🚨 CONFIGURATION ERROR: Required database variables are missing!"
  echo "================================================================================"
  echo ""
  echo "The following required variables were NOT provided:"
  for var in "${MISSING_VARS[@]}"; do
    echo "  $var"
  done
  echo ""
  echo "👉 How to fix:"
  echo ""
  echo "1. In Docker Desktop:"
  echo "   Click 'Optional settings' -> 'Environment variables' and set:"
  echo "     - DB_URL"
  echo "     - DB_USERNAME"
  echo "     - DB_PASSWORD"
  echo ""
  echo "2. In Terminal (Docker CLI):"
  echo "   docker run -p 3000:3000 \\"
  echo "     -e DB_URL=jdbc:postgresql://host.docker.internal:5432/my_database \\"
  echo "     -e DB_USERNAME=your_username \\"
  echo "     -e DB_PASSWORD=your_password \\"
  echo "     auth-service:latest"
  echo "================================================================================"
  exit 1
fi

# ==============================================================================
# 2. Configure Keycloak Database & Networking
# ==============================================================================
export KC_DB="${DB_VENDOR:-postgres}"
export KC_DB_URL="$DB_URL"
export KC_DB_USERNAME="${DB_USERNAME:-postgres}"
export KC_DB_PASSWORD="${DB_PASSWORD:-postgres}"
export KC_DB_SCHEMA="${DB_SCHEMA:-keycloak}"
export KC_HTTP_ENABLED="true"
export KC_HOSTNAME="${KEYCLOAK_PUBLIC_URL:-http://localhost:8080}"
export KC_HOSTNAME_BACKCHANNEL_DYNAMIC="true"
export KEYCLOAK_ADMIN="${KEYCLOAK_ADMIN:-admin}"
export KEYCLOAK_ADMIN_PASSWORD="${KEYCLOAK_ADMIN_PASSWORD:-admin}"

# ==============================================================================
# 3. Configure Go Auth Microservice
# ==============================================================================
export KEYCLOAK_BASE_URL="${KEYCLOAK_BASE_URL:-http://127.0.0.1:8080}"
export KEYCLOAK_PUBLIC_URL="${KEYCLOAK_PUBLIC_URL:-http://localhost:8080}"
export KEYCLOAK_ISSUER_URL="${KEYCLOAK_ISSUER_URL:-http://localhost:8080/realms/auth}"
export KEYCLOAK_REALM="${KEYCLOAK_REALM:-auth}"
export KEYCLOAK_CLIENT_ID="${KEYCLOAK_CLIENT_ID:-auth-backend}"
export KEYCLOAK_CLIENT_SECRET="${KEYCLOAK_CLIENT_SECRET:-Dr4G0j6vvfWMdvrKdHAYBrDNdmopUgkEF0YRZuWrWRZOv7RWnJqJ26MseNiPrXjDQDo0ZCiOGbdcHvu2cWNuk8}"
export PORT="${PORT:-:3000}"
export ENV="${ENV:-production}"
export GIN_MODE="${GIN_MODE:-release}"
export ALLOWED_ORIGINS="${ALLOWED_ORIGINS:-*}"

# ==============================================================================
# 4. Graceful Shutdown Signal Handler
# ==============================================================================
terminate_processes() {
  echo ""
  echo "🛑 Received termination signal. Stopping services..."
  if [ -n "$GO_PID" ]; then kill -TERM "$GO_PID" 2>/dev/null || true; fi
  if [ -n "$KC_PID" ]; then kill -TERM "$KC_PID" 2>/dev/null || true; fi
  wait "$GO_PID" "$KC_PID" 2>/dev/null || true
  echo "✅ All services stopped."
  exit 0
}
trap terminate_processes SIGTERM SIGINT

# ==============================================================================
# 5. Launch Keycloak Identity Provider
# ==============================================================================
echo "🚀 [All-in-One] Starting Keycloak Identity Provider (Database: $KC_DB)..."
/opt/keycloak/bin/kc.sh start-dev --import-realm --features=token-exchange,admin-fine-grained-authz &
KC_PID=$!

# ==============================================================================
# 6. Wait for Keycloak to Finish Starting Before Launching Backend
# ==============================================================================
MAX_ATTEMPTS=${KEYCLOAK_MAX_ATTEMPTS:-300}
echo "⏳ [All-in-One] Waiting for Keycloak to initialize on port 8080 (timeout: ${MAX_ATTEMPTS}s)..."
echo "ℹ️  [All-in-One] Note: On a new database, Keycloak performs initial table creation and migrations (~20-40s)."
ATTEMPT=1
until (echo > /dev/tcp/127.0.0.1/8080) 2>/dev/null; do
  if [ "$ATTEMPT" -ge "$MAX_ATTEMPTS" ]; then
    echo "❌ ERROR: Timed out waiting for Keycloak to start after ${MAX_ATTEMPTS} seconds."
    kill -TERM "$KC_PID" 2>/dev/null || true
    exit 1
  fi
  if [ $((ATTEMPT % 10)) -eq 0 ]; then
    echo "⏳ [All-in-One] Still applying database migrations & initializing Keycloak... (${ATTEMPT}s / ${MAX_ATTEMPTS}s elapsed)"
  fi
  sleep 1
  ATTEMPT=$((ATTEMPT + 1))
done

# Brief grace period for Quarkus realm context to finalize
sleep 2
echo "✅ Keycloak is online and ready!"

# ==============================================================================
# 7. Launch Go Auth Microservice Gateway
# ==============================================================================
echo "🚀 [All-in-One] Starting Go Auth Microservice Gateway on port $PORT..."
echo "🎉 Auth API Gateway live at http://localhost:${PORT#:}"
/app/auth-server &
GO_PID=$!

# Wait for either process to terminate
wait -n "$KC_PID" "$GO_PID"
