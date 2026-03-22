#!/bin/bash
# SoulGate Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/M4MEET/soulgate/main/install.sh | bash
set -e

REPO="M4MEET/soulgate"
BINARY="soulgate"

# Colors
C='\033[36m' G='\033[32m' Y='\033[33m' R='\033[31m' D='\033[2m' B='\033[1m' N='\033[0m'

echo -e "${C}${B}SoulGate${N} — Your AI, everywhere."
echo ""

# Detect OS/arch
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo -e "${R}Unsupported architecture: $ARCH${N}"; exit 1 ;;
esac
case "$OS" in
    linux|darwin) ;;
    mingw*|msys*|cygwin*) OS="windows" ;;
    *) echo -e "${R}Unsupported OS: $OS${N}"; exit 1 ;;
esac

echo -e "${D}Platform: ${OS}/${ARCH}${N}"

# Get latest version
echo -e "${D}Fetching latest release...${N}"
if command -v curl >/dev/null 2>&1; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
elif command -v wget >/dev/null 2>&1; then
    VERSION=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
else
    echo -e "${R}Neither curl nor wget found${N}"; exit 1
fi

if [ -z "$VERSION" ]; then
    VERSION="v1.0.0"
    echo -e "${Y}Could not detect latest version, using ${VERSION}${N}"
fi
echo -e "${D}Version: ${VERSION}${N}"

# Build download URL (goreleaser format)
ARCHIVE="soulgate_${OS}_${ARCH}.tar.gz"
if [ "$OS" = "windows" ]; then
    ARCHIVE="soulgate_${OS}_${ARCH}.zip"
fi
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

# Download
TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

echo -e "${D}Downloading ${URL}...${N}"
if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$URL" -o "$TMPDIR/$ARCHIVE"
else
    wget -q "$URL" -O "$TMPDIR/$ARCHIVE"
fi

# Extract
cd "$TMPDIR"
if [ "$OS" = "windows" ]; then
    unzip -q "$ARCHIVE"
else
    tar -xzf "$ARCHIVE"
fi

# Install
INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        for rc in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.profile"; do
            [ -f "$rc" ] && echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$rc"
        done
        export PATH="$INSTALL_DIR:$PATH"
    fi
fi

cp "$BINARY" "$INSTALL_DIR/$BINARY"
chmod +x "$INSTALL_DIR/$BINARY"

echo ""
echo -e "${G}✓ Installed to ${INSTALL_DIR}/${BINARY}${N}"

# Verify
if command -v soulgate >/dev/null 2>&1; then
    echo -e "${G}✓ $(soulgate --version 2>/dev/null || echo 'soulgate ready')${N}"
fi

echo ""
echo -e "  Get started:"
echo -e "    ${C}soulgate tui${N}              Interactive terminal"
echo -e "    ${C}soulgate gateway start${N}    Start gateway + web UI"
echo -e "    ${C}soulgate doctor${N}           Check installation"
echo ""
