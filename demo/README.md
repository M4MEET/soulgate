# SoulGate Demo

This directory contains a demonstration of SoulGate v0.1 functionality.

## Quick Start

```bash
# Run the demo script
./demo.sh
```

The demo script will:
1. Show the default security policy
2. List installed plugins (none by default)
3. Check the audit log
4. Execute a test run
5. Display audit events

## Manual Testing

### Initialize a new workspace

```bash
cd workspace
../../bin/soulgate init
```

### View policy

```bash
../../bin/soulgate policy show
```

### List plugins

```bash
../../bin/soulgate plugin list
```

### Run with a model (requires API key)

```bash
export OPENAI_API_KEY=sk-...
../../bin/soulgate run "Read example.txt and tell me what it says"
```

### View audit log

```bash
../../bin/soulgate audit tail --last 10
```

### View audit log as JSON

```bash
../../bin/soulgate audit tail --json --last 5
```

## Workspace Structure

```
workspace/
├── .soulgate/
│   ├── config.yml   # Workspace configuration
│   ├── policy.yml   # Security policies
│   └── audit.db     # SQLite audit log
├── plugins/         # Plugin directory
└── example.txt      # Test file
```

## Default Policy

The default policy allows:
- Reading files within workspace (`./**`)
- Listing directories within workspace
- Stat operations within workspace

The default policy denies:
- Access to parent directories (`../**`)
- Absolute paths outside workspace

## Testing Security

Try these commands to verify security enforcement:

```bash
# Should succeed - within workspace
../../bin/soulgate run "Read example.txt"

# Should fail - parent directory access
../../bin/soulgate run "Read ../README.md"

# Should fail - absolute path
../../bin/soulgate run "Read /etc/passwd"
```

All attempts are logged in the audit database.

## v0.1 Limitations

This is a minimal viable prototype. The following are planned for future versions:

- **Model Integration**: Currently returns mock responses. Full OpenAI/Anthropic integration coming in Phase 8.
- **WASM Plugins**: Basic WASM runtime is implemented but simplified. Full plugin bridge coming in Phase 6.
- **File Write**: Only read-only operations supported in v0.1.
- **Additional Brokers**: Network, secrets, and exec brokers planned for v0.2+.

## Next Steps

1. Review the generated policy file: `.soulgate/policy.yml`
2. Customize policies for your use case
3. Build or install plugins in the `plugins/` directory
4. Set up your model provider API key
5. Start building agent workflows!
