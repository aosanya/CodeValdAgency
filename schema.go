// Package codevaldagency — pre-delivered schema definition.
//
// This file exposes [DefaultAgencySchema], which returns the fixed
// [types.Schema] for CodeValdAgency. cmd/main.go seeds this schema
// idempotently on startup via AgencySchemaManager.SetSchema.
//
// The schema declares twenty-six TypeDefinitions:
//   - Agency                    — root entity (mutable)
//   - Goal                      — strategic objective (mutable)
//   - Workflow                  — ordered container of WorkItems (mutable)
//   - WorkItem                  — unit of work within a Workflow (mutable)
//   - Instruction               — ordered rule or constraint attached to a Workflow or WorkItem (mutable)
//   - Deliverable               — spec: expected output a WorkItem must produce (mutable)
//   - DeliverableResult         — instance: actual output submitted against a Deliverable spec (immutable)
//   - ContentRef                — path to a single artifact in CodeValdGit; attachable to DeliverableResult, Instruction, or WorkItem (immutable)
//   - ConfiguredRole            — custom role beyond super_admin/admin (mutable)
//   - AgencyDraft               — parallel editable copy of the agency graph, stored in agency_drafts (mutable)
//   - DraftGoal                 — copy of Goal inside a draft, stored in agency_draft_entities (mutable)
//   - DraftWorkflow             — copy of Workflow inside a draft, stored in agency_draft_entities (mutable)
//   - DraftWorkItem             — copy of WorkItem inside a draft, stored in agency_draft_entities (mutable)
//   - DraftConfiguredRole       — copy of ConfiguredRole inside a draft, stored in agency_draft_entities (mutable)
//   - DraftInstruction          — copy of Instruction inside a draft, stored in agency_draft_entities (mutable)
//   - DraftDeliverable          — copy of Deliverable inside a draft, stored in agency_draft_entities (mutable)
//   - DraftDeliverableResult    — copy of DeliverableResult inside a draft, stored in agency_draft_entities (mutable)
//   - AgencySnapshot            — immutable activation record (draft → active)
//   - AgencyPublication         — immutable versioned publication snapshot
//   - AgencyPublicationStatus   — mutable status node for a publication (active → archived)
//   - WorkPlan                  — execution plan binding a trigger topic, handler_service, and optional function_code/function_params or agent_id/instructions (mutable)
//   - GitContextSource          — context source: CodeValdGit file signals (mutable)
//   - CommContextSource         — context source: CodeValdComm thread lookback (mutable)
//   - WorkContextSource         — context source: CodeValdWork task details (mutable)
//   - AIProvider                — LLM provider configuration declared in ai_config (mutable)
//   - AIAgent                   — LLM agent configuration declared in ai_config (mutable)
//
// Relationship graph:
//
//	Agency ──has_goal──────────────► Goal
//	       ──has_workflow──────────► Workflow ──has_work_item──────► WorkItem
//	       ──has_configured_role──► ConfiguredRole                        │
//	       ──has_draft─────────────► AgencyDraft                          ├──has_instruction──► Instruction
//	       ──has_snapshot─────────► AgencySnapshot   (Immutable)          └──has_deliverable──► Deliverable
//	       ──has_publication──────► AgencyPublication (Immutable)                                    │
//	                                    │                                                            ├──has_result──► DeliverableResult ──has_content_ref──► ContentRef
//	                                    ├──has_status──────────────► AgencyPublicationStatus (mutable)
//	                                    └──has_instruction──► Instruction ──has_content_ref──► ContentRef
//	                                    WorkItem ──has_content_ref──► ContentRef
//
//	Deliverable ──reviewer_role──► ConfiguredRole  (waiver authority)
//
//	Goal                  ──belongs_to_agency──────────►   Agency            (ToMany=false, inverse)
//	Goal                  ──belongs_to_draft──────────►   AgencyDraft        (ToMany=false, inverse, optional)
//	Workflow              ──belongs_to_agency──────────►   Agency            (ToMany=false, inverse)
//	Workflow              ──belongs_to_draft──────────►   AgencyDraft        (ToMany=false, inverse, optional)
//	WorkItem              ──belongs_to_workflow──────────► Workflow          (ToMany=false, inverse)
//	Instruction           ──belongs_to_workflow──────────► Workflow          (ToMany=false, inverse, optional)
//	Instruction           ──belongs_to_work_item──────────► WorkItem         (ToMany=false, inverse, optional)
//	Deliverable           ──belongs_to_work_item──────────► WorkItem         (ToMany=false, inverse)
//	DeliverableResult     ──belongs_to_deliverable──────►   Deliverable      (ToMany=false, inverse)
//	ConfiguredRole        ──belongs_to_agency──────────►   Agency            (ToMany=false, inverse)
//	ConfiguredRole        ──belongs_to_draft──────────►   AgencyDraft        (ToMany=false, inverse, optional)
//	AgencyDraft           ──belongs_to_agency──────────►   Agency            (ToMany=false, inverse)
//	AgencySnapshot        ──belongs_to_agency──────────►   Agency            (ToMany=false, inverse)
//	AgencyPublication     ──belongs_to_agency──────────►   Agency            (ToMany=false, inverse)
//	AgencyPublicationStatus ──belongs_to_publication──► AgencyPublication   (ToMany=false, inverse)
//
// Relationship graph:
//
//	Agency ──has_goal──────────────► Goal
//	       ──has_workflow──────────► Workflow ──has_work_item──────► WorkItem
//	       ──has_configured_role──► ConfiguredRole                        │
//	       ──has_snapshot─────────► AgencySnapshot   (Immutable)          ├──has_instruction──► Instruction
//	       ──has_publication──────► AgencyPublication (Immutable)         └──has_deliverable──► Deliverable
//	                                    │                                                            │
//	                                    ├──has_status──────────────► AgencyPublicationStatus (mutable)
//	                                    └──has_instruction──► Instruction ──has_content_ref──► ContentRef
//	                                                              has_result ──┤
//	                                                                          ▼
//	                                                              DeliverableResult ──has_content_ref──► ContentRef
//	                                    WorkItem ──has_content_ref──► ContentRef
//
//	Deliverable ──reviewer_role──► ConfiguredRole  (waiver authority)
//
//	Goal                  ──belongs_to_agency──────────►   Agency            (ToMany=false, inverse)
//	Workflow              ──belongs_to_agency──────────►   Agency            (ToMany=false, inverse)
//	WorkItem              ──belongs_to_workflow──────────► Workflow          (ToMany=false, inverse)
//	Instruction           ──belongs_to_workflow──────────► Workflow          (ToMany=false, inverse, optional)
//	Instruction           ──belongs_to_work_item──────────► WorkItem         (ToMany=false, inverse, optional)
//	Deliverable           ──belongs_to_work_item──────────► WorkItem         (ToMany=false, inverse)
//	DeliverableResult     ──belongs_to_deliverable──────►   Deliverable      (ToMany=false, inverse)
//	ConfiguredRole        ──belongs_to_agency──────────►   Agency            (ToMany=false, inverse)
//	AgencySnapshot        ──belongs_to_agency──────────►   Agency            (ToMany=false, inverse)
//	AgencyPublication     ──belongs_to_agency──────────►   Agency            (ToMany=false, inverse)
//	AgencyPublicationStatus ──belongs_to_publication──► AgencyPublication   (ToMany=false, inverse)
package codevaldagency

import "github.com/aosanya/CodeValdSharedLib/types"

// DefaultAgencySchema returns the pre-delivered [types.Schema] for
// CodeValdAgency. It is called once by cmd/main.go on startup and seeded
// idempotently via AgencySchemaManager.SetSchema.
//
// AgencySnapshot and AgencyPublication are marked Immutable — UpdateEntity
// returns ErrImmutableType for those type IDs. Their StorageCollection fields
// route writes to dedicated ArangoDB collections instead of the default
// agency_entities collection.
//
// All RelationshipDefinitions with a non-empty Inverse field are validated by
// entitygraph.ValidateSchema — each named inverse must exist on the target
// TypeDefinition.
func DefaultAgencySchema() types.Schema {
	return types.Schema{
		ID:      "agency-schema-v1",
		Version: 1,
		Tag:     "v1",
		Types: []types.TypeDefinition{
			{
				Name:              "Agency",
				DisplayName:       "Agency",
				PathSegment:       "",
				StorageCollection: "agency_details",
				Properties: []types.PropertyDefinition{{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true}, {Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "name", Type: types.PropertyTypeString, Required: true},
					{Name: "mission", Type: types.PropertyTypeString, Required: false},
					{Name: "vision", Type: types.PropertyTypeString, Required: false},
					// enabled is false until the first PromoteDraft call succeeds,
					// at which point it is set to true. Set to false to disable the agency.
					{Name: "enabled", Type: types.PropertyTypeBoolean, Required: false},
					// default_failure_pipeline_budget is the per-WorkflowRun cap on
					// recovery-pipeline activations for runs created under this agency.
					// Overridden by the work.pipeline.requested payload; falls back to
					// the WORKFLOW_RUN_FAILURE_BUDGET env default when zero/unset.
					// See FEAT-20260602-007.
					{Name: "default_failure_pipeline_budget", Type: types.PropertyTypeInteger, Required: false},
					// inactivity_timeout_seconds is the per-agency override for the run-level
					// inactivity timeout. When set, overrides the global WORKFLOW_RUN_STALE_TIMEOUT
					// env var for all WorkflowRuns created under this agency.
					// Resolution order: step_timeout > inactivity_timeout_seconds > env default.
					// See FEAT-20260602-006.
					{Name: "inactivity_timeout_seconds", Type: types.PropertyTypeInteger, Required: false},
					// event_flows stores a JSON-encoded object { flows: []EventFlow }.
					// Each EventFlow entry is one of two types, distinguished by the "type" field:
					//
					// START NODE  (type: "start")
					//   type         string — "start": marks a workflow entry point.
					//   step         string — hierarchical step number, e.g. "1", "2". Groups start
					//                         nodes and their descendant steps under one chain.
					//   name         string — short display name for this entry point (optional).
					//   emits_topic  string — the topic this entry point produces. Nothing triggers
					//                         a start node — it is the origin of the flow chain.
					//   description  string — what external event or condition causes this topic to fire.
					//
					// STEP NODE  (type omitted or "step")
					//   step               string             — hierarchical step number, e.g. "1.1", "1.2".
					//                                           Sorts steps within their start-node chain.
					//   trigger            string             — PubSub topic that causes this step.
					//   trigger_publisher  string             — service that publishes the trigger.
					//   consumer           string             — service that handles the trigger. The consumer
					//                                           is always the publisher of any emits_topic in
					//                                           its branches — no separate emits_publisher field.
					//   description        string             — human-readable explanation of the step.
					//   name               string             — short display name (optional).
					//   branches           []EventFlowBranch  — always present; one entry minimum. Each entry
					//                                           documents one possible downstream outcome.
					//                                           Single-outcome steps have one branch with
					//                                           condition: "".
					//
					// EventFlowBranch sub-object:
					//   condition          string — when this branch fires. Empty string for single-outcome
					//                               steps. Documentation only — not machine-evaluated.
					//   emits_topic        string — downstream topic produced by this branch. Empty string
					//                               when the step is terminal (no further event emitted).
					//   description        string — plain-language explanation of the outcome.
					//   handler            string — work_plan.code that executes this branch. Omit for
					//                               pure service-internal reactions with no work plan.
					{Name: "event_flows", Type: types.PropertyTypeString, Required: false},
				},
				Relationships: []types.RelationshipDefinition{
					{Name: "has_goal", Label: "Goals", PathSegment: "goals", ToType: "Goal", ToMany: true, Inverse: "belongs_to_agency"},
					{Name: "has_workflow", Label: "Workflows", PathSegment: "workflows", ToType: "Workflow", ToMany: true, Inverse: "belongs_to_agency"},
					{Name: "has_configured_role", Label: "Configured Roles", PathSegment: "configured-roles", ToType: "ConfiguredRole", ToMany: true, Inverse: "belongs_to_agency"},
					// ToMany=true: an agency may have many open, promoted, or archived drafts.
					// Inverse of AgencyDraft.belongs_to_agency.
					{Name: "has_draft", Label: "Drafts", PathSegment: "drafts", ToType: "AgencyDraft", ToMany: true, Inverse: "belongs_to_agency"},
					{Name: "has_snapshot", Label: "Snapshots", PathSegment: "snapshots", ToType: "AgencySnapshot", ToMany: true, Inverse: "belongs_to_agency"},
					{Name: "has_publication", Label: "Publications", PathSegment: "publications", ToType: "AgencyPublication", ToMany: true, Inverse: "belongs_to_agency"},
					{Name: "has_work_plan", Label: "Work Plans", PathSegment: "work-plans", ToType: "WorkPlan", ToMany: true, Inverse: "belongs_to_agency"},
					{Name: "has_ai_provider", Label: "AI Providers", PathSegment: "ai-providers", ToType: "AIProvider", ToMany: true, Inverse: "belongs_to_agency"},
					{Name: "has_ai_agent", Label: "AI Agents", PathSegment: "ai-agents", ToType: "AIAgent", ToMany: true, Inverse: "belongs_to_agency"},
				},
			},
			{
				Name:              "Goal",
				DisplayName:       "Goal",
				StorageCollection: "agency_goals",
				// PathSegment is intentionally empty — live Goals are created by PromoteDraft,
				// not via direct HTTP CRUD. CRUD routes exist only for DraftGoal, scoped under
				// /agency/{agencyId}/drafts/{draftId}/goals.
				UniqueKey: []string{"code"},
				Properties: []types.PropertyDefinition{{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true}, {Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "title", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
				},
				Relationships: []types.RelationshipDefinition{
					// ToMany=false, Required=true: a Goal must belong to exactly one Agency.
					// Inverse of Agency.has_goal — auto-created by CreateRelationship.
					{Name: "belongs_to_agency", Label: "Agency", PathSegment: "agency", ToType: "Agency", ToMany: false, Required: true},
					// ToMany=false, optional: set when this Goal is a draft copy.
					// Inverse of AgencyDraft.has_goal — written atomically by CreateDraft.
					{Name: "belongs_to_draft", Label: "Draft", PathSegment: "draft", ToType: "AgencyDraft", ToMany: false},
					// ToMany=true: work items that contribute to this goal.
					// Inverse of WorkItem.belongs_to_goal.
					{Name: "has_work_item", Label: "Work Items", PathSegment: "work-items", ToType: "WorkItem", ToMany: true, Inverse: "belongs_to_goal"},
				},
			},
			{
				Name:              "Workflow",
				DisplayName:       "Workflow",
				StorageCollection: "agency_workflows",
				// PathSegment is intentionally empty — live Workflows are created by PromoteDraft,
				// not via direct HTTP CRUD. CRUD routes exist only for DraftWorkflow, scoped under
				// /agency/{agencyId}/drafts/{draftId}/workflows.
				UniqueKey: []string{"code"},
				Properties: []types.PropertyDefinition{{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true}, {Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "name", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
					// event_flows holds the JSON-encoded { name, steps: [...] } block scoped
					// to THIS workflow only. Imported from flows_<workflow.code>.json files
					// via the per-workflow-file convention (see project_agency_flows_folder_structure
					// memory). Same node shape as Agency.event_flows above (start/step nodes
					// with branches). Optional: workflows authored before FEAT-20260609-002
					// have no per-workflow flows; readers fall back to the agency-level blob.
					{Name: "event_flows", Type: types.PropertyTypeString, Required: false},
				},
				Relationships: []types.RelationshipDefinition{
					{Name: "has_work_item", Label: "Work Items", PathSegment: "work-items", ToType: "WorkItem", ToMany: true, Inverse: "belongs_to_workflow"},
					{Name: "has_instruction", Label: "Instructions", PathSegment: "instructions", ToType: "Instruction", ToMany: true, Inverse: "belongs_to_workflow"},
					// ToMany=false, Required=true: a Workflow must belong to exactly one Agency.
					// Inverse of Agency.has_workflow — auto-created by CreateRelationship.
					{Name: "belongs_to_agency", Label: "Agency", PathSegment: "agency", ToType: "Agency", ToMany: false, Required: true},
					// ToMany=false, optional: set when this Workflow is a draft copy.
					// Inverse of AgencyDraft.has_workflow — written atomically by CreateDraft.
					{Name: "belongs_to_draft", Label: "Draft", PathSegment: "draft", ToType: "AgencyDraft", ToMany: false},
				},
			},
			{
				// EventFlowStep is the structured projection of a single step inside a
				// workflow's event_flows block. One EventFlowStep per `steps[*]` entry
				// in the per-workflow flow file (e.g. flows_planning.json step 1.1.1.2.1).
				//
				// Created by ImportDraft (as DraftEventFlowStep, then promoted) so
				// downstream services can query the active publication's flow without
				// re-parsing the opaque JSON blob on Workflow.event_flows. The blob is
				// preserved on Workflow for round-trip / debugging; this entity is the
				// queryable form. See BUG-20260610-002.
				Name:              "EventFlowStep",
				DisplayName:       "Event Flow Step",
				StorageCollection: "agency_event_flow_steps",
				// PathSegment empty — live steps are created by PromoteDraft, not via
				// direct HTTP CRUD. CRUD routes exist only for DraftEventFlowStep.
				UniqueKey: []string{"code"},
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					// code is a deterministic identifier "<workflow_code>:<step_number>"
					// e.g. "planning:1.1.1.2.1". Required for UniqueKey enforcement so
					// re-imports update existing steps instead of duplicating.
					{Name: "code", Type: types.PropertyTypeString, Required: true},
					// workflow_code denormalises the parent workflow's code for fast
					// filtering. The belongs_to_workflow edge is the authoritative link.
					{Name: "workflow_code", Type: types.PropertyTypeString, Required: true},
					// step is the dotted step number from the source flow file
					// (e.g. "1.1.1.2.1"). Human-readable, hierarchical.
					{Name: "step", Type: types.PropertyTypeString, Required: true},
					{Name: "name", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					// step_type is "start" | "function-call" | "handler" per the flow
					// schema (see CodeValdImplementations/Agencies/FLOWS_FORMAT.md).
					{Name: "step_type", Type: types.PropertyTypeString, Required: false},
					// trigger_topic is the Cross topic that fires this step. Empty for
					// "start" steps (which are entry points, not triggered).
					{Name: "trigger_topic", Type: types.PropertyTypeString, Required: false},
					{Name: "trigger_publisher", Type: types.PropertyTypeString, Required: false},
					// consumer is the CodeVald service that runs the step's handler
					// (e.g. "codevaldai", "codevaldwork", "codevaldfunctions").
					{Name: "consumer", Type: types.PropertyTypeString, Required: false},
					// handler_code is the WorkPlan.code that executes the step. This is
					// the lookup key Work uses to ask "what does the active publication
					// declare for the step that produced this AgentRun?"
					{Name: "handler_code", Type: types.PropertyTypeString, Required: false},
					// emits_topics is a comma-separated list of Cross topics this step
					// publishes on success.
					{Name: "emits_topics", Type: types.PropertyTypeString, Required: false},
					// on_error_emits_topics is a comma-separated list of Cross topics
					// this step publishes on error.
					{Name: "on_error_emits_topics", Type: types.PropertyTypeString, Required: false},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: false},
				},
				Relationships: []types.RelationshipDefinition{
					// ToMany=false, Required=true: a step must belong to exactly one Workflow.
					{Name: "belongs_to_workflow", Label: "Workflow", PathSegment: "workflow", ToType: "Workflow", ToMany: false, Required: true},
					// ToMany=false, optional: set when this step is a draft copy.
					{Name: "belongs_to_draft", Label: "Draft", PathSegment: "draft", ToType: "AgencyDraft", ToMany: false},
				},
			},
			{
				Name:              "WorkItem",
				DisplayName:       "Work Item",
				StorageCollection: "agency_work_items",
				// PathSegment is intentionally empty — live WorkItems are created by PromoteDraft,
				// not via direct HTTP CRUD. CRUD routes exist only for DraftWorkItem, scoped under
				// /agency/{agencyId}/drafts/{draftId}/work-items.
				UniqueKey: []string{"code"},
				Properties: []types.PropertyDefinition{{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true}, {Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "title", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					// WorkItems with the same ordinality value run in parallel.
					// WorkItems with a higher ordinality run after all items at the lower value complete.
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
					// prompt is the task-specific input sent to the actor at dispatch time.
					// For ai_agent: the LLM prompt. For compute_agent: the function input payload.
					// For human: the task brief shown in the UI.
					{Name: "prompt", Type: types.PropertyTypeString, Required: false},
				},
				Relationships: []types.RelationshipDefinition{
					// ToMany=false, Required=true: a WorkItem must belong to exactly one Workflow.
					// Inverse of Workflow.has_work_item — auto-created by CreateRelationship.
					{Name: "belongs_to_workflow", Label: "Workflow", PathSegment: "workflow", ToType: "Workflow", ToMany: false, Required: true},
					// ToMany=false: inverse of Goal.has_work_item (optional — set when linked to a Goal).
					{Name: "belongs_to_goal", Label: "Goal", PathSegment: "goal", ToType: "Goal", ToMany: false},
					// ToMany=true: standing rules attached to this work item.
					{Name: "has_instruction", Label: "Instructions", PathSegment: "instructions", ToType: "Instruction", ToMany: true, Inverse: "belongs_to_work_item"},
					// ToMany=true: expected outputs this work item must produce.
					{Name: "has_deliverable", Label: "Deliverables", PathSegment: "deliverables", ToType: "Deliverable", ToMany: true, Inverse: "belongs_to_work_item"},
					// ToMany=true: reference artifacts attached to this work item (e.g. input data, templates).
					{Name: "has_content_ref", Label: "Content Refs", PathSegment: "content-refs", ToType: "ContentRef", ToMany: true, Inverse: "belongs_to_work_item"},
				},
			},
			{
				Name:              "Instruction",
				DisplayName:       "Instruction",
				StorageCollection: "agency_instructions",
				// PathSegment is intentionally empty — live Instructions are created by PromoteDraft,
				// not via direct HTTP CRUD. CRUD routes exist only for DraftInstruction, scoped under
				// /agency/{agencyId}/drafts/{draftId}/instructions.
				UniqueKey: []string{"code"},
				Properties: []types.PropertyDefinition{{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true}, {Name: "code", Type: types.PropertyTypeString, Required: false},
					// content is the rule or constraint text delivered to the actor.
					{Name: "content", Type: types.PropertyTypeString, Required: true},
					// ordinality controls the order in which instructions are applied.
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
				},
				Relationships: []types.RelationshipDefinition{
					// ToMany=false: inverse of Workflow.has_instruction (optional — set when attached to a Workflow).
					{Name: "belongs_to_workflow", Label: "Workflow", PathSegment: "workflow", ToType: "Workflow", ToMany: false},
					// ToMany=false: inverse of WorkItem.has_instruction (optional — set when attached to a WorkItem).
					{Name: "belongs_to_work_item", Label: "Work Item", PathSegment: "work-item", ToType: "WorkItem", ToMany: false},
					// ToMany=true: reference artifacts attached to this instruction.
					{Name: "has_content_ref", Label: "Content Refs", PathSegment: "content-refs", ToType: "ContentRef", ToMany: true, Inverse: "belongs_to_instruction"},
				},
			},
			{
				Name:              "Deliverable",
				DisplayName:       "Deliverable",
				StorageCollection: "agency_deliverables",
				// PathSegment is intentionally empty — live Deliverables are created by PromoteDraft,
				// not via direct HTTP CRUD. CRUD routes exist only for DraftDeliverable, scoped under
				// /agency/{agencyId}/drafts/{draftId}/deliverables.
				UniqueKey: []string{"code"},
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					// title names the expected output (e.g. "Analysis Report", "Migration Script").
					{Name: "title", Type: types.PropertyTypeString, Required: true},
					// description defines what the output must contain or satisfy.
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					// ordinality controls the order deliverables are listed and evaluated.
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
					// blocking=true halts the Workflow from advancing past this WorkItem when the
					// corresponding DeliverableResult is rejected. A reviewer_role actor must
					// set the result status to "waived" to unblock.
					{Name: "blocking", Type: types.PropertyTypeBoolean, Required: true},
				},
				Relationships: []types.RelationshipDefinition{
					// ToMany=false, Required=true: a Deliverable must belong to exactly one WorkItem.
					// Inverse of WorkItem.has_deliverable — written atomically by CreateRelationship.
					{Name: "belongs_to_work_item", Label: "Work Item", PathSegment: "work-item", ToType: "WorkItem", ToMany: false, Required: true},
					// ToMany=true: runtime results submitted against this spec.
					{Name: "has_result", Label: "Results", PathSegment: "results", ToType: "DeliverableResult", ToMany: true, Inverse: "belongs_to_deliverable"},
					// ToMany=false: the ConfiguredRole whose actor can waive a rejected result.
					// Optional — if unset, no waiver path exists; only re-attempt can produce a new result.
					{Name: "reviewer_role", Label: "Reviewer Role", PathSegment: "reviewer-role", ToType: "ConfiguredRole", ToMany: false, Inverse: "reviews_deliverable"},
				},
			},
			{
				Name:              "DeliverableResult",
				DisplayName:       "Deliverable Result",
				PathSegment:       "results",
				EntityIDParam:     "resultId",
				Immutable:         true,
				StorageCollection: "agency_deliverable_results",
				Properties: []types.PropertyDefinition{{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true}, {Name: "code", Type: types.PropertyTypeString, Required: false},
					// status valid values: "pending", "completed", "rejected", "waived"
					//   pending   — submitted by the actor; awaiting review
					//   completed — accepted by the reviewer (or auto-accepted when blocking=false)
					//   rejected  — reviewer rejected; workflow blocked if Deliverable.blocking=true
					//   waived    — reviewer waived the rejection; workflow unblocked
					{Name: "status", Type: types.PropertyTypeOption, Required: true},
					// produced_at is when the actor submitted the result.
					{Name: "produced_at", Type: types.PropertyTypeDatetime, Required: false},
				},
				Relationships: []types.RelationshipDefinition{
					// ToMany=false, Required=true: a result must belong to exactly one Deliverable spec.
					// Inverse of Deliverable.has_result — written atomically by CreateRelationship.
					{Name: "belongs_to_deliverable", Label: "Deliverable", PathSegment: "deliverable", ToType: "Deliverable", ToMany: false, Required: true},
					// ToMany=true: one ContentRef per artifact path committed to CodeValdGit.
					{Name: "has_content_ref", Label: "Content Refs", PathSegment: "content-refs", ToType: "ContentRef", ToMany: true, Inverse: "belongs_to_result"},
				},
			},
			{
				Name:              "ContentRef",
				DisplayName:       "Content Ref",
				PathSegment:       "content-refs",
				EntityIDParam:     "contentRefId",
				Immutable:         true,
				StorageCollection: "agency_content_refs",
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					// path is the location of the artifact in CodeValdGit (e.g. "output/report.md").
					{Name: "path", Type: types.PropertyTypeString, Required: true},
				},
				Relationships: []types.RelationshipDefinition{
					// The three belongs_to_* relationships follow the Instruction multi-parent pattern:
					// none is Required — a ContentRef is attached to whichever parent is relevant.
					// Inverse of DeliverableResult.has_content_ref (optional).
					{Name: "belongs_to_result", Label: "Result", PathSegment: "result", ToType: "DeliverableResult", ToMany: false},
					// Inverse of Instruction.has_content_ref (optional).
					{Name: "belongs_to_instruction", Label: "Instruction", PathSegment: "instruction", ToType: "Instruction", ToMany: false},
					// Inverse of WorkItem.has_content_ref (optional).
					{Name: "belongs_to_work_item", Label: "Work Item", PathSegment: "work-item", ToType: "WorkItem", ToMany: false},
				},
			},
			{
				Name:              "ConfiguredRole",
				DisplayName:       "Configured Role",
				StorageCollection: "agency_configured_roles",
				// PathSegment is intentionally empty — live ConfiguredRoles are created by PromoteDraft,
				// not via direct HTTP CRUD. CRUD routes exist only for DraftConfiguredRole, scoped under
				// /agency/{agencyId}/drafts/{draftId}/configured-roles.
				UniqueKey: []string{"code"},
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "name", Type: types.PropertyTypeString, Required: true},
					// description is the role brief — responsibilities, boundaries, and context
					// shown to a human or injected into an AI agent's system prompt at dispatch.
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					// actor_type valid values: "human", "ai_agent", "compute_agent"
					//   human         — person; performs judgment, approval, manual action
					//   ai_agent      — LLM-backed agent; non-deterministic, prompt-driven
					//   compute_agent — pure function; deterministic, retryable, no LLM needed
					{Name: "actor_type", Type: types.PropertyTypeOption, Required: true},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
				},
				Relationships: []types.RelationshipDefinition{
					// ToMany=false, Required=true: a ConfiguredRole must belong to exactly one Agency.
					// Inverse of Agency.has_configured_role — auto-created by CreateRelationship.
					{Name: "belongs_to_agency", Label: "Agency", PathSegment: "agency", ToType: "Agency", ToMany: false, Required: true},
					// ToMany=true: inverse of WorkPlan.assigned_role — work plans assigned to this role.
					{Name: "assigned_work_plan", Label: "Work Plans", PathSegment: "work-plans", ToType: "WorkPlan", ToMany: true, Inverse: "assigned_role"},
					// ToMany=true: inverse of Deliverable.reviewer_role — deliverables this role can waive.
					{Name: "reviews_deliverable", Label: "Reviewed Deliverables", PathSegment: "reviewed-deliverables", ToType: "Deliverable", ToMany: true},
					// ToMany=false, optional: set when this ConfiguredRole is a draft copy.
					// Inverse of AgencyDraft.has_configured_role — written atomically by CreateDraft.
					{Name: "belongs_to_draft", Label: "Draft", PathSegment: "draft", ToType: "AgencyDraft", ToMany: false},
				},
			},
			// AgencyDraft is a parallel, editable copy of the agency sub-graph. It is
			// stored in the dedicated agency_drafts collection (not agency_entities).
			// Sub-entities copied into a draft are stored in agency_draft_entities
			// and queried by TypeID ("DraftGoal", etc.) + draft_ref_code property.
			{
				Name:              "AgencyDraft",
				DisplayName:       "Agency Draft",
				PathSegment:       "drafts",
				EntityIDParam:     "draftId",
				StorageCollection: "agency_drafts",
				UniqueKey:         []string{"code"},
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					// status valid values: "open", "promoted", "archived"
					//   open     — actively being edited
					//   promoted — draft entities copied to live; terminal
					//   archived — discarded without promotion; terminal
					{Name: "status", Type: types.PropertyTypeOption, Required: true},
					// forked_from_ref_code is the ref_code of the Agency (forked_from_type=="live") or
					// AgencyDraft (forked_from_type=="draft") this was copied from.
					{Name: "forked_from_ref_code", Type: types.PropertyTypeString, Required: false},
					// forked_from_type is either "live" or "draft".
					{Name: "forked_from_type", Type: types.PropertyTypeString, Required: false},
				},
				Relationships: []types.RelationshipDefinition{
					// ToMany=false, Required=true: a Draft must belong to exactly one Agency.
					// Inverse of Agency.has_draft — auto-created by CreateRelationship.
					{Name: "belongs_to_agency", Label: "Agency", PathSegment: "agency", ToType: "Agency", ToMany: false, Required: true},
				},
			},
			// Draft sub-entity types — stored in agency_draft_entities.
			// These are editable copies of the live sub-entity types created by
			// CreateDraft. They carry a draft_ref_code property for scoping. Each type
			// has a PathSegment so the registrar can generate CRUD routes scoped
			// under /agency/{agencyId}/drafts/{draftId}/...
			{
				Name:              "DraftGoal",
				DisplayName:       "Draft Goal",
				PathSegment:       "drafts/{draftRefCode}/goals",
				EntityIDParam:     "goalId",
				StorageCollection: "agency_draft_entities",
				UniqueKey:         []string{"draft_ref_code", "code"},
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "draft_ref_code", Type: types.PropertyTypeString, Required: true},
					{Name: "title", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
				},
			},
			{
				Name:              "DraftWorkflow",
				DisplayName:       "Draft Workflow",
				PathSegment:       "drafts/{draftRefCode}/workflows",
				EntityIDParam:     "workflowId",
				StorageCollection: "agency_draft_entities",
				UniqueKey:         []string{"draft_ref_code", "code"},
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "draft_ref_code", Type: types.PropertyTypeString, Required: true},
					{Name: "name", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
					// event_flows mirrors the live Workflow property; carried through draft
					// promotion so per-workflow flow blocks survive draft→promote.
					{Name: "event_flows", Type: types.PropertyTypeString, Required: false},
				},
			},
			{
				// DraftEventFlowStep — draft-scoped projection of a flow step. Created
				// by ImportDraft from each per-workflow flows_<code>.json steps[*],
				// promoted to EventFlowStep by PromoteDraft. See BUG-20260610-002.
				Name:              "DraftEventFlowStep",
				DisplayName:       "Draft Event Flow Step",
				PathSegment:       "drafts/{draftRefCode}/event-flow-steps",
				EntityIDParam:     "stepId",
				StorageCollection: "agency_draft_entities",
				UniqueKey:         []string{"draft_ref_code", "code"},
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: true},
					{Name: "draft_ref_code", Type: types.PropertyTypeString, Required: true},
					// draft_workflow_ref_code links to the parent DraftWorkflow.
					{Name: "draft_workflow_ref_code", Type: types.PropertyTypeString, Required: true},
					{Name: "workflow_code", Type: types.PropertyTypeString, Required: true},
					{Name: "step", Type: types.PropertyTypeString, Required: true},
					{Name: "name", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					{Name: "step_type", Type: types.PropertyTypeString, Required: false},
					{Name: "trigger_topic", Type: types.PropertyTypeString, Required: false},
					{Name: "trigger_publisher", Type: types.PropertyTypeString, Required: false},
					{Name: "consumer", Type: types.PropertyTypeString, Required: false},
					{Name: "handler_code", Type: types.PropertyTypeString, Required: false},
					{Name: "emits_topics", Type: types.PropertyTypeString, Required: false},
					{Name: "on_error_emits_topics", Type: types.PropertyTypeString, Required: false},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: false},
				},
			},
			{
				Name:              "DraftWorkItem",
				DisplayName:       "Draft Work Item",
				PathSegment:       "drafts/{draftRefCode}/work-items",
				EntityIDParam:     "workItemId",
				StorageCollection: "agency_draft_entities",
				UniqueKey:         []string{"draft_ref_code", "code"},
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "draft_ref_code", Type: types.PropertyTypeString, Required: true},
					// draft_workflow_ref_code references the parent DraftWorkflow entity.
					{Name: "draft_workflow_ref_code", Type: types.PropertyTypeString, Required: false},
					{Name: "title", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
					{Name: "prompt", Type: types.PropertyTypeString, Required: false},
				},
			},
			{
				Name:              "DraftConfiguredRole",
				DisplayName:       "Draft Configured Role",
				PathSegment:       "drafts/{draftRefCode}/configured-roles",
				EntityIDParam:     "configuredRoleId",
				StorageCollection: "agency_draft_entities",
				UniqueKey:         []string{"draft_ref_code", "code"},
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "draft_ref_code", Type: types.PropertyTypeString, Required: true},
					{Name: "name", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					{Name: "actor_type", Type: types.PropertyTypeOption, Required: true},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
				},
			},
			{
				Name:              "DraftInstruction",
				DisplayName:       "Draft Instruction",
				PathSegment:       "drafts/{draftRefCode}/instructions",
				EntityIDParam:     "instructionId",
				StorageCollection: "agency_draft_entities",
				UniqueKey:         []string{"draft_ref_code", "code"},
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "draft_ref_code", Type: types.PropertyTypeString, Required: true},
					// draft_workflow_ref_code or draft_work_item_ref_code — whichever parent applies.
					{Name: "draft_workflow_ref_code", Type: types.PropertyTypeString, Required: false},
					{Name: "draft_work_item_ref_code", Type: types.PropertyTypeString, Required: false},
					{Name: "content", Type: types.PropertyTypeString, Required: true},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
				},
			},
			{
				Name:              "DraftDeliverable",
				DisplayName:       "Draft Deliverable",
				PathSegment:       "drafts/{draftRefCode}/deliverables",
				EntityIDParam:     "deliverableId",
				StorageCollection: "agency_draft_entities",
				UniqueKey:         []string{"draft_ref_code", "code"},
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "draft_ref_code", Type: types.PropertyTypeString, Required: true},
					// draft_work_item_ref_code references the parent DraftWorkItem entity.
					{Name: "draft_work_item_ref_code", Type: types.PropertyTypeString, Required: false},
					{Name: "title", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
					{Name: "blocking", Type: types.PropertyTypeBoolean, Required: true},
					// reviewer_role_code stores the ConfiguredRole code (e.g. "domain-expert")
					// from agency.json. Resolved to a reviewer_role edge on the live Deliverable
					// during PromoteDraft. Optional.
					{Name: "reviewer_role_code", Type: types.PropertyTypeString, Required: false},
				},
			},
			{
				Name:              "DraftDeliverableResult",
				DisplayName:       "Draft Deliverable Result",
				PathSegment:       "drafts/{draftRefCode}/results",
				EntityIDParam:     "resultId",
				StorageCollection: "agency_draft_entities",
				UniqueKey:         []string{"draft_ref_code", "code"},
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "draft_ref_code", Type: types.PropertyTypeString, Required: true},
					// draft_deliverable_ref_code references the parent DraftDeliverable entity.
					{Name: "draft_deliverable_ref_code", Type: types.PropertyTypeString, Required: false},
					// status valid values: "pending", "completed", "rejected", "waived"
					{Name: "status", Type: types.PropertyTypeOption, Required: true},
					{Name: "produced_at", Type: types.PropertyTypeDatetime, Required: false},
				},
			},
			{
				Name:              "AgencySnapshot",
				DisplayName:       "Agency Snapshot",
				PathSegment:       "snapshots",
				EntityIDParam:     "snapshotId",
				Immutable:         true,
				StorageCollection: "agency_snapshots",
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "snapshot_at", Type: types.PropertyTypeDatetime, Required: true},
					// draft_ref_code references the AgencyDraft entity that was promoted to create this snapshot.
					{Name: "draft_ref_code", Type: types.PropertyTypeString, Required: true},
				},
				Relationships: []types.RelationshipDefinition{
					// ToMany=false, Required=true: a Snapshot must belong to exactly one Agency.
					// Inverse of Agency.has_snapshot — auto-created by CreateRelationship.
					{Name: "belongs_to_agency", Label: "Agency", PathSegment: "agency", ToType: "Agency", ToMany: false, Required: true},
				},
			},
			{
				Name:              "AgencyPublication",
				DisplayName:       "Agency Publication",
				PathSegment:       "publications",
				EntityIDParam:     "publicationId",
				Immutable:         true,
				StorageCollection: "agency_publications",
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "version", Type: types.PropertyTypeInteger, Required: true},
					{Name: "tag", Type: types.PropertyTypeString, Required: true},
					{Name: "published_at", Type: types.PropertyTypeDatetime, Required: true},
					// draft_ref_code references the AgencyDraft entity that was published.
					{Name: "draft_ref_code", Type: types.PropertyTypeString, Required: true},
					// content_hash is the SHA-256 fingerprint of the draft sub-entity
					// content at publish time. Used to guard against re-publishing
					// identical content.
					{Name: "content_hash", Type: types.PropertyTypeString, Required: false},
				},
				Relationships: []types.RelationshipDefinition{
					// ToMany=false, Required=true: a Publication must belong to exactly one Agency.
					// Inverse of Agency.has_publication — auto-created by CreateRelationship.
					{Name: "belongs_to_agency", Label: "Agency", PathSegment: "agency", ToType: "Agency", ToMany: false, Required: true},
					// ToMany=false: at-most-one mutable status node per publication.
					// Inverse of AgencyPublicationStatus.belongs_to_publication.
					{Name: "has_status", Label: "Status", PathSegment: "status", ToType: "AgencyPublicationStatus", ToMany: false, Inverse: "belongs_to_publication"},
				},
			},
			// AgencyPublicationStatus is the mutable counterpart to the immutable
			// AgencyPublication entity. It holds the current lifecycle status
			// (active → archived) so that UpdatePublicationStatus can call
			// UpdateEntity on this type without hitting ErrImmutableType.
			{
				Name:              "AgencyPublicationStatus",
				DisplayName:       "Agency Publication Status",
				PathSegment:       "publication-statuses",
				EntityIDParam:     "publicationStatusId",
				StorageCollection: "agency_publication_statuses",
				UniqueKey:         []string{"code"},
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					// status valid values: "active", "archived"
					{Name: "status", Type: types.PropertyTypeOption, Required: true},
					// draft_ref_code references the AgencyDraft entity that was published to create the parent publication.
					{Name: "draft_ref_code", Type: types.PropertyTypeString, Required: false},
				},
				Relationships: []types.RelationshipDefinition{
					// ToMany=false: each status node belongs to exactly one publication.
					// Inverse of AgencyPublication.has_status.
					// Required=false: the link is created via an explicit CreateRelationship
					// call in PublishAgency immediately after entity creation.
					{Name: "belongs_to_publication", Label: "Publication", PathSegment: "publication", ToType: "AgencyPublication", ToMany: false, Required: false, Inverse: "has_status"},
				},
			},
			// WorkPlan binds a trigger topic, a ConfiguredRole, and a WorkItem into
			// an executable plan. CodeValdAI calls MatchWorkPlans with the incoming
			// Cross topic and raw JSON payload; the agency returns all enabled work
			// plans whose trigger_topic regex matches the topic and whose
			// payload_condition regex (if set) matches the payload string.
			{
				Name:              "WorkPlan",
				DisplayName:       "Work Plan",
				PathSegment:       "work-plans",
				EntityIDParam:     "workPlanId",
				StorageCollection: "agency_work_plans",
				UniqueKey:         []string{"code"},
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "name", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					// trigger_topic is a Go regex matched against the incoming Cross topic.
					{Name: "trigger_topic", Type: types.PropertyTypeString, Required: true},
					// payload_condition is a Go regex matched against the raw JSON payload.
					// Empty string means match all payloads.
					{Name: "payload_condition", Type: types.PropertyTypeString, Required: false},
					// instructions is a prompt template injected into the triggered AgentRun.
					// Only used when handler_service == "codevaldai".
					{Name: "instructions", Type: types.PropertyTypeString, Required: false},
					// agent_id is a cross-service reference to a CodeValdAI Agent entity ID.
					// Only used when handler_service == "codevaldai".
					{Name: "agent_id", Type: types.PropertyTypeString, Required: false},
					// agent_code is the symbolic code of the AIAgent (e.g. "deepseek-v4-developer")
					// declared in the agency's ai_config section. CodeValdAI resolves this to
					// agent_id at startup via bootstrap. Stored here so the mapping survives
					// reimport without requiring a fresh agent_id wire step.
					{Name: "agent_code", Type: types.PropertyTypeString, Required: false},
					// handler_service is the CodeVald service responsible for executing this plan.
					// Valid values: "codevaldai", "codevaldfunction", "codevaldcomm".
					{Name: "handler_service", Type: types.PropertyTypeString, Required: false},
					// function_code identifies the function binary to invoke.
					// Only used when handler_service == "codevaldfunction".
					{Name: "function_code", Type: types.PropertyTypeString, Required: false},
					// function_params is a JSON-encoded object of named parameters forwarded to
					// the function binary at invocation time. Only used when handler_service ==
					// "codevaldfunction". Follows the same encoding pattern as RunField.options.
					// Example: {"todo_type":"compile-fix","max_runs":3,"on_run_limit_exceeded":"work.task.fail"}
					{Name: "function_params", Type: types.PropertyTypeString, Required: false},
					{Name: "enabled", Type: types.PropertyTypeBoolean, Required: true},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
					// success_event is the Cross topic the plan's handler publishes when
					// the step completes successfully (e.g. "functions.job.completed").
					// Recovery pipelines for this plan synthesize this event on terminal
					// success — restoring the downstream event sequence. Optional; defaults
					// to a handler-service-specific topic. See FEAT-20260602-005.
					{Name: "success_event", Type: types.PropertyTypeString, Required: false},
					// failure_event is the Cross topic the plan's handler publishes when
					// the step fails. Cross's failure-dispatch matches inbound events
					// against this field via FindByFailureEvent and routes to
					// on_failure_pipeline when set. Optional; defaults handler-specific.
					{Name: "failure_event", Type: types.PropertyTypeString, Required: false},
					// on_failure_pipeline is the `code` of another work plan in the same
					// agency that recovers from this plan's failure. When empty the
					// failure is terminal and Cross publishes work.run.failed. The
					// recovery's terminal success must publish this plan's success_event
					// (synthesized-success contract). See FEAT-20260602-005.
					{Name: "on_failure_pipeline", Type: types.PropertyTypeString, Required: false},
					// step_timeout is the per-step inactivity duration (e.g. "10m")
					// after which the watchdog publishes work.task.timeout for the
					// current step. Falls back to WORKFLOW_RUN_STEP_STALE_TIMEOUT env
					// var (default 10m) when empty. See FEAT-20260602-006.
					{Name: "step_timeout", Type: types.PropertyTypeString, Required: false},
					// review_step_type declares the kind of reviewer that gates task
					// progression. Valid values: "ai_review", "human_review",
					// "functional_review". Empty means no review step is configured.
					// See FEAT-20260605-002.
					{Name: "review_step_type", Type: types.PropertyTypeString, Required: false},
					// review_trigger_topic is the Cross topic that fires the review agent
					// (e.g. "work.task.completed"). Must be non-empty when review_step_type
					// is set.
					{Name: "review_trigger_topic", Type: types.PropertyTypeString, Required: false},
					// review_success_topic is the Cross topic the reviewer emits on pass
					// (e.g. "work.review.passed"). The next WorkPlan step's
					// trigger_conditions should reference this topic.
					{Name: "review_success_topic", Type: types.PropertyTypeString, Required: false},
					// review_failure_topic is the Cross topic the reviewer emits on fail
					// (e.g. "work.review.failed"). Routes to on_failure_pipeline when set.
					{Name: "review_failure_topic", Type: types.PropertyTypeString, Required: false},
				},
				Relationships: []types.RelationshipDefinition{
					// ToMany=false, Required=true: a WorkPlan must belong to exactly one Agency.
					// Inverse of Agency.has_work_plan — auto-created by CreateRelationship.
					{Name: "belongs_to_agency", Label: "Agency", PathSegment: "agency", ToType: "Agency", ToMany: false, Required: true},
					// ToMany=true: context sources linked to this work plan for assembling the AgentRun bundle.
					// Inverse written on each ContextSource type as belongs_to_work_plan.
					{Name: "has_context_source", Label: "Context Sources", PathSegment: "context-sources", ToType: "GitContextSource", ToMany: true},
					// ToMany=false: the ConfiguredRole assigned to this work plan.
					{Name: "assigned_role", Label: "Assigned Role", PathSegment: "assigned-role", ToType: "ConfiguredRole", ToMany: false, Inverse: "assigned_work_plan"},
					// ToMany=false: the WorkItem this work plan executes.
					{Name: "has_work_item", Label: "Work Item", PathSegment: "work-item", ToType: "WorkItem", ToMany: false},
				},
			},
			// GitContextSource configures what to fetch from CodeValdGit when this
			// task is dispatched. Multiple sources may be linked to a single Task.
			{
				Name:              "GitContextSource",
				DisplayName:       "Git Context Source",
				PathSegment:       "git-context-sources",
				EntityIDParam:     "sourceId",
				StorageCollection: "agency_git_context_sources",
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					// signals is a comma-separated list of signal layers, e.g. "authority,contributor".
					{Name: "signals", Type: types.PropertyTypeString, Required: false},
					// max_results caps the number of files returned (default 20).
					{Name: "max_results", Type: types.PropertyTypeInteger, Required: false},
					// match_mode is "AND" or "OR" (default "OR").
					{Name: "match_mode", Type: types.PropertyTypeString, Required: false},
					// cascade expands keywords to taxonomy descendants when true.
					{Name: "cascade", Type: types.PropertyTypeBoolean, Required: false},
					// file_types is a comma-separated extension filter, e.g. ".go,.ts".
					{Name: "file_types", Type: types.PropertyTypeString, Required: false},
				},
				Relationships: []types.RelationshipDefinition{
					// ToMany=false, Required=true: a GitContextSource must belong to exactly one Task.
					{Name: "belongs_to_work_plan", Label: "Work Plan", PathSegment: "work-plan", ToType: "WorkPlan", ToMany: false, Required: true},
				},
			},
			// CommContextSource configures what to fetch from CodeValdComm (conversation
			// threads) when this task is dispatched.
			{
				Name:              "CommContextSource",
				DisplayName:       "Comm Context Source",
				PathSegment:       "comm-context-sources",
				EntityIDParam:     "sourceId",
				StorageCollection: "agency_comm_context_sources",
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					// lookback_days controls how far back to search for threads.
					{Name: "lookback_days", Type: types.PropertyTypeInteger, Required: false},
					// max_results caps the number of threads returned.
					{Name: "max_results", Type: types.PropertyTypeInteger, Required: false},
				},
				Relationships: []types.RelationshipDefinition{
					{Name: "belongs_to_work_plan", Label: "Work Plan", PathSegment: "work-plan", ToType: "WorkPlan", ToMany: false, Required: true},
				},
			},
			// WorkContextSource configures what to fetch from CodeValdWork (task details)
			// when this task is dispatched.
			{
				Name:              "WorkContextSource",
				DisplayName:       "Work Context Source",
				PathSegment:       "work-context-sources",
				EntityIDParam:     "sourceId",
				StorageCollection: "agency_work_context_sources",
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					// include_description includes the full task description in the context bundle.
					{Name: "include_description", Type: types.PropertyTypeBoolean, Required: false},
					// include_history includes the status transition history in the context bundle.
					{Name: "include_history", Type: types.PropertyTypeBoolean, Required: false},
				},
				Relationships: []types.RelationshipDefinition{
					{Name: "belongs_to_work_plan", Label: "Work Plan", PathSegment: "work-plan", ToType: "WorkPlan", ToMany: false, Required: true},
				},
			},
			{
				Name:              "AIProvider",
				DisplayName:       "AI Provider",
				PathSegment:       "ai-providers",
				EntityIDParam:     "aiProviderId",
				StorageCollection: "agency_ai_providers",
				UniqueKey:         []string{"code"},
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: true},
					{Name: "name", Type: types.PropertyTypeString, Required: true},
					{Name: "provider_type", Type: types.PropertyTypeString, Required: false},
					{Name: "api_key_env", Type: types.PropertyTypeString, Required: false},
					{Name: "base_url", Type: types.PropertyTypeString, Required: false},
					{Name: "provider_route", Type: types.PropertyTypeString, Required: false},
				},
				Relationships: []types.RelationshipDefinition{
					{Name: "belongs_to_agency", Label: "Agency", PathSegment: "agency", ToType: "Agency", ToMany: false, Required: true},
				},
			},
			{
				Name:              "AIAgent",
				DisplayName:       "AI Agent",
				PathSegment:       "ai-agents",
				EntityIDParam:     "aiAgentId",
				StorageCollection: "agency_ai_agents",
				UniqueKey:         []string{"code"},
				Properties: []types.PropertyDefinition{
					{Name: "ref_code", Type: types.PropertyTypeUUID, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: true},
					{Name: "name", Type: types.PropertyTypeString, Required: true},
					{Name: "provider_code", Type: types.PropertyTypeString, Required: false},
					{Name: "model", Type: types.PropertyTypeString, Required: false},
					{Name: "system_prompt", Type: types.PropertyTypeString, Required: false},
					{Name: "temperature", Type: types.PropertyTypeFloat, Required: false},
					{Name: "max_tokens", Type: types.PropertyTypeInteger, Required: false},
					{Name: "session_max_seconds", Type: types.PropertyTypeInteger, Required: false},
					{Name: "session_max_tokens", Type: types.PropertyTypeInteger, Required: false},
					{Name: "session_max_sessions", Type: types.PropertyTypeInteger, Required: false},
				},
				Relationships: []types.RelationshipDefinition{
					{Name: "belongs_to_agency", Label: "Agency", PathSegment: "agency", ToType: "Agency", ToMany: false, Required: true},
				},
			},
		},
	}
}
