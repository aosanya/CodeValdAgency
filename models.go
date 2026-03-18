package codevaldagency

import "time"

// RACILabel is the RACI designation for a role assignment on a Work Item edge.
type RACILabel string

const (
// RACIResponsible is assigned to the actor who performs the work.
RACIResponsible RACILabel = "R"
// RACIAccountable is assigned to the actor who owns the outcome.
RACIAccountable RACILabel = "A"
// RACIConsulted is assigned to the actor whose input is sought.
RACIConsulted RACILabel = "C"
// RACIInformed is assigned to the actor who receives status updates.
RACIInformed RACILabel = "I"
)

// AgencyLifecycle is the progression state of an [Agency].
// Transitions are strictly forward-only; see [AgencyLifecycle.CanTransitionTo].
type AgencyLifecycle string

const (
// LifecycleDraft is the initial state — the agency is configured but not
// yet running.
LifecycleDraft AgencyLifecycle = "draft"

// LifecycleActive means work is currently in progress within the agency.
LifecycleActive AgencyLifecycle = "active"

// LifecycleAchieved is a terminal state — all goals have been met.
// No further lifecycle transitions are permitted.
LifecycleAchieved AgencyLifecycle = "achieved"
)

// CanTransitionTo reports whether transitioning from the receiver lifecycle
// state to next is a valid forward move.
//
// Allowed transitions:
//
//draft    → active
//active   → achieved
//achieved → (none — terminal)
func (l AgencyLifecycle) CanTransitionTo(next AgencyLifecycle) bool {
switch l {
case LifecycleDraft:
return next == LifecycleActive
case LifecycleActive:
return next == LifecycleAchieved
default:
// achieved is terminal — no further transitions.
return false
}
}

// Agency is the root entity of an agency graph. One Agency entity exists per
// database. Sub-resources (Goals, Workflows, WorkItems, ConfiguredRoles) are
// separate entities linked via edges in the entity graph.
type Agency struct {
// ID is the unique identifier for this agency.
ID string

// Name is the human-readable label for the agency.
Name string

// Mission describes the agency's core purpose.
Mission string

// Vision describes the long-term aspiration of the agency.
Vision string

// Status is the current lifecycle state of the agency.
Status AgencyLifecycle

// CreatedAt is the time at which the agency was first persisted.
CreatedAt time.Time

// UpdatedAt is the time at which the agency was most recently modified.
UpdatedAt time.Time
}

// Goal is a strategic objective entity linked to the Agency via a has_goal edge.
type Goal struct {
// ID is the unique identifier for this goal.
ID string

// Title is a short, human-readable label for the goal.
Title string

// Description provides additional context about the intended outcome.
Description string

// Ordinality is the priority or execution order among goals on this agency.
// Lower values indicate higher priority.
Ordinality int
}

// Workflow is a named, ordered container of [WorkItem] entities, linked to
// the Agency via a has_workflow edge.
type Workflow struct {
// ID is the unique identifier for this workflow.
ID string

// Name is a human-readable label for the workflow.
Name string

// WorkItems is the ordered list of work items in this workflow.
// Populated by AgencyManager.GetWorkflows; empty in raw entity reads.
WorkItems []WorkItem
}

// WorkItem is a single unit of work within a [Workflow], linked via a
// has_work_item edge. Relationships to Goals and ConfiguredRoles are stored as
// advances_goal and assigned_role edges respectively.
type WorkItem struct {
// ID is the unique identifier for this work item.
ID string

// Title is a short, human-readable label for the work item.
Title string

// Description provides additional context about what must be done.
Description string

// Order is the explicit execution sequence within the workflow.
Order int

// Parallel indicates that this item may run concurrently with adjacent
// items that share the same Order value.
Parallel bool
}

// ConfiguredRole is a named role entity defined by an agency beyond the
// built-in roles. It is linked to the Agency and may be referenced via
// assigned_role edges on WorkItems.
type ConfiguredRole struct {
// ID is the unique identifier for this configured role.
ID string

// Name is the human-readable label for the role (e.g. "domain_expert").
Name string
}

// AgencySnapshot is an immutable point-in-time record captured at the
// draft → active lifecycle transition. Written once, never modified.
type AgencySnapshot struct {
// ID is the unique identifier for this snapshot.
ID string

// AgencyID is the identifier of the agency this snapshot belongs to.
AgencyID string

// SnapshotAt is the exact time the draft → active transition occurred.
SnapshotAt time.Time
}

// AgencyPublication is an immutable, versioned snapshot created by an explicit
// publish action. The agency lifecycle Status is never changed by publishing.
// Publications are written once and never modified.
type AgencyPublication struct {
// ID is the unique identifier for this publication.
ID string

// AgencyID is the identifier of the agency this publication belongs to.
AgencyID string

// Version is the auto-incrementing publication number (1, 2, 3, …).
Version int

// Tag is the human-readable version label (e.g. "v1", "v2").
Tag string

// PublishedAt is the exact time this publication was created.
PublishedAt time.Time
}

// UpdateAgencyRequest carries the mutable fields of an existing agency.
// Set only the fields you want to change; the manager validates lifecycle
// transitions before delegating to the storage backend.
type UpdateAgencyRequest struct {
Name    string
Mission string
Vision  string
Status  AgencyLifecycle
}
