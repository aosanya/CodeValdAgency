// Package codevaldagency — pre-delivered schema definition.
//
// This file exposes [DefaultAgencySchema], which returns the fixed
// [types.Schema] for CodeValdAgency. cmd/main.go seeds this schema
// idempotently on startup via AgencySchemaManager.SetSchema.
//
// The schema declares twelve TypeDefinitions:
//   - Agency                    — root entity (mutable)
//   - Goal                      — strategic objective (mutable)
//   - Workflow                  — ordered container of WorkItems (mutable)
//   - WorkItem                  — unit of work within a Workflow (mutable)
//   - Instruction               — ordered rule or constraint attached to a Workflow or WorkItem (mutable)
//   - Deliverable               — spec: expected output a WorkItem must produce (mutable)
//   - DeliverableResult         — instance: actual output submitted against a Deliverable spec (immutable)
//   - ContentRef                — path to a single artifact in CodeValdGit; attachable to DeliverableResult, Instruction, or WorkItem (immutable)
//   - ConfiguredRole            — custom role beyond super_admin/admin (mutable)
//   - AgencySnapshot            — immutable activation record (draft → active)
//   - AgencyPublication         — immutable versioned publication snapshot
//   - AgencyPublicationStatus   — mutable status node for a publication (draft → active → archived)
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
				PathSegment:       "", // no top-level routes — Agency IS the agency context
				StorageCollection: "agencies",
				Properties: []types.PropertyDefinition{
					{Name: "name", Type: types.PropertyTypeString, Required: true},
					{Name: "mission", Type: types.PropertyTypeString, Required: false},
					{Name: "vision", Type: types.PropertyTypeString, Required: false},
				},
				Relationships: []types.RelationshipDefinition{
					{Name: "has_goal", Label: "Goals", PathSegment: "goals", ToType: "Goal", ToMany: true, Inverse: "belongs_to_agency"},
					{Name: "has_workflow", Label: "Workflows", PathSegment: "workflows", ToType: "Workflow", ToMany: true, Inverse: "belongs_to_agency"},
					{Name: "has_configured_role", Label: "Configured Roles", PathSegment: "configured-roles", ToType: "ConfiguredRole", ToMany: true, Inverse: "belongs_to_agency"},
					{Name: "has_snapshot", Label: "Snapshots", PathSegment: "snapshots", ToType: "AgencySnapshot", ToMany: true, Inverse: "belongs_to_agency"},
					{Name: "has_publication", Label: "Publications", PathSegment: "publications", ToType: "AgencyPublication", ToMany: true, Inverse: "belongs_to_agency"},
				},
			},
			{
				Name:              "Goal",
				DisplayName:       "Goal",
				PathSegment:       "goals",
				EntityIDParam:     "goalId",
				StorageCollection: "agency_goals",
				Properties: []types.PropertyDefinition{
					{Name: "title", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
				},
				Relationships: []types.RelationshipDefinition{
					// ToMany=false, Required=true: a Goal must belong to exactly one Agency.
					// Inverse of Agency.has_goal — auto-created by CreateRelationship.
					{Name: "belongs_to_agency", Label: "Agency", PathSegment: "agency", ToType: "Agency", ToMany: false, Required: true},
				},
			},
			{
				Name:              "Workflow",
				DisplayName:       "Workflow",
				PathSegment:       "workflows",
				EntityIDParam:     "workflowId",
				StorageCollection: "agency_workflows",
				Properties: []types.PropertyDefinition{
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
				},
			},
			{
				Name:              "WorkItem",
				DisplayName:       "Work Item",
				PathSegment:       "work-items",
				EntityIDParam:     "workItemId",
				StorageCollection: "agency_work_items",
				Properties: []types.PropertyDefinition{
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
				Name:              "Instruction",
				DisplayName:       "Instruction",
				PathSegment:       "instructions",
				EntityIDParam:     "instructionId",
				StorageCollection: "agency_instructions",
				Properties: []types.PropertyDefinition{
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
				PathSegment:       "deliverables",
				EntityIDParam:     "deliverableId",
				StorageCollection: "agency_deliverables",
				Properties: []types.PropertyDefinition{
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
				StorageCollection: "deliverable_results",
				Properties: []types.PropertyDefinition{
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
				StorageCollection: "content_refs",
				Properties: []types.PropertyDefinition{
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
				PathSegment:       "configured-roles",
				EntityIDParam:     "configuredRoleId",
				StorageCollection: "agency_configured_roles",
				Properties: []types.PropertyDefinition{
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
					{Name: "snapshot_at", Type: types.PropertyTypeDatetime, Required: true},
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
					{Name: "version", Type: types.PropertyTypeInteger, Required: true},
					{Name: "tag", Type: types.PropertyTypeString, Required: true},
					{Name: "published_at", Type: types.PropertyTypeDatetime, Required: true},
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
				Name:              "AgencyPublicationStatus",
				DisplayName:       "Agency Publication Status",
				PathSegment:       "publication-statuses",
				EntityIDParam:     "publicationStatusId",
				StorageCollection: "publication_statuses",
				Properties: []types.PropertyDefinition{
					// status valid values: "draft", "active", "archived"
					{Name: "status", Type: types.PropertyTypeOption, Required: true},
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
