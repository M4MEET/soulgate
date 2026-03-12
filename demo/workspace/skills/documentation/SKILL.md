# Documentation

Expert at writing clear, comprehensive technical documentation.

## Documentation Principles

### Clarity First
- Write for the reader, not yourself
- Use simple, direct language
- Define technical terms on first use
- Provide examples for complex concepts
- Use active voice ("the system performs" not "is performed by")

### Structure
- Start with overview/summary
- Organize hierarchically (sections, subsections)
- Use consistent heading levels
- Add table of contents for long docs
- Include "Quick Start" section

### Completeness
- Cover all important use cases
- Document error conditions
- Include examples (code snippets, commands)
- Show expected outputs
- List prerequisites and dependencies

## Documentation Types

### API Documentation
```markdown
## Function: CreateSession

Creates a new session with the given configuration.

**Parameters:**
- `config` (SessionConfig): Session configuration
  - `id` (string): Unique session identifier
  - `timeout` (int): Timeout in seconds (default: 300)

**Returns:**
- `*Session`: Created session object
- `error`: Error if creation fails

**Example:**
```go
session, err := CreateSession(SessionConfig{
    ID: "user-123",
    Timeout: 600,
})
if err != nil {
    return fmt.Errorf("failed to create session: %w", err)
}
```

**Errors:**
- `ErrInvalidConfig`: If config is invalid
- `ErrSessionExists`: If session ID already exists
```

### README Files
- Project overview (what it does, why it exists)
- Installation instructions (step-by-step)
- Quick start guide (5-minute example)
- Architecture overview (high-level design)
- Configuration options
- Contributing guidelines
- License information

### Tutorials/Guides
- Step-by-step instructions
- Explain the "why" not just the "how"
- Include screenshots or diagrams
- Show expected outputs
- Troubleshooting section
- Next steps/further reading

### Architecture Documents
- System overview diagram
- Component descriptions
- Data flow diagrams
- Key design decisions
- Trade-offs and alternatives considered
- Security model
- Performance characteristics

## Writing Guidelines

### Code Examples
- Use realistic, runnable examples
- Include necessary imports/setup
- Show error handling
- Add comments for complex parts
- Use consistent formatting
- Test that examples actually work

### Commands and CLI
- Show exact command syntax
- Include all required flags
- Provide example output
- Explain what each flag does
- Show common use cases

```bash
# List all sessions
soulgate sessions list

# Output:
# SESSION ID          CREATED              MESSAGES
# telegram:12345      2024-01-15 10:30     12
# slack:67890         2024-01-15 11:45     8

# View specific session
soulgate sessions show telegram:12345

# Delete old sessions
soulgate sessions delete telegram:12345
```

### Tables
Use tables for structured data:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `id` | string | Yes | - | Session ID |
| `timeout` | int | No | 300 | Timeout in seconds |

### Diagrams
Use ASCII diagrams for simple flows:

```
┌──────────┐       ┌──────────┐       ┌──────────┐
│  Client  │──────▶│ Gateway  │──────▶│  Agent   │
└──────────┘       └──────────┘       └──────────┘
     │                  │                   │
     │                  ▼                   ▼
     │             ┌──────────┐       ┌──────────┐
     └────────────▶│ Observer │       │  Model   │
                   └──────────┘       └──────────┘
```

## Common Documentation Mistakes to Avoid

❌ **Don't:**
- Assume knowledge ("obviously", "simply")
- Use vague language ("might work", "usually")
- Leave steps implicit ("configure the system")
- Ignore error cases
- Use inconsistent terminology
- Write wall-of-text paragraphs
- Skip examples

✅ **Do:**
- Be explicit and precise
- Use concrete examples
- Break into digestible chunks
- Show error handling
- Define terms consistently
- Use formatting (headings, lists, code blocks)
- Include visual aids

## Documentation Checklist

Before finalizing documentation:

- [ ] Overview/summary at the top
- [ ] All parameters/options documented
- [ ] Working code examples included
- [ ] Expected outputs shown
- [ ] Error conditions covered
- [ ] Prerequisites listed
- [ ] Consistent terminology used
- [ ] Proper formatting (headings, code blocks)
- [ ] Spelling and grammar checked
- [ ] Links verified (if any)

When writing documentation, think: "If I knew nothing about this project, could I understand and use it from this documentation alone?"
