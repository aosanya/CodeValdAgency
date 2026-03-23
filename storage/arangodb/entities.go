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
	col := b.collectionFor(req.TypeID)
	if _, err := col.CreateDocument(ctx, doc); err != nil {
		if driver.IsConflict(err) {
			return entitygraph.Entity{}, fmt.Errorf("CreateEntity: %w", errEntityAlreadyExists)
		}
		return entitygraph.Entity{}, fmt.Errorf("CreateEntity: %w", err)
	}
	return toEntity(doc, id), nil
}

// GetEntity returns the entity identified by agencyID and entityID.
// Searches every entity collection derived from the schema. Returns
// errEntityNotFound if the entity is absent from all collections.
func (b *Backend) GetEntity(ctx context.Context, agencyID, entityID string) (entitygraph.Entity, error) {
	for _, col := range b.allEntityCollections() {
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
	if b.isImmutable(existing.TypeID) {
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
	col := b.collectionFor(existing.TypeID)
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
	col := b.collectionFor(existing.TypeID)
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
	// When TypeID is set we go directly to that type's collection.
	// When TypeID is empty we query every entity collection from the schema.
	var cols []driver.Collection
	if filter.TypeID != "" {
		cols = []driver.Collection{b.collectionFor(filter.TypeID)}
	} else {
		cols = b.allEntityCollections()
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
