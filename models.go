package codevaldagency

import "time"

// RACILabel is the RACI designation for a role assignment on a Work Item.
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

// AgencyRole is the role type for agency management.
// Both human actors and AI agents may be assigned any role.
type AgencyRole string

const (
	// RoleSuperAdmin is a platform-level default role present on every agency;
	// it provides full agency management access and cannot be removed.
	RoleSuperAdmin AgencyRole = "super_admin"

	// RoleAdmin is an agency-level default role present on every agency;
	// it manages members, workflows, and configuration and cannot be removed.
	RoleAdmin AgencyRole = "admin"
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
//	draft    → active
//	active   → achieved
//	achieved → (none — terminal)
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

// RoleAssignment binds an [AgencyRole] to a [RACILabel] for a specific
// [WorkItem].
type RoleAssignment struct {
	Role AgencyRole
	RACI RACILabel
}

// WorkItem is a single unit of work within a [Workflow].
type WorkItem struct {
	// ID is the unique identifier for this work item within its workflow.
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

	// GoalIDs references one or more [Goal] IDs that this item advances.
	GoalIDs []string

	// Assignments lists the [RoleAssignment] pairs for this work item.
	Assignments []RoleAssignment
}

// Workflow is a named, ordered container of [WorkItem]s.
// Workflows have no own lifecycle — they inherit the [Agency] lifecycle.
type Workflow struct {
	// ID is the unique identifier for this workflow within its agency.
	ID string

	// Name is a human-readable label for the workflow.
	Name string

	// WorkItems is the ordered list of work items in this workflow.
	WorkItems []WorkItem
}

// Goal is a strategic objective that one or more [WorkItem]s contribute to.
type Goal struct {
	// ID is the unique identifier for this goal within its agency.
	ID string

	// Title is a short, human-readable label for the goal.
	Title string

	// Description provides additional context about the intended outcome.
	Description string

	// Ordinality is the priority or execution order among goals on this agency.
	// Lower values indicate higher priority.
	Ordinality int
}

// Agency is the top-level organisational unit with Mission, Vision, Goals,
// and Workflows. All other CodeVald services scope their operations by AgencyID.
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

	// Goals is the list of strategic objectives for this agency.
	Goals []Goal

	// Workflows is the list of ordered work containers for this agency.
	Workflows []Workflow

	// ConfiguredRoles lists additional role names beyond [RoleSuperAdmin] and
	// [RoleAdmin]; these are free-form strings defined by the agency.
	ConfiguredRoles []string

	// CreatedAt is the time at which the agency was first persisted.
	CreatedAt time.Time

	// UpdatedAt is the time at which the agency was most recently modified.
	UpdatedAt time.Time
}

// AgencySnapshot is an immutable point-in-time copy of an [Agency] captured
// at the moment it transitions from [LifecycleDraft] to [LifecycleActive].
// It is written once and never updated or deleted.
type AgencySnapshot struct {
	// ID is the unique identifier for this snapshot (distinct from AgencyID).
	ID string

	// AgencyID is the foreign key identifying the agency this snapshot belongs to.
	AgencyID string

	Name            string
	Mission         string
	Vision          string
	Goals           []Goal
	Workflows       []Workflow
	ConfiguredRoles []string

	// SnapshotAt is the exact time the draft → active transition occurred.
	SnapshotAt time.Time
}

// CreateAgencyRequest carries the fields required to create a new agency.
// The agency starts in [LifecycleDraft] with the supplied Name, Mission, and
// Vision.
type CreateAgencyRequest struct {
	Name    string
	Mission string
	Vision  string
}

// UpdateAgencyRequest carries the mutable fields of an existing agency.
// Set only the fields you want to change; the manager validates lifecycle
// transitions before delegating to the storage backend.
type UpdateAgencyRequest struct {
	Name            string
	Mission         string
	Vision          string
	Status          AgencyLifecycle
	Goals           []Goal
	Workflows       []Workflow
	ConfiguredRoles []string
}

// AgencyFilter constrains the result set returned by [AgencyManager.ListAgencies].
type AgencyFilter struct {
	// Offset is the zero-based index of the first result to return.
	Offset int

	// Limit is the maximum number of results to return.
	// A value of 0 means no limit.
	Limit int

	// Status optionally restricts results to agencies in the given lifecycle
	// state. An empty string means no filter is applied.
	Status AgencyLifecycle
}
