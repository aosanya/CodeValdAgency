// Package arangodb implements the codevaldagency.Backend interface backed by
// ArangoDB. Agency documents are stored in the `agency_details` collection,
// activation snapshots in `agency_snapshots`, and publications in
// `agency_publications`.
//
// Use [NewBackend] to construct; pass the result to
// codevaldagency.NewAgencyManager.
//
// File layout:
//   - storage.go â Config, Backend struct, constructors, collection setup
//   - docs.go    â ArangoDB document types and domainâdocument conversions
//   - ops.go     â Backend interface method implementations
package arangodb

import (
	"context"
	"fmt"
	"time"

	driver "github.com/arangodb/go-driver"

	"github.com/aosanya/CodeValdSharedLib/arangoutil"
)

const (
	colAgencies     = "agency_details"
	colSnapshots    = "agency_snapshots"
	colPublications = "agency_publications"
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

// Backend is the ArangoDB implementation of [codevaldagency.Backend].
type Backend struct {
	db            driver.Database
	agencyDetails driver.Collection
	snapshots     driver.Collection
	publications  driver.Collection
}

// NewBackend connects to ArangoDB, ensures all collections exist, and returns
// a ready-to-use [Backend].
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
	agencyDetails, err := ensureCollection(ctx, db, colAgencies)
	if err != nil {
		return nil, fmt.Errorf("arangodb: ensure %q: %w", colAgencies, err)
	}

	snapshots, err := ensureCollection(ctx, db, colSnapshots)
	if err != nil {
		return nil, fmt.Errorf("arangodb: ensure %q: %w", colSnapshots, err)
	}

	publications, err := ensureCollection(ctx, db, colPublications)
	if err != nil {
		return nil, fmt.Errorf("arangodb: ensure %q: %w", colPublications, err)
	}

	return &Backend{
		db:            db,
		agencyDetails: agencyDetails,
		snapshots:     snapshots,
		publications:  publications,
	}, nil
}

func ensureCollection(ctx context.Context, db driver.Database, name string) (driver.Collection, error) {
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
