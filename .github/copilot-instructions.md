````instructions
# CodeValdAgency — AI Agent Development Instructions

## Project Overview

**CodeValdAgency** is a **Go gRPC microservice** that manages the full lifecycle
of **Agencies** in the CodeVald platform.

An Agency is the top-level organisational unit. Every other service in the
platform scopes its data by `agencyID`. CodeValdAgency owns the authoritative
record of what agencies exist, registers those agencies with
[CodeValdCross](../CodeValdCross/README.md), and publishes `cross.agency.created`
so that CodeValdCross can trigger downstream onboarding (git repo init, work
setup, etc.).

**Core Concept**: CodeValdAgency has exactly one job — create, read, update, and
delete agencies. It knows nothing about tasks, git, AI agents, or communication.
Those concerns belong to other services.

---

## Service Architecture

> **Full architecture details live in the documentation.**
> See `documentation/2-SoftwareDesignAndArchitecture/architecture.md` for:
> - `AgencyManager` and `Backend` interface contracts with full method signatures
> - `Agency`, `CreateAgencyRequest`, `UpdateAgencyRequest`, `AgencyFilter` data models
> - Project directory structure and file responsibilities
> - gRPC `AgencyService` proto definition and generated-code location
> - ArangoDB collection schema and index design
> - CodeValdCross `RegisterRequest` payload — service name, addr, routes, pub/sub topics
> - Heartbeat / liveness registration pattern

**Key invariants to keep in mind while coding:**

- CodeValdAgency **never** imports CodeValdGit, CodeValdWork, or CodeValdCross packages — gRPC only
- All cross-service communication flows through the `Register` RPC on CodeValdCross
- The `AgencyManager` interface is the only business-logic entry point — gRPC handlers delegate to it
- `cross.agency.created` **must** be published after every successful `CreateAgency` call
- Storage backends are injected — the core library is backend-agnostic

---

## Developer Workflows

### Build & Run Commands

```bash
# Build the binary
go build -o bin/codevaldagency-server ./cmd/...

# Run the service
./bin/codevaldagency-server

# Run all tests with race detector
go test -v -race ./...

# Run tests with coverage
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Static analysis
go vet ./...

# Format code
go fmt ./...

# Lint
golangci-lint run ./...

# Regenerate protobuf (requires buf)
buf generate
```

### Task Management Workflow

**Every task follows strict branch management:**

```bash
# 1. Create feature branch from main
git checkout -b feature/AGENCY-XXX_description

# 2. Implement changes

# 3. Build validation before merge
go build ./...           # Must succeed
go vet ./...             # Must show 0 issues
go test -v -race ./...   # Must pass
golangci-lint run ./...  # Must pass

# 4. Merge when complete
git checkout main
git merge feature/AGENCY-XXX_description --no-ff
git branch -d feature/AGENCY-XXX_description
```

---

## Technology Stack

| Component | Choice | Rationale |
|---|---|---|
| Language | Go 1.21+ | Matches all other CodeVald services |
| Service communication | gRPC + protobuf | Typed contracts; Cross dials Agency via gRPC |
| Storage | ArangoDB | Matches CodeValdWork and CodeValdGit patterns |
| Configuration | YAML + env overrides | Consistent with other services |
| Registration | CodeValdCross `Register` RPC | Standard onboarding pattern |

---

## Code Quality Rules

### Service-Specific Rules

- **No business logic in `cmd/main.go`** — wire dependencies only; logic lives in `internal/`
- **Interface-first for `AgencyManager`** — the gRPC server holds the interface, not the concrete type
- **No direct imports of other CodeVald services** — all cross-service calls go through gRPC
- **All public functions must have godoc comments**
- **Context propagation** — every public method takes `context.Context` as first argument
- **`cross.agency.created` must be published** on every successful `CreateAgency` — never silently skip

### Naming Conventions

- **Package name**: `agency` (root), `manager`, `server`, `config`, `arangodb`
- **Interfaces**: noun-only, no `I` prefix — `AgencyManager`, `Backend`
- **Branch naming**: `feature/AGENCY-XXX_description` (lowercase with underscores)
- **gRPC service**: `AgencyService`
- **No abbreviations in exported names** — prefer `AgencyID` over `AgID`

### File Organisation

- **Max file size**: 500 lines (prefer smaller, focused files)
- **Max function length**: 50 lines (prefer 20-30)
- **One primary concern per file**
- **Error types in `errors.go`** — `ErrAgencyNotFound`, `ErrAgencyAlreadyExists`
- **Value types in `models.go`** — `Agency`, `CreateAgencyRequest`, `AgencyFilter`

### Anti-Patterns to Avoid

- ❌ **AI agent logic, LLM calls, or streaming responses** — not in this service
- ❌ **Frontend serving, HTML templates, or React/Vite code** — belongs in CodeValdFortex
- ❌ **Task or work item management** — belongs in CodeValdWork
- ❌ **Git operations** — belongs in CodeValdGit
- ❌ **Business logic in gRPC handlers** — handlers delegate to `AgencyManager`
- ❌ **Hardcoded storage** — inject `Backend` via constructor
- ❌ **Skipping `cross.agency.created`** — always publish on agency creation

---

## Integration with CodeValdCross

CodeValdCross consumes `cross.agency.created` to:

1. Call `CodeValdGit.InitRepo(agencyID)` — create the agency git repository
2. Notify CodeValdWork to prepare the agency task scope
3. Emit `cross.agency.created` downstream to any other listeners

CodeValdAgency does **not** orchestrate these steps — it only publishes the event
and lets Cross handle sequencing.
````
