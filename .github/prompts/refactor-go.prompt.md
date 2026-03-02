````prompt
---
agent: agent
---

# Refactor Go Code

Guides a safe, incremental Go refactoring for **CodeValdAgency**.

---

## When to Refactor

- File exceeds **500 lines** (hard limit)
- Function exceeds **50 lines**
- Multiple concerns in one file (e.g., manager + storage in the same file)
- Business logic leaked into `cmd/main.go` or gRPC handler
- AI/LLM/frontend logic crept into the service — remove it

---

## Refactoring Workflow

### Step 1: Understand the File

```bash
wc -l internal/manager/manager.go
grep -n "^func " internal/manager/manager.go
```

### Step 2: Plan the Split

Identify distinct responsibilities. For CodeValdAgency, typical splits:

```
internal/manager/manager.go      # AgencyManager concrete implementation
internal/server/server.go        # gRPC handler + server lifecycle
internal/config/config.go        # Configuration loading
storage/arangodb/storage.go      # ArangoDB Backend implementation
errors.go                        # All exported error types
models.go                        # Agency, CreateAgencyRequest, AgencyFilter
cmd/main.go                      # Wiring only — no logic
```

### Step 3: Extract — One File at a Time

1. Create the new file with its package declaration
2. Move types / functions
3. Update imports
4. Run `go build ./...` — must succeed after each file move
5. Run `go test -v -race ./...`

### Step 4: Handle Shared Dependencies

If a type is used across multiple files, move it to `models.go`.
If an error type is referenced by multiple packages, keep it in `errors.go` at module root.

### Step 5: Validate

```bash
go build ./...           # must succeed
go vet ./...             # must show 0 issues
go test -v -race ./...   # must pass
golangci-lint run ./...  # must pass
```

---

## Specific Concerns for CodeValdAgency

### Remove legacy AI/agent logic if found
- Any AI streaming, LLM calls, agent runtime code does NOT belong here
- Move to CodeValdAI or delete if no longer needed
- After removal: `go build ./...` must still succeed

### Keep Cross registration separate from business logic
- Cross registration/heartbeat lives in `cmd/main.go` or a dedicated `internal/registrar/` package
- Never mix registration retry logic with `CreateAgency` business logic
````
