# CodeValdAgency — Architecture

> **Split docs** — this file is the index. Details live in focused companion files:
> - [architecture-interfaces.md](architecture-interfaces.md) — `AgencyManager`, `AgencySchemaManager`, data models
> - [architecture-graph.md](architecture-graph.md) — graph topology, entity types, pre-delivered schema
> - [architecture-storage.md](architecture-storage.md) — ArangoDB collections, document shapes, indexes
> - [architecture-flows.md](architecture-flows.md) — lifecycle rules, flows, error types, gRPC service

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
| Lifecycle enforcement | Forward-only (`draft → active → achieved`) | Prevents rollback; `achieved` is terminal and read-only |
| Activation snapshot | `AgencySnapshot` entity written on `draft → active` | Immutable entity type (`Immutable: true`) — audit record |
| Publications | `AgencyPublication` entity written on explicit publish | Immutable entity type; version auto-incremented |
| Pre-delivered schema | `DefaultAgencySchema()` seeded on startup (idempotent) | All TypeDefinitions for Agency, Goal, Workflow, WorkItem, ConfiguredRole, AgencySnapshot, AgencyPublication |
| Error types | `errors.go` at module root | All exported errors in one place; no scattered sentinels |
| Value types | `models.go` at module root | Pure data structs; no methods except `AgencyLifecycle.CanTransitionTo` |

---

## 2. Package Structure

```
CodeValdAgency/
├── cmd/
│   └── main.go                  # Wires dependencies; seeds schema; reads agencyID at startup
├── go.mod
├── errors.go                    # All exported error types
├── models.go                    # Agency, Goal, Workflow, WorkItem, ConfiguredRole, lifecycle consts
├── agency.go                    # AgencyManager interface + agencyManager implementation
├── schema.go                    # DefaultAgencySchema() — pre-delivered TypeDefinitions
├── internal/
│   ├── config/
│   │   └── config.go            # Config struct + loader (env / YAML)
│   ├── registrar/
│   │   └── registrar.go         # Cross registration heartbeat loop + CrossPublisher impl
│   └── server/
│       └── server.go            # Inbound gRPC server — AgencyService handlers
├── storage/
│   └── arangodb/
│       ├── storage.go   # Config, Backend struct, constructors, ensureCollection
│       ├── docs.go      # ArangoDB document types and domain↔document conversions
│       └── ops.go       # Backend interface method implementations
│       (008-D will split further into entities.go, relationships.go, schemaops.go)
├── proto/
│   └── codevaldagency/
│       └── agency.proto         # AgencyService gRPC definition
├── gen/
│   └── go/                      # Generated protobuf code (buf generate — do not hand-edit)
└── bin/
    └── codevaldagency           # Compiled binary
```

---

> Detailed specifications:
> - **Interfaces & models** → [architecture-interfaces.md](architecture-interfaces.md)
> - **Graph topology & schema** → [architecture-graph.md](architecture-graph.md)
> - **ArangoDB storage** → [architecture-storage.md](architecture-storage.md)
> - **Lifecycle, flows & errors** → [architecture-flows.md](architecture-flows.md)
