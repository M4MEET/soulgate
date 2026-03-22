#!/usr/bin/env python3
"""Example SoulGate script plugin — reads JSON from stdin, echoes the message."""
import json
import sys

data = json.load(sys.stdin)
message = data.get("message", "")
print(json.dumps({"echoed": message, "status": "ok"}))
