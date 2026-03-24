// Package codevaldagency — pre-delivered schema definition.
//
// This file exposes [DefaultAgencySchema], which returns the fixed
// [types.Schema] for CodeValdAgency. cmd/main.go seeds this schema
// idempotently on startup via AgencySchemaManager.SetSchema.
//
// The schema declares twenty TypeDefinitions:
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
//   - AgencyPublicationStatus   — mutable status node for a publication (draft → active → archived)
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
				Name:        "Agency",
				DisplayName: "Agency",
				PathSegment: "",
				Properties: []types.PropertyDefinition{
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "name", Type: types.PropertyTypeString, Required: true},
					{Name: "mission", Type: types.PropertyTypeString, Required: false},
					{Name: "vision", Type: types.PropertyTypeString, Required: false},
					// enabled is false until the first PromoteDraft call succeeds,
					// at which point it is set to true. Set to false to disable the agency.
					{Name: "enabled", Type: types.PropertyTypeBoolean, Required: false},
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
				},
			},
			{
				Name:        "Goal",
				DisplayName: "Goal",
				// PathSegment is intentionally empty — live Goals are created by PromoteDraft,
				// not via direct HTTP CRUD. CRUD routes exist only for DraftGoal, scoped under
				// /agency/{agencyId}/drafts/{draftId}/goals.
				UniqueKey: []string{"code"},
				Properties: []types.PropertyDefinition{
					{Name: "code", Type: types.PropertyTypeString, Required: false},
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
				},
			},
			{
				Name:        "Workflow",
				DisplayName: "Workflow",
				// PathSegment is intentionally empty — live Workflows are created by PromoteDraft,
				// not via direct HTTP CRUD. CRUD routes exist only for DraftWorkflow, scoped under
				// /agency/{agencyId}/drafts/{draftId}/workflows.
				UniqueKey: []string{"code"},
				Properties: []types.PropertyDefinition{
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "name", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
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
				Name:        "WorkItem",
				DisplayName: "Work Item",
				// PathSegment is intentionally empty — live WorkItems are created by PromoteDraft,
				// not via direct HTTP CRUD. CRUD routes exist only for DraftWorkItem, scoped under
				// /agency/{agencyId}/drafts/{draftId}/work-items.
				UniqueKey: []string{"code"},
				Properties: []types.PropertyDefinition{
					{Name: "code", Type: types.PropertyTypeString, Required: false},
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
					// ToMany=true, Required=false: zero or more eligible roles may be assigned.
					// Whichever actor (human or ai_agent) claims the task first executes it.
					{Name: "assigned_role", Label: "Assigned Roles", PathSegment: "assigned-roles", ToType: "ConfiguredRole", ToMany: true, Inverse: "assigned_work_item"},
					// ToMany=true: standing rules attached to this work item.
					{Name: "has_instruction", Label: "Instructions", PathSegment: "instructions", ToType: "Instruction", ToMany: true, Inverse: "belongs_to_work_item"},
					// ToMany=true: expected outputs this work item must produce.
					{Name: "has_deliverable", Label: "Deliverables", PathSegment: "deliverables", ToType: "Deliverable", ToMany: true, Inverse: "belongs_to_work_item"},
					// ToMany=true: reference artifacts attached to this work item (e.g. input data, templates).
					{Name: "has_content_ref", Label: "Content Refs", PathSegment: "content-refs", ToType: "ContentRef", ToMany: true, Inverse: "belongs_to_work_item"},
				},
			},
			{
				Name:        "Instruction",
				DisplayName: "Instruction",
				// PathSegment is intentionally empty — live Instructions are created by PromoteDraft,
				// not via direct HTTP CRUD. CRUD routes exist only for DraftInstruction, scoped under
				// /agency/{agencyId}/drafts/{draftId}/instructions.
				UniqueKey: []string{"code"},
				Properties: []types.PropertyDefinition{
					{Name: "code", Type: types.PropertyTypeString, Required: false},
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
				Name:        "Deliverable",
				DisplayName: "Deliverable",
				// PathSegment is intentionally empty — live Deliverables are created by PromoteDraft,
				// not via direct HTTP CRUD. CRUD routes exist only for DraftDeliverable, scoped under
				// /agency/{agencyId}/drafts/{draftId}/deliverables.
				UniqueKey: []string{"code"},
				Properties: []types.PropertyDefinition{
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
				Name:          "DeliverableResult",
				DisplayName:   "Deliverable Result",
				PathSegment:   "results",
				EntityIDParam: "resultId",
				Immutable:     true,
				Properties: []types.PropertyDefinition{
					{Name: "code", Type: types.PropertyTypeString, Required: false},
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
				Name:          "ContentRef",
				DisplayName:   "Content Ref",
				PathSegment:   "content-refs",
				EntityIDParam: "contentRefId",
				Immutable:     true,
				Properties: []types.PropertyDefinition{
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
				Name:        "ConfiguredRole",
				DisplayName: "Configured Role",
				// PathSegment is intentionally empty — live ConfiguredRoles are created by PromoteDraft,
				// not via direct HTTP CRUD. CRUD routes exist only for DraftConfiguredRole, scoped under
				// /agency/{agencyId}/drafts/{draftId}/configured-roles.
				UniqueKey: []string{"code"},
				Properties: []types.PropertyDefinition{
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
					// ToMany=true: inverse of WorkItem.assigned_role — auto-created by CreateRelationship.
					{Name: "assigned_work_item", Label: "Work Items", PathSegment: "work-items", ToType: "WorkItem", ToMany: true},
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
			// and queried by TypeID ("DraftGoal", etc.) + draft_id property.
			{
				Name:              "AgencyDraft",
				DisplayName:       "Agency Draft",
				PathSegment:       "drafts",
				EntityIDParam:     "draftId",
				StorageCollection: "agency_drafts",
				UniqueKey:         []string{"code"},
				Properties: []types.PropertyDefinition{
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					// status valid values: "open", "promoted", "archived"
					//   open     — actively being edited
					//   promoted — draft entities copied to live; terminal
					//   archived — discarded without promotion; terminal
					{Name: "status", Type: types.PropertyTypeOption, Required: true},
					// forked_from_id is the Agency ID (forked_from_type=="live") or
					// AgencyDraft ID (forked_from_type=="draft") this was copied from.
					{Name: "forked_from_id", Type: types.PropertyTypeString, Required: false},
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
			// CreateDraft. They carry a draft_id property for scoping. Each type
			// has a PathSegment so the registrar can generate CRUD routes scoped
			// under /agency/{agencyId}/drafts/{draftId}/...
			{
				Name:              "DraftGoal",
				DisplayName:       "Draft Goal",
				PathSegment:       "drafts/{draftId}/goals",
				EntityIDParam:     "goalId",
				StorageCollection: "agency_draft_entities",
				UniqueKey:         []string{"draft_id", "code"},
				Properties: []types.PropertyDefinition{
					{Name: "draft_id", Type: types.PropertyTypeString, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "title", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
				},
			},
			{
				Name:              "DraftWorkflow",
				DisplayName:       "Draft Workflow",
				PathSegment:       "drafts/{draftId}/workflows",
				EntityIDParam:     "workflowId",
				StorageCollection: "agency_draft_entities",
				UniqueKey:         []string{"draft_id", "code"},
				Properties: []types.PropertyDefinition{
					{Name: "draft_id", Type: types.PropertyTypeString, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "name", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
				},
			},
			{
				Name:              "DraftWorkItem",
				DisplayName:       "Draft Work Item",
				PathSegment:       "drafts/{draftId}/work-items",
				EntityIDParam:     "workItemId",
				StorageCollection: "agency_draft_entities",
				UniqueKey:         []string{"draft_id", "code"},
				Properties: []types.PropertyDefinition{
					{Name: "draft_id", Type: types.PropertyTypeString, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					// draft_workflow_id references the parent DraftWorkflow entity ID.
					{Name: "draft_workflow_id", Type: types.PropertyTypeString, Required: false},
					{Name: "title", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
					{Name: "prompt", Type: types.PropertyTypeString, Required: false},
				},
			},
			{
				Name:              "DraftConfiguredRole",
				DisplayName:       "Draft Configured Role",
				PathSegment:       "drafts/{draftId}/configured-roles",
				EntityIDParam:     "configuredRoleId",
				StorageCollection: "agency_draft_entities",
				UniqueKey:         []string{"draft_id", "code"},
				Properties: []types.PropertyDefinition{
					{Name: "draft_id", Type: types.PropertyTypeString, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "name", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					{Name: "actor_type", Type: types.PropertyTypeOption, Required: true},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
				},
			},
			{
				Name:              "DraftInstruction",
				DisplayName:       "Draft Instruction",
				PathSegment:       "drafts/{draftId}/instructions",
				EntityIDParam:     "instructionId",
				StorageCollection: "agency_draft_entities",
				UniqueKey:         []string{"draft_id", "code"},
				Properties: []types.PropertyDefinition{
					{Name: "draft_id", Type: types.PropertyTypeString, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					// draft_workflow_id or draft_work_item_id — whichever parent applies.
					{Name: "draft_workflow_id", Type: types.PropertyTypeString, Required: false},
					{Name: "draft_work_item_id", Type: types.PropertyTypeString, Required: false},
					{Name: "content", Type: types.PropertyTypeString, Required: true},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
				},
			},
			{
				Name:              "DraftDeliverable",
				DisplayName:       "Draft Deliverable",
				PathSegment:       "drafts/{draftId}/deliverables",
				EntityIDParam:     "deliverableId",
				StorageCollection: "agency_draft_entities",
				UniqueKey:         []string{"draft_id", "code"},
				Properties: []types.PropertyDefinition{
					{Name: "draft_id", Type: types.PropertyTypeString, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					// draft_work_item_id references the parent DraftWorkItem entity ID.
					{Name: "draft_work_item_id", Type: types.PropertyTypeString, Required: false},
					{Name: "title", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
					{Name: "blocking", Type: types.PropertyTypeBoolean, Required: true},
				},
			},
			{
				Name:              "DraftDeliverableResult",
				DisplayName:       "Draft Deliverable Result",
				PathSegment:       "drafts/{draftId}/results",
				EntityIDParam:     "resultId",
				StorageCollection: "agency_draft_entities",
				UniqueKey:         []string{"draft_id", "code"},
				Properties: []types.PropertyDefinition{
					{Name: "draft_id", Type: types.PropertyTypeString, Required: true},
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					// draft_deliverable_id references the parent DraftDeliverable entity ID.
					{Name: "draft_deliverable_id", Type: types.PropertyTypeString, Required: false},
					// status valid values: "pending", "completed", "rejected", "waived"
					{Name: "status", Type: types.PropertyTypeOption, Required: true},
					{Name: "produced_at", Type: types.PropertyTypeDatetime, Required: false},
				},
			},
			{
				Name:          "AgencySnapshot",
				DisplayName:   "Agency Snapshot",
				PathSegment:   "snapshots",
				EntityIDParam: "snapshotId",
				Immutable:     true,
				Properties: []types.PropertyDefinition{
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "snapshot_at", Type: types.PropertyTypeDatetime, Required: true},
					// draft_id references the AgencyDraft entity that was promoted to create this snapshot.
					{Name: "draft_id", Type: types.PropertyTypeString, Required: true},
				},
				Relationships: []types.RelationshipDefinition{
					// ToMany=false, Required=true: a Snapshot must belong to exactly one Agency.
					// Inverse of Agency.has_snapshot — auto-created by CreateRelationship.
					{Name: "belongs_to_agency", Label: "Agency", PathSegment: "agency", ToType: "Agency", ToMany: false, Required: true},
				},
			},
			{
				Name:          "AgencyPublication",
				DisplayName:   "Agency Publication",
				PathSegment:   "publications",
				EntityIDParam: "publicationId",
				Immutable:     true,
				Properties: []types.PropertyDefinition{
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					{Name: "version", Type: types.PropertyTypeInteger, Required: true},
					{Name: "tag", Type: types.PropertyTypeString, Required: true},
					{Name: "published_at", Type: types.PropertyTypeDatetime, Required: true},
					// draft_id references the AgencyDraft entity that was published.
					{Name: "draft_id", Type: types.PropertyTypeString, Required: true},
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
			// (draft → active → archived) so that UpdatePublicationStatus can call
			// UpdateEntity on this type without hitting ErrImmutableType.
			{
				Name:          "AgencyPublicationStatus",
				DisplayName:   "Agency Publication Status",
				PathSegment:   "publication-statuses",
				EntityIDParam: "publicationStatusId",
				UniqueKey:     []string{"code"},
				Properties: []types.PropertyDefinition{
					{Name: "code", Type: types.PropertyTypeString, Required: false},
					// status valid values: "draft", "active", "archived"
					{Name: "status", Type: types.PropertyTypeOption, Required: true},
					// draft_id references the AgencyDraft entity that was published to create the parent publication.
					{Name: "draft_id", Type: types.PropertyTypeString, Required: false},
				},
				Relationships: []types.RelationshipDefinition{
					// ToMany=false: each status node belongs to exactly one publication.
					// Inverse of AgencyPublication.has_status.
					// Required=false: the link is created via an explicit CreateRelationship
					// call in PublishAgency immediately after entity creation.
					{Name: "belongs_to_publication", Label: "Publication", PathSegment: "publication", ToType: "AgencyPublication", ToMany: false, Required: false, Inverse: "has_status"},
				},
			},
		},
	}
}
