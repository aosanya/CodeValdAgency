// Package codevaldagency — pre-delivered schema definition.
//
// This file exposes [DefaultAgencySchema], which returns the fixed
// [types.Schema] for CodeValdAgency. cmd/main.go seeds this schema
// idempotently on startup via AgencySchemaManager.SetSchema.
//
// The schema declares seven TypeDefinitions:
//   - Agency           — root entity (mutable)
//   - Goal             — strategic objective (mutable)
//   - Workflow         — ordered container of WorkItems (mutable)
//   - WorkItem         — unit of work within a Workflow (mutable)
//   - ConfiguredRole   — custom role beyond super_admin/admin (mutable)
//   - AgencySnapshot   — immutable activation record (draft → active)
//   - AgencyPublication — immutable versioned publication snapshot
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
func DefaultAgencySchema() types.Schema {
	return types.Schema{
		ID:      "agency-schema-v1",
		Version: 1,
		Tag:     "v1",
		Types: []types.TypeDefinition{
			{
				Name:        "Agency",
				DisplayName: "Agency",
				Immutable:   false,
				Properties: []types.PropertyDefinition{
					{Name: "name", Type: types.PropertyTypeString, Required: true},
					{Name: "mission", Type: types.PropertyTypeString, Required: false},
					{Name: "vision", Type: types.PropertyTypeString, Required: false},
					{Name: "status", Type: types.PropertyTypeString, Required: true},
				},
			},
			{
				Name:        "Goal",
				DisplayName: "Goal",
				Immutable:   false,
				Properties: []types.PropertyDefinition{
					{Name: "title", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					{Name: "ordinality", Type: types.PropertyTypeInteger, Required: true},
				},
			},
			{
				Name:        "Workflow",
				DisplayName: "Workflow",
				Immutable:   false,
				Properties: []types.PropertyDefinition{
					{Name: "name", Type: types.PropertyTypeString, Required: true},
				},
			},
			{
				Name:        "WorkItem",
				DisplayName: "Work Item",
				Immutable:   false,
				Properties: []types.PropertyDefinition{
					{Name: "title", Type: types.PropertyTypeString, Required: true},
					{Name: "description", Type: types.PropertyTypeString, Required: false},
					{Name: "order", Type: types.PropertyTypeInteger, Required: true},
					{Name: "parallel", Type: types.PropertyTypeBoolean, Required: false},
				},
			},
			{
				Name:        "ConfiguredRole",
				DisplayName: "Configured Role",
				Immutable:   false,
				Properties: []types.PropertyDefinition{
					{Name: "name", Type: types.PropertyTypeString, Required: true},
					{Name: "actor_type", Type: types.PropertyTypeString, Required: true},
				},
			},
			{
				Name:              "AgencySnapshot",
				DisplayName:       "Agency Snapshot",
				Immutable:         true,
				StorageCollection: "agency_snapshots",
				Properties: []types.PropertyDefinition{
					{Name: "snapshot_at", Type: types.PropertyTypeDatetime, Required: true},
				},
			},
			{
				Name:              "AgencyPublication",
				DisplayName:       "Agency Publication",
				Immutable:         true,
				StorageCollection: "agency_publications",
				Properties: []types.PropertyDefinition{
					{Name: "version", Type: types.PropertyTypeInteger, Required: true},
					{Name: "tag", Type: types.PropertyTypeString, Required: true},
					{Name: "published_at", Type: types.PropertyTypeDatetime, Required: true},
				},
			},
		},
	}
}
