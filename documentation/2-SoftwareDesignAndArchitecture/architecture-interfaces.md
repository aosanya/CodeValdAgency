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
    // ── Live agency (read-only once published) ────────────────────────────────

    // GetAgency returns the current live Agency.
    // Returns ErrAgencyNotPublished if no draft has been promoted yet.
    GetAgency(ctx context.Context) (Agency, error)

    // GetGoals returns all Goal entities linked to the live Agency.
    GetGoals(ctx context.Context) ([]Goal, error)

    // GetWorkflows returns all Workflow entities linked to the live Agency.
    GetWorkflows(ctx context.Context) ([]Workflow, error)

    // GetConfiguredRoles returns all ConfiguredRole entities linked to the live Agency.
    GetConfiguredRoles(ctx context.Context) ([]ConfiguredRole, error)

    // ── Bootstrap (first-time setup only) ────────────────────────────────────

    // SetAgencyDetails is the bootstrap path for first-time database setup.
    // It creates the initial Agency entity and the first open draft.
    // Returns ErrAgencyReadOnly if a live agency already exists — use
    // CreateDraft + PromoteDraft for subsequent changes.
    // Publishes cross.agency.created after a successful create.
    SetAgencyDetails(ctx context.Context, jsonStr string) (Agency, error)

    // ── Drafts ────────────────────────────────────────────────────────────────

    // CreateDraft forks the agency graph into a new open AgencyDraft.
    // forkedFromType must be "live" or "draft".
    // Returns ErrAgencyNotPublished if forkedFromType="live" and no live agency exists.
    // Returns ErrDraftNotFound if forkedFromType="draft" and the source does not exist.
    // Returns ErrDraftNotOpen if forkedFromType="draft" and the source is not open.
    // description must be non-empty.
    CreateDraft(ctx context.Context, description, forkedFromID, forkedFromType string) (AgencyDraft, error)

    // GetDraft retrieves a single draft by its ID.
    // Returns ErrDraftNotFound if no draft with that ID exists.
    GetDraft(ctx context.Context, draftID string) (AgencyDraft, error)

    // ListDrafts returns all drafts in descending creation order.
    ListDrafts(ctx context.Context) ([]AgencyDraft, error)

    // UpdateDraftDescription updates the human-readable description of an open draft.
    // Returns ErrDraftNotFound if the draft does not exist.
    // Returns ErrDraftNotOpen if the draft is not open.
    UpdateDraftDescription(ctx context.Context, draftID, description string) (AgencyDraft, error)

    // PromoteDraft replaces the live agency with the full sub-graph of the given draft.
    // The draft transitions from "open" to "promoted". Other open drafts are unaffected.
    // Publishes cross.agency.promoted on success.
    // Returns ErrDraftNotFound if the draft does not exist.
    // Returns ErrDraftNotOpen if the draft is not open.
    PromoteDraft(ctx context.Context, draftID string) (Agency, error)

    // ArchiveDraft soft-discards an open draft.
    // The draft transitions from "open" to "archived".
    // Returns ErrDraftNotFound if the draft does not exist.
    // Returns ErrDraftNotOpen if the draft is not open.
    ArchiveDraft(ctx context.Context, draftID string) (AgencyDraft, error)

    // ── Publications ──────────────────────────────────────────────────────────

    // PublishAgency creates an immutable AgencyPublication snapshot of the
    // current live Agency state. Version numbers are auto-incremented.
    // Returns ErrAgencyNotPublished if no live agency exists.
    PublishAgency(ctx context.Context) (AgencyPublication, error)

    // GetPublication retrieves a publication by its version number.
    GetPublication(ctx context.Context, version int) (AgencyPublication, error)

    // ListPublications returns all publications in ascending version order.
    ListPublications(ctx context.Context) ([]AgencyPublication, error)

    // UpdatePublicationStatus transitions a publication to a new status.
    // Allowed: draft → active, active → archived (archived is terminal).
    // Returns ErrPublicationNotFound if no publication with that version exists.
    // Returns ErrInvalidPublicationStatus if the transition is not permitted.
    UpdatePublicationStatus(ctx context.Context, version int, status string) (AgencyPublication, error)
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

Defined in `models.go` at the module root. All types are pure value structs.
The only method is `AgencyDraftStatus.CanTransitionTo`.

Sub-resources (WorkItems, Instructions, Deliverables, ContentRefs) are **never
embedded** as slices on their parent struct — they are fetched via separate
`AgencyManager` methods.

### Core types

```go
// ActorType constrains who may fill a ConfiguredRole.
type ActorType string

const (
    ActorTypeHuman        ActorType = "human"
    ActorTypeAIAgent      ActorType = "ai_agent"
    ActorTypeComputeAgent ActorType = "compute_agent"
)

// AgencyDraftStatus is the lifecycle state of an AgencyDraft.
type AgencyDraftStatus string

const (
    // DraftStatusOpen means the draft is being edited and can be promoted.
    DraftStatusOpen AgencyDraftStatus = "open"

    // DraftStatusPromoted is a terminal state: this draft became the live agency.
    DraftStatusPromoted AgencyDraftStatus = "promoted"

    // DraftStatusArchived is a terminal state: this draft was soft-discarded.
    DraftStatusArchived AgencyDraftStatus = "archived"
)

// CanTransitionTo returns true if transitioning from the receiver status to
// next is a permitted move.
//
//	open → promoted | archived
//	promoted → (none — terminal)
//	archived → (none — terminal)
func (s AgencyDraftStatus) CanTransitionTo(next AgencyDraftStatus) bool
```

### Domain types (graph entity counterparts)

```go
// Agency is the live, published version of the agency.
// It is read-only — all edits flow through an AgencyDraft.
// GetAgency returns ErrAgencyNotPublished until the first draft is promoted.
type Agency struct {
    ID        string
    Name      string
    Mission   string
    Vision    string
    Enabled   bool      // true = agency is active; false = disabled
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

// Workflow is an ordered container of WorkItems, linked to the Agency
// via a has_workflow edge. WorkItems are fetched separately.
type Workflow struct {
    ID          string
    Name        string
    Description string
    Ordinality  int // execution order among Workflows on this Agency
}

// WorkItem is a single unit of work within a Workflow.
// Instructions and Deliverables are fetched separately.
type WorkItem struct {
    ID          string
    Title       string
    Description string
    Ordinality  int    // execution order within the Workflow
    Prompt      string // optional agent prompt delivered to the actor at dispatch
}

// Instruction is an ordered rule or constraint attached to a Workflow or
// WorkItem. Uses the multi-parent pattern — the parent is whichever
// belongs_to_* relationship was set at creation.
type Instruction struct {
    ID         string
    Content    string
    Ordinality int // sort order among Instructions on the parent
}

// Deliverable is the spec of an expected output for a WorkItem.
// DeliverableResult is the corresponding instance (one per submission).
type Deliverable struct {
    ID          string
    Title       string
    Description string
    Ordinality  int  // sort order among Deliverables on the WorkItem
    Blocking    bool // if true, a rejected result halts the Workflow until waived
}

// DeliverableResult is an immutable record of a single submission against
// a Deliverable. Multiple results may exist per Deliverable (full audit trail).
// ProducedAt is server-stamped at creation — callers do not set it.
//
// Status lifecycle: pending → completed | rejected → waived
type DeliverableResult struct {
    ID         string
    Status     string    // "pending" | "completed" | "rejected" | "waived"
    ProducedAt time.Time // server-stamped; zero value until persisted
}

// ContentRef is an immutable pointer to a single artifact path in CodeValdGit.
// Uses the multi-parent pattern — attachable to a DeliverableResult,
// an Instruction, or a WorkItem.
type ContentRef struct {
    ID   string
    Path string // relative path within the CodeValdGit repo for this agency
}

// ConfiguredRole is a named role entity defined by the agency beyond the
// built-in roles. Linked to the Agency and referenced via assigned_role
// edges on WorkItems.
type ConfiguredRole struct {
    ID          string
    Name        string
    Description string
    ActorType   ActorType // human | ai_agent | compute_agent
    Ordinality  int       // sort order among ConfiguredRoles on this Agency
}

// AgencyDraft is a mutable, full deep-copy of the agency graph.
// It holds Goals, Workflows, WorkItems, ConfiguredRoles, Instructions,
// and Deliverables forked from the live agency or another open draft.
// Sub-resources are fetched via the same AgencyManager convenience methods
// scoped to the draft's own entity set.
type AgencyDraft struct {
    ID             string
    Description    string            // required; human-readable label
    ForkedFromID   string            // ID of the source Agency or AgencyDraft
    ForkedFromType string            // "live" or "draft"
    Status         AgencyDraftStatus
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

// AgencySnapshot is an immutable point-in-time record written as a side-effect
// of PromoteDraft. Written once per promotion, never modified.
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
}
```

### Request types

```go
// UpdateAgencyRequest is used to edit the scalar fields of an open AgencyDraft.
// Sub-resources (Goals, Workflows, etc.) are managed through EntityService CRUD.
type UpdateDraftDetailsRequest struct {
    Name    string
    Mission string
    Vision  string
    Enabled bool
}
```
