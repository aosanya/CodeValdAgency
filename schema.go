// Package codevaldagency — pre-delivered schema definition.
//
// This file exposes [DefaultAgencySchema], which returns the fixed
// [types.Schema] for CodeValdAgency. cmd/main.go seeds this schema
// idempotently on startup via AgencySchemaManager.SetSchema.
//
// The schema declares seven TypeDefinitions:
//   - Agency            — root entity (mutable)
//   - Goal              — strategic objective (mutable)
//   - Workflow          — ordered container of WorkItems (mutable)
//   - WorkItem          — unit of work within a Workflow (mutable)
//   - ConfiguredRole    — custom role beyond super_admin/admin (mutable)
//   - AgencySnapshot    — immutable activation record (draft → active)
//   - AgencyPublication — immutable versioned publication snapshot
//
// Relationship graph:
//
//	Agency ──has_goal──────────────► Goal
//	       ──has_workflow──────────► Workflow ──has_work_item──► WorkItem
//	       ──has_configured_role──► ConfiguredRole
//	       ──has_snapshot─────────► AgencySnapshot   (Immutable)
//	       ──has_publication──────► AgencyPublication (Immutable)
//
//	Goal              ──belongs_to_agency──►    Agency   (ToMany=false, inverse)
//	Workflow          ──belongs_to_agency──►    Agency   (ToMany=false, inverse)
//	WorkItem          ──belongs_to_workflow──►  Workflow (ToMany=false, inverse)
//	ConfiguredRole    ──belongs_to_agency──►    Agency   (ToMany=false, inverse)
//	AgencySnapshot    ──belongs_to_agency──►    Agency   (ToMany=false, inverse)
//	AgencyPublication ──belongs_to_agency──►    Agency   (ToMany=false, inverse)
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
				PathSegment: "", // no top-level routes — Agency IS the agency context
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
				Name:        "Goal",
				DisplayName: "Goal",
				PathSegment: "goals",
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
				Name:        "Workflow",
				DisplayName: "Workflow",
				PathSegment: "workflows",
				Properties: []types.PropertyDefinition{
					{Name: "name",       Type: types.PropertyTypeString,  Required: true},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
				},
				Relationships: []types.RelationshipDefinition{
					{Name: "has_work_item", Label: "Work Items", PathSegment: "work-items", ToType: "WorkItem", ToMany: true, Inverse: "belongs_to_workflow"},
					// ToMany=false, Required=true: a Workflow must belong to exactly one Agency.
					// Inverse of Agency.has_workflow — auto-created by CreateRelationship.
					{Name: "belongs_to_agency", Label: "Agency", PathSegment: "agency", ToType: "Agency", ToMany: false, Required: true},
				},
			},
			{
				Name:        "WorkItem",
				DisplayName: "Work Item",
				PathSegment: "work-items",
				Properties: []types.PropertyDefinition{
					{Name: "title",       Type: types.PropertyTypeString,  Required: true},
					{Name: "description", Type: types.PropertyTypeString,  Required: false},
					// WorkItems with the same ordinality value run in parallel.
					// WorkItems with a higher ordinality run after all items at the lower value complete.
					{Name: "ordinality",  Type: types.PropertyTypeInteger, Required: true},
				},
				Relationships: []types.RelationshipDefinition{
					// ToMany=false, Required=true: a WorkItem must belong to exactly one Workflow.
					// Inverse of Workflow.has_work_item — auto-created by CreateRelationship.
					{Name: "belongs_to_workflow", Label: "Workflow", PathSegment: "workflow", ToType: "Workflow", ToMany: false, Required: true},
				},
			},
			{
				Name:        "ConfiguredRole",
				DisplayName: "Configured Role",
				PathSegment: "configured-roles",
				Properties: []types.PropertyDefinition{
					{Name: "name",       Type: types.PropertyTypeString,  Required: true},
					{Name: "actor_type", Type: types.PropertyTypeString,  Required: true},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
				},
				Relationships: []types.RelationshipDefinition{
					// ToMany=false, Required=true: a ConfiguredRole must belong to exactly one Agency.
					// Inverse of Agency.has_configured_role — auto-created by CreateRelationship.
					{Name: "belongs_to_agency", Label: "Agency", PathSegment: "agency", ToType: "Agency", ToMany: false, Required: true},
				},
			},
			{
				Name:              "AgencySnapshot",
				DisplayName:       "Agency Snapshot",
				PathSegment:       "snapshots",
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
				Immutable:         true,
				StorageCollection: "agency_publications",
				Properties: []types.PropertyDefinition{
					{Name: "version", Type: types.PropertyTypeInteger, Required: true},
					{Name: "tag", Type: types.PropertyTypeString, Required: true},
					{Name: "published_at", Type: types.PropertyTypeDatetime, Required: true},
					// status valid values: "draft", "active", "archived"
					// Stored as runtime option data — not enforced by the schema layer.
					{Name: "status", Type: types.PropertyTypeOption, Required: true},
				},
				Relationships: []types.RelationshipDefinition{
					// ToMany=false, Required=true: a Publication must belong to exactly one Agency.
					// Inverse of Agency.has_publication — auto-created by CreateRelationship.
					{Name: "belongs_to_agency", Label: "Agency", PathSegment: "agency", ToType: "Agency", ToMany: false, Required: true},
				},
			},
		},
	}
}
