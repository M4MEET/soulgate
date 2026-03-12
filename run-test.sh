#!/bin/bash

echo "╔═══════════════════════════════════════════════════════╗"
echo "║          SoulGate Local Test                         ║"
echo "╚═══════════════════════════════════════════════════════╝"
echo ""
echo "Test directory: ~/test-soulgate-demo"
echo ""
echo "Files in test directory:"
ls -la ~/test-soulgate-demo
echo ""
echo "================================================"
echo "Now running SoulGate in test directory..."
echo "================================================"
echo ""
echo "Try these commands once it starts:"
echo "  - status    (show workspace status)"
echo "  - agents    (list available agents)"
echo "  - help      (show all commands)"
echo "  - exit      (quit)"
echo ""
echo "Press Enter to start..."
read

cd ~/test-soulgate-demo
./soulgate
