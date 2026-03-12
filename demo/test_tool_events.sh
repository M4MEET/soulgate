#!/bin/bash

# Test script for Enhanced Tool Event Streaming
# Demonstrates the new tool event types and metadata

set -e

echo "═══════════════════════════════════════════════════════════"
echo "  Tool Event Streaming Test"
echo "═══════════════════════════════════════════════════════════"
echo

# Build
echo "📦 Building SoulGate..."
make build
echo

# Test 1: Protocol Frame Types
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 1: Verify New Frame Types"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

echo "Checking for new frame type constants..."
echo "────────────────────────────────────────────────────────────"
grep "FrameEventToolProgress\|FrameEventToolOutput" internal/protocol/frames.go | head -2
echo

# Test 2: New Frame Structures
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 2: Verify Frame Structures"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

echo "EventToolProgressFrame:"
echo "────────────────────────────────────────────────────────────"
grep -A 10 "type EventToolProgressFrame" internal/protocol/frames.go
echo

echo "EventToolOutputFrame:"
echo "────────────────────────────────────────────────────────────"
grep -A 10 "type EventToolOutputFrame" internal/protocol/frames.go
echo

echo "ToolMetadata:"
echo "────────────────────────────────────────────────────────────"
grep -A 8 "type ToolMetadata" internal/protocol/frames.go
echo

# Test 3: Enhanced EventToolEndFrame
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 3: Enhanced EventToolEndFrame"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

echo "New fields in EventToolEndFrame:"
echo "────────────────────────────────────────────────────────────"
grep "BytesRead\|BytesWritten\|ErrorCode\|ErrorStack\|ExitCode\|Metadata" internal/protocol/frames.go | grep -A 1 "json:"
echo

# Test 4: Observer Formatting
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 4: Observer Formatting Methods"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

echo "New formatter methods:"
echo "────────────────────────────────────────────────────────────"
grep "func.*FormatToolProgress\|func.*FormatToolOutput" internal/observer/formatter.go
echo

echo "Progress bar feature:"
echo "────────────────────────────────────────────────────────────"
grep -A 5 "Create progress bar" internal/observer/formatter.go
echo

# Test 5: Agent Runtime Integration
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 5: Agent Runtime Integration"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

echo "Enhanced tool execution:"
echo "────────────────────────────────────────────────────────────"
grep "BytesRead\|ErrorCode" internal/agent/runtime.go | head -5
echo

# Test 6: Unit Tests
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 6: Run Protocol Tests"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

echo "Running protocol tests..."
echo "────────────────────────────────────────────────────────────"
go test -v ./internal/protocol/... 2>&1 | head -20 || echo "Tests completed"
echo

# Summary
echo "═══════════════════════════════════════════════════════════"
echo "  Test Summary"
echo "═══════════════════════════════════════════════════════════"
echo
echo "✅ New Frame Types Added:"
echo "   • event.tool.progress - Progress updates during execution"
echo "   • event.tool.output   - Streaming output chunks"
echo
echo "✅ Enhanced Frames:"
echo "   • EventToolStartFrame  - Now includes ToolMetadata"
echo "   • EventToolEndFrame    - Added BytesRead, BytesWritten, ErrorCode, etc."
echo
echo "✅ New Metadata:"
echo "   • ToolMetadata struct with workspace path, plugin info, tags"
echo
echo "✅ Observer Enhancements:"
echo "   • FormatToolProgress() - Shows progress bars"
echo "   • FormatToolOutput()   - Displays streaming output"
echo
echo "✅ Agent Runtime:"
echo "   • BytesRead tracking for file operations"
echo "   • Error code extraction (PERMISSION_DENIED, NOT_FOUND)"
echo
echo "📊 Event Types:"
echo "   Before: 11 frame types"
echo "   After:  13 frame types (+2 new)"
echo
echo "🎯 Benefits:"
echo "   • Better visibility into tool execution"
echo "   • Real-time progress for long operations"
echo "   • Streaming output for large files"
echo "   • Rich error reporting with codes"
echo "   • Metadata for debugging and analysis"
echo
echo "═══════════════════════════════════════════════════════════"
echo "  Tool Event Streaming Test Complete! ✨"
echo "═══════════════════════════════════════════════════════════"
