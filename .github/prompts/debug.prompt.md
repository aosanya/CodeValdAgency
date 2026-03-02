````prompt
---
agent: agent
---

# Debug a CodeValdAgency Issue

## How to Use This Prompt

When you encounter a bug in CodeValdAgency, describe the failing behaviour and
use the guidelines below to add targeted debug logging, isolate the cause, and
clean up before merging.

## Common Failure Scenarios

### Scenario 1: `CreateAgency` Succeeds but Cross Never Receives `cross.agency.created`
**Symptom**: Agency appears in ArangoDB but CodeValdCross never triggers git init
**Cause**: `cross.agency.created` publish is missing or the Cross client is nil
**Check**: Confirm `m.crossClient.Publish(...)` is called after `backend.Insert`; check Cross client wiring in `cmd/main.go`

### Scenario 2: `Register` Always Fails with `DeadlineExceeded`
**Symptom**: Heartbeat loop logs `ping CodeValdCross at :50052: rpc error: code = DeadlineExceeded`
**Cause**: CodeValdCross is not running, or wrong address configured
**Check**: Verify `CROSS_ADDR` env var; confirm CodeValdCross is up before starting CodeValdAgency

### Scenario 3: `GetAgency` Returns `ErrAgencyNotFound` for Existing Agency
**Symptom**: Agency was created successfully but `GetAgency` can't find it
**Cause**: ArangoDB collection name mismatch, or wrong `agencyID` used as document key
**Check**: Print the ArangoDB collection name and the key used in `storage/arangodb/storage.go`

### Scenario 4: Context Cancellation Not Respected in Heartbeat Loop
**Symptom**: Service does not shut down cleanly; heartbeat goroutine leaks
**Cause**: Missing `ctx.Done()` select case in the registration loop
**Check**: Ensure heartbeat loop has `case <-ctx.Done(): return` in the select

### Scenario 5: Backend Not Injected — Nil Pointer Panic
**Symptom**: `nil pointer dereference` in `internal/manager/manager.go`
**Cause**: `cmd/main.go` did not construct and inject the `Backend` before calling `NewAgencyManager`
**Check**: Trace wiring in `cmd/main.go`; ensure `arangodb.NewBackend(cfg)` is called first

## Debug Print Guidelines

### Prefix Format
All debug prints MUST be prefixed with: `[AGENCY-XXX]`

### Go
```go
log.Printf("[AGENCY-XXX] Function called: %s with args: %+v", functionName, args)
log.Printf("[AGENCY-XXX] State before: %+v", state)
log.Printf("[AGENCY-XXX] Error in operation: %v", err)
```

### Strategic Placement

Add debug prints at:

1. **Function Entry Points**
   - `log.Printf("[AGENCY-XXX] CreateAgency called: name=%s", req.Name)`

2. **After Storage Operations**
   - `log.Printf("[AGENCY-XXX] Agency inserted: id=%s", agency.ID)`

3. **Before and After Pub/Sub Publish**
   - `log.Printf("[AGENCY-XXX] Publishing cross.agency.created: agencyID=%s", agency.ID)`

4. **Heartbeat Loop**
   - `log.Printf("[AGENCY-XXX] Register: attempt addr=%s err=%v", addr, err)`

5. **Error Handling**
   - `log.Printf("[AGENCY-XXX] CreateAgency failed: %v", err)`

### What NOT to Debug

- Simple getters
- Trivial utility functions
- Already well-instrumented production logs

### Debug Print Structure

Use descriptive messages that answer:
1. **WHERE**: Which function/block is executing
2. **WHAT**: What operation is happening
3. **VALUES**: Relevant variable values

**Good Example:**
```go
log.Printf("[AGENCY-XXX] CreateAgency: name=%s published=%v", req.Name, published)
```

**Bad Example:**
```go
log.Printf("here")
```
````
