#!/bin/bash
# Complete end-to-end test: Gateway + Observer + Agent + Telegram Connector

set -e

echo "🎉 Testing COMPLETE SoulGate Architecture"
echo "=========================================="
echo ""

# Check for API key
if [ -z "$OPENAI_API_KEY" ] && [ -z "$ANTHROPIC_API_KEY" ]; then
    echo "❌ Error: No AI API key found"
    echo "Set OPENAI_API_KEY or ANTHROPIC_API_KEY environment variable"
    exit 1
fi

# Check for Telegram bot token
if [ -z "$TELEGRAM_BOT_TOKEN" ]; then
    echo "❌ Error: TELEGRAM_BOT_TOKEN not set"
    echo "Get a bot token from @BotFather on Telegram"
    exit 1
fi

# Build first
echo "📦 Building..."
make build

echo ""
echo "Starting all components..."
echo ""

# Start Gateway
echo "🚀 Starting Gateway..."
./bin/soulgate gateway start --port 8080 > /tmp/soulgate-gateway.log 2>&1 &
GATEWAY_PID=$!
sleep 2

if ! kill -0 $GATEWAY_PID 2>/dev/null; then
    echo "❌ Gateway failed to start"
    cat /tmp/soulgate-gateway.log
    exit 1
fi
echo "   ✓ Gateway running (PID: $GATEWAY_PID)"

# Start Observer
echo "👀 Starting Observer..."
./bin/soulgate observe --gateway ws://localhost:8080/ws > /tmp/soulgate-observer.log 2>&1 &
OBSERVER_PID=$!
sleep 1

if ! kill -0 $OBSERVER_PID 2>/dev/null; then
    echo "❌ Observer failed to start"
    kill $GATEWAY_PID 2>/dev/null
    exit 1
fi
echo "   ✓ Observer running (PID: $OBSERVER_PID)"

# Start Agent
echo "🤖 Starting Agent..."
./bin/soulgate agent start --gateway ws://localhost:8080/ws > /tmp/soulgate-agent.log 2>&1 &
AGENT_PID=$!
sleep 2

if ! kill -0 $AGENT_PID 2>/dev/null; then
    echo "❌ Agent failed to start"
    cat /tmp/soulgate-agent.log
    kill $GATEWAY_PID $OBSERVER_PID 2>/dev/null
    exit 1
fi
echo "   ✓ Agent running (PID: $AGENT_PID)"

# Start Telegram Connector
echo "📱 Starting Telegram Connector..."
./bin/soulgate connector telegram --gateway ws://localhost:8080/ws > /tmp/soulgate-telegram.log 2>&1 &
TELEGRAM_PID=$!
sleep 2

if ! kill -0 $TELEGRAM_PID 2>/dev/null; then
    echo "❌ Telegram Connector failed to start"
    cat /tmp/soulgate-telegram.log
    kill $GATEWAY_PID $OBSERVER_PID $AGENT_PID 2>/dev/null
    exit 1
fi
echo "   ✓ Telegram Connector running (PID: $TELEGRAM_PID)"

# Health check
echo ""
echo "🏥 Health Check..."
HEALTH=$(curl -s http://localhost:8080/health)
echo "   Response: $HEALTH"

if echo "$HEALTH" | grep -q "healthy"; then
    CLIENTS=$(echo "$HEALTH" | grep -o '"clients":[0-9]*' | grep -o '[0-9]*')
    echo "   ✓ Gateway is healthy ($CLIENTS clients connected)"
else
    echo "   ❌ Health check failed"
    kill $GATEWAY_PID $OBSERVER_PID $AGENT_PID $TELEGRAM_PID 2>/dev/null
    exit 1
fi

echo ""
echo "╔════════════════════════════════════════════════════════╗"
echo "║  ✅ COMPLETE SOULGATE ARCHITECTURE IS RUNNING!        ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""
echo "Components:"
echo "  🌐 Gateway:   http://localhost:8080 (PID: $GATEWAY_PID)"
echo "  👀 Observer:  Watching all events  (PID: $OBSERVER_PID)"
echo "  🤖 Agent:     AI brain ready       (PID: $AGENT_PID)"
echo "  📱 Telegram:  Bot listening        (PID: $TELEGRAM_PID)"
echo ""
echo "Logs:"
echo "  Gateway:   tail -f /tmp/soulgate-gateway.log"
echo "  Observer:  tail -f /tmp/soulgate-observer.log"
echo "  Agent:     tail -f /tmp/soulgate-agent.log"
echo "  Telegram:  tail -f /tmp/soulgate-telegram.log"
echo ""
echo "╔════════════════════════════════════════════════════════╗"
echo "║  🎯 NOW SEND A MESSAGE TO YOUR TELEGRAM BOT!          ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""
echo "What will happen:"
echo "  1. You send message to Telegram bot"
echo "  2. Telegram Connector → Gateway (event.message)"
echo "  3. Gateway → Agent (event.message)"
echo "  4. Agent processes with AI + executes tools"
echo "  5. Agent → Gateway (cmd.channel.send)"
echo "  6. Gateway → Telegram Connector (cmd.channel.send)"
echo "  7. Telegram bot sends response to you"
echo "  8. Observer shows EVERYTHING in real-time! 🎉"
echo ""
echo "Try saying:"
echo "  - \"Hello\""
echo "  - \"Read the README.md file\""
echo "  - \"List files in the current directory\""
echo ""
echo "Press Ctrl+C to stop all components..."
echo ""

# Cleanup function
cleanup() {
    echo ""
    echo "🛑 Stopping all components..."
    kill $GATEWAY_PID $OBSERVER_PID $AGENT_PID $TELEGRAM_PID 2>/dev/null
    sleep 1
    echo "✅ All components stopped"
    exit 0
}

# Setup trap
trap cleanup INT TERM

# Keep script running
wait
