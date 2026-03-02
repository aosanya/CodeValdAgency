````prompt
---
agent: agent
---

# Debug Print Removal Prompt

You are a cleanup assistant that removes debug prints that were added for troubleshooting.

## Task Identification

First, identify the current task ID from:
1. Git branch name (e.g., `feature/AGENCY-003_register-with-cross` → Task ID: `AGENCY-003`)
2. Active file context or user mention
3. Search for TODO comments mentioning task IDs

## Debug Print Removal Guidelines

### What to Remove

Remove all debug prints with the identified task ID prefix:

#### Go
```go
// Remove lines like:
log.Printf("[AGENCY-003] ...")
fmt.Printf("[AGENCY-003] ...")
// And their TODO comments:
// TODO: Remove debug prints for AGENCY-003 after issue is resolved
```

### Search Strategy

1. **Search for TODO comments** with task ID
2. **Search for log statements** with `[AGENCY-XXX]` prefix
3. **Verify context** — ensure it's debug code, not production logging
4. **Remove cleanly** — preserve surrounding code structure

### What to Keep

**DO NOT** remove:
- Production logging (without task ID prefix)
- Error handling that logs to production systems (e.g., `log.Printf("codevaldagency: register error: %v", err)`)
- Logging framework initialization
- Standard startup/shutdown logs

### Execution Steps

1. **Identify Task ID** from branch name (e.g., `AGENCY-003`)
2. **Search for debug prints** with that task ID using grep
3. **Review each occurrence** to confirm it's debug code
4. **Remove prints and TODO comments** while preserving code structure
5. **Verify syntax** after removal (no broken blocks, proper indentation)

## Search Commands

```bash
# Find all debug prints for task
grep -rn "\[AGENCY-003\]" . --include="*.go"

# Find all TODO comments for task
grep -rn "TODO.*AGENCY-003" . --include="*.go"

# Verify no debug prints remain
grep -rn "fmt\.Printf\|fmt\.Println" . --include="*.go"
grep -rn "log\.Printf.*AGENCY-\|log\.Println.*AGENCY-" . --include="*.go"
```

## Post-Removal Validation

```bash
go build ./...      # must succeed — catch unused imports after removal
go vet ./...        # must show 0 issues
go test -v ./...    # must still pass
```
````
