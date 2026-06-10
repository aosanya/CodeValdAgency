# CodeValdAgency — Data Models

> Part of the split architecture. Index: [architecture.md](architecture.md)
>
> Companion files:
> - [architecture-interfaces.md](architecture-interfaces.md) — `AgencyManager`, `AgencySchemaManager`, `CrossPublisher`, gRPC service definitions
> - [architecture-graph.md](architecture-graph.md) — graph topology, entity types
> - [architecture-storage.md](architecture-storage.md) — ArangoDB document shapes

---

## 1. Overview

Defined in `models.go` at the module root. All types are pure value structs.
The only method is `AgencyDraftStatus.CanTransitionTo`.

Sub-resources (WorkItems, Instructions, Deliverables, ContentRefs) are **never
embedded** as slices on their parent struct — they are fetched via separate
`AgencyManager` methods.

---

## 2. Enumerations

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

---

## 3. Domain Types (Graph Entity Counterparts)

```go
// Agency is the live, published version of the agency.
// It is read-only — all edits flow through an AgencyDraft.
// GetAgency returns ErrAgencyNotPublished until the first draft is promoted.
type Agency struct {
    ID         string
    Name       string
    Mission    string
    Vision     string
    Enabled    bool   // true = agency is active; false = disabled
    EventFlows string // JSON-encoded { flows: [...] } legacy monolithic blob
    CreatedAt  time.Time
    UpdatedAt  time.Time
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
//
// EventFlows carries the per-workflow `{ flows: [{ name, steps: [...] }] }`
// block scoped to THIS workflow only. Imported from `flows_<code>.json`
// files bundled into agency.json by the caller (see FEAT-20260609-002 and
// [architecture-flows.md § ImportDraft Flow](architecture-flows.md)). Empty
// when the workflow predates the per-workflow-flows convention — readers
// fall back to Agency.EventFlows.
type Workflow struct {
    ID          string
    Name        string
    Description string
    Ordinality  int    // execution order among Workflows on this Agency
    EventFlows  string // per-workflow JSON blob (FEAT-20260609-002); may be ""
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

---

## 4. Request Types

```go
// UpdateDraftDetailsRequest is used to edit the scalar fields of an open AgencyDraft.
// Sub-resources (Goals, Workflows, etc.) are managed through EntityService CRUD.
type UpdateDraftDetailsRequest struct {
    Name    string
    Mission string
    Vision  string
    Enabled bool
}
```
