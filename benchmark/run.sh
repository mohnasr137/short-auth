#!/bin/bash
set -e

echo "=========================================="
echo "⚡ Starting 10,000 RPS Token Verification Benchmark"
echo "=========================================="

# 1. Check open file limits
CURRENT_ULIMIT=$(ulimit -n)
echo "Current ulimit (file descriptors): $CURRENT_ULIMIT"
if [ "$CURRENT_ULIMIT" -lt 10000 ]; then
    echo "Increasing ulimit -n to 65535..."
    ulimit -n 65535 2>/dev/null || true
fi

# 2. Verify Auth Service is responding
echo "Verifying Auth Service health..."
HEALTH_STATUS=$(curl -s http://localhost:3000/health | jq -r .status 2>/dev/null || echo "failed")
if [ "$HEALTH_STATUS" != "ok" ]; then
    echo "❌ Error: Auth service is not responding at http://localhost:3000/health"
    exit 1
fi
echo "✅ Auth Service is healthy!"

# 3. Run k6 Benchmark
echo ""
echo "🚀 Launching k6..."
k6 run --summary-export=benchmark/results_summary.json benchmark/k6_benchmark.js
echo "✅ Benchmark complete! Metrics saved to benchmark/results_summary.json and benchmark/RESULTS.md"
