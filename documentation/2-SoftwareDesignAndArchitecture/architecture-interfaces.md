# CodeValdAgency — Interfaces & Models

> Part of the split architecture. Index: [architecture.md](architecture.md)

---

## 1. AgencyManager Interface

`AgencyManager` is the sole business-logic entry point. gRPC handlers hold
the interface, never the concrete type. The implementation wraps
`entitygraph.DataManager` to expose agency-specific convenience methods.

```go
// AgencyManager is the top-level interface for managing a single Agency.
// All gRPC handlers delegate to AgencyManager — never to storage directly.
type AgencyManager interface {
    // SetAgencyDetails persists a full Agency record from a JSON payload.
    // Creates or upserts the root Agency entity. Publishes cross.agency.created
    // after a successful create.
    SetAgencyDetails(ctx context.Context, jsonStr string) (Agency, error)

    // GetAgency returns the single Agency entity stored in this database.
    GetAgency(ctx context.Context) (Agency, error)

    // UpdateAgency applies partial updates and enforces lifecycle transitions.
    // Writing Status: active triggers an immutable AgencySnapshot entity.
    UpdateAgency(ctx context.Context, req UpdateAgencyRequest) (Agency, error)

    // GetGoals returns all Goal entities linked to the Agency.
    GetGoals(ctx context.Context) ([]Goal, error)

    // GetWorkflows returns all Workflow entities linked to the Agency,
    // each populated with its ordered WorkItems, top-level Instructions,
    // and each WorkItem's Instructions and Deliverables.
    GetWorkflows(ctx context.Context) ([]Workflow, error)

    // GetConfiguredRoles returns all ConfiguredRole entities linked to the Agency.
    GetConfiguredRoles(ctx context.Context) ([]ConfiguredRole, error)

    // PublishAgency creates an immutable AgencyPublication snapshot of the
    // current Agency state. Version numbers are auto-incremented.
    PublishAgency(ctx context.Context) (AgencyPublication, error)

    // GetPublication retrieves a publication by its version number.
    GetPublication(ctx context.Context, version int) (AgencyPublication, error)

    // ListPublications returns all publications in ascending version order.
    ListPublications(ctx context.Context) ([]AgencyPublication, error)
}
```

---

## 2. agencyManager Struct

The concrete implementation is unexported. `cmd/main.go` constructs it via
`NewAgencyManager` and passes it to the gRPC server.

```go
// agencyManager is the concrete implementation of AgencyManager.
// It wraps entitygraph.DataManager to provide agency-specific convenience methods.
type agencyManager struct {
    dataManager   entitygraph.DataManager   // graph CRUD — injected by cmd/main.go
    schemaManager AgencySchemaManager       // schema versioning — injected by cmd/main.go
    publisher     CrossPublisher            // publishes cross.agency.* events
    agencyID      string                    // loaded from the stored Agency entity at startup
}

// NewAgencyManager constructs an AgencyManager. The agencyID is read from the
// single Agency entity already present in the database, or left empty if no
// Agency has been created yet.
func NewAgencyManager(
    dm entitygraph.DataManager,
    sm AgencySchemaManager,
    pub CrossPublisher,
    agencyID string,
) AgencyManager {
    return &agencyManager{dataManager: dm, schemaManager: sm, publisher: pub, agencyID: agencyID}
}
```

---

## 3. AgencySchemaManager

`AgencySchemaManager` is a type alias for `entitygraph.SchemaManager` from
`CodeValdSharedLib`. No wrapping is needed — the Agency-specific logic is in
`DefaultAgencySchema()` (see [architecture-graph.md](architecture-graph.md)).

```go
// AgencySchemaManager manages schema versions for the Agency entity graph.
// It is a type alias for entitygraph.SchemaManager.
type AgencySchemaManager = entitygraph.SchemaManager
```

---

## 4. CrossPublisher Interface

```go
// CrossPublisher publishes events to CodeValdCross over gRPC.
// It is injected into agencyManager and implemented by internal/registrar.
type CrossPublisher interface {
    // Publish sends a named event with a string payload to CodeValdCross.
    // Errors are logged but never returned to the caller — the agency record
    // has already been persisted at the point of publication.
    Publish(ctx context.Context, topic, payload string) error
}
```

---

## 5. Data Models

Defined in `models.go` at the module root. All types are pure value structs;
methods are limited to `AgencyLifecycle.CanTransitionTo`.

### Core types

```go
// ActorType constrains who may fill a ConfiguredRole.
type ActorType string

const (
	ActorTypeHuman         ActorType = "human"
	ActorTypeAIAgent       ActorType = "ai_agent"
	ActorTypeComputeAgent  ActorType = "compute_agent"
type AgencyLifecycle string

const (
    LifecycleDraft    AgencyLifecycle = "draft"
    LifecycleActive   AgencyLifecycle = "active"
    LifecycleAchieved AgencyLifecycle = "achieved"
)

// CanTransitionTo returns true if moving from the current state to next is valid.
func (l AgencyLifecycle) CanTransitionTo(next AgencyLifecycle) bool
```

### Domain types (graph entity counterparts)

> **Note:** `models.go` is currently being updated to align with this specification.
> This section reflects the intended design as of the current schema revision.

```go
// Agency is the root entity. One Agency per database.
type Agency struct {
	ID        string
	Name      string
	Mission   string
	Vision    string
	Status    AgencyLifecycle
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Goal is a strategic objective linked to the Agency via a has_goal edge.
type Goal struct {
	ID          string
	Title       string
	Description string
	Ordinality  int // sort order among Goals; lower = higher priority
}

// Workflow is an ordered container of WorkItems and top-level Instructions,
// linked to the Agency via a has_workflow edge.
type Workflow struct {
	ID           string
	Name         string
	Description  string
	Ordinality   int           // execution order among Workflows on this Agency
	WorkItems    []WorkItem
	Instructions []Instruction
}

// WorkItem is a single unit of work within a Workflow.
// An agent executes a WorkItem by following its Instructions and producing
// the required Deliverables. May be assigned to a ConfiguredRole via
// assigned_role edges.
type WorkItem struct {
	ID           string
	Title        string
	Description  string
	Ordinality   int           // execution order within the Workflow
	Prompt       string        // optional agent prompt override for this item
	Instructions []Instruction
	Deliverables []Deliverable
}

// Instruction is an ordered rule or constraint attached to either a Workflow
// or a WorkItem. Uses the multi-parent pattern — the parent is determined by
// whichever belongs_to_* relationship was set at creation.
type Instruction struct {
	ID         string
	Content    string
	Ordinality int // sort order among Instructions on the parent
}

// Deliverable is the specification of an expected output for a WorkItem.
// It is a spec entity — DeliverableResult is the corresponding instance.
type Deliverable struct {
	ID             string
	Title          string
	Description    string
	Ordinality     int    // sort order among Deliverables on the WorkItem
	Blocking       bool   // if true, a rejected result halts the workflow until waived
	ReviewerRoleID string // ID of the ConfiguredRole with waiver authority; may be empty
}

// DeliverableResult is an immutable record of a single submission against
// a Deliverable. Multiple results may exist per Deliverable (full history).
// ProducedAt is server-stamped at creation — callers do not set it.
//
// Status lifecycle: pending → completed | rejected → waived
type DeliverableResult struct {
	ID          string
	Status      string    // "pending" | "completed" | "rejected" | "waived"
	ProducedAt  time.Time // server-stamped; zero value until persisted
	ContentRefs []ContentRef
}

// ContentRef is an immutable pointer to a single artifact path in CodeValdGit.
// Uses the multi-parent pattern — attachable to a DeliverableResult,
// an Instruction, or a WorkItem.
type ContentRef struct {
	ID   string
	Path string // relative path within the CodeValdGit repo for this agency
}

// ConfiguredRole is a named role entity defined by the agency beyond the
// built-in roles (super_admin, admin). Linked to the Agency and may be
// referenced by assigned_role edges on WorkItems.
type ConfiguredRole struct {
	ID          string
	Name        string
	Description string
	ActorType   ActorType // human | ai_agent | compute_agent
	Ordinality  int       // sort order among ConfiguredRoles on this Agency
}

// AgencySnapshot is an immutable point-in-time record captured at the
// draft → active lifecycle transition. Written once, never modified.
type AgencySnapshot struct {
	ID         string
	AgencyID   string
	SnapshotAt time.Time
}

// AgencyPublication is an immutable versioned snapshot created by an explicit
// publish action. Written once, never modified.
type AgencyPublication struct {
	ID          string
	AgencyID    string
	Version     int
	Tag         string
	PublishedAt time.Time
	Status      string // "draft" | "active" | "archived"

### Request types

```go
// UpdateAgencyRequest carries the fields that may be changed via UpdateAgency.
type UpdateAgencyRequest struct {
    Name    string
    Mission string
    Vision  string
    Status  AgencyLifecycle // forward-only; triggers guards and side-effects
}
```
