// Package arangodb implements the ArangoDB backend for CodeValdAgency.
// All implementation logic lives in
// [github.com/aosanya/CodeValdSharedLib/entitygraph/arangodb]; this package
// is a thin service-scoped adapter that fixes the collection and graph names
// to their Agency-specific values (agency_entities, agency_relationships,
// agency_schemas_draft, agency_schemas_published, agency_graph).
//
// Use [New] to obtain a (DataManager, SchemaManager) pair from an open database.
// Use [NewBackend] to connect and construct in a single call.
// Use [NewBackendFromDB] in tests that manage their own database lifecycle.
package arangodb

import (
	"context"
	"fmt"
	"time"

	driver "github.com/arangodb/go-driver"

	"github.com/aosanya/CodeValdSharedLib/arangoutil"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	"github.com/aosanya/CodeValdSharedLib/types"
)

// Fixed infrastructure collection names — not schema-driven.
const (
	colEntities         = "agency_entities" // fallback for TypeIDs with no StorageCollection
	colRelationships    = "agency_relationships"
	colSchemasDraft     = "agency_schemas_draft"
	colSchemasPublished = "agency_schemas_published"
	graphName           = "agency_graph"
)

// Config holds the connection parameters for the ArangoDB backend.
type Config struct {
	// Endpoint is the ArangoDB HTTP endpoint (e.g. "http://localhost:8529").
	Endpoint string

	// Username is the ArangoDB username (default "root").
	Username string

	// Password is the ArangoDB password.
	Password string

	// Database is the ArangoDB database name (default "codevaldagency").
	Database string

	// Schema is the agency schema. Collection names and immutability are derived
	// from TypeDefinition.StorageCollection and TypeDefinition.Immutable.
	// Pass DefaultAgencySchema() in production; a custom schema in tests.
	Schema types.Schema
}

// Backend is the ArangoDB implementation of both [entitygraph.DataManager] and
// [entitygraph.SchemaManager] for CodeValdAgency. It is obtained via [New],
// [NewBackend], or [NewBackendFromDB].
//
// entityColMap maps collection name → driver.Collection for every collection
// referenced by the schema TypeDefinitions plus the fallback colEntities.
// typeDefs maps TypeID → TypeDefinition for O(1) immutability and
// StorageCollection lookups.
type Backend struct {
	db               driver.Database
	entityColMap     map[string]driver.Collection    // collection name → driver.Collection
	typeDefs         map[string]types.TypeDefinition // TypeID → TypeDefinition
	fallback         driver.Collection               // agency_entities
	relationships    driver.Collection
	schemasDraft     driver.Collection
	schemasPublished driver.Collection
}

// collectionFor returns the driver.Collection for the given TypeID,
// falling back to agency_entities when StorageCollection is empty.
func (b *Backend) collectionFor(typeID string) driver.Collection {
	if td, ok := b.typeDefs[typeID]; ok && td.StorageCollection != "" {
		if col, ok := b.entityColMap[td.StorageCollection]; ok {
			return col
		}
	}
	return b.fallback
}

// isImmutable returns true when the TypeDefinition for typeID has Immutable set.
func (b *Backend) isImmutable(typeID string) bool {
	if td, ok := b.typeDefs[typeID]; ok {
		return td.Immutable
	}
	return false
}

// allEntityCollections returns every distinct entity collection derived from the
// schema — used for graph vertex lists and cross-collection searches.
func (b *Backend) allEntityCollections() []driver.Collection {
	seen := make(map[string]struct{})
	var cols []driver.Collection
	for _, td := range b.typeDefs {
		name := td.StorageCollection
		if name == "" {
			name = colEntities
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		if col, ok := b.entityColMap[name]; ok {
			cols = append(cols, col)
		}
	}
	// Always include the fallback even if no TypeID maps to it explicitly.
	if _, ok := seen[colEntities]; !ok {
		cols = append(cols, b.fallback)
	}
	return cols
}

// New constructs a Backend from an already-open driver.Database using the
// provided schema, ensures all collections and the named graph exist, and
// returns the Backend as both a DataManager and a SchemaManager.
func New(db driver.Database, schema types.Schema) (entitygraph.DataManager, entitygraph.SchemaManager, error) {
	b, err := newBackendFromDB(context.Background(), db, schema)
	if err != nil {
		return nil, nil, err
	}
	return b, b, nil
}

// NewBackend connects to ArangoDB using cfg, ensures all collections exist, and
// returns a ready-to-use Backend. cfg.Schema drives which entity collections
// are created.
func NewBackend(cfg Config) (*Backend, error) {
	if cfg.Endpoint == "" {
		cfg.Endpoint = "http://localhost:8529"
	}
	if cfg.Username == "" {
		cfg.Username = "root"
	}
	if cfg.Database == "" {
		cfg.Database = "codevaldagency"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := arangoutil.Connect(ctx, arangoutil.Config{
		Endpoint: cfg.Endpoint,
		Username: cfg.Username,
		Password: cfg.Password,
		Database: cfg.Database,
	})
	if err != nil {
		return nil, fmt.Errorf("arangodb: %w", err)
	}

	return newBackendFromDB(ctx, db, cfg.Schema)
}

// NewBackendFromDB constructs a Backend from an already-open driver.Database
// using the provided schema. Intended for tests that manage their own database
// lifecycle.
func NewBackendFromDB(db driver.Database, schema types.Schema) (*Backend, error) {
	if db == nil {
		return nil, fmt.Errorf("arangodb: NewBackendFromDB: database must not be nil")
	}
	return newBackendFromDB(context.Background(), db, schema)
}

func newBackendFromDB(ctx context.Context, db driver.Database, schema types.Schema) (*Backend, error) {
	// Build typeDefs index for O(1) lookups.
	typeDefs := make(map[string]types.TypeDefinition, len(schema.Types))
	for _, td := range schema.Types {
		typeDefs[td.Name] = td
	}

	// Collect unique StorageCollection names from the schema.
	colNames := make(map[string]struct{})
	for _, td := range schema.Types {
		if td.StorageCollection != "" {
			colNames[td.StorageCollection] = struct{}{}
		}
	}
	// Always ensure the fallback collection exists even if nothing maps to it.
	colNames[colEntities] = struct{}{}

	// Ensure every entity collection exists in ArangoDB.
	entityColMap := make(map[string]driver.Collection, len(colNames))
	for name := range colNames {
		col, err := ensureDocumentCollection(ctx, db, name)
		if err != nil {
			return nil, fmt.Errorf("ensure entity collection %q: %w", name, err)
		}
		entityColMap[name] = col
	}

	// Ensure infrastructure collections.
	relationships, err := ensureEdgeCollection(ctx, db, colRelationships)
	if err != nil {
		return nil, fmt.Errorf("ensure %q: %w", colRelationships, err)
	}
	schemasDraft, err := ensureDocumentCollection(ctx, db, colSchemasDraft)
	if err != nil {
		return nil, fmt.Errorf("ensure %q: %w", colSchemasDraft, err)
	}
	schemasPublished, err := ensureDocumentCollection(ctx, db, colSchemasPublished)
	if err != nil {
		return nil, fmt.Errorf("ensure %q: %w", colSchemasPublished, err)
	}

	b := &Backend{
		db:               db,
		entityColMap:     entityColMap,
		typeDefs:         typeDefs,
		fallback:         entityColMap[colEntities],
		relationships:    relationships,
		schemasDraft:     schemasDraft,
		schemasPublished: schemasPublished,
	}

	if err := ensureGraph(ctx, b); err != nil {
		return nil, err
	}
	return b, nil
}

// ensureGraph creates the named ArangoDB graph agency_graph if it does not
// already exist. Vertex collections are derived from the backend schema.
func ensureGraph(ctx context.Context, b *Backend) error {
	exists, err := b.db.GraphExists(ctx, graphName)
	if err != nil {
		return fmt.Errorf("ensureGraph: check exists: %w", err)
	}
	if exists {
		return nil
	}
	all := b.allEntityCollections()
	names := make([]string, len(all))
	for i, col := range all {
		names[i] = col.Name()
	}
	_, err = b.db.CreateGraph(ctx, graphName, &driver.CreateGraphOptions{
		EdgeDefinitions: []driver.EdgeDefinition{
			{
				Collection: colRelationships,
				From:       names,
				To:         names,
			},
		},
	})
	if err != nil && !driver.IsConflict(err) {
		return fmt.Errorf("ensureGraph: create: %w", err)
	}
	return nil
}

func ensureDocumentCollection(ctx context.Context, db driver.Database, name string) (driver.Collection, error) {
	exists, err := db.CollectionExists(ctx, name)
	if err != nil {
		return nil, err
	}
	if exists {
		return db.Collection(ctx, name)
	}
	col, err := db.CreateCollection(ctx, name, nil)
	if err != nil {
		if driver.IsConflict(err) {
			return db.Collection(ctx, name)
		}
		return nil, err
	}
	return col, nil
}

func ensureEdgeCollection(ctx context.Context, db driver.Database, name string) (driver.Collection, error) {
	exists, err := db.CollectionExists(ctx, name)
	if err != nil {
		return nil, err
	}
	if exists {
		return db.Collection(ctx, name)
	}
	col, err := db.CreateCollection(ctx, name, &driver.CreateCollectionOptions{
		Type: driver.CollectionTypeEdge,
	})
	if err != nil {
		if driver.IsConflict(err) {
			return db.Collection(ctx, name)
		}
		return nil, err
	}
	return col, nil
}
