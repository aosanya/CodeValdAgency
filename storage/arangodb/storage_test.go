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
//AGENCY_ARANGO_ENDPOINT=http://localhost:8529 go test -v -race ./storage/arangodb/
package arangodb_test

import (
"context"
"errors"
"fmt"
"os"
"testing"
"time"

driver "github.com/arangodb/go-driver"
driverhttp "github.com/arangodb/go-driver/http"

codevaldagency "github.com/aosanya/CodeValdAgency"
"github.com/aosanya/CodeValdAgency/storage/arangodb"
)

// openTestBackend connects to the ArangoDB instance at AGENCY_ARANGO_ENDPOINT
// (default http://localhost:8529) and opens AGENCY_ARANGO_DATABASE_TEST
// (default "codevald_tests"). Skips the test if the server is unreachable.
func openTestBackend(t *testing.T) *arangodb.Backend {
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
return b
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

// detailsJSON builds a minimal valid agency JSON payload for SetDetails.
func detailsJSON(id, name string) string {
return fmt.Sprintf(`{"id":%q,"name":%q,"status":"draft"}`, id, name)
}

// ── SetDetails -> Get round-trip ─────────────────────────────────────────────

func TestArangoDB_SetDetails_ValidJSON_RoundTrip(t *testing.T) {
b := openTestBackend(t)
ctx := context.Background()

id := uniqueID("rt")
name := "Alpha-" + id

created, err := b.SetDetails(ctx, detailsJSON(id, name))
if err != nil {
t.Fatalf("SetDetails: %v", err)
}
if created.ID != id {
t.Errorf("ID mismatch: want %q, got %q", id, created.ID)
}
if created.Status != codevaldagency.LifecycleDraft {
t.Errorf("expected status draft, got %q", created.Status)
}
if created.Name != name {
t.Errorf("name mismatch: want %q, got %q", name, created.Name)
}

got, err := b.Get(ctx, id)
if err != nil {
t.Fatalf("Get: %v", err)
}
if got.ID != id {
t.Errorf("ID mismatch on Get: want %q, got %q", id, got.ID)
}
if got.Name != name {
t.Errorf("name round-trip: want %q, got %q", name, got.Name)
}
}

// ── SetDetails called twice replaces the document ────────────────────────────

func TestArangoDB_SetDetails_CalledTwice_Replaces(t *testing.T) {
b := openTestBackend(t)
ctx := context.Background()

id := uniqueID("replace")

_, err := b.SetDetails(ctx, detailsJSON(id, "OriginalName"))
if err != nil {
t.Fatalf("first SetDetails: %v", err)
}

_, err = b.SetDetails(ctx, detailsJSON(id, "ReplacedName"))
if err != nil {
t.Fatalf("second SetDetails: %v", err)
}

got, err := b.Get(ctx, id)
if err != nil {
t.Fatalf("Get after replace: %v", err)
}
if got.Name != "ReplacedName" {
t.Errorf("expected replaced name %q, got %q", "ReplacedName", got.Name)
}
}

// ── SetDetails with invalid JSON returns ErrInvalidJSON ──────────────────────

func TestArangoDB_SetDetails_InvalidJSON(t *testing.T) {
b := openTestBackend(t)
ctx := context.Background()

_, err := b.SetDetails(ctx, "not-json{{{")
if !errors.Is(err, codevaldagency.ErrInvalidJSON) {
t.Fatalf("expected ErrInvalidJSON, got %v", err)
}
}

// ── SetDetails with missing ID returns ErrInvalidJSON ────────────────────────

func TestArangoDB_SetDetails_MissingID_ReturnsInvalidJSON(t *testing.T) {
b := openTestBackend(t)
ctx := context.Background()

_, err := b.SetDetails(ctx, `{"name":"NoID","status":"draft"}`)
if !errors.Is(err, codevaldagency.ErrInvalidJSON) {
t.Fatalf("expected ErrInvalidJSON for missing ID, got %v", err)
}
}

// ── Get non-existent → ErrAgencyNotFound ─────────────────────────────────────

func TestArangoDB_Get_NotFound(t *testing.T) {
b := openTestBackend(t)
ctx := context.Background()

_, err := b.Get(ctx, "does-not-exist-"+uniqueID("nf"))
if !errors.Is(err, codevaldagency.ErrAgencyNotFound) {
t.Fatalf("expected ErrAgencyNotFound, got %v", err)
}
}

// ── InsertSnapshot ────────────────────────────────────────────────────────────

func TestArangoDB_InsertSnapshot(t *testing.T) {
b := openTestBackend(t)
ctx := context.Background()

id := uniqueID("snap")
_, err := b.SetDetails(ctx, fmt.Sprintf(
`{"id":%q,"name":"Snapshot-%s","status":"draft","mission":"Snap mission"}`, id, id,
))
if err != nil {
t.Fatalf("SetDetails: %v", err)
}

snap := codevaldagency.AgencySnapshot{
ID:         uniqueID("snapid"),
AgencyID:   id,
Name:       "Snapshot-" + id,
Mission:    "Snap mission",
SnapshotAt: time.Now().UTC(),
}

if err := b.InsertSnapshot(ctx, snap); err != nil {
t.Fatalf("InsertSnapshot: %v", err)
}
}
