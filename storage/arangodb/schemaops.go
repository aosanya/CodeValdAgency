// schemaops.go contains the SchemaManager implementation for Backend,
// providing draft/published schema lifecycle for CodeValdAgency.
//
// Two ArangoDB collections back this implementation:
//
//   - agency_schemas_draft     — one mutable document per agency, keyed by agencyID.
//   - agency_schemas_published — immutable append-only snapshots; exactly one
//     Active==true document per agency at a time.
//
// Workflow: SetSchema (update draft) → Publish (snapshot to published, version N)
// → Activate (promote version N to active). Only the active published version is
// used by CreateEntity / CreateRelationship at runtime.
package arangodb

import (
"context"
"fmt"
"time"

driver "github.com/arangodb/go-driver"
"github.com/google/uuid"

"github.com/aosanya/CodeValdSharedLib/entitygraph"
"github.com/aosanya/CodeValdSharedLib/types"
)

// draftDoc is the document shape for agency_schemas_draft.
// The _key equals agencyID — at most one draft document exists per agency.
type draftDoc struct {
Key       string                 `json:"_key,omitempty"`
AgencyID  string                 `json:"agency_id"`
Tag       string                 `json:"tag"`
Types     []types.TypeDefinition `json:"types"`
UpdatedAt time.Time              `json:"updated_at"`
}

// publishedDoc is the document shape for agency_schemas_published.
// The _key is a UUID; Active is true for the single live version per agency.
type publishedDoc struct {
Key       string                 `json:"_key,omitempty"`
AgencyID  string                 `json:"agency_id"`
Version   int                    `json:"version"`
Tag       string                 `json:"tag"`
Types     []types.TypeDefinition `json:"types"`
Active    bool                   `json:"active"`
CreatedAt time.Time              `json:"created_at"`
}

// SetSchema overwrites the agency's current draft in agency_schemas_draft.
// The draft is keyed by agencyID — at most one draft exists per agency.
// ValidateSchema is NOT called here; invalid drafts are permitted until Publish.
func (b *Backend) SetSchema(ctx context.Context, schema types.Schema) error {
doc := draftDoc{
Key:       schema.AgencyID,
AgencyID:  schema.AgencyID,
Tag:       schema.Tag,
Types:     schema.Types,
UpdatedAt: time.Now().UTC(),
}
exists, err := b.schemasDraft.DocumentExists(ctx, schema.AgencyID)
if err != nil {
return fmt.Errorf("SetSchema %s: check exists: %w", schema.AgencyID, err)
}
if exists {
if _, err := b.schemasDraft.ReplaceDocument(ctx, schema.AgencyID, doc); err != nil {
return fmt.Errorf("SetSchema %s: replace: %w", schema.AgencyID, err)
}
return nil
}
if _, err := b.schemasDraft.CreateDocument(ctx, doc); err != nil {
return fmt.Errorf("SetSchema %s: create: %w", schema.AgencyID, err)
}
return nil
}

// GetSchema returns the agency's current draft schema from agency_schemas_draft.
// Returns [entitygraph.ErrSchemaNotFound] if no draft has been created yet.
func (b *Backend) GetSchema(ctx context.Context, agencyID string) (types.Schema, error) {
var doc draftDoc
meta, err := b.schemasDraft.ReadDocument(ctx, agencyID, &doc)
if err != nil {
if driver.IsNotFound(err) {
return types.Schema{}, fmt.Errorf("GetSchema %s: %w", agencyID, entitygraph.ErrSchemaNotFound)
}
return types.Schema{}, fmt.Errorf("GetSchema %s: read: %w", agencyID, err)
}
return toDraftSchema(doc, meta.Key), nil
}

// Publish validates the current draft and snapshots it into agency_schemas_published
// as a new version with Active = false. The version number is max(existing)+1,
// starting at 1. Returns an error if ValidateSchema fails or no draft exists.
func (b *Backend) Publish(ctx context.Context, agencyID string) error {
draft, err := b.GetSchema(ctx, agencyID)
if err != nil {
return fmt.Errorf("Publish %s: get draft: %w", agencyID, err)
}
if err := entitygraph.ValidateSchema(draft); err != nil {
return fmt.Errorf("Publish %s: validate: %w", agencyID, err)
}
nextVer, err := b.nextPublishedVersion(ctx, agencyID)
if err != nil {
return fmt.Errorf("Publish %s: next version: %w", agencyID, err)
}
doc := publishedDoc{
Key:       uuid.NewString(),
AgencyID:  agencyID,
Version:   nextVer,
Tag:       draft.Tag,
Types:     draft.Types,
Active:    false,
CreatedAt: time.Now().UTC(),
}
if _, err := b.schemasPublished.CreateDocument(ctx, doc); err != nil {
return fmt.Errorf("Publish %s v%d: create: %w", agencyID, nextVer, err)
}
return nil
}

// Activate sets Active=true on the specified published version and Active=false
// on all others for the agency. Returns [entitygraph.ErrSchemaNotFound] if the
// version does not exist.
func (b *Backend) Activate(ctx context.Context, agencyID string, version int) error {
// Resolve the document key for the target version.
keyQ := fmt.Sprintf(
"FOR doc IN %s FILTER doc.agency_id == @agencyID AND doc.version == @version LIMIT 1 RETURN doc._key",
colSchemasPublished,
)
keyCursor, err := b.db.Query(ctx, keyQ, map[string]interface{}{
"agencyID": agencyID,
"version":  version,
})
if err != nil {
return fmt.Errorf("Activate %s v%d: query key: %w", agencyID, version, err)
}
var targetKey string
if keyCursor.HasMore() {
if _, err := keyCursor.ReadDocument(ctx, &targetKey); err != nil {
keyCursor.Close()
return fmt.Errorf("Activate %s v%d: read key: %w", agencyID, version, err)
}
}
keyCursor.Close()
if targetKey == "" {
return fmt.Errorf("Activate %s v%d: %w", agencyID, version, entitygraph.ErrSchemaNotFound)
}

// Deactivate all versions for this agency.
deactivateQ := fmt.Sprintf(
"FOR doc IN %s FILTER doc.agency_id == @agencyID UPDATE doc WITH { active: false } IN %s",
colSchemasPublished, colSchemasPublished,
)
deactivateCursor, err := b.db.Query(ctx, deactivateQ, map[string]interface{}{"agencyID": agencyID})
if err != nil {
return fmt.Errorf("Activate %s: deactivate all: %w", agencyID, err)
}
deactivateCursor.Close()

// Activate the target version.
activateQ := fmt.Sprintf(
"UPDATE { _key: @key } WITH { active: true } IN %s",
colSchemasPublished,
)
activateCursor, err := b.db.Query(ctx, activateQ, map[string]interface{}{"key": targetKey})
if err != nil {
return fmt.Errorf("Activate %s v%d: activate: %w", agencyID, version, err)
}
activateCursor.Close()
return nil
}

// GetActive returns the single published version where Active == true.
// Returns [entitygraph.ErrSchemaNotFound] if no version has been activated yet.
func (b *Backend) GetActive(ctx context.Context, agencyID string) (types.Schema, error) {
q := fmt.Sprintf(
"FOR doc IN %s FILTER doc.agency_id == @agencyID AND doc.active == true LIMIT 1 RETURN doc",
colSchemasPublished,
)
cursor, err := b.db.Query(ctx, q, map[string]interface{}{"agencyID": agencyID})
if err != nil {
return types.Schema{}, fmt.Errorf("GetActive %s: query: %w", agencyID, err)
}
defer cursor.Close()
if !cursor.HasMore() {
return types.Schema{}, fmt.Errorf("GetActive %s: %w", agencyID, entitygraph.ErrSchemaNotFound)
}
var doc publishedDoc
meta, err := cursor.ReadDocument(ctx, &doc)
if err != nil {
return types.Schema{}, fmt.Errorf("GetActive %s: read: %w", agencyID, err)
}
return toPublishedSchema(doc, meta.Key), nil
}

// GetVersion returns a specific published version.
// Returns [entitygraph.ErrSchemaNotFound] if the version does not exist.
func (b *Backend) GetVersion(ctx context.Context, agencyID string, version int) (types.Schema, error) {
q := fmt.Sprintf(
"FOR doc IN %s FILTER doc.agency_id == @agencyID AND doc.version == @version LIMIT 1 RETURN doc",
colSchemasPublished,
)
cursor, err := b.db.Query(ctx, q, map[string]interface{}{
"agencyID": agencyID,
"version":  version,
})
if err != nil {
return types.Schema{}, fmt.Errorf("GetVersion %s v%d: query: %w", agencyID, version, err)
}
defer cursor.Close()
if !cursor.HasMore() {
return types.Schema{}, fmt.Errorf("GetVersion %s v%d: %w", agencyID, version, entitygraph.ErrSchemaNotFound)
}
var doc publishedDoc
meta, err := cursor.ReadDocument(ctx, &doc)
if err != nil {
return types.Schema{}, fmt.Errorf("GetVersion %s v%d: read: %w", agencyID, version, err)
}
return toPublishedSchema(doc, meta.Key), nil
}

// ListVersions returns all published versions for the agency in ascending
// version order. Returns an empty slice if no versions have been published.
func (b *Backend) ListVersions(ctx context.Context, agencyID string) ([]types.Schema, error) {
q := fmt.Sprintf(
"FOR doc IN %s FILTER doc.agency_id == @agencyID SORT doc.version ASC RETURN doc",
colSchemasPublished,
)
cursor, err := b.db.Query(ctx, q, map[string]interface{}{"agencyID": agencyID})
if err != nil {
return nil, fmt.Errorf("ListVersions %s: query: %w", agencyID, err)
}
defer cursor.Close()
var schemas []types.Schema
for cursor.HasMore() {
var doc publishedDoc
meta, err := cursor.ReadDocument(ctx, &doc)
if err != nil {
return nil, fmt.Errorf("ListVersions %s: read: %w", agencyID, err)
}
schemas = append(schemas, toPublishedSchema(doc, meta.Key))
}
return schemas, nil
}

// nextPublishedVersion returns max(existing version)+1, or 1 if no published
// versions exist for the agency yet.
func (b *Backend) nextPublishedVersion(ctx context.Context, agencyID string) (int, error) {
q := fmt.Sprintf(
"FOR doc IN %s FILTER doc.agency_id == @agencyID SORT doc.version DESC LIMIT 1 RETURN doc.version",
colSchemasPublished,
)
cursor, err := b.db.Query(ctx, q, map[string]interface{}{"agencyID": agencyID})
if err != nil {
return 0, fmt.Errorf("nextPublishedVersion: query: %w", err)
}
defer cursor.Close()
if !cursor.HasMore() {
return 1, nil
}
var maxVer int
if _, err := cursor.ReadDocument(ctx, &maxVer); err != nil {
return 0, fmt.Errorf("nextPublishedVersion: read: %w", err)
}
return maxVer + 1, nil
}

// toDraftSchema converts a draftDoc and its ArangoDB _key to a [types.Schema].
// Draft schemas always have Version 0 and Active false.
func toDraftSchema(doc draftDoc, key string) types.Schema {
s := types.Schema{
ID:       key,
AgencyID: doc.AgencyID,
Tag:      doc.Tag,
Types:    doc.Types,
}
if s.Types == nil {
s.Types = []types.TypeDefinition{}
}
return s
}

// toPublishedSchema converts a publishedDoc and its ArangoDB _key to a [types.Schema].
func toPublishedSchema(doc publishedDoc, key string) types.Schema {
s := types.Schema{
ID:        key,
AgencyID:  doc.AgencyID,
Version:   doc.Version,
Tag:       doc.Tag,
Types:     doc.Types,
Active:    doc.Active,
CreatedAt: doc.CreatedAt,
}
if s.Types == nil {
s.Types = []types.TypeDefinition{}
}
return s
}
