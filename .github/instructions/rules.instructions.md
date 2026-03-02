````instructions
---
applyTo: '**'
---

# CodeValdAgency — Code Structure Rules

## Service Design Principles

CodeValdAgency is a **Go gRPC microservice** — not a library and not a monolith.
These rules reflect that:

- **Has a `cmd/main.go` binary entry point** — wires all dependencies and starts the server
- **No business logic in `cmd/`** — `main.go` only constructs dependencies and calls `server.Run`
- **Callers inject dependencies** — `Backend` and any cross-service clients are never hardcoded
- **Exported API surface is minimal** — expose only what other packages within this module need
- **No AI agent logic, LLM integration, or frontend code** — this service owns agencies only

---

## Interface-First Design

**Always define interfaces before concrete types.**

```go
// ✅ CORRECT — interface at root package level; concrete impl is unexported in internal/manager/
type AgencyManager interface {
    CreateAgency(ctx context.Context, req CreateAgencyRequest) (Agency, error)
    GetAgency(ctx context.Context, agencyID string) (Agency, error)
    UpdateAgency(ctx context.Context, agencyID string, req UpdateAgencyRequest) (Agency, error)
    DeleteAgency(ctx context.Context, agencyID string) error
    ListAgencies(ctx context.Context, filter AgencyFilter) ([]Agency, error)
}

// ❌ WRONG — leaking a concrete storage struct to callers
type ArangoAgencyManager struct {
    db driver.Database
}
```

**File layout — one primary concern per file:**

```
errors.go                            → ErrAgencyNotFound, ErrAgencyAlreadyExists
models.go                            → Agency, CreateAgencyRequest, UpdateAgencyRequest, AgencyFilter
internal/manager/manager.go          → Concrete AgencyManager implementation
internal/server/server.go            → Inbound gRPC server (AgencyService handlers)
internal/config/config.go            → Configuration struct + loader
storage/arangodb/storage.go          → ArangoDB Backend implementation
cmd/main.go                          → Dependency wiring only
```

---

## Agency Lifecycle Rules

**`CreateAgency` is the most critical operation in this service.**

```go
// ✅ CORRECT — publish cross.agency.created after every successful creation
func (m *manager) CreateAgency(ctx context.Context, req CreateAgencyRequest) (Agency, error) {
    agency, err := m.backend.Insert(ctx, req)
    if err != nil {
        return Agency{}, err
    }
    // MANDATORY: publish so Cross can trigger git init + work setup
    m.crossClient.Publish(ctx, "cross.agency.created", agency.ID)
    return agency, nil
}

// ❌ WRONG — creating an agency without publishing the event
func (m *manager) CreateAgency(ctx context.Context, req CreateAgencyRequest) (Agency, error) {
    return m.backend.Insert(ctx, req)
    // silent return — Cross never learns about the new agency
}
```

---

## gRPC Handler Rules

**Handlers are thin — delegate immediately to `AgencyManager`.**

```go
// ✅ CORRECT — handler delegates to interface
func (s *server) CreateAgency(ctx context.Context, req *pb.CreateAgencyRequest) (*pb.Agency, error) {
    agency, err := s.manager.CreateAgency(ctx, toModel(req))
    if err != nil {
        return nil, toGRPCError(err)
    }
    return toProto(agency), nil
}

// ❌ WRONG — business logic inside handler
func (s *server) CreateAgency(ctx context.Context, req *pb.CreateAgencyRequest) (*pb.Agency, error) {
    // don't put ArangoDB calls, validation, or pub/sub here
    doc, err := s.db.Collection("agencies").CreateDocument(ctx, req)
    ...
}
```

---

## Storage Backend Rules

The `Backend` interface is the injection point. The caller (`cmd/main.go`) constructs
the desired `Backend` and passes it to `NewAgencyManager`. The root package and
`internal/manager/` never import ArangoDB drivers directly.

```go
// Backend interface — defined in root package or internal/manager/
type Backend interface {
    Insert(ctx context.Context, req CreateAgencyRequest) (Agency, error)
    Get(ctx context.Context, agencyID string) (Agency, error)
    Update(ctx context.Context, agencyID string, req UpdateAgencyRequest) (Agency, error)
    Delete(ctx context.Context, agencyID string) error
    List(ctx context.Context, filter AgencyFilter) ([]Agency, error)
}

// ✅ CORRECT — Backend injected by cmd/main.go
b, _ := arangodb.NewBackend(cfg.ArangoDB)
mgr := manager.NewAgencyManager(b)

// ❌ WRONG — hardcoded driver inside manager
func NewAgencyManager() AgencyManager {
    db, _ := arangodb.NewDatabase(...)
    return &agencyManager{db: db}
}
```

---

## CodeValdCross Registration Rules

**Registration must happen on startup and repeat as a liveness heartbeat.**

```go
// ✅ CORRECT — register on startup with heartbeat loop
func register(ctx context.Context, crossAddr string) {
    req := &pb.RegisterRequest{
        ServiceName: "codevaldagency",
        Addr:        ":50053",
        Produces:    []string{"cross.agency.created"},
        Consumes:    []string{},
        Routes:      agencyRoutes(),
    }
    for {
        if err := crossClient.Register(ctx, req); err != nil {
            log.Printf("codevaldagency: register error: %v", err)
        }
        select {
        case <-ctx.Done():
            return
        case <-time.After(20 * time.Second):
        }
    }
}

// ❌ WRONG — register once and forget (Cross will drop the service after timeout)
func main() {
    crossClient.Register(ctx, req)
    server.Run(ctx)
}
```

---

## Error Types

All exported errors live in `errors.go`. Never scatter sentinel errors across files.

```go
// errors.go — all exported error types
var (
    ErrAgencyNotFound      = errors.New("agency not found")
    ErrAgencyAlreadyExists = errors.New("agency already exists")
)
```

Map errors to gRPC status codes in the server layer, not in the manager:

```go
// internal/server/server.go
func toGRPCError(err error) error {
    switch {
    case errors.Is(err, ErrAgencyNotFound):
        return status.Error(codes.NotFound, err.Error())
    case errors.Is(err, ErrAgencyAlreadyExists):
        return status.Error(codes.AlreadyExists, err.Error())
    default:
        return status.Error(codes.Internal, err.Error())
    }
}
```

---

## Context & Cancellation Rules

- Every public method takes `context.Context` as the first argument
- Check `ctx.Err()` in loops (heartbeat, retry loops)
- Pass `ctx` to all storage calls and cross-service calls
- Never use `context.Background()` inside library code — accept context from caller

---

## Naming Conventions

| Category | Convention | Example |
|---|---|---|
| Branch | `feature/AGENCY-XXX_description` | `feature/AGENCY-001_create-agency` |
| Commit | `AGENCY-XXX: message` | `AGENCY-001: Add CreateAgency gRPC handler` |
| Package | lowercase, no abbreviations | `agency`, `manager`, `server` |
| Interfaces | noun-only | `AgencyManager`, `Backend` |
| Exported types | PascalCase | `Agency`, `CreateAgencyRequest` |
| gRPC service | `AgencyService` | in `proto/codevaldagency/agency.proto` |

---

## Anti-Patterns

- ❌ **AI/LLM calls** — not in this service
- ❌ **Frontend routes or HTML templates** — CodeValdFortex only
- ❌ **Work item or task management** — CodeValdWork only
- ❌ **Git operations** — CodeValdGit only
- ❌ **Pub/sub topic strings as raw literals** — define as constants
- ❌ **Business logic in gRPC handlers** — delegate to `AgencyManager`
- ❌ **Hardcoded ArangoDB connection in manager** — inject `Backend`
- ❌ **Skipping `cross.agency.created`** — always publish on creation
````
