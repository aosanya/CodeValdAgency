// entities.go contains CreateEntity, GetEntity, UpdateEntity, DeleteEntity,
// and ListEntities for the Backend.
package arangodb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	driver "github.com/arangodb/go-driver"
	"github.com/google/uuid"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
)

// sentinel errors for storage-layer entity operations.
var (
	errEntityNotFound      = errors.New("entity not found")
	errEntityAlreadyExists = errors.New("entity already exists")
	errImmutableType       = errors.New("entity type is immutable")
)

// immutableTypeIDs lists the TypeDefinition names that are marked Immutable in
// the pre-delivered agency schema. UpdateEntity returns errImmutableType for
// any entity whose TypeID appears here.
var immutableTypeIDs = map[string]bool{
	"AgencySnapshot":    true,
	"AgencyPublication": true,
}

// entityDoc is the ArangoDB document representation of an [entitygraph.Entity].
type entityDoc struct {
	Key        string         `json:"_key,omitempty"`
	TypeID     string         `json:"type_id"`
	AgencyID   string         `json:"agency_id"`
	Properties map[string]any `json:"properties"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	Deleted    bool           `json:"deleted"`
	DeletedAt  *time.Time     `json:"deleted_at,omitempty"`
}

// collectionForTypeID returns the driver.Collection in which entities of the
// given TypeID should be stored. AgencySnapshot and AgencyPublication entities
// use dedicated immutable collections; all others use agency_entities.
func (b *Backend) collectionForTypeID(typeID string) driver.Collection {
	switch typeID {
	case "AgencySnapshot":
		return b.snapshots
	case "AgencyPublication":
		return b.publications
	default:
		return b.entities
	}
}

// // collectionNameForTypeID returns the ArangoDB collection name string for the
// // given TypeID. Used when constructing _from / _to handles for edge documents.
// func collectionNameForTypeID(typeID string) string {
// 	switch typeID {
// 	case "AgencySnapshot":
// 		return colSnapshots
// 	case "AgencyPublication":
// 		return colPublications
// 	default:
// 		return colEntities
// 	}
// }

// CreateEntity creates a new entity document in the appropriate collection.
// Returns errEntityAlreadyExists if a document with the same key already exists.
func (b *Backend) CreateEntity(ctx context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
	now := time.Now().UTC()
	id := uuid.NewString()
	doc := entityDoc{
		Key:        id,
		TypeID:     req.TypeID,
		AgencyID:   req.AgencyID,
		Properties: req.Properties,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if doc.Properties == nil {
		doc.Properties = make(map[string]any)
	}
	col := b.collectionForTypeID(req.TypeID)
	if _, err := col.CreateDocument(ctx, doc); err != nil {
		if driver.IsConflict(err) {
			return entitygraph.Entity{}, fmt.Errorf("CreateEntity: %w", errEntityAlreadyExists)
		}
		return entitygraph.Entity{}, fmt.Errorf("CreateEntity: %w", err)
	}
	return toEntity(doc, id), nil
}

// GetEntity returns the entity identified by agencyID and entityID.
// Searches agency_entities first, then agency_snapshots, then
// agency_publications. Returns errEntityNotFound if not present in any.
func (b *Backend) GetEntity(ctx context.Context, agencyID, entityID string) (entitygraph.Entity, error) {
	for _, col := range []driver.Collection{b.entities, b.snapshots, b.publications} {
		var doc entityDoc
		if _, err := col.ReadDocument(ctx, entityID, &doc); err == nil {
			if doc.AgencyID != agencyID {
				continue
			}
			return toEntity(doc, entityID), nil
		} else if !driver.IsNotFound(err) {
			return entitygraph.Entity{}, fmt.Errorf("GetEntity: %w", err)
		}
	}
	return entitygraph.Entity{}, fmt.Errorf("GetEntity %s: %w", entityID, errEntityNotFound)
}

// UpdateEntity patches the mutable properties of an entity.
// Returns errImmutableType if the entity's TypeID is in immutableTypeIDs.
// Returns errEntityNotFound if the entity does not exist.
func (b *Backend) UpdateEntity(
	ctx context.Context,
	agencyID, entityID string,
	req entitygraph.UpdateEntityRequest,
) (entitygraph.Entity, error) {
	existing, err := b.GetEntity(ctx, agencyID, entityID)
	if err != nil {
		return entitygraph.Entity{}, fmt.Errorf("UpdateEntity %s: %w", entityID, err)
	}
	if immutableTypeIDs[existing.TypeID] {
		return entitygraph.Entity{}, fmt.Errorf("UpdateEntity %s: %w", entityID, errImmutableType)
	}
	if existing.Properties == nil {
		existing.Properties = make(map[string]any)
	}
	for k, v := range req.Properties {
		existing.Properties[k] = v
	}
	existing.UpdatedAt = time.Now().UTC()
	updated := entityDoc{
		Key:        entityID,
		TypeID:     existing.TypeID,
		AgencyID:   existing.AgencyID,
		Properties: existing.Properties,
		CreatedAt:  existing.CreatedAt,
		UpdatedAt:  existing.UpdatedAt,
		Deleted:    existing.Deleted,
		DeletedAt:  existing.DeletedAt,
	}
	col := b.collectionForTypeID(existing.TypeID)
	if _, err := col.ReplaceDocument(ctx, entityID, updated); err != nil {
		if driver.IsNotFound(err) {
			return entitygraph.Entity{}, fmt.Errorf("UpdateEntity %s: %w", entityID, errEntityNotFound)
		}
		return entitygraph.Entity{}, fmt.Errorf("UpdateEntity %s: %w", entityID, err)
	}
	return toEntity(updated, entityID), nil
}

// DeleteEntity soft-deletes the entity by setting Deleted=true and recording
// DeletedAt. The document is never hard-deleted.
func (b *Backend) DeleteEntity(ctx context.Context, agencyID, entityID string) error {
	existing, err := b.GetEntity(ctx, agencyID, entityID)
	if err != nil {
		return fmt.Errorf("DeleteEntity %s: %w", entityID, err)
	}
	now := time.Now().UTC()
	updated := entityDoc{
		Key:        entityID,
		TypeID:     existing.TypeID,
		AgencyID:   existing.AgencyID,
		Properties: existing.Properties,
		CreatedAt:  existing.CreatedAt,
		UpdatedAt:  now,
		Deleted:    true,
		DeletedAt:  &now,
	}
	col := b.collectionForTypeID(existing.TypeID)
	if _, err := col.ReplaceDocument(ctx, entityID, updated); err != nil {
		if driver.IsNotFound(err) {
			return fmt.Errorf("DeleteEntity %s: %w", entityID, errEntityNotFound)
		}
		return fmt.Errorf("DeleteEntity %s: %w", entityID, err)
	}
	return nil
}

// ListEntities returns non-deleted entities matching the filter.
// Zero-value filter fields are treated as "no restriction".
func (b *Backend) ListEntities(
	ctx context.Context,
	filter entitygraph.EntityFilter,
) ([]entitygraph.Entity, error) {
	bindVars := map[string]interface{}{}
	var conditions []string
	conditions = append(conditions, "doc.deleted != true")
	if filter.AgencyID != "" {
		conditions = append(conditions, "doc.agency_id == @agencyID")
		bindVars["agencyID"] = filter.AgencyID
	}
	if filter.TypeID != "" {
		conditions = append(conditions, "doc.type_id == @typeID")
		bindVars["typeID"] = filter.TypeID
	}
	where := strings.Join(conditions, " AND ")

	// Determine which collection(s) to query based on the TypeID filter.
	var cols []driver.Collection
	switch filter.TypeID {
	case "AgencySnapshot":
		cols = []driver.Collection{b.snapshots}
	case "AgencyPublication":
		cols = []driver.Collection{b.publications}
	case "":
		cols = []driver.Collection{b.entities, b.snapshots, b.publications}
	default:
		cols = []driver.Collection{b.entities}
	}

	var results []entitygraph.Entity
	for _, col := range cols {
		q := fmt.Sprintf("FOR doc IN %s FILTER %s RETURN doc", col.Name(), where)
		cursor, qErr := b.db.Query(ctx, q, bindVars)
		if qErr != nil {
			return nil, fmt.Errorf("ListEntities: query %s: %w", col.Name(), qErr)
		}
		var readErr error
		for cursor.HasMore() {
			var doc entityDoc
			meta, rErr := cursor.ReadDocument(ctx, &doc)
			if rErr != nil {
				readErr = fmt.Errorf("ListEntities: read: %w", rErr)
				break
			}
			results = append(results, toEntity(doc, meta.Key))
		}
		cursor.Close()
		if readErr != nil {
			return nil, readErr
		}
	}
	return results, nil
}

// toEntity converts an entityDoc and its ArangoDB _key to an
// [entitygraph.Entity].
func toEntity(doc entityDoc, key string) entitygraph.Entity {
	e := entitygraph.Entity{
		ID:         key,
		AgencyID:   doc.AgencyID,
		TypeID:     doc.TypeID,
		Properties: doc.Properties,
		CreatedAt:  doc.CreatedAt,
		UpdatedAt:  doc.UpdatedAt,
		Deleted:    doc.Deleted,
		DeletedAt:  doc.DeletedAt,
	}
	if e.Properties == nil {
		e.Properties = make(map[string]any)
	}
	return e
}
