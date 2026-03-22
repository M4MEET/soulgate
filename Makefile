.PHONY: build test test-all test-unit test-integration test-security test-coverage \
        test-audit test-policy test-config test-broker test-plugin test-model test-core test-cli \
        bench bench-audit bench-policy bench-broker \
        clean install run demo lint check dev build-plugin build-release \
        build-cli build-all dist install-cli quickstart docker doctor

# Build the soulgate binary
build:
	go build -o bin/soulgate ./cmd/soulgate

# Build with version information
build-release:
	go build -ldflags="-s -w" -o bin/soulgate ./cmd/soulgate

# =============================================================================
# Testing Targets (Multi-Agent Workflow)
# =============================================================================

# Run all tests
test: test-unit test-integration

# Run all tests (alias for compatibility)
test-all: test

# Run unit tests only
test-unit:
	go test -v -race ./...

# Run integration tests
test-integration:
	@if [ -d "tests/integration" ]; then \
		go test -v -race -tags=integration ./tests/integration/...; \
	else \
		echo "Integration tests directory does not exist yet. Skipping..."; \
	fi

# Run security tests (for security-critical agents)
test-security:
	@echo "Running security tests for Policy Agent..."
	go test -v -tags=security ./internal/policy/...
	@echo "Running security tests for FileBroker Agent..."
	go test -v -tags=security ./internal/brokers/files/...
	@echo "Running security tests for Plugin Agent..."
	go test -v -tags=security ./internal/plugins/...

# =============================================================================
# Per-Agent Test Targets (Parallel Development)
# =============================================================================

# Agent 1: Audit Specialist
test-audit:
	@echo "Testing Audit Agent..."
	go test -v -race -coverprofile=coverage-audit.txt ./internal/audit/...

# Agent 2: Policy Specialist
test-policy:
	@echo "Testing Policy Agent..."
	go test -v -race -coverprofile=coverage-policy.txt ./internal/policy/...

# Agent 3: Config Specialist
test-config:
	@echo "Testing Config Agent..."
	go test -v -race -coverprofile=coverage-config.txt ./internal/config/...

# Agent 4: Security/FileBroker Specialist
test-broker:
	@echo "Testing FileBroker Agent..."
	go test -v -race -coverprofile=coverage-broker.txt ./internal/brokers/...

# Agent 5: Plugin Specialist
test-plugin:
	@echo "Testing Plugin Agent..."
	go test -v -race -coverprofile=coverage-plugin.txt ./internal/plugins/...

# Agent 6: Model Integration Specialist
test-model:
	@echo "Testing Model Agent..."
	go test -v -race -coverprofile=coverage-model.txt ./internal/model/...

# Agent 7: Orchestration Specialist
test-core:
	@echo "Testing Orchestration Agent..."
	go test -v -race -coverprofile=coverage-core.txt ./internal/core/...

# Agent 8: CLI/UX Specialist
test-cli:
	@echo "Testing CLI Agent..."
	go test -v -race -coverprofile=coverage-cli.txt ./cmd/soulgate/...

# =============================================================================
# Coverage Targets
# =============================================================================

# Run tests with overall coverage report
test-coverage:
	go test -v -race -coverprofile=coverage.txt ./...
	go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report: coverage.html"

# Generate coverage for all agents
test-coverage-all: test-audit test-policy test-config test-broker test-plugin test-model test-core test-cli
	@echo "Merging coverage reports..."
	@echo "mode: atomic" > coverage-merged.txt
	@grep -h -v "mode:" coverage-*.txt >> coverage-merged.txt 2>/dev/null || true
	go tool cover -html=coverage-merged.txt -o coverage-all.html
	@echo "Merged coverage report: coverage-all.html"

# =============================================================================
# Performance Benchmarks
# =============================================================================

# Run all benchmarks
bench:
	go test -v -bench=. -benchmem ./...

# Benchmark Audit Agent
bench-audit:
	go test -v -bench=. -benchmem ./internal/audit/...

# Benchmark Policy Agent
bench-policy:
	go test -v -bench=. -benchmem ./internal/policy/...

# Benchmark FileBroker Agent
bench-broker:
	go test -v -bench=. -benchmem ./internal/brokers/...

# =============================================================================
# Build and Development
# =============================================================================

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage*.txt coverage*.html coverage*.out

# Install soulgate to $GOPATH/bin
install:
	go install ./cmd/soulgate

# Run demo
demo: build
	cd demo && ./demo.sh

# Lint code
lint:
	go vet ./...
	go fmt ./...

# Build example plugin
build-plugin:
	cd plugins/examples/file_reader && \
	cargo build --target wasm32-unknown-unknown --release && \
	cp target/wasm32-unknown-unknown/release/file_reader.wasm plugin.wasm

# Run all checks before commit
check: lint test-unit test-security

# Development build and run
dev: build
	./bin/soulgate

# =============================================================================
# Agent Workflow Helpers
# =============================================================================

# Show agent test status
agent-status:
	@echo "=== Agent Test Status ==="
	@echo "Agent 1 (Audit):         $$(go test ./internal/audit/... 2>&1 | grep -c PASS || echo 0) tests passing"
	@echo "Agent 2 (Policy):        $$(go test ./internal/policy/... 2>&1 | grep -c PASS || echo 0) tests passing"
	@echo "Agent 3 (Config):        $$(go test ./internal/config/... 2>&1 | grep -c PASS || echo 0) tests passing"
	@echo "Agent 4 (FileBroker):    $$(go test ./internal/brokers/... 2>&1 | grep -c PASS || echo 0) tests passing"
	@echo "Agent 5 (Plugin):        $$(go test ./internal/plugins/... 2>&1 | grep -c PASS || echo 0) tests passing"
	@echo "Agent 6 (Model):         $$(go test ./internal/model/... 2>&1 | grep -c PASS || echo 0) tests passing"
	@echo "Agent 7 (Orchestration): $$(go test ./internal/core/... 2>&1 | grep -c PASS || echo 0) tests passing"
	@echo "Agent 8 (CLI):           $$(go test ./cmd/soulgate/... 2>&1 | grep -c PASS || echo 0) tests passing"

# =============================================================================
# Production CLI Targets
# =============================================================================

# Build production-ready CLI binary with version info
VERSION := 0.1.0
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)

build-cli:
	@echo "Building SoulGate CLI v$(VERSION)..."
	@mkdir -p bin
	go build -ldflags="$(LDFLAGS)" -o bin/soulgate ./cmd/soulgate
	@echo "✓ Built: bin/soulgate"

# Build for all platforms
build-all:
	@echo "Building SoulGate for all platforms..."
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/soulgate-linux-amd64 ./cmd/soulgate
	GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/soulgate-linux-arm64 ./cmd/soulgate
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/soulgate-darwin-amd64 ./cmd/soulgate
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/soulgate-darwin-arm64 ./cmd/soulgate
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/soulgate-windows-amd64.exe ./cmd/soulgate
	@echo "✓ Built all platforms in dist/"

# Create distribution packages
dist: build-all
	@echo "Creating distribution packages..."
	@mkdir -p dist/packages
	cd dist && tar -czf packages/soulgate-v$(VERSION)-linux-amd64.tar.gz soulgate-linux-amd64
	cd dist && tar -czf packages/soulgate-v$(VERSION)-linux-arm64.tar.gz soulgate-linux-arm64
	cd dist && tar -czf packages/soulgate-v$(VERSION)-darwin-amd64.tar.gz soulgate-darwin-amd64
	cd dist && tar -czf packages/soulgate-v$(VERSION)-darwin-arm64.tar.gz soulgate-darwin-arm64
	cd dist && zip -q packages/soulgate-v$(VERSION)-windows-amd64.zip soulgate-windows-amd64.exe
	@echo "✓ Created distribution packages in dist/packages/"
	@ls -lh dist/packages/

# Create GitHub release assets (for install.sh)
release: build-all
	@echo "Creating GitHub release assets..."
	@mkdir -p dist/release
	cp dist/soulgate-linux-amd64 dist/release/soulgate-v$(VERSION)-linux-amd64
	cp dist/soulgate-linux-arm64 dist/release/soulgate-v$(VERSION)-linux-arm64
	cp dist/soulgate-darwin-amd64 dist/release/soulgate-v$(VERSION)-darwin-amd64
	cp dist/soulgate-darwin-arm64 dist/release/soulgate-v$(VERSION)-darwin-arm64
	cp dist/soulgate-windows-amd64.exe dist/release/soulgate-v$(VERSION)-windows-amd64.exe
	@echo "✓ Created release assets in dist/release/"
	@echo ""
	@echo "Upload these to GitHub release v$(VERSION):"
	@ls -lh dist/release/

# Install CLI to system
install-cli: build-cli
	@echo "Installing soulgate to /usr/local/bin..."
	@sudo cp bin/soulgate /usr/local/bin/
	@sudo chmod +x /usr/local/bin/soulgate
	@echo "✓ Installed: /usr/local/bin/soulgate"
	@echo ""
	@soulgate --version

# Quick start - build and setup
quickstart: build-cli
	@echo "╔═══════════════════════════════════════════════════════╗"
	@echo "║          SoulGate Quick Start                        ║"
	@echo "╚═══════════════════════════════════════════════════════╝"
	@echo ""
	@echo "✓ SoulGate CLI built successfully"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Run interactive setup:"
	@echo "     ./bin/soulgate setup"
	@echo ""
	@echo "  2. Or quick initialization:"
	@echo "     ./bin/soulgate init"
	@echo ""
	@echo "  3. Check status:"
	@echo "     ./bin/soulgate status"
	@echo ""
	@echo "  4. Start using SoulGate:"
	@echo "     ./bin/soulgate run \"<your prompt>\""
	@echo ""

# =============================================================================
# Help Target
# =============================================================================

help:
	@echo "SoulGate Multi-Agent Development Makefile"
	@echo ""
	@echo "🚀 Quick Start:"
	@echo "  make quickstart      - Build and show getting started guide"
	@echo "  make build-cli       - Build production-ready CLI"
	@echo "  make install-cli     - Install CLI to /usr/local/bin"
	@echo ""
	@echo "📦 Build Targets:"
	@echo "  make build           - Build soulgate binary"
	@echo "  make build-release   - Build with optimizations"
	@echo "  make build-all       - Build for all platforms"
	@echo "  make dist            - Create distribution packages"
	@echo "  make release         - Create GitHub release assets"
	@echo "  make install         - Install to \$$GOPATH/bin"
	@echo ""
	@echo "🧪 Testing Targets:"
	@echo "  make test            - Run all tests (unit + integration)"
	@echo "  make test-unit       - Run unit tests only"
	@echo "  make test-integration - Run integration tests"
	@echo "  make test-security   - Run security tests"
	@echo "  make test-coverage   - Run tests with coverage report"
	@echo ""
	@echo "🤖 Per-Agent Testing:"
	@echo "  make test-audit      - Test Audit Agent (Agent 1)"
	@echo "  make test-policy     - Test Policy Agent (Agent 2)"
	@echo "  make test-config     - Test Config Agent (Agent 3)"
	@echo "  make test-broker     - Test FileBroker Agent (Agent 4)"
	@echo "  make test-plugin     - Test Plugin Agent (Agent 5)"
	@echo "  make test-model      - Test Model Agent (Agent 6)"
	@echo "  make test-core       - Test Orchestration Agent (Agent 7)"
	@echo "  make test-cli        - Test CLI Agent (Agent 8)"
	@echo ""
	@echo "⚡ Benchmarks:"
	@echo "  make bench           - Run all benchmarks"
	@echo "  make bench-audit     - Benchmark Audit Agent"
	@echo "  make bench-policy    - Benchmark Policy Agent"
	@echo "  make bench-broker    - Benchmark FileBroker Agent"
	@echo ""
	@echo "🛠️  Development:"
	@echo "  make dev             - Build and run"
	@echo "  make lint            - Run linters"
	@echo "  make check           - Run all checks (lint + test + security)"
	@echo "  make clean           - Remove build artifacts"
	@echo "  make agent-status    - Show test status for all agents"
	@echo ""
	@echo "See docs/AGENTS.md for agent responsibilities and coordination."

.DEFAULT_GOAL := build

# Build and run Docker container
docker:
	docker compose up --build

# Run diagnostics
doctor:
	./bin/soulgate doctor
