// Package arangodb_test provides integration tests for the ArangoDB backend.
//
// Tests in this file require a running ArangoDB instance. They connect to a
// single persistent database (AGENCY_ARANGO_DATABASE_TEST, default
// "codevald_tests") and use unique agency IDs per test for isolation.
//
// Tests are skipped automatically when AGENCY_ARANGO_ENDPOINT is not set or
// the server is unreachable.
//
// To run:
//
//	AGENCY_ARANGO_ENDPOINT=http://localhost:8529 go test -v -race ./storage/arangodb/
package arangodb_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	driver "github.com/arangodb/go-driver"
	driverhttp "github.com/arangodb/go-driver/http"

	"github.com/aosanya/CodeValdAgency/storage/arangodb"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	"github.com/aosanya/CodeValdSharedLib/types"
)

// openTestBackend connects to the ArangoDB instance at AGENCY_ARANGO_ENDPOINT
// (default http://localhost:8529) and opens AGENCY_ARANGO_DATABASE_TEST
// (default "codevald_tests"). Skips the test if the server is unreachable.
func openTestBackend(t *testing.T) (*arangodb.Backend, driver.Database) {
	t.Helper()

	endpoint := envOrDefault("AGENCY_ARANGO_ENDPOINT", "")
	if endpoint == "" {
		t.Skip("AGENCY_ARANGO_ENDPOINT not set — skipping ArangoDB integration tests")
	}

	conn, err := driverhttp.NewConnection(driverhttp.ConnectionConfig{
		Endpoints: []string{endpoint},
	})
	if err != nil {
		t.Skipf("ArangoDB connection config error (AGENCY_ARANGO_ENDPOINT=%s): %v", endpoint, err)
	}

	user := envOrDefault("AGENCY_ARANGO_USER", "root")
	pass := os.Getenv("AGENCY_ARANGO_PASSWORD")

	client, err := driver.NewClient(driver.ClientConfig{
		Connection:     conn,
		Authentication: driver.BasicAuthentication(user, pass),
	})
	if err != nil {
		t.Skipf("ArangoDB client error: %v", err)
	}

	// Quick ping — skip if unreachable (CI without ArangoDB).
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := client.Version(ctx); err != nil {
		t.Skipf("ArangoDB unreachable at %s: %v", endpoint, err)
	}

	dbName := envOrDefault("AGENCY_ARANGO_DATABASE_TEST", "codevald_tests")
	ctx2 := context.Background()
	exists, err := client.DatabaseExists(ctx2, dbName)
	if err != nil {
		t.Fatalf("DatabaseExists: %v", err)
	}
	var db driver.Database
	if exists {
		db, err = client.Database(ctx2, dbName)
	} else {
		db, err = client.CreateDatabase(ctx2, dbName, nil)
	}
	if err != nil {
		t.Fatalf("open/create test database %q: %v", dbName, err)
	}

	b, err := arangodb.NewBackendFromDB(db)
	if err != nil {
		t.Fatalf("NewBackendFromDB: %v", err)
	}
	return b, db
}

// uniqueID returns a string that is unique within the current test run.
func uniqueID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ── CreateEntity / GetEntity round-trip ──────────────────────────────────────

func TestArangoDB_CreateEntity_RoundTrip(t *testing.T) {
	b, _ := openTestBackend(t)
	ctx := context.Background()

	agencyID := uniqueID("agency")
	req := entitygraph.CreateEntityRequest{
		AgencyID:   agencyID,
		TypeID:     "Agency",
		Properties: map[string]any{"name": "Acme Corp"},
	}

	created, err := b.CreateEntity(ctx, req)
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty entity ID")
	}
	if created.TypeID != "Agency" {
		t.Errorf("TypeID: want %q, got %q", "Agency", created.TypeID)
	}
	if created.AgencyID != agencyID {
		t.Errorf("AgencyID: want %q, got %q", agencyID, created.AgencyID)
	}

	got, err := b.GetEntity(ctx, agencyID, created.ID)
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID mismatch: want %q, got %q", created.ID, got.ID)
	}
	if got.Properties["name"] != "Acme Corp" {
		t.Errorf("name property: want %q, got %v", "Acme Corp", got.Properties["name"])
	}
}

// ── UpdateEntity patches properties ──────────────────────────────────────────

func TestArangoDB_UpdateEntity_PatchesProperties(t *testing.T) {
	b, _ := openTestBackend(t)
	ctx := context.Background()

	agencyID := uniqueID("agency")
	created, err := b.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID:   agencyID,
		TypeID:     "Agency",
		Properties: map[string]any{"name": "Old Name"},
	})
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}

	updated, err := b.UpdateEntity(ctx, agencyID, created.ID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{"name": "New Name", "status": "active"},
	})
	if err != nil {
		t.Fatalf("UpdateEntity: %v", err)
	}
	if updated.Properties["name"] != "New Name" {
		t.Errorf("name: want %q, got %v", "New Name", updated.Properties["name"])
	}
	if updated.Properties["status"] != "active" {
		t.Errorf("status: want %q, got %v", "active", updated.Properties["status"])
	}
}

// ── UpdateEntity on immutable type returns an error ───────────────────────────

func TestArangoDB_UpdateEntity_ImmutableType_ReturnsError(t *testing.T) {
	b, _ := openTestBackend(t)
	ctx := context.Background()

	agencyID := uniqueID("agency")
	created, err := b.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID:   agencyID,
		TypeID:     "AgencySnapshot",
		Properties: map[string]any{"tag": "v1"},
	})
	if err != nil {
		t.Fatalf("CreateEntity (snapshot): %v", err)
	}

	_, err = b.UpdateEntity(ctx, agencyID, created.ID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{"tag": "v2"},
	})
	if err == nil {
		t.Fatal("expected error updating immutable AgencySnapshot, got nil")
	}
}

// ── DeleteEntity soft-deletes ─────────────────────────────────────────────────

func TestArangoDB_DeleteEntity_SoftDelete(t *testing.T) {
	b, _ := openTestBackend(t)
	ctx := context.Background()

	agencyID := uniqueID("agency")
	created, err := b.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: agencyID,
		TypeID:   "Agency",
	})
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}

	if err := b.DeleteEntity(ctx, agencyID, created.ID); err != nil {
		t.Fatalf("DeleteEntity: %v", err)
	}

	// Soft-deleted entities should not appear in ListEntities.
	entities, err := b.ListEntities(ctx, entitygraph.EntityFilter{AgencyID: agencyID})
	if err != nil {
		t.Fatalf("ListEntities: %v", err)
	}
	for _, e := range entities {
		if e.ID == created.ID {
			t.Errorf("expected deleted entity %q to be excluded from ListEntities", created.ID)
		}
	}
}

// ── ListEntities filters correctly ────────────────────────────────────────────

func TestArangoDB_ListEntities_FilterByTypeID(t *testing.T) {
	b, _ := openTestBackend(t)
	ctx := context.Background()

	agencyID := uniqueID("agency")
	if _, err := b.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID:   agencyID,
		TypeID:     "Agency",
		Properties: map[string]any{"name": "A"},
	}); err != nil {
		t.Fatalf("CreateEntity Agency: %v", err)
	}
	if _, err := b.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID:   agencyID,
		TypeID:     "Goal",
		Properties: map[string]any{"title": "G"},
	}); err != nil {
		t.Fatalf("CreateEntity Goal: %v", err)
	}

	goals, err := b.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: agencyID,
		TypeID:   "Goal",
	})
	if err != nil {
		t.Fatalf("ListEntities Goal: %v", err)
	}
	if len(goals) != 1 {
		t.Errorf("expected 1 Goal entity, got %d", len(goals))
	}
	if len(goals) > 0 && goals[0].TypeID != "Goal" {
		t.Errorf("TypeID: want %q, got %q", "Goal", goals[0].TypeID)
	}
}

// ── CreateRelationship / GetRelationship round-trip ──────────────────────────

func TestArangoDB_CreateRelationship_RoundTrip(t *testing.T) {
	b, _ := openTestBackend(t)
	ctx := context.Background()

	agencyID := uniqueID("agency")
	from, err := b.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: agencyID,
		TypeID:   "Agency",
	})
	if err != nil {
		t.Fatalf("CreateEntity from: %v", err)
	}
	to, err := b.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: agencyID,
		TypeID:   "Goal",
	})
	if err != nil {
		t.Fatalf("CreateEntity to: %v", err)
	}

	rel, err := b.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID:   agencyID,
		FromID:     from.ID,
		ToID:       to.ID,
		Name:       "has_goal",
		Properties: map[string]any{"priority": "high"},
	})
	if err != nil {
		t.Fatalf("CreateRelationship: %v", err)
	}
	if rel.ID == "" {
		t.Fatal("expected non-empty relationship ID")
	}
	if rel.Name != "has_goal" {
		t.Errorf("Name: want %q, got %q", "has_goal", rel.Name)
	}
	if rel.FromID != from.ID {
		t.Errorf("FromID: want %q, got %q", from.ID, rel.FromID)
	}
	if rel.ToID != to.ID {
		t.Errorf("ToID: want %q, got %q", to.ID, rel.ToID)
	}

	got, err := b.GetRelationship(ctx, agencyID, rel.ID)
	if err != nil {
		t.Fatalf("GetRelationship: %v", err)
	}
	if got.ID != rel.ID {
		t.Errorf("ID mismatch: want %q, got %q", rel.ID, got.ID)
	}
}

// ── DeleteRelationship removes the edge ──────────────────────────────────────

func TestArangoDB_DeleteRelationship(t *testing.T) {
	b, _ := openTestBackend(t)
	ctx := context.Background()

	agencyID := uniqueID("agency")
	from, _ := b.CreateEntity(ctx, entitygraph.CreateEntityRequest{AgencyID: agencyID, TypeID: "Agency"})
	to, _ := b.CreateEntity(ctx, entitygraph.CreateEntityRequest{AgencyID: agencyID, TypeID: "Goal"})
	rel, err := b.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: agencyID,
		FromID:   from.ID,
		ToID:     to.ID,
		Name:     "has_goal",
	})
	if err != nil {
		t.Fatalf("CreateRelationship: %v", err)
	}

	if err := b.DeleteRelationship(ctx, agencyID, rel.ID); err != nil {
		t.Fatalf("DeleteRelationship: %v", err)
	}

	_, err = b.GetRelationship(ctx, agencyID, rel.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

// ── ListRelationships filters by agency ──────────────────────────────────────

func TestArangoDB_ListRelationships_ByAgency(t *testing.T) {
	b, _ := openTestBackend(t)
	ctx := context.Background()

	agencyID := uniqueID("agency")
	from, _ := b.CreateEntity(ctx, entitygraph.CreateEntityRequest{AgencyID: agencyID, TypeID: "Agency"})
	to, _ := b.CreateEntity(ctx, entitygraph.CreateEntityRequest{AgencyID: agencyID, TypeID: "Goal"})
	if _, err := b.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: agencyID,
		FromID:   from.ID,
		ToID:     to.ID,
		Name:     "has_goal",
	}); err != nil {
		t.Fatalf("CreateRelationship: %v", err)
	}

	rels, err := b.ListRelationships(ctx, entitygraph.RelationshipFilter{AgencyID: agencyID})
	if err != nil {
		t.Fatalf("ListRelationships: %v", err)
	}
	if len(rels) == 0 {
		t.Error("expected at least one relationship")
	}
	for _, r := range rels {
		if r.AgencyID != agencyID {
			t.Errorf("unexpected agencyID %q in results", r.AgencyID)
		}
	}
}

// ── SetSchema / GetSchema round-trip ─────────────────────────────────────────

func TestArangoDB_SetSchema_GetSchema_RoundTrip(t *testing.T) {
	b, _ := openTestBackend(t)
	ctx := context.Background()

	agencyID := uniqueID("agency")
	schema := types.Schema{
		AgencyID: agencyID,
		Tag:      "v1",
		Types: []types.TypeDefinition{
			{
				Name: "Agency",
				Properties: []types.PropertyDefinition{
					{Name: "name", Type: types.PropertyTypeString},
				},
			},
		},
	}

	if err := b.SetSchema(ctx, schema); err != nil {
		t.Fatalf("SetSchema: %v", err)
	}

	got, err := b.GetSchema(ctx, agencyID, 1)
	if err != nil {
		t.Fatalf("GetSchema v1: %v", err)
	}
	if got.Version != 1 {
		t.Errorf("Version: want 1, got %d", got.Version)
	}
	if got.Tag != "v1" {
		t.Errorf("Tag: want %q, got %q", "v1", got.Tag)
	}
	if len(got.Types) != 1 {
		t.Errorf("Types: want 1 type, got %d", len(got.Types))
	}
}

// ── SetSchema increments version on second call ───────────────────────────────

func TestArangoDB_SetSchema_IncrementsVersion(t *testing.T) {
	b, _ := openTestBackend(t)
	ctx := context.Background()

	agencyID := uniqueID("agency")
	base := types.Schema{AgencyID: agencyID}

	if err := b.SetSchema(ctx, base); err != nil {
		t.Fatalf("first SetSchema: %v", err)
	}
	if err := b.SetSchema(ctx, base); err != nil {
		t.Fatalf("second SetSchema: %v", err)
	}

	versions, err := b.ListSchemaVersions(ctx, agencyID)
	if err != nil {
		t.Fatalf("ListSchemaVersions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	if versions[0].Version != 1 {
		t.Errorf("first version: want 1, got %d", versions[0].Version)
	}
	if versions[1].Version != 2 {
		t.Errorf("second version: want 2, got %d", versions[1].Version)
	}
}

// ── GetSchema not found returns an error ──────────────────────────────────────

func TestArangoDB_GetSchema_NotFound(t *testing.T) {
	b, _ := openTestBackend(t)
	ctx := context.Background()

	agencyID := uniqueID("agency")
	_, err := b.GetSchema(ctx, agencyID, 99)
	if err == nil {
		t.Fatal("expected error for missing schema, got nil")
	}
}

// ── TraverseGraph returns reachable vertices ──────────────────────────────────

func TestArangoDB_TraverseGraph_ReturnsVertices(t *testing.T) {
	b, _ := openTestBackend(t)
	ctx := context.Background()

	agencyID := uniqueID("agency")
	agency, err := b.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: agencyID,
		TypeID:   "Agency",
	})
	if err != nil {
		t.Fatalf("CreateEntity agency: %v", err)
	}
	goal, err := b.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: agencyID,
		TypeID:   "Goal",
	})
	if err != nil {
		t.Fatalf("CreateEntity goal: %v", err)
	}
	if _, err := b.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: agencyID,
		FromID:   agency.ID,
		ToID:     goal.ID,
		Name:     "has_goal",
	}); err != nil {
		t.Fatalf("CreateRelationship: %v", err)
	}

	result, err := b.TraverseGraph(ctx, entitygraph.TraverseGraphRequest{
		AgencyID:  agencyID,
		StartID:   agency.ID,
		Direction: "OUTBOUND",
		Depth:     1,
	})
	if err != nil {
		t.Fatalf("TraverseGraph: %v", err)
	}
	found := false
	for _, v := range result.Vertices {
		if v.ID == goal.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected goal %q in traversal result, got %v", goal.ID, result.Vertices)
	}
}
