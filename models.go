package codevaldagency

import "time"

// ActorType constrains who may fill a [ConfiguredRole].
type ActorType string

const (
	// ActorTypeHuman restricts the role to human actors only.
	ActorTypeHuman ActorType = "human"

	// ActorTypeAIAgent restricts the role to LLM-backed AI agent actors.
	ActorTypeAIAgent ActorType = "ai_agent"

	// ActorTypeComputeAgent restricts the role to deterministic compute agents.
	ActorTypeComputeAgent ActorType = "compute_agent"
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
// draft    → active
// active   → achieved
// achieved → (none — terminal)
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
// separate entities linked via edges in the entity graph and are fetched via
// dedicated AgencyManager methods.
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

	// Description provides context about the purpose of this workflow.
	Description string

	// Ordinality controls the execution order among Workflows on this Agency.
	// Lower values run first; equal values run in parallel.
	Ordinality int
}

// WorkItem is a single unit of work within a [Workflow], linked via a
// has_work_item edge.
type WorkItem struct {
	// ID is the unique identifier for this work item.
	ID string

	// Title is a short, human-readable label for the work item.
	Title string

	// Description provides additional context about what must be done.
	Description string

	// Ordinality controls the execution order within the Workflow.
	// Items with the same value run in parallel; higher values run after lower.
	Ordinality int

	// Prompt is the task-specific input sent to the actor at dispatch time.
	// For ai_agent: the LLM prompt. For compute_agent: the function input.
	// For human: the task brief shown in the UI.
	Prompt string
}

// ConfiguredRole is a named role entity defined by an agency beyond the
// built-in roles. It is linked to the Agency and may be referenced via
// assigned_role edges on WorkItems.
type ConfiguredRole struct {
	// ID is the unique identifier for this configured role.
	ID string

	// Name is the human-readable label for this role (e.g. "domain_expert").
	Name string

	// Description is the role brief — responsibilities, boundaries, and
	// context shown to a human or injected into an AI agent's system prompt.
	Description string

	// ActorType constrains who may fill this role.
	ActorType ActorType

	// Ordinality controls the sort order among ConfiguredRoles on this Agency.
	Ordinality int
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
// publish action. Written once, never modified.
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

	// Status is the lifecycle state of this publication.
	// Valid values: "draft", "active", "archived".
	Status string
}

// Instruction is an ordered rule or constraint attached to a Workflow or
// WorkItem. Uses the multi-parent pattern — the parent is whichever
// belongs_to_* relationship was set at creation. Mutable.
type Instruction struct {
	// ID is the unique identifier for this instruction.
	ID string

	// Content is the rule or constraint text delivered to the actor.
	Content string

	// Ordinality controls the sort order among Instructions on the parent.
	Ordinality int
}

// Deliverable is the specification of an expected output for a WorkItem.
// It is the spec entity — [DeliverableResult] is the corresponding instance.
// Mutable.
type Deliverable struct {
	// ID is the unique identifier for this deliverable.
	ID string

	// Title names the expected output (e.g. "Analysis Report").
	Title string

	// Description defines what the output must contain or satisfy.
	Description string

	// Ordinality controls the sort order among Deliverables on the WorkItem.
	Ordinality int

	// Blocking indicates that a rejected result halts the Workflow from
	// advancing past this WorkItem until a reviewer_role actor waives it.
	Blocking bool
}

// DeliverableResult is an immutable record of a single submission against a
// [Deliverable]. Multiple results may exist per Deliverable (full audit trail).
// ProducedAt is server-stamped at creation — callers do not set it.
//
// Status lifecycle: pending → completed | rejected → waived
type DeliverableResult struct {
	// ID is the unique identifier for this result.
	ID string

	// Status is the current state of this result.
	// Valid values: "pending", "completed", "rejected", "waived".
	Status string

	// ProducedAt is the server-stamped time this result was persisted.
	ProducedAt time.Time
}

// ContentRef is an immutable pointer to a single artifact path in CodeValdGit.
// Uses the multi-parent pattern — attachable to a [DeliverableResult],
// an [Instruction], or a [WorkItem].
type ContentRef struct {
	// ID is the unique identifier for this content reference.
	ID string

	// Path is the relative path within the CodeValdGit repo for this agency
	// (e.g. "output/report.md").
	Path string
}

// UpdateAgencyRequest carries the scalar fields of an existing Agency that
// may be changed via UpdateAgency. Sub-resources (Goals, Workflows,
// ConfiguredRoles) are managed through their own dedicated manager methods
// and are not part of an agency update.
type UpdateAgencyRequest struct {
	Name    string
	Mission string
	Vision  string
	Status  AgencyLifecycle
}
