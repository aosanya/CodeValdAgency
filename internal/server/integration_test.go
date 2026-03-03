// Package server_test provides end-to-end integration tests for the
// AgencyService gRPC handler.
//
// Tests in this file require a running ArangoDB instance. They are skipped
// automatically when AGENCY_ARANGO_ENDPOINT is not set or the server is
// unreachable.
//
// To run:
//
//AGENCY_ARANGO_ENDPOINT=http://localhost:8529 go test -v -race ./internal/server/
package server_test

import (
"context"
"fmt"
"net"
"os"
"testing"
"time"

driver "github.com/arangodb/go-driver"
driverhttp "github.com/arangodb/go-driver/http"
"google.golang.org/grpc"
"google.golang.org/grpc/codes"
"google.golang.org/grpc/credentials/insecure"

codevaldagency "github.com/aosanya/CodeValdAgency"
pb "github.com/aosanya/CodeValdAgency/gen/go/codevaldagency/v1"
"github.com/aosanya/CodeValdAgency/internal/server"
"github.com/aosanya/CodeValdAgency/storage/arangodb"
)

// openIntegrationEnv spins up an in-process gRPC server backed by a real
// ArangoDB instance and returns a ready-to-use client, the raw driver.Database
// (for direct collection verification), and a cleanup function.
//
// The test is skipped when AGENCY_ARANGO_ENDPOINT is unset or the ArangoDB
// server is unreachable.
func openIntegrationEnv(t *testing.T) (pb.AgencyServiceClient, driver.Database, func()) {
t.Helper()

endpoint := os.Getenv("AGENCY_ARANGO_ENDPOINT")
if endpoint == "" {
endpoint = "http://localhost:8529"
}

conn, err := driverhttp.NewConnection(driverhttp.ConnectionConfig{
Endpoints: []string{endpoint},
})
if err != nil {
t.Skipf("ArangoDB connection config error (AGENCY_ARANGO_ENDPOINT=%s): %v", endpoint, err)
}

user := os.Getenv("AGENCY_ARANGO_USER")
if user == "" {
user = "root"
}
pass := os.Getenv("AGENCY_ARANGO_PASSWORD")

arangoClient, err := driver.NewClient(driver.ClientConfig{
Connection:     conn,
Authentication: driver.BasicAuthentication(user, pass),
})
if err != nil {
t.Skipf("ArangoDB client error: %v", err)
}

// Quick ping — skip if unreachable (CI without ArangoDB).
pingCtx, pingCancel := context.WithTimeout(context.Background(), 3*time.Second)
defer pingCancel()
if _, err := arangoClient.Version(pingCtx); err != nil {
t.Skipf("ArangoDB unreachable at %s: %v", endpoint, err)
}

dbName := os.Getenv("AGENCY_ARANGO_DATABASE_TEST")
if dbName == "" {
dbName = "codevald_tests"
}
ctx := context.Background()
exists, err := arangoClient.DatabaseExists(ctx, dbName)
if err != nil {
t.Fatalf("DatabaseExists: %v", err)
}
var db driver.Database
if exists {
db, err = arangoClient.Database(ctx, dbName)
} else {
db, err = arangoClient.CreateDatabase(ctx, dbName, nil)
}
if err != nil {
t.Fatalf("open/create test database %q: %v", dbName, err)
}

backend, err := arangodb.NewBackendFromDB(db)
if err != nil {
t.Fatalf("NewBackendFromDB: %v", err)
}

mgr, err := codevaldagency.NewAgencyManager(backend)
if err != nil {
t.Fatalf("NewAgencyManager: %v", err)
}

// Start an in-process gRPC server on a random port.
lis, err := net.Listen("tcp", "127.0.0.1:0")
if err != nil {
t.Fatalf("net.Listen: %v", err)
}
grpcSrv := grpc.NewServer()
pb.RegisterAgencyServiceServer(grpcSrv, server.New(mgr))
go func() { _ = grpcSrv.Serve(lis) }()

// Dial a gRPC client.
grpcConn, err := grpc.NewClient(
lis.Addr().String(),
grpc.WithTransportCredentials(insecure.NewCredentials()),
)
if err != nil {
grpcSrv.Stop()
t.Fatalf("grpc.NewClient: %v", err)
}

agencyClient := pb.NewAgencyServiceClient(grpcConn)
cleanup := func() {
_ = grpcConn.Close()
grpcSrv.Stop()
}
return agencyClient, db, cleanup
}

// uniqueIntegrationID returns an ID unique within the current test run.
func uniqueIntegrationID(prefix string) string {
return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// snapshotCountForAgency counts documents in the agency_snapshots collection
// whose agency_id field equals agencyID.
func snapshotCountForAgency(t *testing.T, db driver.Database, agencyID string) int {
t.Helper()
ctx := context.Background()
query := `FOR s IN agency_snapshots FILTER s.agency_id == @id RETURN s`
cursor, err := db.Query(ctx, query, map[string]interface{}{"id": agencyID})
if err != nil {
t.Fatalf("snapshot query: %v", err)
}
defer cursor.Close()
count := 0
for cursor.HasMore() {
var doc map[string]interface{}
if _, err := cursor.ReadDocument(ctx, &doc); err != nil {
t.Fatalf("snapshot ReadDocument: %v", err)
}
count++
}
return count
}

// integrationJSON builds a minimal valid agency JSON payload.
func integrationJSON(id, name string) string {
return fmt.Sprintf(`{"id":%q,"name":%q,"status":"draft"}`, id, name)
}

// ── Integration tests ─────────────────────────────────────────────────────────

// TestIntegration_SetGet_RoundTrip — SetAgencyDetails → Get returns same agency.
func TestIntegration_SetGet_RoundTrip(t *testing.T) {
client, _, cleanup := openIntegrationEnv(t)
defer cleanup()

ctx := context.Background()
id := uniqueIntegrationID("rt")
name := "Alpha-" + id

created, err := client.SetAgencyDetails(ctx, &pb.SetAgencyDetailsRequest{
Json: integrationJSON(id, name),
})
if err != nil {
t.Fatalf("SetAgencyDetails: %v", err)
}
if created.GetId() != id {
t.Fatalf("expected ID %q, got %q", id, created.GetId())
}
if created.GetStatus() != pb.AgencyLifecycle_AGENCY_LIFECYCLE_DRAFT {
t.Errorf("expected DRAFT, got %v", created.GetStatus())
}

got, err := client.GetAgency(ctx, &pb.GetAgencyRequest{})
if err != nil {
t.Fatalf("GetAgency: %v", err)
}
if got.GetId() != id {
t.Errorf("ID mismatch: want %q, got %q", id, got.GetId())
}
if got.GetName() != name {
t.Errorf("Name mismatch: want %q, got %q", name, got.GetName())
}
}

// TestIntegration_SetAgencyDetails_CalledTwice_Replaces — second call overwrites.
func TestIntegration_SetAgencyDetails_CalledTwice_Replaces(t *testing.T) {
client, _, cleanup := openIntegrationEnv(t)
defer cleanup()

ctx := context.Background()
id := uniqueIntegrationID("replace")

_, err := client.SetAgencyDetails(ctx, &pb.SetAgencyDetailsRequest{
Json: integrationJSON(id, "OriginalName"),
})
if err != nil {
t.Fatalf("first SetAgencyDetails: %v", err)
}

_, err = client.SetAgencyDetails(ctx, &pb.SetAgencyDetailsRequest{
Json: integrationJSON(id, "ReplacedName"),
})
if err != nil {
t.Fatalf("second SetAgencyDetails: %v", err)
}

got, err := client.GetAgency(ctx, &pb.GetAgencyRequest{})
if err != nil {
t.Fatalf("GetAgency: %v", err)
}
if got.GetName() != "ReplacedName" {
t.Errorf("expected name %q, got %q", "ReplacedName", got.GetName())
}
}

// TestIntegration_SetAgencyDetails_InvalidJSON_ReturnsInvalidArgument —
// malformed JSON payload returns INVALID_ARGUMENT.
func TestIntegration_SetAgencyDetails_InvalidJSON_ReturnsInvalidArgument(t *testing.T) {
client, _, cleanup := openIntegrationEnv(t)
defer cleanup()

ctx := context.Background()
_, err := client.SetAgencyDetails(ctx, &pb.SetAgencyDetailsRequest{Json: "not-json"})
requireCode(t, err, codes.InvalidArgument)
}

// TestIntegration_Update_DraftToActive — draft → active is a valid transition.
func TestIntegration_Update_DraftToActive(t *testing.T) {
client, _, cleanup := openIntegrationEnv(t)
defer cleanup()

ctx := context.Background()
id := uniqueIntegrationID("d2a")
_, err := client.SetAgencyDetails(ctx, &pb.SetAgencyDetailsRequest{
Json: integrationJSON(id, "Beta-"+id),
})
if err != nil {
t.Fatalf("SetAgencyDetails: %v", err)
}

updated, err := client.UpdateAgency(ctx, &pb.UpdateAgencyRequest{
	Status: pb.AgencyLifecycle_AGENCY_LIFECYCLE_ACTIVE,
})
if err != nil {
t.Fatalf("UpdateAgency draft->active: %v", err)
}
if updated.GetStatus() != pb.AgencyLifecycle_AGENCY_LIFECYCLE_ACTIVE {
t.Errorf("expected ACTIVE, got %v", updated.GetStatus())
}
}

// TestIntegration_Update_InvalidTransition_ReturnsFailedPrecondition —
// an invalid lifecycle transition returns FAILED_PRECONDITION.
func TestIntegration_Update_InvalidTransition_ReturnsFailedPrecondition(t *testing.T) {
client, _, cleanup := openIntegrationEnv(t)
defer cleanup()

ctx := context.Background()
id := uniqueIntegrationID("inv")
_, err := client.SetAgencyDetails(ctx, &pb.SetAgencyDetailsRequest{
Json: integrationJSON(id, "Gamma-"+id),
})
if err != nil {
t.Fatalf("SetAgencyDetails: %v", err)
}

// draft -> achieved is not a valid transition.
_, err = client.UpdateAgency(ctx, &pb.UpdateAgencyRequest{
	Status: pb.AgencyLifecycle_AGENCY_LIFECYCLE_ACHIEVED,
})
requireCode(t, err, codes.FailedPrecondition)
}

// TestIntegration_Update_ActiveToAchieved_ThenBlocked — active -> achieved
// succeeds; any subsequent status update returns FAILED_PRECONDITION.
func TestIntegration_Update_ActiveToAchieved_ThenBlocked(t *testing.T) {
client, _, cleanup := openIntegrationEnv(t)
defer cleanup()

ctx := context.Background()
id := uniqueIntegrationID("ach")
_, err := client.SetAgencyDetails(ctx, &pb.SetAgencyDetailsRequest{
Json: integrationJSON(id, "Delta-"+id),
})
if err != nil {
t.Fatalf("SetAgencyDetails: %v", err)
}

// draft -> active
_, err = client.UpdateAgency(ctx, &pb.UpdateAgencyRequest{
	Status: pb.AgencyLifecycle_AGENCY_LIFECYCLE_ACTIVE,
})
if err != nil {
t.Fatalf("draft->active: %v", err)
}

// active -> achieved
updated, err := client.UpdateAgency(ctx, &pb.UpdateAgencyRequest{
	Status: pb.AgencyLifecycle_AGENCY_LIFECYCLE_ACHIEVED,
})
if err != nil {
t.Fatalf("active->achieved: %v", err)
}
if updated.GetStatus() != pb.AgencyLifecycle_AGENCY_LIFECYCLE_ACHIEVED {
t.Errorf("expected ACHIEVED, got %v", updated.GetStatus())
}

// achieved is terminal.
for _, next := range []pb.AgencyLifecycle{
pb.AgencyLifecycle_AGENCY_LIFECYCLE_DRAFT,
pb.AgencyLifecycle_AGENCY_LIFECYCLE_ACTIVE,
pb.AgencyLifecycle_AGENCY_LIFECYCLE_ACHIEVED,
} {
_, err = client.UpdateAgency(ctx, &pb.UpdateAgencyRequest{
	Status: next,
})
requireCode(t, err, codes.FailedPrecondition)
}
}

// TestIntegration_DraftToActive_SnapshotWritten — transitioning draft -> active
// writes exactly one document to the agency_snapshots collection.
func TestIntegration_DraftToActive_SnapshotWritten(t *testing.T) {
client, db, cleanup := openIntegrationEnv(t)
defer cleanup()

ctx := context.Background()
id := uniqueIntegrationID("snap")
_, err := client.SetAgencyDetails(ctx, &pb.SetAgencyDetailsRequest{
Json: fmt.Sprintf(`{"id":%q,"name":"Zeta-%s","status":"draft","mission":"Snapshot mission"}`, id, id),
})
if err != nil {
t.Fatalf("SetAgencyDetails: %v", err)
}

before := snapshotCountForAgency(t, db, id)

_, err = client.UpdateAgency(ctx, &pb.UpdateAgencyRequest{
Status: pb.AgencyLifecycle_AGENCY_LIFECYCLE_ACTIVE,
})
if err != nil {
t.Fatalf("UpdateAgency draft->active: %v", err)
}

after := snapshotCountForAgency(t, db, id)
if after != before+1 {
t.Errorf("expected %d snapshot(s) after activation, got %d", before+1, after)
}
}
