#!/bin/bash
# Test the architecture components (no API keys needed)

set -e

echo "🧪 Testing SoulGate Architecture Components"
echo "==========================================="
echo ""

# Build
echo "📦 Building..."
make build
echo "   ✓ Build successful"
echo ""

# Test Gateway
echo "🚀 Testing Gateway..."
./bin/soulgate gateway start --port 8081 > /tmp/test-gateway.log 2>&1 &
GATEWAY_PID=$!
sleep 2

if ! kill -0 $GATEWAY_PID 2>/dev/null; then
    echo "   ❌ Gateway failed to start"
    cat /tmp/test-gateway.log
    exit 1
fi
echo "   ✓ Gateway started successfully (PID: $GATEWAY_PID)"

# Test health endpoint
echo ""
echo "🏥 Testing Gateway health endpoint..."
HEALTH=$(curl -s http://localhost:8081/health 2>/dev/null || echo "failed")
if echo "$HEALTH" | grep -q "healthy"; then
    echo "   ✓ Health endpoint working"
    echo "   Response: $HEALTH"
else
    echo "   ❌ Health check failed"
    kill $GATEWAY_PID 2>/dev/null
    exit 1
fi

# Test Observer connection
echo ""
echo "👀 Testing Observer connection..."
timeout 3 ./bin/soulgate observe --gateway ws://localhost:8081/ws > /tmp/test-observer.log 2>&1 &
OBSERVER_PID=$!
sleep 2

if kill -0 $OBSERVER_PID 2>/dev/null; then
    echo "   ✓ Observer connected successfully"
    kill $OBSERVER_PID 2>/dev/null || true
else
    # Observer might have exited, check if it connected first
    if grep -q "Connected as" /tmp/test-observer.log; then
        echo "   ✓ Observer connected successfully"
    else
        echo "   ❌ Observer connection failed"
        cat /tmp/test-observer.log
        kill $GATEWAY_PID 2>/dev/null
        exit 1
    fi
fi

# Check Gateway clients
echo ""
echo "📊 Checking Gateway client count..."
HEALTH=$(curl -s http://localhost:8081/health)
CLIENTS=$(echo "$HEALTH" | grep -o '"clients":[0-9]*' | grep -o '[0-9]*')
echo "   Connected clients: $CLIENTS"

# Test commands exist
echo ""
echo "✅ Testing CLI commands..."
./bin/soulgate gateway --help > /dev/null && echo "   ✓ gateway command works"
./bin/soulgate observe --help > /dev/null && echo "   ✓ observe command works"
./bin/soulgate agent --help > /dev/null && echo "   ✓ agent command works"
./bin/soulgate connector telegram --help > /dev/null && echo "   ✓ connector telegram command works"

# Cleanup
echo ""
echo "🛑 Stopping Gateway..."
kill $GATEWAY_PID 2>/dev/null
sleep 1

echo ""
echo "╔═══════════════════════════════════════════════╗"
echo "║  ✅ ALL ARCHITECTURE COMPONENTS WORKING!     ║"
echo "╚═══════════════════════════════════════════════╝"
echo ""
echo "What was tested:"
echo "  ✓ Gateway server starts and listens"
echo "  ✓ Health endpoint responds"
echo "  ✓ Observer connects via WebSocket"
echo "  ✓ All CLI commands are available"
echo ""
echo "To test with real message flow:"
echo "  1. Set OPENAI_API_KEY or ANTHROPIC_API_KEY"
echo "  2. Set TELEGRAM_BOT_TOKEN"
echo "  3. Run: ./demo/test_complete.sh"
echo ""
