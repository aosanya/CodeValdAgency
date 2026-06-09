package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	codevaldagency "github.com/aosanya/CodeValdAgency"
	pb "github.com/aosanya/CodeValdAgency/gen/go/codevaldagency/v1"
	"github.com/aosanya/CodeValdAgency/internal/server"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// importFakeDM is a minimal entitygraph.DataManager that records every
// UpsertEntity call so the FEAT-20260609-002 test can assert per-workflow
// event_flows is written through. Only the methods the importer touches are
// implemented; the rest return zero values.
type importFakeDM struct {
	upserts []entitygraph.CreateEntityRequest
	counter int
}

func (f *importFakeDM) nextID() string {
	f.counter++
	return fmt.Sprintf("imp-%04d", f.counter)
}

func (f *importFakeDM) CreateEntity(_ context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
	return entitygraph.Entity{
		ID: f.nextID(), AgencyID: req.AgencyID, TypeID: req.TypeID,
		Properties: req.Properties, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}, nil
}

func (f *importFakeDM) UpsertEntity(_ context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
	f.upserts = append(f.upserts, req)
	return entitygraph.Entity{
		ID: f.nextID(), AgencyID: req.AgencyID, TypeID: req.TypeID,
		Properties: req.Properties, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}, nil
}

func (f *importFakeDM) ListEntities(_ context.Context, _ entitygraph.EntityFilter) ([]entitygraph.Entity, error) {
	return nil, nil
}

func (f *importFakeDM) GetEntity(_ context.Context, _, _ string) (entitygraph.Entity, error) {
	return entitygraph.Entity{}, errors.New("not implemented")
}

func (f *importFakeDM) UpdateEntity(_ context.Context, _, _ string, _ entitygraph.UpdateEntityRequest) (entitygraph.Entity, error) {
	return entitygraph.Entity{}, errors.New("not implemented")
}

func (f *importFakeDM) DeleteEntity(_ context.Context, _, _ string) error { return nil }

func (f *importFakeDM) CreateRelationship(_ context.Context, _ entitygraph.CreateRelationshipRequest) (entitygraph.Relationship, error) {
	return entitygraph.Relationship{}, nil
}

func (f *importFakeDM) GetRelationship(_ context.Context, _, _ string) (entitygraph.Relationship, error) {
	return entitygraph.Relationship{}, errors.New("not implemented")
}

func (f *importFakeDM) DeleteRelationship(_ context.Context, _, _ string) error { return nil }

func (f *importFakeDM) ListRelationships(_ context.Context, _ entitygraph.RelationshipFilter) ([]entitygraph.Relationship, error) {
	return nil, nil
}

func (f *importFakeDM) TraverseGraph(_ context.Context, _ entitygraph.TraverseGraphRequest) (entitygraph.TraverseGraphResult, error) {
	return entitygraph.TraverseGraphResult{}, nil
}

// findWorkflowUpsert returns the upsert request for the DraftWorkflow whose
// `code` property equals workflowCode, or nil if missing.
func findWorkflowUpsert(reqs []entitygraph.CreateEntityRequest, workflowCode string) *entitygraph.CreateEntityRequest {
	for i := range reqs {
		r := reqs[i]
		if r.TypeID != "DraftWorkflow" {
			continue
		}
		if c, _ := r.Properties["code"].(string); c == workflowCode {
			return &r
		}
	}
	return nil
}

// TestImportDraft_PerWorkflowEventFlows_Stored verifies FEAT-20260609-002:
// when agency.json bundles a workflow with an inline `event_flows` block,
// the importer marshals it to a JSON string and writes it as the
// `event_flows` property on the DraftWorkflow upsert.
func TestImportDraft_PerWorkflowEventFlows_Stored(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{setDetailsResult: codevaldagency.Agency{ID: "uab", Name: "Utility App Builder"}}
	dm := &importFakeDM{}
	srv := server.New(mgr, dm, nil)

	body := `{
	  "agency": {"code": "uab", "name": "Utility App Builder"},
	  "workflows": [
	    {
	      "code": "planning",
	      "name": "Planning",
	      "ordinality": 1,
	      "event_flows": {
	        "name": "Planning",
	        "steps": [
	          {"type": "start", "step": "1", "emits_topic": "task.assigned"}
	        ]
	      }
	    }
	  ]
	}`

	_, err := srv.ImportDraft(context.Background(), &pb.ImportDraftRequest{Body: body})
	if err != nil {
		t.Fatalf("ImportDraft returned error: %v", err)
	}

	got := findWorkflowUpsert(dm.upserts, "planning")
	if got == nil {
		t.Fatalf("expected an upsert for DraftWorkflow code=planning; got %d upserts: %+v", len(dm.upserts), dm.upserts)
	}
	rawEF, ok := got.Properties["event_flows"]
	if !ok {
		t.Fatalf("DraftWorkflow upsert is missing the event_flows property; got props: %+v", got.Properties)
	}
	efStr, ok := rawEF.(string)
	if !ok {
		t.Fatalf("event_flows property should be a JSON string; got %T (%v)", rawEF, rawEF)
	}
	// Re-decode and verify the round-trip preserves the inline block.
	var decoded map[string]any
	if err := json.Unmarshal([]byte(efStr), &decoded); err != nil {
		t.Fatalf("event_flows is not valid JSON: %v (raw: %q)", err, efStr)
	}
	if decoded["name"] != "Planning" {
		t.Errorf("event_flows.name: want %q, got %q", "Planning", decoded["name"])
	}
	steps, ok := decoded["steps"].([]any)
	if !ok || len(steps) != 1 {
		t.Fatalf("event_flows.steps: want one entry, got %v", decoded["steps"])
	}
}

// TestImportDraft_PerWorkflowEventFlows_OmittedNoProp verifies that workflows
// without an event_flows block do not write the property at all — readers fall
// back to Agency.event_flows.
func TestImportDraft_PerWorkflowEventFlows_OmittedNoProp(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{setDetailsResult: codevaldagency.Agency{ID: "uab"}}
	dm := &importFakeDM{}
	srv := server.New(mgr, dm, nil)

	body := `{
	  "agency": {"code": "uab", "name": "Utility App Builder"},
	  "workflows": [{"code": "legacy", "name": "Legacy", "ordinality": 1}]
	}`

	_, err := srv.ImportDraft(context.Background(), &pb.ImportDraftRequest{Body: body})
	if err != nil {
		t.Fatalf("ImportDraft returned error: %v", err)
	}

	got := findWorkflowUpsert(dm.upserts, "legacy")
	if got == nil {
		t.Fatalf("expected an upsert for DraftWorkflow code=legacy")
	}
	if _, present := got.Properties["event_flows"]; present {
		t.Errorf("event_flows should NOT be written when omitted; got: %v", got.Properties["event_flows"])
	}
}

// promoteCountingMockManager wraps mockManager to count PromoteDraft invocations
// so the FEAT-20260609-003 tests can assert auto-promote actually fired (or did
// not) along the import path.
type promoteCountingMockManager struct {
	mockManager
	promoteCalls   int
	lastPromoteArg string
}

func (m *promoteCountingMockManager) PromoteDraft(_ context.Context, draftID string) (codevaldagency.Agency, error) {
	m.promoteCalls++
	m.lastPromoteArg = draftID
	return m.promoteDraftResult, m.promoteDraftErr
}

// TestImportDraft_AutoPromote_FalseOnUnpublished verifies FEAT-20260609-003:
// when auto_promote is false against an unpublished agency, the importer
// returns promoted=false and does NOT call PromoteDraft (existing behavior
// preserved).
func TestImportDraft_AutoPromote_FalseOnUnpublished(t *testing.T) {
	t.Parallel()
	mgr := &promoteCountingMockManager{mockManager: mockManager{
		setDetailsResult: codevaldagency.Agency{ID: "uab"},
	}}
	srv := server.New(mgr, &importFakeDM{}, nil)

	body := `{"agency": {"code": "uab", "name": "Utility App Builder"}}`
	resp, err := srv.ImportDraft(context.Background(), &pb.ImportDraftRequest{Body: body, AutoPromote: false})
	if err != nil {
		t.Fatalf("ImportDraft returned error: %v", err)
	}
	if resp.GetPromoted() {
		t.Errorf("Promoted = true, want false when auto_promote=false")
	}
	if mgr.promoteCalls != 0 {
		t.Errorf("PromoteDraft called %d times, want 0", mgr.promoteCalls)
	}
}

// TestImportDraft_AutoPromote_TrueOnUnpublished verifies FEAT-20260609-003:
// when auto_promote is true and the agency is unpublished, the importer runs
// the existing flow plus PromoteDraft and returns promoted=true.
func TestImportDraft_AutoPromote_TrueOnUnpublished(t *testing.T) {
	t.Parallel()
	mgr := &promoteCountingMockManager{mockManager: mockManager{
		setDetailsResult:   codevaldagency.Agency{ID: "uab"},
		promoteDraftResult: codevaldagency.Agency{ID: "uab", Enabled: true},
	}}
	srv := server.New(mgr, &importFakeDM{}, nil)

	body := `{"agency": {"code": "uab", "name": "Utility App Builder"}}`
	resp, err := srv.ImportDraft(context.Background(), &pb.ImportDraftRequest{Body: body, AutoPromote: true})
	if err != nil {
		t.Fatalf("ImportDraft returned error: %v", err)
	}
	if !resp.GetPromoted() {
		t.Errorf("Promoted = false, want true when auto_promote=true succeeds")
	}
	if mgr.promoteCalls != 1 {
		t.Errorf("PromoteDraft called %d times, want 1", mgr.promoteCalls)
	}
	if resp.GetDraftId() == "" {
		t.Error("DraftId is empty; expected the auto-created draft ID")
	}
	if mgr.lastPromoteArg != resp.GetDraftId() {
		t.Errorf("PromoteDraft called with %q, want the response DraftId %q", mgr.lastPromoteArg, resp.GetDraftId())
	}
}

// TestImportDraft_AutoPromote_TrueOnPublishedSwallowsReadOnly verifies the
// primary FEAT-20260609-003 use case: re-import against a published agency
// with auto_promote=true succeeds end-to-end — SetAgencyDetails' ReadOnly
// error is swallowed, draft entities are upserted, and PromoteDraft is called.
func TestImportDraft_AutoPromote_TrueOnPublishedSwallowsReadOnly(t *testing.T) {
	t.Parallel()
	mgr := &promoteCountingMockManager{mockManager: mockManager{
		setDetailsErr:      codevaldagency.ErrAgencyReadOnly,
		promoteDraftResult: codevaldagency.Agency{ID: "uab", Enabled: true},
	}}
	srv := server.New(mgr, &importFakeDM{}, nil)

	body := `{"agency": {"code": "uab", "name": "Utility App Builder"}}`
	resp, err := srv.ImportDraft(context.Background(), &pb.ImportDraftRequest{Body: body, AutoPromote: true})
	if err != nil {
		t.Fatalf("ImportDraft returned error: %v (expected ReadOnly to be swallowed)", err)
	}
	if !resp.GetPromoted() {
		t.Errorf("Promoted = false, want true when auto_promote=true against published agency")
	}
	if mgr.promoteCalls != 1 {
		t.Errorf("PromoteDraft called %d times, want 1", mgr.promoteCalls)
	}
}

// TestImportDraft_AutoPromote_FalseOnPublishedReturnsFailedPrecondition
// verifies FEAT-20260609-003: re-import against a published agency without
// auto_promote returns FAILED_PRECONDITION (not INTERNAL) and never calls
// PromoteDraft.
func TestImportDraft_AutoPromote_FalseOnPublishedReturnsFailedPrecondition(t *testing.T) {
	t.Parallel()
	mgr := &promoteCountingMockManager{mockManager: mockManager{
		setDetailsErr: codevaldagency.ErrAgencyReadOnly,
	}}
	srv := server.New(mgr, &importFakeDM{}, nil)

	body := `{"agency": {"code": "uab", "name": "Utility App Builder"}}`
	resp, err := srv.ImportDraft(context.Background(), &pb.ImportDraftRequest{Body: body, AutoPromote: false})
	if err == nil {
		t.Fatalf("ImportDraft returned no error; want FAILED_PRECONDITION (resp=%+v)", resp)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("returned error is not a gRPC status: %v", err)
	}
	if st.Code() != codes.FailedPrecondition {
		t.Errorf("status code = %v, want %v", st.Code(), codes.FailedPrecondition)
	}
	if mgr.promoteCalls != 0 {
		t.Errorf("PromoteDraft called %d times, want 0", mgr.promoteCalls)
	}
}
