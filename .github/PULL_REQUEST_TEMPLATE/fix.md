# Bug Fix Summary

Describe the bug and the fix in 2-5 bullets.

- 
- 

# Problem

What was broken? Who or what was affected?

# Root Cause

What caused the bug?

# Fix

What changed to resolve it?

- 
- 

# Risk Areas

Mark any areas touched by this fix.

- [ ] Public API behavior
- [ ] Worker lifecycle
- [ ] Dispatch or message delivery
- [ ] Panic recovery or backoff
- [ ] Metrics emission
- [ ] Logging
- [ ] Concurrency or synchronization

# Tests

- [ ] `go test ./... -count=1`
- [ ] `go test ./... -count=1 -race -timeout 5m`
- [ ] Added or updated black-box tests
- [ ] Added or updated white-box tests
- [ ] Reproduced the bug before the fix

Test notes:

- 

# Documentation

- [ ] No documentation changes needed
- [ ] Updated godoc comments
- [ ] Updated `README.md`
- [ ] Updated `DOCUMENTATION.md`

# Checklist

- [ ] I followed the project conventions in `AGENTS.md`
- [ ] New behavior includes tests
- [ ] Errors use existing or new sentinel errors where appropriate
- [ ] I considered regression risk and backward compatibility

# Related Issues

- Closes #
- Related to #
