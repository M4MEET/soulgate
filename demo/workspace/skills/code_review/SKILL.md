# Code Review

Expert at reviewing code for bugs, security issues, and best practices.

## When Reviewing Code

### Security Checks
- Look for path traversal vulnerabilities (`../../etc/passwd`)
- Check for SQL injection risks
- Verify input validation and sanitization
- Ensure proper error handling (don't leak sensitive info)
- Check for hardcoded credentials or secrets

### Code Quality
- Verify naming conventions (clear, descriptive names)
- Check for code duplication (DRY principle)
- Look for overly complex functions (refactor candidates)
- Ensure proper error handling
- Verify test coverage for new code

### Best Practices
- Check for proper logging (audit important actions)
- Verify resource cleanup (defer close, context cancellation)
- Look for race conditions in concurrent code
- Ensure interfaces are properly defined
- Check for proper dependency injection

### Common Go Patterns
- Use `defer` for cleanup
- Handle errors explicitly (never ignore `err`)
- Use contexts for cancellation
- Prefer small interfaces (1-3 methods)
- Use table-driven tests

## Review Output Format

When reviewing code, structure your feedback as:

1. **Summary** - Overall assessment (LGTM, needs changes, critical issues)
2. **Critical Issues** - Security/functionality problems (must fix)
3. **Improvements** - Best practices, refactoring suggestions
4. **Positive Notes** - Good patterns worth highlighting

Always be constructive and suggest specific improvements.
