#!/bin/bash
set -e

REQUESTS=${1:-50000}
CONCURRENCY=${2:-100}

echo "=========================================="
echo "⚡ Starting 'hey' Benchmark against Token Verification"
echo "Requests:    $REQUESTS"
echo "Concurrency: $CONCURRENCY workers"
echo "=========================================="

# 1. Elevate ulimit if permitted
ulimit -n 65535 2>/dev/null || true

# 2. Check health
HEALTH=$(curl -s http://localhost:3000/health | jq -r .status 2>/dev/null || echo "down")
if [ "$HEALTH" != "ok" ]; then
    echo "❌ Error: Service is not reachable at http://localhost:3000/health"
    exit 1
fi

# 3. Authenticate to get fresh JWT token
echo "Authenticating as bench_user..."
LOGIN_JSON=$(curl -s -X POST http://localhost:3000/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "bench_user", "password": "Password123!"}')

TOKEN=$(echo "$LOGIN_JSON" | jq -r .access_token)
if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    echo "❌ Error: Failed to acquire access token. Response: $LOGIN_JSON"
    exit 1
fi
echo "✅ Token acquired! Starting benchmark..."
echo ""

# 4. Run hey load test
~/go/bin/hey -n "$REQUESTS" -c "$CONCURRENCY" \
  -H "Authorization: Bearer $TOKEN" \
  http://localhost:3000/api/auth/verify
