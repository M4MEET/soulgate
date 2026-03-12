# SoulGate Dependencies

## Summary

- **Total dependencies:** 78
- **Direct dependencies:** 12
- **Transitive dependencies:** 66

## Why We Have 78 Dependencies

### Core Frameworks (Required)

#### Terminal UI - Charmbracelet Stack (18 deps)
SoulGate's interactive TUI is a core feature providing:
- Beautiful, intuitive terminal interface
- Model selection UI
- Onboarding wizard
- Real-time chat interface

**Direct:**
- `github.com/charmbracelet/bubbletea` - TUI framework (Elm architecture)
- `github.com/charmbracelet/bubbles` - Reusable TUI components
- `github.com/charmbracelet/lipgloss` - Terminal styling

**Transitive:** 15 styling/terminal dependencies

**Alternative:** Remove TUI entirely → Save 18 deps, lose major feature

#### Pure-Go SQLite - Modernc.org (13 deps)
Audit logging requires persistent storage:
- No CGO requirement (easier cross-compilation)
- Embedded database (no external setup)
- Full SQL support

**Direct:**
- `modernc.org/sqlite` - Pure Go SQLite implementation

**Transitive:** 12 compiler/runtime dependencies

**Alternative:** Switch to `mattn/go-sqlite3` → Requires CGO, harder builds

#### CLI Framework - Cobra (3 deps)
Professional CLI with subcommands:
- `github.com/spf13/cobra` - Command structure
- Plus 2 transitive deps (pflag, mousetrap)

**Alternative:** Standard library → Lose subcommands, help generation

#### Testing - Testify (3 deps)
Comprehensive test assertions:
- `github.com/stretchr/testify` - Test framework
- Plus 2 transitive deps

**Alternative:** Standard testing → Verbose test code

### Essential Libraries (Required)

**YAML Configuration:**
- `gopkg.in/yaml.v3` - Config file parsing

**WebSocket Gateway:**
- `github.com/gorilla/websocket` - Gateway server communication

**Glob Patterns:**
- `github.com/gobwas/glob` - Policy resource matching

**UUID Generation:**
- `github.com/google/uuid` - Session/Run IDs

**WASM Runtime:**
- `github.com/tetratelabs/wazero` - Plugin execution (pure Go)

### Optional Features

**Telegram Connector:**
- `github.com/go-telegram/bot` - Telegram bot integration

**Can be removed if:** Telegram support not needed → Save 1 dep

## Dependency Reduction Analysis

### Attempted Reductions

✅ **Removed 3 unused dependencies** (81 → 78)
- `github.com/atotto/clipboard` (not used)
- `github.com/dustin/go-humanize` (not used)
- `github.com/kballard/go-shellquote` (not used)

### Why We Can't Reach 50

**Minimum viable dependencies:**
- TUI framework: 18 deps (core feature)
- SQLite pure-Go: 13 deps (audit logging)
- CLI framework: 3 deps (professional CLI)
- Testing: 3 deps (test quality)
- Essential libs: 10 deps (YAML, WebSocket, glob, UUID, WASM, etc.)

**Baseline:** ~47 dependencies minimum

**To reach 50, we would need to:**
- ❌ Remove TUI entirely (lose 18 deps, major feature loss)
- ❌ Switch to CGO SQLite (lose 12 deps, gain build complexity)
- ❌ Remove CLI framework (lose 3 deps, lose subcommands)

### Recommendation

**Current 78 dependencies is optimal** because:
1. All dependencies serve essential purposes
2. Pure-Go implementation enables easy cross-compilation
3. TUI is a differentiating feature
4. No heavy/redundant dependencies remain

**Focus instead on:**
- ✅ Keeping dependencies up to date
- ✅ Regular security audits
- ✅ Monitoring for vulnerabilities
- ✅ Understanding dependency tree

## Dependency Health

**All dependencies:**
- ✅ Well-maintained
- ✅ Popular with active communities
- ✅ Pure Go (no CGO except optional)
- ✅ MIT/Apache licensed
- ✅ No known critical vulnerabilities

## Commands

Check dependencies:
```bash
go list -m all                    # List all dependencies
go mod graph                      # Show dependency tree
go mod why github.com/foo/bar     # Why is this dependency included?
go mod tidy                       # Remove unused dependencies
```

