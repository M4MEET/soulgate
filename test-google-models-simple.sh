#!/bin/bash

# Test script to fetch actual Google Gemini models (no jq required)

if [ -z "$GOOGLE_API_KEY" ]; then
    echo "❌ GOOGLE_API_KEY not set"
    echo "Please run: export GOOGLE_API_KEY=\"your-key\""
    exit 1
fi

echo "🔍 Fetching models from Google Gemini API..."
echo ""

# Google uses a different endpoint structure
curl -s "https://generativelanguage.googleapis.com/v1beta/models?key=$GOOGLE_API_KEY"

echo ""
echo ""
echo "✅ Done! Check the model names above"
echo ""
echo "Look for model IDs like: gemini-1.5-pro, gemini-2.0-flash-exp, etc."
