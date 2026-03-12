#!/bin/bash

# Test script to fetch actual Google Gemini models from the API

if [ -z "$GOOGLE_API_KEY" ]; then
    echo "❌ GOOGLE_API_KEY not set"
    echo "Please run: export GOOGLE_API_KEY=\"your-key\""
    exit 1
fi

echo "🔍 Fetching models from Google Gemini API..."
echo ""

# Google uses a different endpoint structure
curl -s "https://generativelanguage.googleapis.com/v1beta/models?key=$GOOGLE_API_KEY" | jq '.models[] | {name: .name, displayName: .displayName, description: .description}'

echo ""
echo "✅ Done! Check the model names above"
