# CodeValdAgency — Architecture

## 1. Core Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Business-logic entry point | `AgencyManager` interface | gRPC handlers delegate to it; never put logic in handlers |
| Downstream communication | gRPC only — no direct Go imports | Stable, versioned contracts; independent deployment |
| Storage injection | `Backend` interface injected by `cmd/main.go` | Backend-agnostic core; easy to test with mocks |
| Cross registration | `OrchestratorService.Register` RPC on startup + heartbeat | Standard CodeVald onboarding pattern; liveness via repeat calls |
| Pub/sub event | `cross.agency.created` published on every `CreateAgency` | Cross listens to trigger `GitInitRepo` + Work setup |
| Default roles | `SuperAdmin` + `Admin` always present; cannot be removed | Every Agency must always have management roles; avoids zero-admin state |
| Configurable roles | Free-form strings in `Agency.ConfiguredRoles[]` | No fixed enum beyond the two defaults; Agencies name their own roles |
| Lifecycle enforcement | Forward-only (`draft → active → achieved`) | Prevents rollback; `achieved` is terminal and read-only |
| Activation snapshot | Full copy written to `agency_snapshots` on `draft → active` | Immutable audit record of the Agency config at the moment it went live |
| Error types | `errors.go` at module root | All exported errors in one place; no scattered sentinels |
| Value types | `models.go` at module root | `Agency`, `CreateAgencyRequest`, `AgencyFilter` — pure data, no methods |

---

## 2. AgencyManager Interface

```go
// AgencyManager is the sole business-logic entry point for agency operations.
// gRPC handlers hold this interface — never the concrete type.
type AgencyManager interface {
    CreateAgency(ctx context.Context, req CreateAgencyRequest) (Agency, error)
    GetAgency(ctx context.Context, agencyID string) (Agency, error)
    UpdateAgency(ctx context.Context, agencyID string, req UpdateAgencyRequest) (Agency, error)
    DeleteAgency(ctx context.Context, agencyID string) error
    ListAgencies(ctx context.Context, filter AgencyFilter) ([]Agency, error)
}
```

### Backend Interface (storage injection point)

```go
// Backend is the storage contract injected into AgencyManager.
// cmd/main.go constructs the chosen implementation (e.g. arangodb.NewBackend).
type Backend interface {
    Insert(ctx context.Context, req CreateAgencyRequest) (Agency, error)
    Get(ctx context.Context, agencyID string) (Agency, error)
    Update(ctx context.Context, agencyID string, req UpdateAgencyRequest) (Agency, error)
    Delete(ctx context.Context, agencyID string) error
    List(ctx context.Context, filter AgencyFilter) ([]Agency, error)

    // InsertSnapshot writes a point-in-time copy of an Agency to agency_snapshots.
    // Called by AgencyManager.UpdateAgency immediately after a draft → active transition.
    InsertSnapshot(ctx context.Context, snapshot AgencySnapshot) error
}
```

---

## 3. Data Models

```go
// RACILabel is the RACI designation for a role assignment on a Work Item.
type RACILabel string

const (
    RACIResponsible RACILabel = "R" // Does the work
    RACIAccountable RACILabel = "A" // Owns the outcome
    RACIConsulted   RACILabel = "C" // Provides input
    RACIInformed    RACILabel = "I" // Receives updates
)

// AgencyRole is the role type for Agency management.
// Both human actors and AI agents may be assigned any role.
type AgencyRole string

const (
    // Default roles — always present on every Agency; cannot be removed.
    // No configuration required; every new Agency has these automatically.
    RoleSuperAdmin AgencyRole = "super_admin" // Platform-level; full Agency management access
    RoleAdmin      AgencyRole = "admin"       // Agency-level; manages members, Workflows, config
)

// AgencyLifecycle is the progression of an Agency.
type AgencyLifecycle string

const (
    LifecycleDraft    AgencyLifecycle = "draft"    // Configured, not yet running
    LifecycleActive   AgencyLifecycle = "active"   // Work is in progress
    LifecycleAchieved AgencyLifecycle = "achieved" // All Goals met; terminal state
)

// RoleAssignment binds a role to a RACI label for a specific Work Item.
type RoleAssignment struct {
    Role AgencyRole
    RACI RACILabel
}

// WorkItem is a single unit of work within a Workflow.
type WorkItem struct {
    ID          string
    Title       string
    Description string
    Order       int              // Explicit execution sequence within the Workflow
    Parallel    bool             // If true, may run concurrently with adjacent items
    GoalIDs     []string         // References one or more Goal IDs this item advances
    Assignments []RoleAssignment // Role + RACI pairs for this step
}

// Workflow is a named container of ordered Work Items.
// Workflows have no own lifecycle — they inherit the Agency lifecycle.
type Workflow struct {
    ID        string
    Name      string
    WorkItems []WorkItem
}

// Goal is a strategic objective that one or more Work Items contribute to.
type Goal struct {
    ID          string
    Title       string
    Description string
    Ordinality  int // Priority/execution order among Goals on this Agency
}

// Agency is the top-level organisational unit with Mission, Vision, Goals, and Workflows.
// All other services scope operations by AgencyID.
type Agency struct {
    ID              string
    Name            string
    Mission         string          // "To coordinate AI agents toward a defined objective..."
    Vision          string          // Long-term aspiration for this Agency
    Status          AgencyLifecycle
    Goals           []Goal
    Workflows       []Workflow
    ConfiguredRoles []string        // Additional role names beyond SuperAdmin + Admin; free-form
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

// AgencySnapshot is an immutable point-in-time copy of an Agency captured the
// moment it transitions from draft → active. Written once; never updated or deleted.
type AgencySnapshot struct {
    ID              string          // Own UUID (distinct from AgencyID)
    AgencyID        string          // Foreign key — the Agency this snapshot belongs to
    Name            string
    Mission         string
    Vision          string
    Goals           []Goal
    Workflows       []Workflow
    ConfiguredRoles []string
    SnapshotAt      time.Time       // Exact time the draft → active transition occurred
}

type CreateAgencyRequest struct {
    Name    string
    Mission string
    Vision  string
}

type UpdateAgencyRequest struct {
    Name            string
    Mission         string
    Vision          string
    Status          AgencyLifecycle
    Goals           []Goal
    Workflows       []Workflow
    ConfiguredRoles []string // Additional role names beyond the two defaults; free-form
}

type AgencyFilter struct {
    Offset int
    Limit  int
    Status AgencyLifecycle // Optional: filter by lifecycle state
}
```

---

## 4. Project Structure

```
CodeValdAgency/
├── cmd/
│   └── main.go                  # Wires dependencies only — no business logic
├── go.mod
├── errors.go                    # ErrAgencyNotFound, ErrAgencyAlreadyExists
├── models.go                    # Agency, Goal, Workflow, WorkItem, RoleAssignment,
│                                # AgencyRole, RACILabel, AgencyLifecycle, request types
├── internal/
│   ├── config/
│   │   └── config.go            # Configuration struct + loader (env / YAML)
│   ├── manager/
│   │   └── manager.go           # Concrete AgencyManager — holds Backend + CrossClient
│   └── server/
│       └── server.go            # Inbound gRPC server — AgencyService handlers
├── storage/
│   └── arangodb/
│       └── storage.go           # ArangoDB Backend implementation
├── proto/
│   └── codevaldagency/
│       └── agency.proto         # AgencyService gRPC definition
├── gen/
│   └── go/                      # Generated protobuf code (buf generate — do not hand-edit)
└── bin/
    └── codevaldagency-server    # Compiled binary
```

---

## 5. gRPC Service Definition

```protobuf
syntax = "proto3";
package codevaldagency.v1;

service AgencyService {
    rpc CreateAgency (CreateAgencyRequest) returns (Agency);
    rpc GetAgency    (GetAgencyRequest)    returns (Agency);
    rpc UpdateAgency (UpdateAgencyRequest) returns (Agency);
    rpc DeleteAgency (DeleteAgencyRequest) returns (DeleteAgencyResponse);
    rpc ListAgencies (ListAgenciesRequest) returns (ListAgenciesResponse);
}
```

Generated Go code lives in `gen/go/`. **Never hand-edit generated files.**

---

## 6. CodeValdCross Registration

On startup, `cmd/main.go` starts a registration heartbeat. The loop calls
`OrchestratorService.Register` on CodeValdCross every **20 seconds**.

```go
RegisterRequest{
    ServiceName: "codevaldagency",
    Addr:        ":50053",          // gRPC address Cross dials back on
    Produces:    []string{"cross.agency.created"},
    Consumes:    []string{},
    Routes: []Route{
        {Method: "POST",   Pattern: "/agencies"},
        {Method: "GET",    Pattern: "/agencies"},
        {Method: "GET",    Pattern: "/agencies/{agencyID}"},
        {Method: "PUT",    Pattern: "/agencies/{agencyID}"},
        {Method: "DELETE", Pattern: "/agencies/{agencyID}"},
    },
}
```

The repeat call is the **liveness signal** — Cross expires services that stop
registering. If Cross is not yet up, the loop retries silently.

---

## 7. ArangoDB Schema

| Collection | Document key | Key fields |
|---|---|---|
| `agencies` | `agency.ID` (UUID) | `name`, `mission`, `vision`, `status`, `goals[]`, `workflows[]`, `created_at`, `updated_at` |
| `agency_snapshots` | `snapshot.ID` (UUID) | `agency_id`, `name`, `mission`, `vision`, `goals[]`, `workflows[]`, `configured_roles[]`, `snapshot_at` |

**Embedded sub-documents** (stored inline in the agency document):

```
agencies/{id}
├── name             string
├── mission          string
├── vision           string
├── status           string  ("draft" | "active" | "achieved")
├── configured_roles []string  // Roles beyond the two defaults
├── goals[]
│   └── { id, title, description, ordinality }
├── workflows[]
│   └── { id, name,
│         work_items[]
│           └── { id, title, description, order, parallel,
│                 goal_ids[], assignments[{ role, raci }] } }
├── created_at     time
└── updated_at     time
```

**`agency_snapshots/{id}` document shape:**

```
agency_snapshots/{id}
├── agency_id        string  // FK → agencies/{id}
├── name             string
├── mission          string
├── vision           string
├── configured_roles []string
├── goals[]
│   └── { id, title, description, ordinality }
├── workflows[]
│   └── { id, name,
│         work_items[]
│           └── { id, title, description, order, parallel,
│                 goal_ids[], assignments[{ role, raci }] } }
└── snapshot_at      time  // draft → active transition timestamp
```

**Indexes:**
- Unique persistent index on `name` (prevent duplicate agency names)
- Persistent index on `status` (efficient lifecycle-filtered list queries)
- Persistent index on `agency_snapshots.agency_id` (fetch all snapshots for an Agency)
- Persistent index on `agency_snapshots.snapshot_at` (chronological ordering)

---

## 8. Error Types

Defined in `errors.go`:

```go
var (
    ErrAgencyNotFound      = errors.New("agency not found")
    ErrAgencyAlreadyExists = errors.New("agency already exists")
)
```

Map to gRPC status codes in `internal/server/server.go`:

| Error | gRPC code |
|---|---|
| `ErrAgencyNotFound` | `codes.NotFound` |
| `ErrAgencyAlreadyExists` | `codes.AlreadyExists` |
| all others | `codes.Internal` |

---

## 9. CreateAgency Flow (Critical Path)

```
gRPC handler
    │
    ▼
AgencyManager.CreateAgency(ctx, req)
    │
    ├── backend.Insert(ctx, req)   → ArangoDB write
    │       returns Agency{ID, Name, ...}
    │
    └── crossClient.Publish(ctx, "cross.agency.created", agencyID)
            │
            ▼
        CodeValdCross receives event
            ├── GitInitRepo(agencyID)
            └── Work onboarding
```

**`cross.agency.created` MUST be published** — it is the trigger for all
downstream onboarding. Never return successfully from `CreateAgency` without
publishing this event.

---

## 10. Agency Lifecycle

The Agency status progresses **forward only**. No backward transitions are permitted.

```
Draft ──► Active ──► Achieved
```

### State Definitions

| State | Meaning | Mutability |
|---|---|---|
| `draft` | Agency is being configured; Goals and Workflows are being defined | Fully mutable |
| `active` | Work is in progress; agents are executing Work Items | Mutable (Goals, Workflows, ConfiguredRoles) |
| `achieved` | Mission fulfilled; all Goals reached | **Read-only** — no further updates permitted |

### Transition Rules

| From | To | Trigger | Guard | Side-effect |
|---|---|---|---|---|
| `draft` | `active` | `UpdateAgency(Status: active)` | Agency must have at least one Goal and at least one Workflow containing at least one Work Item | Snapshot written to `agency_snapshots` |
| `active` | `achieved` | `UpdateAgency(Status: achieved)` | Caller must hold `super_admin` or `admin` role | — |
| any | any (backward) | — | **Rejected** with `codes.InvalidArgument` — lifecycle never moves backward | — |

### Delete Rules

| State | Delete Permitted |
|---|---|
| `draft` | ✅ Yes |
| `active` | ❌ No — must first transition to `achieved` or be force-deleted by Super Admin |
| `achieved` | ❌ No — terminal record; preserved for audit |

### Error Mapping for Invalid Transitions

| Violation | gRPC code |
|---|---|
| Backward status transition | `codes.InvalidArgument` |
| Activating Agency with no Goals/Workflows | `codes.FailedPrecondition` |
| Updating an `achieved` Agency | `codes.FailedPrecondition` |
| Deleting an `active` Agency without Super Admin role | `codes.PermissionDenied` |

---

## 11. Activation Snapshot Flow

When `UpdateAgency` drives a `draft → active` transition, the manager captures
a full copy of the Agency **after** the status update is persisted.

```
AgencyManager.UpdateAgency(ctx, agencyID, req{Status: active})
    │
    ├── guard: agency has ≥1 Goal + ≥1 Workflow with ≥1 WorkItem
    │       → codes.FailedPrecondition if violated
    │
    ├── backend.Update(ctx, agencyID, req)   → agencies collection (status = active)
    │       returns updated Agency
    │
    └── backend.InsertSnapshot(ctx, AgencySnapshot{
                ID:              newUUID(),
                AgencyID:        agency.ID,
                Name:            agency.Name,
                Mission:         agency.Mission,
                Vision:          agency.Vision,
                Goals:           agency.Goals,
                Workflows:       agency.Workflows,
                ConfiguredRoles: agency.ConfiguredRoles,
                SnapshotAt:      time.Now(),
            })  → agency_snapshots collection
```

**Rules:**
- The snapshot is written **after** the `agencies` document is updated so the
  snapshot always reflects the live-published state.
- `InsertSnapshot` failure is **not** propagated to the caller — the agency
  is already active; log the error and continue. (Best-effort audit record.)
- Snapshots are **immutable** — `agency_snapshots` has no update or delete path
  in the `Backend` interface.
- Multiple snapshots per Agency are **not** expected under normal operation
  (an agency transitions to `active` once), but the schema and index support
  multiple rows per `agency_id` in case of future replay or migration needs.
