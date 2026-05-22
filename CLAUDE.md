# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make build            # verify module compiles (go build ./...)
make build-server     # produce bin/codevaldagency
make run-server       # build + run (auto-loads .env)
make restart          # kill running instance, rebuild, run

make test             # unit tests with race detector
make test-arango      # ArangoDB integration tests only (requires .env)
make test-all         # all tests including integration

make vet              # go vet ./...
make lint             # golangci-lint run ./...
make proto            # regenerate gen/go/ from proto via buf generate
make reset-db         # truncate all agency ArangoDB collections (dev only)
```

Run a single test package: `go test -v -race -count=1 ./internal/server/`  
Run a single test: `go test -v -run TestPromoteDraft ./...`

## Architecture

**CodeValdAgency** is a Go gRPC microservice (port `50053`) with a single job: manage the full lifecycle of **Agencies** — the root entity every other CodeVald service scopes by `agencyID`. One running instance = one agency.

### Layer stack

```
gRPC handlers (internal/server/)
        │  delegates all business logic
        ▼
AgencyManager interface (agency.go, drafts.go, raci.go)
        │  persists via injected interface
        ▼
entitygraph.DataManager (storage/arangodb/ wraps SharedLib)
        │
        ▼
ArangoDB — agency_entities / agency_relationships / agency_graph
```

The gRPC handlers **never** contain business logic; they validate proto input and delegate to `AgencyManager`. `cmd/main.go` is wiring only — no logic.

### Entity model

There are 26 entity types defined in `schema.go` (TypeDefinitions) and `models.go` (Go value types). Every entity is a graph node; relationships are explicit edges. The important split:

- **Live/mutable** — Agency, Goal, Workflow, WorkItem, ConfiguredRole, Instruction, Deliverable, DeliverableResult, ContentRef, WorkPlan, ContextSource
- **Versioning/immutable** — AgencyDraft (open→promoted|archived), AgencySnapshot (immutable activation record), AgencyPublication (versioned release), AgencyPublicationStatus (draft→active→archived)

Every field on every entity is a typed `PropertyDefinition` in `schema.go` — no freeform attribute maps.

### Draft-based versioning

Live agency entities are immutable once published. All edits flow: `CreateDraft` (deep-copies live sub-graph into a draft scope) → edit draft entities → `PromoteDraft` (copies draft back to live, writes immutable AgencySnapshot) → `PublishAgency` (SHA-256 content-hashed AgencyPublication). `ArchiveDraft` is a terminal dead-end.

### Event publishing

After every mutating operation the manager publishes a Cross event (`agency.created`, `agency.promoted`, `agency.published`) via the injected `eventbus.Publisher`. Publish failures are **logged and ignored** — never fatal. CodeValdAgency only publishes its own domain events (`agency.*`); it never emits `work.*`, `git.*`, etc.

### Cross registration

`internal/registrar/` dials `CROSS_GRPC_ADDR` on startup and sends a heartbeat `Register` RPC every `CROSS_PING_INTERVAL` (default 20s). CodeValdAgency never imports CodeValdCross, CodeValdGit, CodeValdWork, or CodeValdAI packages — all cross-service calls are gRPC only.

### Work plans and context sources

A WorkPlan holds a set of regex-based `TriggerConditions` (Cross topic + payload pattern). `MatchWorkPlans` is called by CodeValdAI when a dispatched work task arrives; it returns which plans match. Each WorkPlan can have ContextSources (type: Git | Comm | Work) that describe how to fetch context when the plan executes.

### Testing pattern

Unit tests mock `AgencyManager` (see `internal/server/server_test.go`) and mock `DataManager`. Integration tests in `internal/server/integration_test.go` spin up a real ArangoDB connection, create a throwaway database, start an in-process gRPC server, and clean up after. Integration tests auto-skip if ArangoDB is unreachable — they are not required for local unit work.

## Key invariants

- `agency.created` **must** be published after every successful agency creation — never silently skip.
- AgencySnapshot and AgencyPublication are write-once — never mutate them.
- All entity references are read through graph edges, not flat FK fields.
- Schema is seeded idempotently in `cmd/main.go` before the server starts.
- Generated code under `gen/go/` is committed and must be regenerated with `make proto` after any `.proto` change — never hand-edit it.

## Naming conventions

- Interfaces: noun-only, no `I` prefix — `AgencyManager`, `Backend`
- Exported error sentinels in `errors.go` — `ErrAgencyNotFound`, `ErrDraftNotFound`, etc.
- Value types in `models.go`
- Branch naming: `feature/AGENCY-XXX_description`
- No abbreviations in exported names (`AgencyID` not `AgID`)
