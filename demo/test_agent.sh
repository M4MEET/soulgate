#!/bin/bash
# Test script for Gateway + Observer + Agent

set -e

echo "🧪 Testing Complete Gateway Architecture"
echo "========================================"

# Check for API key
if [ -z "$OPENAI_API_KEY" ] && [ -z "$ANTHROPIC_API_KEY" ]; then
    echo "❌ Error: No API key found"
    echo "Set OPENAI_API_KEY or ANTHROPIC_API_KEY environment variable"
    exit 1
fi

# Build first
echo "📦 Building..."
make build

# Start Gateway in background
echo ""
echo "🚀 Starting Gateway..."
./bin/soulgate gateway start --port 8080 > /tmp/gateway.log 2>&1 &
GATEWAY_PID=$!

# Wait for Gateway to start
sleep 2

if ! kill -0 $GATEWAY_PID 2>/dev/null; then
    echo "❌ Gateway failed to start"
    cat /tmp/gateway.log
    exit 1
fi

echo "✓ Gateway started (PID: $GATEWAY_PID)"

# Start Observer in background
echo ""
echo "👀 Starting Observer..."
./bin/soulgate observe --gateway ws://localhost:8080/ws > /tmp/observer.log 2>&1 &
OBSERVER_PID=$!

sleep 1

if ! kill -0 $OBSERVER_PID 2>/dev/null; then
    echo "❌ Observer failed to start"
    kill $GATEWAY_PID 2>/dev/null
    exit 1
fi

echo "✓ Observer started (PID: $OBSERVER_PID)"

# Start Agent in background
echo ""
echo "🤖 Starting Agent..."
./bin/soulgate agent start --gateway ws://localhost:8080/ws > /tmp/agent.log 2>&1 &
AGENT_PID=$!

sleep 2

if ! kill -0 $AGENT_PID 2>/dev/null; then
    echo "❌ Agent failed to start"
    cat /tmp/agent.log
    kill $GATEWAY_PID $OBSERVER_PID 2>/dev/null
    exit 1
fi

echo "✓ Agent started (PID: $AGENT_PID)"

# Check health
echo ""
echo "🏥 Testing health endpoint..."
HEALTH=$(curl -s http://localhost:8080/health)
echo "Response: $HEALTH"

if echo "$HEALTH" | grep -q "healthy"; then
    echo "✓ Gateway is healthy"
else
    echo "❌ Health check failed"
    kill $GATEWAY_PID $OBSERVER_PID $AGENT_PID 2>/dev/null
    exit 1
fi

# Show component status
echo ""
echo "✅ Complete Gateway Architecture is Running!"
echo ""
echo "Components:"
echo "  - Gateway:  http://localhost:8080 (PID: $GATEWAY_PID)"
echo "  - Observer: Watching events (PID: $OBSERVER_PID)"
echo "  - Agent:    Processing messages (PID: $AGENT_PID)"
echo ""
echo "Logs:"
echo "  - Gateway:  tail -f /tmp/gateway.log"
echo "  - Observer: tail -f /tmp/observer.log"
echo "  - Agent:    tail -f /tmp/agent.log"
echo ""
echo "Next: Add a Connector (Telegram) to complete the loop!"
echo ""
echo "Press Ctrl+C to stop all components..."

# Wait for interrupt
trap "echo ''; echo 'Stopping all components...'; kill $GATEWAY_PID $OBSERVER_PID $AGENT_PID 2>/dev/null; exit 0" INT TERM

# Keep script running
wait
