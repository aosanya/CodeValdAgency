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
