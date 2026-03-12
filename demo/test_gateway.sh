#!/bin/bash
# Test script for Gateway and Observer

set -e

echo "🧪 Testing Gateway Architecture"
echo "================================"

# Build first
echo "📦 Building..."
make build

# Start Gateway in background
echo ""
echo "🚀 Starting Gateway..."
./bin/soulgate gateway start --port 8080 &
GATEWAY_PID=$!

# Wait for Gateway to start
sleep 2

# Check if Gateway is running
if ! kill -0 $GATEWAY_PID 2>/dev/null; then
    echo "❌ Gateway failed to start"
    exit 1
fi

echo "✓ Gateway started (PID: $GATEWAY_PID)"

# Test health endpoint
echo ""
echo "🏥 Testing health endpoint..."
HEALTH=$(curl -s http://localhost:8080/health)
echo "Response: $HEALTH"

if echo "$HEALTH" | grep -q "healthy"; then
    echo "✓ Gateway is healthy"
else
    echo "❌ Health check failed"
    kill $GATEWAY_PID
    exit 1
fi

# Start observer in background
echo ""
echo "👀 Starting Observer..."
./bin/soulgate observe --gateway ws://localhost:8080/ws &
OBSERVER_PID=$!

# Wait a bit
sleep 2

# Check if observer is connected
if kill -0 $OBSERVER_PID 2>/dev/null; then
    echo "✓ Observer connected"
else
    echo "❌ Observer failed to connect"
    kill $GATEWAY_PID
    exit 1
fi

# Keep running for demo
echo ""
echo "✅ Gateway Architecture is running!"
echo ""
echo "Components:"
echo "  - Gateway:  http://localhost:8080 (PID: $GATEWAY_PID)"
echo "  - Observer: Connected and watching (PID: $OBSERVER_PID)"
echo ""
echo "Press Ctrl+C to stop..."

# Wait for interrupt
trap "echo ''; echo 'Stopping...'; kill $GATEWAY_PID $OBSERVER_PID 2>/dev/null; exit 0" INT TERM

# Keep script running
wait
