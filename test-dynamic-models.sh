#!/bin/bash
# Test dynamic model discovery

echo "Testing dynamic model discovery..."
echo ""

# Test 1: With real OpenAI API key (if available)
if [ -n "$OPENAI_API_KEY" ]; then
    echo "✓ Found OPENAI_API_KEY - testing real API discovery"
    echo "/model" | timeout 3 ./bin/soulgate chat 2>&1 | grep -A 20 "Select LLM Provider"
else
    echo "⚠ No OPENAI_API_KEY - testing with fallback to static list"
    echo "/model" | timeout 3 ./bin/soulgate chat 2>&1 | grep -A 20 "Select LLM Provider"
fi

echo ""
echo "✓ Test complete!"
