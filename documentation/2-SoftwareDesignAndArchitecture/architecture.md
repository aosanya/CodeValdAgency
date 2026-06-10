# CodeValdAgency — Architecture

> **Split docs** — this file is the index. Details live in focused companion files:
> - [architecture-interfaces.md](architecture-interfaces.md) — `AgencyManager`, `AgencySchemaManager`, `CrossPublisher`, gRPC service definitions, Cross registration
> - [architecture-models.md](architecture-models.md) — value-type definitions (Agency, Workflow, AgencyDraft, …)
> - [architecture-graph.md](architecture-graph.md) — graph topology, entity types, pre-delivered schema
> - [architecture-storage.md](architecture-storage.md) — ArangoDB collections, document shapes, indexes
> - [architecture-flows.md](architecture-flows.md) — lifecycle flows (Set/Create/Promote/Archive/Publish/Import), error types
> - [architecture-configuration.md](architecture-configuration.md) — env vars, boot sequence, local dev

---

## 1. Core Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Business-logic entry point | `AgencyManager` interface (wraps `entitygraph.DataManager`) | gRPC handlers delegate to it; convenience facade over graph operations |
| Graph model | Separate entity nodes per type; edges carry semantic labels | Agency, Goal, Workflow, WorkItem, ConfiguredRole are first-class graph citizens |
| Storage injection | `entitygraph.DataManager` + `AgencySchemaManager` injected by `cmd/main.go` | Backend-agnostic; testable with fake `DataManager` |
| Single-agency-per-database | One `Agency` entity per database; `agencyID` injected at startup | Preserves the existing deployment model; no multi-tenancy in v1 |
| Downstream communication | gRPC only — no direct Go imports | Stable, versioned contracts; independent deployment |
| Cross registration | `OrchestratorService.Register` RPC on startup + heartbeat | Standard CodeVald onboarding pattern; liveness via repeat calls |
| Live agency mutability | Read-only once published; all edits flow through `AgencyDraft` + `PromoteDraft` | Prevents divergent edits; provides an authoring audit trail |
| Promotion snapshot | `AgencySnapshot` entity written on every `PromoteDraft` | Immutable entity type (`Immutable: true`) — promotion record |
| Publications | `AgencyPublication` entity written on explicit publish | Immutable entity type; version auto-incremented |
| Pre-delivered schema | `DefaultAgencySchema()` seeded on startup (idempotent) | All TypeDefinitions for Agency, Goal, Workflow, WorkItem, ConfiguredRole, AgencySnapshot, AgencyPublication |
| EntityService gRPC handler | `egserver.NewEntityServer` from SharedLib `entitygraph/server` | Generic CRUD over entitygraph; identical across all services; single source of truth — no Agency-specific handler code |
| Schema seed | `entitygraph.SeedSchema` from SharedLib | Idempotent startup helper; replaces per-service `seedSchemaIfNeeded`; reusable by all services |
| Entity gRPC route path | `egserver.GRPCServicePath` (`/entitygraph.v1.EntityService`) | Constant from SharedLib; registrar uses it when advertising entity HTTP routes to Cross |
| Error types | `errors.go` at module root | All exported errors in one place; no scattered sentinels |
| Value types | `models.go` at module root | Pure data structs; no methods except `AgencyDraftStatus.CanTransitionTo` |
| Per-workflow `event_flows` | JSON-string property on `Workflow` and `DraftWorkflow` (not just `Agency`) | One flow chart per workflow; legacy monolithic `Agency.event_flows` retained as fallback (FEAT-20260609-002) |
| Flow-file bundling location | Caller bundles `flows_<workflow.code>.json` siblings into agency.json before POST | Importer never touches the filesystem — `ImportDraft` consumes only the request body; the file-naming convention is enforced by the caller (e.g. `/dev-reimport-agency`) |
| Bulk import path | `AgencyService.ImportDraft` writes Draft\* entities directly via `entitygraph.DataManager` | Bypasses `AgencyManager`; idempotent upsert on `(draft_ref_code, code)`; `auto_promote=true` performs CreateDraft+PromoteDraft in one round-trip |

---

## 2. Package Structure

```
CodeValdAgency/
├── cmd/
│   └── main.go                  # Wires dependencies; seeds schema; reads agencyID at startup
├── go.mod
├── errors.go                    # All exported error types
├── models.go                    # Agency, Goal, Workflow, WorkItem, ConfiguredRole, AgencyDraft, AgencyDraftStatus consts
├── agency.go                    # AgencyManager interface + agencyManager implementation
├── schema.go                    # DefaultAgencySchema() — pre-delivered TypeDefinitions
├── internal/
│   ├── config/
│   │   └── config.go            # Config struct + loader (env / YAML)
│   ├── registrar/
│   │   └── registrar.go         # Cross registration heartbeat loop + CrossPublisher impl
│   │                            # Uses egserver.GRPCServicePath for entity route declarations
│   └── server/
│       ├── server.go            # AgencyService gRPC handlers — delegates to AgencyManager
│       ├── entity_server.go     # 14-line re-export of egserver.NewEntityServer from SharedLib
│       ├── import_server.go     # ImportDraft handler
│       └── errors.go            # AgencyService-domain gRPC error mapping (no EntityService mapping)
├── storage/
│   └── arangodb/
│       └── storage.go           # Config, Backend struct, ArangoDB implementation (thin wrapper over entitygraph)
├── proto/
│   └── codevaldagency/
│       └── agency.proto         # AgencyService only — EntityService moved to SharedLib proto/entitygraph/v1/
├── gen/
│   └── go/                      # Generated protobuf code (buf generate — do not hand-edit)
└── bin/
    └── codevaldagency           # Compiled binary
```

---

> Detailed specifications:
> - **Interfaces & gRPC surface** → [architecture-interfaces.md](architecture-interfaces.md)
> - **Value types** → [architecture-models.md](architecture-models.md)
> - **Graph topology & schema** → [architecture-graph.md](architecture-graph.md)
> - **ArangoDB storage** → [architecture-storage.md](architecture-storage.md)
> - **Lifecycle, flows & errors** → [architecture-flows.md](architecture-flows.md)
> - **Runtime configuration** → [architecture-configuration.md](architecture-configuration.md)
