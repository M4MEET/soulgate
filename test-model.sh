#!/bin/bash
export OPENAI_API_KEY="test-key"
echo "/model" | timeout 2 ./bin/soulgate chat 2>&1 | tail -20
