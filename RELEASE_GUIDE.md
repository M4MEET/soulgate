# 🚀 SoulGate Release Guide

This guide explains how to create and publish a new SoulGate release.

---

## Quick Release Process

### 1. Build Release Assets

```bash
# Build binaries for all platforms
make release
```

This creates:
- `dist/release/soulgate-v0.1.0-linux-amd64`
- `dist/release/soulgate-v0.1.0-linux-arm64`
- `dist/release/soulgate-v0.1.0-darwin-amd64`
- `dist/release/soulgate-v0.1.0-darwin-arm64`
- `dist/release/soulgate-v0.1.0-windows-amd64.exe`

### 2. Create GitHub Release

```bash
# Using GitHub CLI
gh release create v0.1.0 \
  --title "SoulGate v0.1.0" \
  --notes "Initial release with interactive AI terminal" \
  dist/release/*

# Or manually:
# 1. Go to https://github.com/M4MEET/soulgate/releases/new
# 2. Tag: v0.1.0
# 3. Title: SoulGate v0.1.0
# 4. Description: See below
# 5. Upload files from dist/release/
```

### 3. Verify Installation Works

```bash
# Test the install script
curl -fsSL https://raw.githubusercontent.com/M4MEET/soulgate/main/install.sh | bash

# Verify
soulgate --version
```

---

## Release Checklist

### Pre-Release

- [ ] All tests passing (`make test`)
- [ ] Security tests passing (`make test-security`)
- [ ] Coverage > 80% (`make test-coverage`)
- [ ] Version updated in `Makefile` (line 184)
- [ ] CHANGELOG.md updated
- [ ] Documentation reviewed

### Build & Test

- [ ] Build release assets (`make release`)
- [ ] Test binaries on each platform:
  - [ ] Linux AMD64
  - [ ] Linux ARM64
  - [ ] macOS Intel
  - [ ] macOS Apple Silicon
  - [ ] Windows AMD64

### Publish

- [ ] Create GitHub release with tag `v0.1.0`
- [ ] Upload all binaries from `dist/release/`
- [ ] Publish release notes
- [ ] Test install script works
- [ ] Update main README.md badge

### Post-Release

- [ ] Announce on Discord/Twitter
- [ ] Update Homebrew formula (if applicable)
- [ ] Monitor GitHub issues for installation problems
- [ ] Update website download links

---

## Release Notes Template

```markdown
# SoulGate v0.1.0 - Initial Release

## ✨ Highlights

- **Interactive AI Terminal**: Just run `soulgate` and chat naturally
- **One-Command Installation**: `curl -fsSL https://soulgate.io/install.sh | bash`
- **Security Built-In**: Policy-based controls and complete audit logging
- **3 Consolidated Agents**: Test & Quality, Docs & API, Project Management
- **Multi-Provider Support**: OpenAI, Anthropic, Ollama (local models)

## 🚀 Quick Start

### Installation

```bash
curl -fsSL https://raw.githubusercontent.com/M4MEET/soulgate/main/install.sh | bash
```

### Usage

```bash
# Start interactive terminal
soulgate

# Just chat naturally!
You: List all files in this directory
You: Run tests and show coverage
You: Generate API documentation
```

## 📦 Installation Options

### One-Line Install (macOS/Linux)
```bash
curl -fsSL https://raw.githubusercontent.com/M4MEET/soulgate/main/install.sh | bash
```

### Homebrew (macOS)
```bash
brew tap M4MEET/soulgate
brew install soulgate
```

### Download Binary
- [Linux AMD64](https://github.com/M4MEET/soulgate/releases/download/v0.1.0/soulgate-v0.1.0-linux-amd64)
- [Linux ARM64](https://github.com/M4MEET/soulgate/releases/download/v0.1.0/soulgate-v0.1.0-linux-arm64)
- [macOS Intel](https://github.com/M4MEET/soulgate/releases/download/v0.1.0/soulgate-v0.1.0-darwin-amd64)
- [macOS Apple Silicon](https://github.com/M4MEET/soulgate/releases/download/v0.1.0/soulgate-v0.1.0-darwin-arm64)
- [Windows AMD64](https://github.com/M4MEET/soulgate/releases/download/v0.1.0/soulgate-v0.1.0-windows-amd64.exe)

## 🆕 What's New

### Features

- **Interactive Terminal** - Chat naturally with AI in your terminal
- **Auto-Setup** - Guided onboarding on first run
- **Security Controls** - Policy-based file access with audit logging
- **Test & Quality Agent** - Generate tests, run coverage, security scans
- **Docs & API Agent** - Generate docs, API specs, changelogs
- **Project Management Agent** - Task assignment, sprint tracking, reports
- **Multi-Provider** - OpenAI, Anthropic, Ollama support

### Architecture

- 3 consolidated agents + 1 notification service
- Policy engine with glob pattern matching
- SQLite audit logging
- Plugin system (WASM runtime in progress)
- 45 tests passing with 85% coverage

## 📚 Documentation

- [Quick Start Guide](https://github.com/M4MEET/soulgate/blob/main/QUICKSTART_INTERACTIVE.md)
- [Interactive Demo](https://github.com/M4MEET/soulgate/blob/main/DEMO.md)
- [Security Guide](https://github.com/M4MEET/soulgate/blob/main/SECURITY.md)
- [CLI Reference](https://github.com/M4MEET/soulgate/blob/main/CLI_GUIDE.md)

## 🐛 Known Issues

- Full LLM integration in progress (demo mode for now)
- Plugin WASM runtime not yet complete
- Model adapters need completion

## 🔮 Coming Soon

- Full OpenAI/Anthropic integration
- WASM plugin runtime
- HTTP connector plugins
- Streaming output support
- Token counting and cost tracking
- More agent templates

## 💬 Support

- **Documentation**: https://github.com/M4MEET/soulgate/tree/main/docs
- **Issues**: https://github.com/M4MEET/soulgate/issues
- **Discussions**: https://github.com/M4MEET/soulgate/discussions

---

**Full Changelog**: https://github.com/M4MEET/soulgate/commits/v0.1.0
```

---

## Version Numbering

SoulGate follows Semantic Versioning (semver):

- **MAJOR.MINOR.PATCH** (e.g., 0.1.0)
- **MAJOR**: Breaking changes to CLI or agent interfaces
- **MINOR**: New features, backward compatible
- **PATCH**: Bug fixes, backward compatible

### Pre-1.0 Versions

- `v0.1.0` - Initial release with interactive terminal
- `v0.2.0` - Full LLM integration
- `v0.3.0` - Plugin runtime complete
- `v0.4.0` - Streaming output
- `v0.5.0` - Advanced security features
- `v1.0.0` - Production-ready, stable API

---

## Updating Version

### 1. Update Makefile

```makefile
# Line 184
VERSION := 0.2.0
```

### 2. Update install.sh

```bash
# Line 86
VERSION="v0.2.0"
```

### 3. Update root.go

```go
// cmd/soulgate/cmd/root.go line 24
Version: "0.2.0",
```

### 4. Update CHANGELOG.md

```markdown
## [0.2.0] - 2024-02-15

### Added
- Full OpenAI integration
- Streaming output support
```

---

## Testing Release Assets

### Linux

```bash
# Download
wget https://github.com/M4MEET/soulgate/releases/download/v0.1.0/soulgate-v0.1.0-linux-amd64
chmod +x soulgate-v0.1.0-linux-amd64
./soulgate-v0.1.0-linux-amd64 --version
```

### macOS

```bash
# Download
curl -L -o soulgate https://github.com/M4MEET/soulgate/releases/download/v0.1.0/soulgate-v0.1.0-darwin-arm64
chmod +x soulgate
./soulgate --version
```

### Windows

```powershell
# Download from GitHub releases page
# Or use curl:
curl -L -o soulgate.exe https://github.com/M4MEET/soulgate/releases/download/v0.1.0/soulgate-v0.1.0-windows-amd64.exe
.\soulgate.exe --version
```

---

## Homebrew Formula

Create `homebrew-soulgate/Formula/soulgate.rb`:

```ruby
class Soulgate < Formula
  desc "SoulGate - Interactive AI Terminal with Security"
  homepage "https://github.com/M4MEET/soulgate"
  version "0.1.0"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/M4MEET/soulgate/releases/download/v0.1.0/soulgate-v0.1.0-darwin-arm64"
    sha256 "..." # Calculate with: shasum -a 256 soulgate-v0.1.0-darwin-arm64
  elsif OS.mac?
    url "https://github.com/M4MEET/soulgate/releases/download/v0.1.0/soulgate-v0.1.0-darwin-amd64"
    sha256 "..." # Calculate with: shasum -a 256 soulgate-v0.1.0-darwin-amd64
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/M4MEET/soulgate/releases/download/v0.1.0/soulgate-v0.1.0-linux-arm64"
    sha256 "..." # Calculate with: shasum -a 256 soulgate-v0.1.0-linux-arm64
  else
    url "https://github.com/M4MEET/soulgate/releases/download/v0.1.0/soulgate-v0.1.0-linux-amd64"
    sha256 "..." # Calculate with: shasum -a 256 soulgate-v0.1.0-linux-amd64
  end

  def install
    bin.install "soulgate-v#{version}-#{OS.kernel_name.downcase}-#{Hardware::CPU.arch}" => "soulgate"
  end

  test do
    system "#{bin}/soulgate", "--version"
  end
end
```

### Publishing Homebrew Formula

```bash
# Create tap repository
gh repo create M4MEET/homebrew-soulgate --public

# Add formula
cd homebrew-soulgate
mkdir Formula
cp soulgate.rb Formula/
git add .
git commit -m "Add SoulGate formula v0.1.0"
git push

# Users can then install with:
# brew tap M4MEET/soulgate
# brew install soulgate
```

---

## Rollback Procedure

If a release has critical bugs:

### 1. Mark Release as Pre-release

Go to GitHub releases and edit to mark as "pre-release"

### 2. Revert Version in install.sh

```bash
# Update install.sh to point to previous version
VERSION="v0.0.9"  # Previous stable version
```

### 3. Publish Hotfix

```bash
# Increment patch version
VERSION := 0.1.1  # in Makefile

# Fix bug, test, rebuild
make release

# Create hotfix release
gh release create v0.1.1 \
  --title "SoulGate v0.1.1 - Hotfix" \
  --notes "Fixes critical bug in v0.1.0" \
  dist/release/*
```

---

## Continuous Deployment (Future)

### GitHub Actions Workflow

Create `.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run tests
        run: make test

      - name: Build release assets
        run: make release

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v1
        with:
          files: dist/release/*
          generate_release_notes: true
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Then releases happen automatically on tag push:

```bash
git tag v0.2.0
git push origin v0.2.0
# GitHub Actions builds and releases automatically
```

---

## Support & Questions

- **Release issues**: https://github.com/M4MEET/soulgate/issues
- **General discussion**: https://github.com/M4MEET/soulgate/discussions
- **Security issues**: security@soulgate.io (private disclosure)

---

**Happy Releasing!** 🎉
