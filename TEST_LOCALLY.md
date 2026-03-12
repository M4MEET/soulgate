# 🧪 Testing SoulGate Locally

This guide shows how to test SoulGate in a different folder to simulate a real user experience.

---

## Option 1: Install to System (Recommended)

This makes `soulgate` available everywhere, just like a real installation.

```bash
# From the SoulGate project directory
make install-cli

# This installs to /usr/local/bin/soulgate
# You'll need to enter your password (sudo)
```

**Then test anywhere:**

```bash
# Go to any directory
cd ~/test-soulgate-demo

# Run soulgate
soulgate

# It will work just like a real installation!
```

**To uninstall:**
```bash
sudo rm /usr/local/bin/soulgate
```

---

## Option 2: Copy Binary to Test Directory

Test without installing system-wide.

```bash
# From the SoulGate project directory
make build-cli

# Copy to test directory
cp bin/soulgate ~/test-soulgate-demo/

# Go to test directory
cd ~/test-soulgate-demo

# Run it
./soulgate
```

---

## Option 3: Use from Anywhere with PATH

Add the bin directory to your PATH temporarily.

```bash
# From the SoulGate project directory
make build-cli

# Add to PATH (temporary, for this terminal session only)
export PATH="/Users/demon/soulGate/bin:$PATH"

# Go to any directory
cd ~/test-soulgate-demo

# Run soulgate
soulgate

# Works anywhere!
```

---

## Option 4: Create a Test Script

Create a helper script for easy testing.

```bash
# From the SoulGate project directory
cat > test-demo.sh << 'EOF'
#!/bin/bash

# Build SoulGate
echo "Building SoulGate..."
make build-cli

# Create fresh test directory
TEST_DIR=~/test-soulgate-demo-$(date +%s)
mkdir -p "$TEST_DIR"
echo "Created test directory: $TEST_DIR"

# Create some sample files for testing
cat > "$TEST_DIR/README.md" << 'SAMPLE'
# Sample Project

This is a test project for SoulGate demo.
SAMPLE

cat > "$TEST_DIR/main.go" << 'SAMPLE'
package main

import "fmt"

func main() {
    fmt.Println("Hello, SoulGate!")
}
SAMPLE

# Copy binary
cp bin/soulgate "$TEST_DIR/"

# Navigate to test directory
cd "$TEST_DIR"

echo ""
echo "================================================"
echo "Test directory ready: $TEST_DIR"
echo "Sample files created:"
ls -la
echo "================================================"
echo ""
echo "Running SoulGate interactive terminal..."
echo ""

# Run SoulGate
./soulgate
EOF

chmod +x test-demo.sh
```

**Then run:**

```bash
./test-demo.sh
```

---

## Full Test Scenario

Here's a complete test scenario:

### Step 1: Build and Install

```bash
# From SoulGate project directory
cd /Users/demon/soulGate

# Build the CLI
make build-cli

# Install system-wide (recommended for testing)
make install-cli
```

### Step 2: Create Test Project

```bash
# Create a fresh test directory
mkdir -p ~/test-project
cd ~/test-project

# Create some sample files
cat > README.md << 'EOF'
# My Test Project

This is a test project for SoulGate.
EOF

cat > main.go << 'EOF'
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
EOF

mkdir -p src tests docs

# Check files
ls -la
```

### Step 3: Run SoulGate

```bash
# Start interactive terminal
soulgate
```

**You'll see:**

```
╔═══════════════════════════════════════════════════════╗
║     🤖 SoulGate Interactive AI Terminal 🤖           ║
╚═══════════════════════════════════════════════════════╝

📝 First time? Let me help you set up SoulGate...

Creating workspace...
✅ Workspace created!

Which AI provider do you want to use?
  1) OpenAI (GPT-4, GPT-3.5)
  2) Anthropic (Claude)
  3) Ollama (Local models)

Choice [1-3]:
```

### Step 4: Test Commands

Once in interactive mode, try:

```
You: status
# Shows workspace status

You: agents
# Shows available agents

You: help
# Shows all commands

You: clear
# Clears screen

You: exit
# Exits
```

---

## Quick Commands Reference

```bash
# Build
make build-cli

# Install system-wide
make install-cli

# Build for all platforms
make build-all

# Create release assets
make release

# Check version
soulgate --version

# Show help
soulgate --help

# Run interactive terminal
soulgate

# Run specific command
soulgate status

# Quick setup wizard
soulgate setup
```

---

## Simulating First-Time User Experience

To test the full first-time user experience:

```bash
# 1. Install SoulGate
cd /Users/demon/soulGate
make install-cli

# 2. Create a completely fresh directory
mkdir -p ~/brand-new-project
cd ~/brand-new-project

# 3. Run soulgate (first time)
soulgate

# This will:
# - Show welcome banner
# - Detect no workspace exists
# - Run auto-setup wizard
# - Ask for AI provider
# - Ask for API key
# - Create .soulgate/ directory
# - Start interactive terminal
```

---

## Testing Without API Key

If you don't have an API key yet:

```bash
# When prompted for API key, just press Enter to skip
Enter your API key (or press Enter to skip): [press Enter]

# You'll still get the interactive terminal
# It will be in demo mode with keyword-based responses
```

---

## Cleanup After Testing

```bash
# Remove test directory
rm -rf ~/test-soulgate-demo

# Uninstall SoulGate (if installed system-wide)
sudo rm /usr/local/bin/soulgate

# Remove workspace from test directory
cd ~/test-project
rm -rf .soulgate
```

---

## Common Issues

### "Command not found: soulgate"

**Solution:**
```bash
# Make sure it's installed
which soulgate

# If not found, install it
cd /Users/demon/soulGate
make install-cli

# Or add to PATH
export PATH="/Users/demon/soulGate/bin:$PATH"
```

### "Permission denied"

**Solution:**
```bash
# Make sure binary is executable
chmod +x bin/soulgate

# Or if copied to test directory
chmod +x ./soulgate
```

### "Workspace not initialized"

**This is normal!** SoulGate will automatically run setup wizard on first run.

---

## Pro Tips

1. **Test in a Docker Container** (Ultimate isolation):
   ```bash
   docker run -it --rm -v /Users/demon/soulGate/bin/soulgate:/usr/local/bin/soulgate golang:1.21 bash
   # Inside container:
   soulgate
   ```

2. **Test Different Scenarios**:
   - Empty directory (no files)
   - Go project (with go.mod)
   - Python project (with requirements.txt)
   - Mixed language project

3. **Test Configuration**:
   ```bash
   # Start fresh each time
   rm -rf .soulgate
   soulgate
   ```

4. **Watch Logs**:
   ```bash
   # If audit is enabled, check logs
   cat .soulgate/audit.db | sqlite3
   ```

---

## Next: Share with Others

Once you're happy with testing:

```bash
# Create release
make release

# Share the install script
# Others can install with:
# curl -fsSL https://raw.githubusercontent.com/M4MEET/soulgate/main/install.sh | bash
```

---

**Happy Testing!** 🧪
