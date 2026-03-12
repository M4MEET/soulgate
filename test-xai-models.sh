#!/bin/bash

# Test script to fetch actual xAI models from the API

if [ -z "$XAI_API_KEY" ]; then
    echo "❌ XAI_API_KEY not set"
    echo "Please run: export XAI_API_KEY=\"your-key\""
    exit 1
fi

echo "🔍 Fetching models from xAI API..."
echo ""

curl -s https://api.x.ai/v1/models \
  -H "Authorization: Bearer $XAI_API_KEY" \
  -H "Content-Type: application/json" | jq '.'

echo ""
echo "✅ Done! Check the model IDs above"
