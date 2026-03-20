// Package arangodb implements the [entitygraph.DataManager] and
// [entitygraph.SchemaManager] interfaces for CodeValdAgency, backed by
// ArangoDB.
//
// All agency entity data is stored in six collections:
//   - agency_entities          — mutable entity documents (Agency, Goal, Workflow, WorkItem, ConfiguredRole)
//   - agency_relationships     — directed graph edges (ArangoDB edge collection)
//   - agency_schemas_draft     — mutable draft schema (one document per agency, keyed by agencyID)
//   - agency_schemas_published — immutable published snapshots (append-only; one Active per agency)
//   - agency_snapshots         — immutable AgencySnapshot entities
//   - agency_publications      — immutable AgencyPublication entities
//
// File layout:
//   - storage.go       — Config, Backend struct, constructors, collection setup
//   - entities.go      — CreateEntity, GetEntity, UpdateEntity, DeleteEntity, ListEntities
//   - relationships.go — CreateRelationship, GetRelationship, DeleteRelationship,
//     ListRelationships, TraverseGraph
//   - schemaops.go     — SetSchema, GetSchema, Publish, Activate, GetActive, GetVersion, ListVersions
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
)

// Collection name constants.
const (
	colEntities         = "agency_entities"
	colRelationships    = "agency_relationships"
	colSchemasDraft     = "agency_schemas_draft"
	colSchemasPublished = "agency_schemas_published"
	colSnapshots        = "agency_snapshots"
	colPublications     = "agency_publications"
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
}

// Backend is the ArangoDB implementation of both [entitygraph.DataManager] and
// [entitygraph.SchemaManager] for CodeValdAgency. It is obtained via [New],
// [NewBackend], or [NewBackendFromDB].
type Backend struct {
	db               driver.Database
	entities         driver.Collection
	relationships    driver.Collection
	schemasDraft     driver.Collection
	schemasPublished driver.Collection
	snapshots        driver.Collection
	publications     driver.Collection
}

// New constructs a [Backend] from an already-open [driver.Database], ensures
// all collections and the named graph exist, and returns the Backend as both a
// [entitygraph.DataManager] and a [entitygraph.SchemaManager].
//
// This is the primary constructor for use in cmd/main.go:
//
// dm, sm, err := arangodb.New(db)
// mgr := codevaldagency.NewAgencyManager(dm, sm, publisher, agencyID)
func New(db driver.Database) (entitygraph.DataManager, entitygraph.SchemaManager, error) {
	b, err := newBackendFromDB(context.Background(), db)
	if err != nil {
		return nil, nil, err
	}
	return b, b, nil
}

// NewBackend connects to ArangoDB using cfg, ensures all collections exist, and
// returns a ready-to-use [Backend].
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

	return newBackendFromDB(ctx, db)
}

// NewBackendFromDB constructs a [Backend] from an already-open [driver.Database].
// Intended for tests that manage their own database lifecycle.
func NewBackendFromDB(db driver.Database) (*Backend, error) {
	if db == nil {
		return nil, fmt.Errorf("arangodb: NewBackendFromDB: database must not be nil")
	}
	return newBackendFromDB(context.Background(), db)
}

func newBackendFromDB(ctx context.Context, db driver.Database) (*Backend, error) {
	entities, relationships, schemasDraft, schemasPublished, snapshots, publications, err := ensureCollections(ctx, db)
	if err != nil {
		return nil, err
	}
	if err := ensureGraph(ctx, db); err != nil {
		return nil, err
	}
	return &Backend{
		db:               db,
		entities:         entities,
		relationships:    relationships,
		schemasDraft:     schemasDraft,
		schemasPublished: schemasPublished,
		snapshots:        snapshots,
		publications:     publications,
	}, nil
}

// ensureCollections creates or opens all six required collections.
// agency_relationships is created as an ArangoDB edge collection.
func ensureCollections(ctx context.Context, db driver.Database) (
	entities, relationships, schemasDraft, schemasPublished, snapshots, publications driver.Collection, err error,
) {
	entities, err = ensureDocumentCollection(ctx, db, colEntities)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("ensure %q: %w", colEntities, err)
	}
	relationships, err = ensureEdgeCollection(ctx, db, colRelationships)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("ensure %q: %w", colRelationships, err)
	}
	schemasDraft, err = ensureDocumentCollection(ctx, db, colSchemasDraft)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("ensure %q: %w", colSchemasDraft, err)
	}
	schemasPublished, err = ensureDocumentCollection(ctx, db, colSchemasPublished)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("ensure %q: %w", colSchemasPublished, err)
	}
	snapshots, err = ensureDocumentCollection(ctx, db, colSnapshots)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("ensure %q: %w", colSnapshots, err)
	}
	publications, err = ensureDocumentCollection(ctx, db, colPublications)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("ensure %q: %w", colPublications, err)
	}
	return entities, relationships, schemasDraft, schemasPublished, snapshots, publications, nil
}

// ensureGraph creates the named ArangoDB graph agency_graph if it does not
// already exist.
func ensureGraph(ctx context.Context, db driver.Database) error {
	exists, err := db.GraphExists(ctx, graphName)
	if err != nil {
		return fmt.Errorf("ensureGraph: check exists: %w", err)
	}
	if exists {
		return nil
	}
	_, err = db.CreateGraph(ctx, graphName, &driver.CreateGraphOptions{
		EdgeDefinitions: []driver.EdgeDefinition{
			{
				Collection: colRelationships,
				From:       []string{colEntities, colSnapshots, colPublications},
				To:         []string{colEntities, colSnapshots, colPublications},
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
