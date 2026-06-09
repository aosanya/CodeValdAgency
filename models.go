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

// AgencyDraftStatus is the lifecycle state of an [AgencyDraft].
// Transitions are strictly forward-only; see [AgencyDraftStatus.CanTransitionTo].
type AgencyDraftStatus string

const (
	// DraftStatusOpen means the draft is actively being edited.
	DraftStatusOpen AgencyDraftStatus = "open"

	// DraftStatusPromoted means this draft was selected and its entities were
	// copied into the live agency. Terminal — no further transitions.
	DraftStatusPromoted AgencyDraftStatus = "promoted"

	// DraftStatusArchived means the draft was discarded without promotion.
	// Terminal — no further transitions.
	DraftStatusArchived AgencyDraftStatus = "archived"
)

// CanTransitionTo reports whether transitioning from the receiver draft status
// to next is valid.
//
// Allowed transitions:
//
//	open → promoted
//	open → archived
func (s AgencyDraftStatus) CanTransitionTo(next AgencyDraftStatus) bool {
	if s == DraftStatusOpen {
		return next == DraftStatusPromoted || next == DraftStatusArchived
	}
	return false // promoted and archived are terminal
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

	// Enabled reports whether the agency is active. When false the agency is
	// disabled (e.g. suspended). Defaults to false until the first
	// [AgencyManager.PromoteDraft] call succeeds, at which point it is set to true.
	Enabled bool

	// DefaultFailurePipelineBudget is the per-WorkflowRun cap on recovery
	// pipeline activations for runs created under this agency. Zero means
	// "unset" — start-pipeline falls back to the WORKFLOW_RUN_FAILURE_BUDGET
	// env default. Overridden by the work.pipeline.requested payload's
	// failure_pipeline_budget. See FEAT-20260602-007.
	DefaultFailurePipelineBudget int

	// InactivityTimeoutSeconds is the per-agency override for the run-level
	// inactivity timeout. Zero means "unset" — the watchdog falls back to the
	// WORKFLOW_RUN_STALE_TIMEOUT env default. Resolution order:
	// WorkPlan.step_timeout > InactivityTimeoutSeconds > env default.
	// See FEAT-20260602-006.
	InactivityTimeoutSeconds int

	// EventFlows is the raw JSON blob of the event_flows section from agency.json.
	// It holds the cross-service event routing rules for display in the execution flow chart.
	EventFlows string

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

	// Code is the machine-readable role code (e.g. "domain-expert"). Unique key.
	Code string

	// Name is the human-readable label for this role (e.g. "Domain Expert").
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

	// DraftID is the ID of the AgencyDraft entity that was published.
	DraftID string

	// ContentHash is the SHA-256 fingerprint of the draft's sub-entity content
	// at publish time. Two publications with the same ContentHash contain
	// identical logical content. Empty when no draftID was provided.
	ContentHash string

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

// ContextSourceType identifies the service a ContextSource fetches from.
type ContextSourceType string

const (
	// ContextSourceGit fetches file-signal context from CodeValdGit.
	ContextSourceGit ContextSourceType = "GitContextSource"

	// ContextSourceComm fetches conversation-thread context from CodeValdComm.
	ContextSourceComm ContextSourceType = "CommContextSource"

	// ContextSourceWork fetches task-detail context from CodeValdWork.
	ContextSourceWork ContextSourceType = "WorkContextSource"
)

// WorkPlan is an execution plan linked to an Agency via a has_work_plan edge.
// It binds a trigger topic, a ConfiguredRole, and a WorkItem together.
// CodeValdAI calls MatchWorkPlans with the incoming Cross event topic and raw
// JSON payload; the agency returns all enabled work plans whose TriggerTopic
// regex matches and whose PayloadCondition regex (if non-empty) also matches.
type WorkPlan struct {
	// ID is the unique identifier for this work plan.
	ID string

	// AgencyID is the identifier of the owning agency.
	AgencyID string

	// Code is the stable slug used to reference this plan in OnFailurePipeline.
	Code string

	// Name is a human-readable label for the work plan.
	Name string

	// Description is the work plan brief.
	Description string

	// TriggerTopic is a Go regex matched against the incoming Cross event topic.
	TriggerTopic string

	// PayloadCondition is a Go regex matched against the raw JSON payload string.
	// An empty string matches all payloads.
	PayloadCondition string

	// Instructions is the prompt template injected into the triggered AgentRun context.
	Instructions string

	// AgentID is a cross-service reference to a CodeValdAI Agent entity ID.
	AgentID string

	// AgentCode is the symbolic AIAgent code from ai_config (e.g. "deepseek-v4-developer").
	// Resolved to AgentID by CodeValdAI bootstrap at startup.
	AgentCode string

	// FunctionCode identifies the function binary when HandlerService="codevaldfunction".
	FunctionCode string

	// FunctionParams is a JSON-encoded parameter map forwarded to the function binary.
	FunctionParams string

	// Enabled controls whether this work plan is included in MatchWorkPlans results.
	Enabled bool

	// HandlerService is the CodeVald service responsible for executing this plan
	// (e.g. "codevaldai", "codevaldcomm").
	HandlerService string

	// Ordinality controls the ascending sort order when multiple work plans match.
	Ordinality int

	// SuccessEvent is the Cross topic the plan's handler publishes on terminal
	// success (e.g. "functions.job.completed"). A recovery pipeline for this
	// plan must synthesize this event on success — restoring the downstream
	// event sequence. Empty means use the handler-service default. See
	// FEAT-20260602-005.
	SuccessEvent string

	// FailureEvent is the Cross topic the plan's handler publishes on failure.
	// Cross's failure-dispatch loop looks up plans by this field and routes to
	// OnFailurePipeline when set. Empty means use the handler-service default.
	FailureEvent string

	// OnFailurePipeline is the `code` of another work plan in the same agency
	// that recovers from this plan's failure. Empty means the failure is
	// terminal (Cross publishes work.run.failed).
	OnFailurePipeline string

	// StepTimeout is the per-step inactivity duration (e.g. "10m") after which
	// the watchdog publishes work.task.timeout for the current step. Empty means
	// fall back to WORKFLOW_RUN_STEP_STALE_TIMEOUT env var. See FEAT-20260602-006.
	StepTimeout string

	// ReviewStepType declares the kind of reviewer that gates task progression.
	// Valid values: "ai_review", "human_review", "functional_review".
	// Empty means no review step is configured (FEAT-20260605-002).
	ReviewStepType string

	// ReviewTriggerTopic is the Cross topic that fires the review agent
	// (e.g. "work.task.completed"). Non-empty only when ReviewStepType is set.
	ReviewTriggerTopic string

	// ReviewSuccessTopic is the Cross topic the reviewer emits on pass
	// (e.g. "work.review.passed"). The next WorkPlan step references this as
	// its trigger_conditions topic.
	ReviewSuccessTopic string

	// ReviewFailureTopic is the Cross topic the reviewer emits on fail
	// (e.g. "work.review.failed"). Routes to OnFailurePipeline when set.
	ReviewFailureTopic string
}

// GitContextSourceConfig holds the configuration for a GitContextSource entity.
type GitContextSourceConfig struct {
	// Signals is a comma-separated list of signal layers, e.g. "authority,contributor".
	Signals string

	// MaxResults caps the number of files returned (default 20).
	MaxResults int

	// MatchMode is "AND" or "OR" (default "OR").
	MatchMode string

	// Cascade expands keywords to taxonomy descendants when true.
	Cascade bool

	// FileTypes is a comma-separated extension filter, e.g. ".go,.ts".
	FileTypes string
}

// CommContextSourceConfig holds the configuration for a CommContextSource entity.
type CommContextSourceConfig struct {
	// LookbackDays controls how far back to search for threads.
	LookbackDays int

	// MaxResults caps the number of threads returned.
	MaxResults int
}

// WorkContextSourceConfig holds the configuration for a WorkContextSource entity.
type WorkContextSourceConfig struct {
	// IncludeDescription includes the full task description in the context bundle.
	IncludeDescription bool

	// IncludeHistory includes the status transition history in the context bundle.
	IncludeHistory bool
}

// ContextSource is a polymorphic wrapper for a typed context source entity.
// Exactly one of Git, Comm, or Work is non-nil, indicated by SourceType.
type ContextSource struct {
	// ID is the unique identifier for this context source.
	ID string

	// WorkPlanID is the identifier of the owning work plan.
	WorkPlanID string

	// SourceType identifies which service this source fetches from.
	SourceType ContextSourceType

	// Git is non-nil when SourceType == ContextSourceGit.
	Git *GitContextSourceConfig

	// Comm is non-nil when SourceType == ContextSourceComm.
	Comm *CommContextSourceConfig

	// Work is non-nil when SourceType == ContextSourceWork.
	Work *WorkContextSourceConfig
}

// WorkPlanMatch pairs a matched WorkPlan with all its linked ContextSource entities.
// Returned by AgencyManager.MatchWorkPlans, ordered by WorkPlan.Ordinality ascending.
type WorkPlanMatch struct {
	// WorkPlan is the matched execution plan.
	WorkPlan WorkPlan

	// ContextSources are all context source entities linked to the work plan.
	ContextSources []ContextSource
}

// CreateWorkPlanRequest carries the fields for creating a new [WorkPlan].
type CreateWorkPlanRequest struct {
	// Name is a human-readable label for the work plan.
	Name string

	// Description is the work plan brief.
	Description string

	// TriggerTopic is a Go regex matched against the incoming Cross event topic.
	TriggerTopic string

	// PayloadCondition is a Go regex matched against the raw JSON payload.
	// Empty string matches all payloads.
	PayloadCondition string

	// Instructions is the prompt template injected into the triggered AgentRun.
	Instructions string

	// AgentID is a cross-service reference to a CodeValdAI Agent entity ID.
	AgentID string

	// AgentCode is the symbolic AIAgent code from ai_config (e.g. "deepseek-v4-developer").
	AgentCode string

	// FunctionCode identifies the function binary (HandlerService="codevaldfunction").
	FunctionCode string

	// FunctionParams is a JSON-encoded parameter map for the function binary.
	FunctionParams string

	// HandlerService is the CodeVald service responsible for executing this plan.
	HandlerService string

	// Enabled controls whether this work plan participates in MatchWorkPlans dispatch.
	Enabled bool

	// Ordinality controls ascending sort order when multiple work plans match.
	Ordinality int

	// SuccessEvent is the Cross topic published on terminal success. Empty for
	// handler-service default. See FEAT-20260602-005.
	SuccessEvent string

	// FailureEvent is the Cross topic published on failure. Empty for default.
	FailureEvent string

	// OnFailurePipeline is the code of the recovery work plan, empty for none.
	OnFailurePipeline string

	// StepTimeout is the per-step inactivity duration (e.g. "10m"). Empty means
	// fall back to WORKFLOW_RUN_STEP_STALE_TIMEOUT. See FEAT-20260602-006.
	StepTimeout string

	// ReviewStepType declares the reviewer kind for this plan. See WorkPlan.ReviewStepType.
	ReviewStepType string

	// ReviewTriggerTopic is the topic that fires the review agent. See WorkPlan.ReviewTriggerTopic.
	ReviewTriggerTopic string

	// ReviewSuccessTopic is the topic the reviewer emits on pass. See WorkPlan.ReviewSuccessTopic.
	ReviewSuccessTopic string

	// ReviewFailureTopic is the topic the reviewer emits on fail. See WorkPlan.ReviewFailureTopic.
	ReviewFailureTopic string
}

// UpdateWorkPlanRequest carries the fields that may be changed on an existing [WorkPlan].
// Only non-nil pointer fields are written; nil means "leave unchanged".
type UpdateWorkPlanRequest struct {
	Name              *string
	Description       *string
	TriggerTopic      *string
	PayloadCondition  *string
	Instructions      *string
	AgentID           *string
	AgentCode         *string
	FunctionCode      *string
	FunctionParams    *string
	HandlerService    *string
	Enabled           *bool
	Ordinality        *int
	SuccessEvent      *string
	FailureEvent      *string
	OnFailurePipeline *string
	StepTimeout       *string
	ReviewStepType      *string
	ReviewTriggerTopic  *string
	ReviewSuccessTopic  *string
	ReviewFailureTopic  *string
}

// AddContextSourceRequest carries the typed configuration for a new [ContextSource].
// Exactly one of Git, Comm, or Work must be non-nil, matching SourceType.
type AddContextSourceRequest struct {
	// SourceType identifies which service this source fetches from.
	SourceType ContextSourceType

	// Git is non-nil when SourceType == ContextSourceGit.
	Git *GitContextSourceConfig

	// Comm is non-nil when SourceType == ContextSourceComm.
	Comm *CommContextSourceConfig

	// Work is non-nil when SourceType == ContextSourceWork.
	Work *WorkContextSourceConfig
}

// AgencyDraft is a parallel, editable copy of an agency's sub-graph. Drafts
// are forked from the live agency or from another open draft, edited freely,
// then promoted to become the new live state — or archived if discarded.
//
// Sub-entities (Goals, Workflows, WorkItems, ConfiguredRoles, Instructions,
// Deliverables) belonging to this draft are stored in the dedicated
// agency_draft_entities collection and scoped by DraftID.
type AgencyDraft struct {
	// ID is the unique identifier for this draft.
	ID string

	// AgencyID is the identifier of the owning agency.
	AgencyID string

	// Description is the human-readable purpose of this draft
	// (e.g. "Q3 restructure — reduce headcount by 20%").
	Description string

	// ForkedFromID is the ID of the entity this draft was copied from.
	// It is either an Agency ID (ForkedFromType == "live") or an AgencyDraft
	// ID (ForkedFromType == "draft").
	ForkedFromID string

	// ForkedFromType is "live" when forked from the current live agency, or
	// "draft" when forked from another open draft.
	ForkedFromType string

	// Status is the current lifecycle state of this draft.
	Status AgencyDraftStatus

	// CreatedAt is the time at which this draft was first persisted.
	CreatedAt time.Time

	// UpdatedAt is the time at which this draft was most recently modified.
	UpdatedAt time.Time
}
