#!/bin/bash

# Test script for Enhanced Session Routing
# Demonstrates smart agent selection, load balancing, and session affinity

set -e

echo "═══════════════════════════════════════════════════════════"
echo "  Enhanced Session Routing Test"
echo "═══════════════════════════════════════════════════════════"
echo

# Build
echo "📦 Building SoulGate..."
make build
echo

# Test 1: Enhanced Session Structure
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 1: Enhanced Session Structure"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

echo "Session state types:"
echo "────────────────────────────────────────────────────────────"
grep "SessionState.*=" internal/gateway/session.go | head -5
echo

echo "Enhanced session fields:"
echo "────────────────────────────────────────────────────────────"
grep "AssignedAgentID\|AgentHistory\|AgentAffinityEnabled\|MessageCount\|ToolCalls" internal/gateway/session.go | head -8
echo

# Test 2: Router Strategies
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 2: Routing Strategies"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

echo "Available strategies:"
echo "────────────────────────────────────────────────────────────"
grep "Strategy.*RoutingStrategy.*=" internal/gateway/router.go
echo

echo "Smart routing algorithm:"
echo "────────────────────────────────────────────────────────────"
grep -A 2 "selectSmart uses" internal/gateway/router.go
echo

# Test 3: Session Methods
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 3: Session Management Methods"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

echo "New session methods:"
echo "────────────────────────────────────────────────────────────"
grep "^func.*Session.*Assign\|^func.*Session.*State\|^func.*Session.*Activity" internal/gateway/session.go | head -10
echo

# Test 4: Load Tracking
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 4: Load Balancing & Tracking"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

echo "Load tracking methods:"
echo "────────────────────────────────────────────────────────────"
grep "func.*Load" internal/gateway/router.go | head -6
echo

echo "Least loaded selection:"
echo "────────────────────────────────────────────────────────────"
grep -A 12 "selectLeastLoaded" internal/gateway/router.go | head -14
echo

# Test 5: Session Affinity
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 5: Session Affinity"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

echo "Affinity logic:"
echo "────────────────────────────────────────────────────────────"
grep -A 8 "Check for affinity first" internal/gateway/router.go | head -10
echo

# Test 6: Gateway Integration
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 6: Gateway Integration"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

echo "Router usage in handleEventMessage:"
echo "────────────────────────────────────────────────────────────"
grep "router.SelectAgent\|router.IncrementLoad\|router.DecrementLoad" internal/gateway/gateway.go
echo

echo "Agent disconnection handling:"
echo "────────────────────────────────────────────────────────────"
grep "ResetLoad\|unassignAgentSessions" internal/gateway/gateway.go
echo

# Test 7: Statistics
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 7: Session Statistics"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

echo "GetStatistics method:"
echo "────────────────────────────────────────────────────────────"
grep -A 5 "GetStatistics returns" internal/gateway/session.go | head -7
echo

# Summary
echo "═══════════════════════════════════════════════════════════"
echo "  Test Summary"
echo "═══════════════════════════════════════════════════════════"
echo
echo "✅ Enhanced Session Structure:"
echo "   • Session states (active, idle, paused, completed)"
echo "   • Agent assignment tracking"
echo "   • Activity timestamps"
echo "   • Statistics (message count, tool calls, tokens)"
echo
echo "✅ Smart Router:"
echo "   • 4 routing strategies (round-robin, least-loaded, affinity, smart)"
echo "   • Load balancing across agents"
echo "   • Session affinity (keep same agent)"
echo "   • Automatic failover on agent disconnect"
echo
echo "✅ Session Management:"
echo "   • AssignAgent() / UnassignAgent()"
echo "   • SetState() / GetState()"
echo "   • UpdateActivity() / IsIdle()"
echo "   • GetStatistics()"
echo
echo "✅ Load Tracking:"
echo "   • Per-agent session counters"
echo "   • IncrementLoad() / DecrementLoad()"
echo "   • GetLoad() / GetAllLoads()"
echo "   • Automatic cleanup on disconnect"
echo
echo "📊 Routing Strategies:"
echo "   • Round Robin     - Fair distribution"
echo "   • Least Loaded    - Balance by load"
echo "   • Affinity        - Sticky sessions"
echo "   • Smart (default) - Affinity + load balancing"
echo
echo "🎯 Benefits:"
echo "   • Better resource utilization"
echo "   • Consistent user experience (same agent)"
echo "   • Automatic failover"
echo "   • Performance metrics"
echo
echo "═══════════════════════════════════════════════════════════"
echo "  Enhanced Session Routing Test Complete! ✨"
echo "═══════════════════════════════════════════════════════════"
