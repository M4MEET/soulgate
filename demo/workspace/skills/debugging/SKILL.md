# Debugging

Expert at finding and fixing bugs in code.

## Debugging Approach

### 1. Understand the Problem
- Read error messages carefully (full stack trace)
- Identify what's expected vs what's happening
- Reproduce the issue if possible
- Check logs for additional context

### 2. Locate the Bug
- Start from the error location (stack trace)
- Trace backwards through function calls
- Check input data and validation
- Look for edge cases and null/nil values
- Verify assumptions (test them!)

### 3. Common Bug Categories

**Nil Pointer/Null Reference**
- Check if variables are initialized
- Verify return values before use
- Add nil checks before dereferencing

**Logic Errors**
- Off-by-one errors (loop boundaries)
- Wrong comparison operators (`<` vs `<=`)
- Incorrect conditional logic
- Missing edge case handling

**Concurrency Bugs**
- Race conditions (shared state without locks)
- Deadlocks (circular waiting)
- Channel deadlocks (sending/receiving on closed channels)
- Missing mutex locks

**Type/Conversion Errors**
- Type assertions without checks
- Integer overflow/underflow
- String/byte conversion issues
- JSON unmarshaling errors

### 4. Debugging Tools

**Code Reading**
- Read surrounding context
- Check function signatures
- Verify types match expectations
- Look for recent changes (git blame)

**Logging**
- Add strategic log statements
- Log inputs and outputs
- Log error paths
- Use structured logging (JSON)

**Testing**
- Write minimal reproduction test
- Test edge cases
- Use table-driven tests
- Check error paths

### 5. Fix and Verify

**Making the Fix**
- Make minimal changes (don't refactor while debugging)
- Fix root cause, not symptoms
- Add validation to prevent recurrence
- Update error messages to be clear

**Verification**
- Test the fix with original error case
- Test edge cases
- Run full test suite
- Check for side effects

## Debugging Checklist

When debugging an issue, systematically check:

- [ ] Error message and stack trace read carefully
- [ ] Input data validated and inspected
- [ ] Return values checked for errors
- [ ] Nil/null checks added where needed
- [ ] Edge cases considered
- [ ] Concurrency issues ruled out
- [ ] Types verified correct
- [ ] Test added to prevent regression

Always explain your debugging process so the user understands your reasoning.
