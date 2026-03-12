#!/bin/bash
set -e

# SoulGate Demo Script
# This demonstrates the core functionality of SoulGate v0.1

echo "======================================"
echo "   SoulGate v0.1 Demo"
echo "======================================"
echo

# Navigate to demo workspace
cd "$(dirname "$0")/workspace"
SOULGATE="../../bin/soulgate"

echo "1. Show policy configuration"
echo "------------------------------"
$SOULGATE policy show | head -20
echo
read -p "Press Enter to continue..."
echo

echo "2. List installed plugins"
echo "------------------------------"
$SOULGATE plugin list
echo
read -p "Press Enter to continue..."
echo

echo "3. Check audit log (should be empty initially)"
echo "------------------------------"
$SOULGATE audit tail --last 5
echo
read -p "Press Enter to continue..."
echo

echo "4. Run a test prompt (no model provider configured)"
echo "------------------------------"
echo "Note: This will demonstrate the orchestrator without actual model calls"
echo "Prompt: 'List files in the workspace'"
echo
$SOULGATE run "List files in the workspace" || true
echo
read -p "Press Enter to continue..."
echo

echo "5. View audit log after run"
echo "------------------------------"
$SOULGATE audit tail --last 10
echo

echo "======================================"
echo "   Demo Complete!"
echo "======================================"
echo
echo "To use with a real LLM provider:"
echo "  export OPENAI_API_KEY=sk-..."
echo "  $SOULGATE run \"Read example.txt and summarize it\""
echo
echo "Files created:"
echo "  - .soulgate/config.yml   (configuration)"
echo "  - .soulgate/policy.yml   (security policies)"
echo "  - .soulgate/audit.db     (audit log)"
echo "  - plugins/               (plugin directory)"
echo
