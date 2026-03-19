// schemaops.go contains SetSchema, GetSchema, and ListSchemaVersions for the
// Backend, implementing [entitygraph.SchemaManager].
package arangodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	driver "github.com/arangodb/go-driver"
	"github.com/google/uuid"

	"github.com/aosanya/CodeValdSharedLib/types"
)

// sentinel error for schema operations.
var errSchemaNotFound = errors.New("schema not found")

// schemaDoc is the ArangoDB document representation of a [types.Schema].
type schemaDoc struct {
	Key       string                 `json:"_key,omitempty"`
	AgencyID  string                 `json:"agency_id"`
	Version   int                    `json:"version"`
	Tag       string                 `json:"tag"`
	Types     []types.TypeDefinition `json:"types"`
	CreatedAt time.Time              `json:"created_at"`
}

// SetSchema stores a new schema version for the given agency. The Version field
// of the supplied schema is ignored; the implementation assigns the next
// sequential version number (max existing version + 1, starting at 1). The Tag
// and Types fields are persisted as supplied.
//
// SetSchema is idempotent with respect to Tag — a second call with the same Tag
// still produces a new version document.
func (b *Backend) SetSchema(ctx context.Context, schema types.Schema) error {
	nextVersion, err := b.nextSchemaVersion(ctx, schema.AgencyID)
	if err != nil {
		return fmt.Errorf("SetSchema %s: determine next version: %w", schema.AgencyID, err)
	}
	now := time.Now().UTC()
	id := uuid.NewString()
	doc := schemaDoc{
		Key:       id,
		AgencyID:  schema.AgencyID,
		Version:   nextVersion,
		Tag:       schema.Tag,
		Types:     schema.Types,
		CreatedAt: now,
	}
	if _, err := b.schemas.CreateDocument(ctx, doc); err != nil {
		return fmt.Errorf("SetSchema %s v%d: %w", schema.AgencyID, nextVersion, err)
	}
	return nil
}

// GetSchema returns the schema at the given version for the agency.
// Returns errSchemaNotFound if no matching document exists.
func (b *Backend) GetSchema(ctx context.Context, agencyID string, version int) (types.Schema, error) {
	q := fmt.Sprintf(
		"FOR doc IN %s FILTER doc.agency_id == @agencyID AND doc.version == @version LIMIT 1 RETURN doc",
		colSchemas,
	)
	bindVars := map[string]interface{}{
		"agencyID": agencyID,
		"version":  version,
	}
	cursor, err := b.db.Query(ctx, q, bindVars)
	if err != nil {
		return types.Schema{}, fmt.Errorf("GetSchema %s v%d: query: %w", agencyID, version, err)
	}
	defer cursor.Close()
	if !cursor.HasMore() {
		return types.Schema{}, fmt.Errorf("GetSchema %s v%d: %w", agencyID, version, errSchemaNotFound)
	}
	var doc schemaDoc
	meta, err := cursor.ReadDocument(ctx, &doc)
	if err != nil {
		return types.Schema{}, fmt.Errorf("GetSchema %s v%d: read: %w", agencyID, version, err)
	}
	return toSchema(doc, meta.Key), nil
}

// ListSchemaVersions returns all schema versions for the agency in ascending
// version order. Returns an empty slice if no schemas exist.
func (b *Backend) ListSchemaVersions(ctx context.Context, agencyID string) ([]types.Schema, error) {
	q := fmt.Sprintf(
		"FOR doc IN %s FILTER doc.agency_id == @agencyID SORT doc.version ASC RETURN doc",
		colSchemas,
	)
	bindVars := map[string]interface{}{
		"agencyID": agencyID,
	}
	cursor, err := b.db.Query(ctx, q, bindVars)
	if err != nil {
		return nil, fmt.Errorf("ListSchemaVersions %s: query: %w", agencyID, err)
	}
	var schemas []types.Schema
	var readErr error
	for cursor.HasMore() {
		var doc schemaDoc
		meta, rErr := cursor.ReadDocument(ctx, &doc)
		if rErr != nil {
			readErr = fmt.Errorf("ListSchemaVersions %s: read: %w", agencyID, rErr)
			break
		}
		schemas = append(schemas, toSchema(doc, meta.Key))
	}
	cursor.Close()
	if readErr != nil {
		return nil, readErr
	}
	return schemas, nil
}

// nextSchemaVersion returns max(existing version)+1 for the agency, or 1 if no
// schemas exist yet.
func (b *Backend) nextSchemaVersion(ctx context.Context, agencyID string) (int, error) {
	q := fmt.Sprintf(
		"FOR doc IN %s FILTER doc.agency_id == @agencyID SORT doc.version DESC LIMIT 1 RETURN doc.version",
		colSchemas,
	)
	bindVars := map[string]interface{}{
		"agencyID": agencyID,
	}
	cursor, err := b.db.Query(ctx, q, bindVars)
	if err != nil {
		return 0, fmt.Errorf("nextSchemaVersion: query: %w", err)
	}
	defer cursor.Close()
	if !cursor.HasMore() {
		return 1, nil
	}
	var maxVersion int
	if _, err := cursor.ReadDocument(ctx, &maxVersion); err != nil {
		if driver.IsNotFound(err) {
			return 1, nil
		}
		return 0, fmt.Errorf("nextSchemaVersion: read: %w", err)
	}
	return maxVersion + 1, nil
}

// toSchema converts a schemaDoc and its ArangoDB _key to a [types.Schema].
func toSchema(doc schemaDoc, key string) types.Schema {
	s := types.Schema{
		ID:        key,
		AgencyID:  doc.AgencyID,
		Version:   doc.Version,
		Tag:       doc.Tag,
		Types:     doc.Types,
		CreatedAt: doc.CreatedAt,
	}
	if s.Types == nil {
		s.Types = []types.TypeDefinition{}
	}
	return s
}
