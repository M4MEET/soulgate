#!/bin/bash
# SoulGate One-Command Installer
# Usage: curl -fsSL https://soulgate.io/install.sh | bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Detect OS and architecture
OS="$(uname -s)"
ARCH="$(uname -m)"

echo -e "${BLUE}"
echo "╔═══════════════════════════════════════════════════════╗"
echo "║          SoulGate Installation                        ║"
echo "╚═══════════════════════════════════════════════════════╝"
echo -e "${NC}"

# Function to print colored messages
print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Detect platform
print_info "Detecting platform..."
case "$OS" in
    Linux*)
        case "$ARCH" in
            x86_64) PLATFORM="linux-amd64" ;;
            aarch64|arm64) PLATFORM="linux-arm64" ;;
            *) print_error "Unsupported architecture: $ARCH"; exit 1 ;;
        esac
        ;;
    Darwin*)
        case "$ARCH" in
            x86_64) PLATFORM="darwin-amd64" ;;
            arm64) PLATFORM="darwin-arm64" ;;
            *) print_error "Unsupported architecture: $ARCH"; exit 1 ;;
        esac
        ;;
    MINGW*|MSYS*|CYGWIN*)
        PLATFORM="windows-amd64"
        print_warning "Windows detected. Please use WSL or download from releases page."
        exit 1
        ;;
    *)
        print_error "Unsupported OS: $OS"
        exit 1
        ;;
esac

print_success "Platform detected: $PLATFORM"

# Set installation directory
INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"

    # Add to PATH if not already there
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        print_warning "Adding $INSTALL_DIR to PATH"
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.bashrc"
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.zshrc" 2>/dev/null || true
    fi
fi

# Download SoulGate
VERSION="v0.1.0"
DOWNLOAD_URL="https://github.com/M4MEET/soulgate/releases/download/${VERSION}/soulgate-${VERSION}-${PLATFORM}"
BINARY_PATH="$INSTALL_DIR/soulgate"

print_info "Downloading SoulGate ${VERSION}..."

# Try curl first, fallback to wget
if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$DOWNLOAD_URL" -o "$BINARY_PATH"
elif command -v wget >/dev/null 2>&1; then
    wget -q "$DOWNLOAD_URL" -O "$BINARY_PATH"
else
    print_error "Neither curl nor wget found. Please install one of them."
    exit 1
fi

# Make executable
chmod +x "$BINARY_PATH"

print_success "SoulGate installed to $BINARY_PATH"

# Verify installation
if command -v soulgate >/dev/null 2>&1; then
    print_success "Installation verified!"
    echo ""
    soulgate --version
else
    print_warning "soulgate not found in PATH. You may need to restart your terminal."
    print_info "Or manually add to PATH: export PATH=\"$INSTALL_DIR:\$PATH\""
fi

echo ""
echo -e "${GREEN}╔═══════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║          Installation Complete! 🎉                    ║${NC}"
echo -e "${GREEN}╚═══════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${BLUE}🚀 Quick Start:${NC}"
echo ""
echo "   1. Start the interactive terminal:"
echo -e "      ${GREEN}soulgate${NC}"
echo ""
echo "   2. Or run setup wizard:"
echo -e "      ${GREEN}soulgate setup${NC}"
echo ""
echo "   3. Get help:"
echo -e "      ${GREEN}soulgate --help${NC}"
echo ""
echo -e "${YELLOW}📝 Note: You'll need an OpenAI or Anthropic API key.${NC}"
echo "   The interactive terminal will help you set it up!"
echo ""
