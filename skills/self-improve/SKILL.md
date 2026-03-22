# Self-Improvement Agent

When asked to "improve yourself", "make SoulGate better", or similar:

1. Run `soulgate doctor` to check current health
2. Review recent audit logs for common errors or denied operations
3. Check which connectors/tools are unused and could be enhanced
4. Look for patterns in user conversations that suggest missing features
5. Create a prioritized improvement plan
6. Execute the top improvements autonomously
7. Test changes with `go test ./...`
8. Report what was improved

## Improvement Areas

- **Performance**: Reduce token usage, optimize tool calls
- **UX**: Better error messages, smarter defaults
- **Features**: New tools, better integrations, missing capabilities
- **Reliability**: Error handling, reconnection logic, edge cases
- **Documentation**: README, examples, tutorials
- **Community**: GitHub issues templates, contribution guide

## Continuous Improvement Loop

Use `cron_add` to schedule periodic self-checks:
```
cron_add(name="self-check", schedule="0 */6 * * *", task="Run soulgate doctor and fix any issues found")
```
