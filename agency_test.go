package codevaldagency_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	codevaldagency "github.com/aosanya/CodeValdAgency"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
)

// fakeDataManager is an in-memory entitygraph.DataManager used for unit tests.
type fakeDataManager struct {
	entities      map[string]entitygraph.Entity
	relationships map[string]entitygraph.Relationship
	idCounter     int
}

func newFakeDataManager() *fakeDataManager {
	return &fakeDataManager{
		entities:      make(map[string]entitygraph.Entity),
		relationships: make(map[string]entitygraph.Relationship),
	}
}

func (f *fakeDataManager) nextID() string {
	f.idCounter++
	return fmt.Sprintf("fake-%04d", f.idCounter)
}

func (f *fakeDataManager) addEntity(e entitygraph.Entity)             { f.entities[e.ID] = e }
func (f *fakeDataManager) addRelationship(r entitygraph.Relationship) { f.relationships[r.ID] = r }

func (f *fakeDataManager) entitiesByType(typeID string) []entitygraph.Entity {
	var out []entitygraph.Entity
	for _, e := range f.entities {
		if !e.Deleted && e.TypeID == typeID {
			out = append(out, e)
		}
	}
	return out
}

func (f *fakeDataManager) CreateEntity(_ context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
	now := time.Now().UTC()
	e := entitygraph.Entity{
		ID: f.nextID(), AgencyID: req.AgencyID, TypeID: req.TypeID,
		Properties: req.Properties, CreatedAt: now, UpdatedAt: now,
	}
	f.entities[e.ID] = e
	return e, nil
}

func (f *fakeDataManager) GetEntity(_ context.Context, agencyID, entityID string) (entitygraph.Entity, error) {
	e, ok := f.entities[entityID]
	if !ok || e.AgencyID != agencyID {
		return entitygraph.Entity{}, errors.New("entity not found")
	}
	return e, nil
}

func (f *fakeDataManager) UpdateEntity(_ context.Context, agencyID, entityID string, req entitygraph.UpdateEntityRequest) (entitygraph.Entity, error) {
	e, ok := f.entities[entityID]
	if !ok || e.AgencyID != agencyID {
		return entitygraph.Entity{}, errors.New("entity not found")
	}
	if e.Properties == nil {
		e.Properties = make(map[string]any)
	}
	for k, v := range req.Properties {
		e.Properties[k] = v
	}
	e.UpdatedAt = time.Now().UTC()
	f.entities[entityID] = e
	return e, nil
}

func (f *fakeDataManager) DeleteEntity(_ context.Context, agencyID, entityID string) error {
	e, ok := f.entities[entityID]
	if !ok || e.AgencyID != agencyID {
		return errors.New("entity not found")
	}
	now := time.Now().UTC()
	e.Deleted = true
	e.DeletedAt = &now
	f.entities[entityID] = e
	return nil
}

func (f *fakeDataManager) ListEntities(_ context.Context, filter entitygraph.EntityFilter) ([]entitygraph.Entity, error) {
	var out []entitygraph.Entity
	for _, e := range f.entities {
		if e.Deleted {
			continue
		}
		if filter.AgencyID != "" && e.AgencyID != filter.AgencyID {
			continue
		}
		if filter.TypeID != "" && e.TypeID != filter.TypeID {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

func (f *fakeDataManager) CreateRelationship(_ context.Context, req entitygraph.CreateRelationshipRequest) (entitygraph.Relationship, error) {
	r := entitygraph.Relationship{
		ID: f.nextID(), AgencyID: req.AgencyID, Name: req.Name,
		FromID: req.FromID, ToID: req.ToID, Properties: req.Properties,
		CreatedAt: time.Now().UTC(),
	}
	f.relationships[r.ID] = r
	return r, nil
}

func (f *fakeDataManager) GetRelationship(_ context.Context, agencyID, relID string) (entitygraph.Relationship, error) {
	r, ok := f.relationships[relID]
	if !ok || r.AgencyID != agencyID {
		return entitygraph.Relationship{}, errors.New("relationship not found")
	}
	return r, nil
}

func (f *fakeDataManager) DeleteRelationship(_ context.Context, agencyID, relID string) error {
	r, ok := f.relationships[relID]
	if !ok || r.AgencyID != agencyID {
		return errors.New("relationship not found")
	}
	delete(f.relationships, r.ID)
	return nil
}

func (f *fakeDataManager) ListRelationships(_ context.Context, filter entitygraph.RelationshipFilter) ([]entitygraph.Relationship, error) {
	var out []entitygraph.Relationship
	for _, r := range f.relationships {
		if filter.AgencyID != "" && r.AgencyID != filter.AgencyID {
			continue
		}
		if filter.FromID != "" && r.FromID != filter.FromID {
			continue
		}
		if filter.ToID != "" && r.ToID != filter.ToID {
			continue
		}
		if filter.Name != "" && r.Name != filter.Name {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

// TraverseGraph is not exercised by current unit tests.
func (f *fakeDataManager) TraverseGraph(_ context.Context, _ entitygraph.TraverseGraphRequest) (entitygraph.TraverseGraphResult, error) {
	return entitygraph.TraverseGraphResult{}, nil
}

// fakePublisher records every Publish call so tests can assert events.
type fakePublisher struct {
	events []struct{ topic, id string }
}

func (fp *fakePublisher) Publish(_ context.Context, topic, agencyID string) error {
	fp.events = append(fp.events, struct{ topic, id string }{topic, agencyID})
	return nil
}

const testAgencyID = "test-agency"

func mustNewManager(t *testing.T) (codevaldagency.AgencyManager, *fakeDataManager) {
	t.Helper()
	fdm := newFakeDataManager()
	return codevaldagency.NewAgencyManager(fdm, nil, nil, testAgencyID), fdm
}

func mustNewManagerWithPublisher(t *testing.T) (codevaldagency.AgencyManager, *fakeDataManager, *fakePublisher) {
	t.Helper()
	fdm := newFakeDataManager()
	fp := &fakePublisher{}
	return codevaldagency.NewAgencyManager(fdm, nil, fp, testAgencyID), fdm, fp
}

// mustSetupAgency creates an agency entity via SetAgencyDetails.
// The "id" field in JSON is validated non-empty; the returned Agency.ID is
// DataManager-assigned (not the JSON id value).
func mustSetupAgency(t *testing.T, mgr codevaldagency.AgencyManager, id, name string) codevaldagency.Agency {
	t.Helper()
	agency, err := mgr.SetAgencyDetails(context.Background(),
		fmt.Sprintf(`{"id":%q,"name":%q}`, id, name))
	if err != nil {
		t.Fatalf("SetAgencyDetails: %v", err)
	}
	return agency
}

// NewAgencyManager

func TestNewAgencyManager_Constructs(t *testing.T) {
	mgr, _ := mustNewManager(t)
	if mgr == nil {
		t.Fatal("expected non-nil AgencyManager")
	}
}

// SetAgencyDetails

func TestSetAgencyDetails_InvalidJSON_ReturnsErrInvalidJSON(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	_, err := mgr.SetAgencyDetails(context.Background(), "not valid json")
	if !errors.Is(err, codevaldagency.ErrInvalidJSON) {
		t.Fatalf("expected ErrInvalidJSON, got %v", err)
	}
}

func TestSetAgencyDetails_MissingID_ReturnsErrInvalidJSON(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	_, err := mgr.SetAgencyDetails(context.Background(), `{"name":"Alpha","status":"draft"}`)
	if !errors.Is(err, codevaldagency.ErrInvalidJSON) {
		t.Fatalf("expected ErrInvalidJSON, got %v", err)
	}
}

func TestSetAgencyDetails_ValidJSON_ReturnsAgency(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	agency, err := mgr.SetAgencyDetails(context.Background(),
		`{"id":"agency-001","name":"Alpha","mission":"Build great software","status":"draft"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agency.ID == "" {
		t.Error("expected non-empty agency ID")
	}
	if agency.Name != "Alpha" {
		t.Errorf("Name: want %q, got %q", "Alpha", agency.Name)
	}
	if agency.Mission != "Build great software" {
		t.Errorf("Mission: want %q, got %q", "Build great software", agency.Mission)
	}
}

func TestSetAgencyDetails_CalledTwice_ReplacesDocument(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")
	updated := mustSetupAgency(t, mgr, "agency-001", "Alpha Updated")
	if updated.Name != "Alpha Updated" {
		t.Errorf("expected Name=%q, got %q", "Alpha Updated", updated.Name)
	}
}

// GetAgency

func TestGetAgency_NotFound(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	_, err := mgr.GetAgency(context.Background())
	if !errors.Is(err, codevaldagency.ErrAgencyNotFound) {
		t.Fatalf("expected ErrAgencyNotFound, got %v", err)
	}
}

func TestGetAgency_RoundTrip(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	set := mustSetupAgency(t, mgr, "agency-001", "Beta")
	got, err := mgr.GetAgency(context.Background())
	if err != nil {
		t.Fatalf("GetAgency: %v", err)
	}
	if got.ID != set.ID {
		t.Errorf("ID mismatch: want %q, got %q", set.ID, got.ID)
	}
	if got.Name != set.Name {
		t.Errorf("Name mismatch: want %q, got %q", set.Name, got.Name)
	}
}

// GetGoals / GetWorkflows / GetConfiguredRoles

func TestGetGoals_ReturnsGoals(t *testing.T) {
	t.Parallel()
	mgr, fdm := mustNewManager(t)
	fdm.addEntity(entitygraph.Entity{
		ID: "g-001", AgencyID: testAgencyID, TypeID: "Goal",
		Properties: map[string]any{"title": "Reduce costs", "ordinality": 1},
	})
	goals, err := mgr.GetGoals(context.Background())
	if err != nil {
		t.Fatalf("GetGoals: %v", err)
	}
	if len(goals) != 1 || goals[0].ID != "g-001" {
		t.Errorf("unexpected goals: %+v", goals)
	}
	if goals[0].Title != "Reduce costs" {
		t.Errorf("Title: want %q, got %q", "Reduce costs", goals[0].Title)
	}
}

func TestGetWorkflows_ReturnsWorkflowsWithItems(t *testing.T) {
	t.Parallel()
	mgr, fdm := mustNewManager(t)
	fdm.addEntity(entitygraph.Entity{
		ID: "wf-001", AgencyID: testAgencyID, TypeID: "Workflow",
		Properties: map[string]any{"name": "Onboarding"},
	})
	fdm.addEntity(entitygraph.Entity{
		ID: "wi-001", AgencyID: testAgencyID, TypeID: "WorkItem",
		Properties: map[string]any{"title": "Collect requirements", "order": 1},
	})
	fdm.addRelationship(entitygraph.Relationship{
		ID: "rel-001", AgencyID: testAgencyID,
		Name: "has_work_item", FromID: "wf-001", ToID: "wi-001",
	})
	wfs, err := mgr.GetWorkflows(context.Background())
	if err != nil {
		t.Fatalf("GetWorkflows: %v", err)
	}
	if len(wfs) != 1 || wfs[0].ID != "wf-001" {
		t.Errorf("unexpected workflows: %+v", wfs)
	}
}

func TestGetConfiguredRoles_ReturnsRoles(t *testing.T) {
	t.Parallel()
	mgr, fdm := mustNewManager(t)
	fdm.addEntity(entitygraph.Entity{
		ID: "role-001", AgencyID: testAgencyID, TypeID: "ConfiguredRole",
		Properties: map[string]any{"name": "domain_expert", "actor_type": "human"},
	})
	roles, err := mgr.GetConfiguredRoles(context.Background())
	if err != nil {
		t.Fatalf("GetConfiguredRoles: %v", err)
	}
	if len(roles) != 1 || roles[0].Name != "domain_expert" {
		t.Errorf("unexpected roles: %+v", roles)
	}
}

// PublishAgency

func TestPublishAgency_NoAgency_ReturnsErrAgencyNotFound(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	_, err := mgr.PublishAgency(context.Background())
	if !errors.Is(err, codevaldagency.ErrAgencyNotFound) {
		t.Fatalf("expected ErrAgencyNotFound, got %v", err)
	}
}

func TestPublishAgency_FirstPublish_VersionIsOne(t *testing.T) {
	t.Parallel()
	mgr, fdm := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")

	pub, err := mgr.PublishAgency(context.Background())
	if err != nil {
		t.Fatalf("PublishAgency: %v", err)
	}
	if pub.Version != 1 {
		t.Errorf("Version: want 1, got %d", pub.Version)
	}
	if pub.Tag != "v1" {
		t.Errorf("Tag: want %q, got %q", "v1", pub.Tag)
	}
	if pub.ID == "" {
		t.Error("ID must not be empty")
	}
	if pub.PublishedAt.IsZero() {
		t.Error("PublishedAt must not be zero")
	}
	if pub.AgencyID != testAgencyID {
		t.Errorf("AgencyID: want %q, got %q", testAgencyID, pub.AgencyID)
	}
	if pubs := fdm.entitiesByType("AgencyPublication"); len(pubs) != 1 {
		t.Fatalf("expected 1 stored publication entity, got %d", len(pubs))
	}
}

func TestPublishAgency_SecondPublish_VersionIsTwo(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")

	if _, err := mgr.PublishAgency(context.Background()); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	pub2, err := mgr.PublishAgency(context.Background())
	if err != nil {
		t.Fatalf("second publish: %v", err)
	}
	if pub2.Version != 2 {
		t.Errorf("Version: want 2, got %d", pub2.Version)
	}
	if pub2.Tag != "v2" {
		t.Errorf("Tag: want %q, got %q", "v2", pub2.Tag)
	}
}

func TestPublishAgency_PublishesEvent(t *testing.T) {
	t.Parallel()
	mgr, _, fp := mustNewManagerWithPublisher(t)
	set := mustSetupAgency(t, mgr, "agency-001", "Alpha")
	fp.events = nil // reset; SetAgencyDetails fires cross.agency.created

	if _, err := mgr.PublishAgency(context.Background()); err != nil {
		t.Fatalf("PublishAgency: %v", err)
	}
	if len(fp.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(fp.events))
	}
	if fp.events[0].topic != "cross.agency.published" {
		t.Errorf("topic: want %q, got %q", "cross.agency.published", fp.events[0].topic)
	}
	if fp.events[0].id != set.ID {
		t.Errorf("agencyID: want %q, got %q", set.ID, fp.events[0].id)
	}
}

// GetPublication

func TestGetPublication_NotFound_ReturnsErrPublicationNotFound(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")
	_, err := mgr.GetPublication(context.Background(), 99)
	if !errors.Is(err, codevaldagency.ErrPublicationNotFound) {
		t.Fatalf("expected ErrPublicationNotFound, got %v", err)
	}
}

func TestGetPublication_RoundTrip(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")
	pub, err := mgr.PublishAgency(context.Background())
	if err != nil {
		t.Fatalf("PublishAgency: %v", err)
	}
	got, err := mgr.GetPublication(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetPublication: %v", err)
	}
	if got.Version != pub.Version {
		t.Errorf("Version: want %d, got %d", pub.Version, got.Version)
	}
	if got.Tag != pub.Tag {
		t.Errorf("Tag: want %q, got %q", pub.Tag, got.Tag)
	}
	if got.AgencyID != pub.AgencyID {
		t.Errorf("AgencyID: want %q, got %q", pub.AgencyID, got.AgencyID)
	}
}

// ListPublications

func TestListPublications_EmptyBeforeAnyPublish(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")
	list, err := mgr.ListPublications(context.Background())
	if err != nil {
		t.Fatalf("ListPublications: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d publications", len(list))
	}
}

func TestListPublications_AscendingVersionOrder(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")
	for i := 0; i < 3; i++ {
		if _, err := mgr.PublishAgency(context.Background()); err != nil {
			t.Fatalf("publish %d: %v", i+1, err)
		}
	}
	list, err := mgr.ListPublications(context.Background())
	if err != nil {
		t.Fatalf("ListPublications: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 publications, got %d", len(list))
	}
	for i, p := range list {
		want := i + 1
		if p.Version != want {
			t.Errorf("list[%d].Version: want %d, got %d", i, want, p.Version)
		}
	}
}

// AgencyDraftStatus.CanTransitionTo

func TestAgencyDraftStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		from    codevaldagency.AgencyDraftStatus
		to      codevaldagency.AgencyDraftStatus
		allowed bool
	}{
		{codevaldagency.DraftStatusOpen, codevaldagency.DraftStatusPromoted, true},
		{codevaldagency.DraftStatusOpen, codevaldagency.DraftStatusArchived, true},
		{codevaldagency.DraftStatusOpen, codevaldagency.DraftStatusOpen, false},
		{codevaldagency.DraftStatusPromoted, codevaldagency.DraftStatusOpen, false},
		{codevaldagency.DraftStatusPromoted, codevaldagency.DraftStatusArchived, false},
		{codevaldagency.DraftStatusArchived, codevaldagency.DraftStatusOpen, false},
		{codevaldagency.DraftStatusArchived, codevaldagency.DraftStatusPromoted, false},
	}
	for _, tt := range tests {
		got := tt.from.CanTransitionTo(tt.to)
		if got != tt.allowed {
			t.Errorf("%q.CanTransitionTo(%q): got %v, want %v", tt.from, tt.to, got, tt.allowed)
		}
	}
}

// SetAgencyDetails — ErrAgencyReadOnly

func TestSetAgencyDetails_WhenEnabled_ReturnsErrAgencyReadOnly(t *testing.T) {
	t.Parallel()
	mgr, fdm := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")
	// Force enabled=true directly on the stored entity so we can test the guard
	// without needing a full draft→promote flow.
	for id, e := range fdm.entities {
		if e.TypeID == "Agency" {
			if e.Properties == nil {
				e.Properties = map[string]any{}
			}
			e.Properties["enabled"] = true
			fdm.entities[id] = e
		}
	}
	_, err := mgr.SetAgencyDetails(context.Background(), `{"id":"agency-001","name":"Beta"}`)
	if !errors.Is(err, codevaldagency.ErrAgencyReadOnly) {
		t.Fatalf("expected ErrAgencyReadOnly, got %v", err)
	}
}

// CreateDraft

func TestCreateDraft_NoAgency_ReturnsErrAgencyNotFound(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	_, err := mgr.CreateDraft(context.Background(), "My draft", "", "live")
	if !errors.Is(err, codevaldagency.ErrAgencyNotFound) {
		t.Fatalf("expected ErrAgencyNotFound, got %v", err)
	}
}

func TestCreateDraft_FromLive_CreatesOpenDraft(t *testing.T) {
	t.Parallel()
	mgr, fdm := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")

	draft, err := mgr.CreateDraft(context.Background(), "First draft", "", "live")
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if draft.ID == "" {
		t.Error("draft ID must not be empty")
	}
	if draft.Status != codevaldagency.DraftStatusOpen {
		t.Errorf("Status: want %q, got %q", codevaldagency.DraftStatusOpen, draft.Status)
	}
	if draft.Description != "First draft" {
		t.Errorf("Description: want %q, got %q", "First draft", draft.Description)
	}
	if draft.AgencyID != testAgencyID {
		t.Errorf("AgencyID: want %q, got %q", testAgencyID, draft.AgencyID)
	}
	_ = fdm
}

func TestCreateDraft_FromLive_CopiesGoals(t *testing.T) {
	t.Parallel()
	mgr, fdm := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")
	fdm.addEntity(entitygraph.Entity{
		ID: "g-001", AgencyID: testAgencyID, TypeID: "Goal",
		Properties: map[string]any{"title": "Reduce costs", "ordinality": 1},
	})

	draft, err := mgr.CreateDraft(context.Background(), "With goals", "", "live")
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	var draftGoals []entitygraph.Entity
	for _, e := range fdm.entities {
		if !e.Deleted && e.TypeID == "DraftGoal" && e.Properties["draft_id"] == draft.ID {
			draftGoals = append(draftGoals, e)
		}
	}
	if len(draftGoals) != 1 {
		t.Fatalf("expected 1 DraftGoal, got %d", len(draftGoals))
	}
	if draftGoals[0].Properties["title"] != "Reduce costs" {
		t.Errorf("DraftGoal.title: want %q, got %v", "Reduce costs", draftGoals[0].Properties["title"])
	}
}

// GetDraft

func TestGetDraft_NotFound_ReturnsErrDraftNotFound(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	_, err := mgr.GetDraft(context.Background(), "nonexistent")
	if !errors.Is(err, codevaldagency.ErrDraftNotFound) {
		t.Fatalf("expected ErrDraftNotFound, got %v", err)
	}
}

func TestGetDraft_RoundTrip(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")

	created, err := mgr.CreateDraft(context.Background(), "Round-trip draft", "", "live")
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	got, err := mgr.GetDraft(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetDraft: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID: want %q, got %q", created.ID, got.ID)
	}
	if got.Status != codevaldagency.DraftStatusOpen {
		t.Errorf("Status: want %q, got %q", codevaldagency.DraftStatusOpen, got.Status)
	}
}

// ListDrafts

func TestListDrafts_EmptyBeforeAnyDraft(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")

	list, err := mgr.ListDrafts(context.Background())
	if err != nil {
		t.Fatalf("ListDrafts: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d drafts", len(list))
	}
}

func TestListDrafts_ReturnsAllDrafts(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")

	for i := 0; i < 3; i++ {
		if _, err := mgr.CreateDraft(context.Background(), fmt.Sprintf("draft-%d", i), "", "live"); err != nil {
			t.Fatalf("CreateDraft %d: %v", i, err)
		}
	}

	list, err := mgr.ListDrafts(context.Background())
	if err != nil {
		t.Fatalf("ListDrafts: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 drafts, got %d", len(list))
	}
}

// UpdateDraftDescription

func TestUpdateDraftDescription_UpdatesDescription(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")

	draft, err := mgr.CreateDraft(context.Background(), "Old description", "", "live")
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	updated, err := mgr.UpdateDraftDescription(context.Background(), draft.ID, "New description")
	if err != nil {
		t.Fatalf("UpdateDraftDescription: %v", err)
	}
	if updated.Description != "New description" {
		t.Errorf("Description: want %q, got %q", "New description", updated.Description)
	}
}

func TestUpdateDraftDescription_NotFound_ReturnsErrDraftNotFound(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	_, err := mgr.UpdateDraftDescription(context.Background(), "nonexistent", "desc")
	if !errors.Is(err, codevaldagency.ErrDraftNotFound) {
		t.Fatalf("expected ErrDraftNotFound, got %v", err)
	}
}

func TestUpdateDraftDescription_PromotedDraft_ReturnsErrDraftNotOpen(t *testing.T) {
	t.Parallel()
	mgr, fdm := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")

	draft, err := mgr.CreateDraft(context.Background(), "desc", "", "live")
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	// Manually mark draft as promoted.
	e := fdm.entities[draft.ID]
	e.Properties["status"] = "promoted"
	fdm.entities[draft.ID] = e

	_, err = mgr.UpdateDraftDescription(context.Background(), draft.ID, "new desc")
	if !errors.Is(err, codevaldagency.ErrDraftNotOpen) {
		t.Fatalf("expected ErrDraftNotOpen, got %v", err)
	}
}

// PromoteDraft

func TestPromoteDraft_SetsAgencyEnabled(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")

	draft, err := mgr.CreateDraft(context.Background(), "Promote me", "", "live")
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	agency, err := mgr.PromoteDraft(context.Background(), draft.ID)
	if err != nil {
		t.Fatalf("PromoteDraft: %v", err)
	}
	if !agency.Enabled {
		t.Error("expected agency.Enabled=true after PromoteDraft")
	}
}

func TestPromoteDraft_MarksDraftPromoted(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")

	draft, err := mgr.CreateDraft(context.Background(), "Promote me", "", "live")
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	if _, err := mgr.PromoteDraft(context.Background(), draft.ID); err != nil {
		t.Fatalf("PromoteDraft: %v", err)
	}

	got, err := mgr.GetDraft(context.Background(), draft.ID)
	if err != nil {
		t.Fatalf("GetDraft after promote: %v", err)
	}
	if got.Status != codevaldagency.DraftStatusPromoted {
		t.Errorf("Status: want %q, got %q", codevaldagency.DraftStatusPromoted, got.Status)
	}
}

func TestPromoteDraft_NotFound_ReturnsErrDraftNotFound(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	_, err := mgr.PromoteDraft(context.Background(), "nonexistent")
	if !errors.Is(err, codevaldagency.ErrDraftNotFound) {
		t.Fatalf("expected ErrDraftNotFound, got %v", err)
	}
}

func TestPromoteDraft_AlreadyPromoted_ReturnsErrDraftNotOpen(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")

	draft, err := mgr.CreateDraft(context.Background(), "desc", "", "live")
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if _, err := mgr.PromoteDraft(context.Background(), draft.ID); err != nil {
		t.Fatalf("first PromoteDraft: %v", err)
	}
	_, err = mgr.PromoteDraft(context.Background(), draft.ID)
	if !errors.Is(err, codevaldagency.ErrDraftNotOpen) {
		t.Fatalf("expected ErrDraftNotOpen, got %v", err)
	}
}

func TestPromoteDraft_CopiesGoalsToLive(t *testing.T) {
	t.Parallel()
	mgr, fdm := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")

	draft, err := mgr.CreateDraft(context.Background(), "with goals", "", "live")
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	// Add a DraftGoal directly to the fake store.
	fdm.addEntity(entitygraph.Entity{
		ID: "dg-001", AgencyID: testAgencyID, TypeID: "DraftGoal",
		Properties: map[string]any{
			"draft_id":   draft.ID,
			"title":      "Promoted goal",
			"ordinality": 1,
		},
	})

	if _, err := mgr.PromoteDraft(context.Background(), draft.ID); err != nil {
		t.Fatalf("PromoteDraft: %v", err)
	}

	var liveGoals []entitygraph.Entity
	for _, e := range fdm.entities {
		if !e.Deleted && e.TypeID == "Goal" && e.AgencyID == testAgencyID {
			liveGoals = append(liveGoals, e)
		}
	}
	if len(liveGoals) != 1 {
		t.Fatalf("expected 1 live Goal after promote, got %d", len(liveGoals))
	}
	if liveGoals[0].Properties["title"] != "Promoted goal" {
		t.Errorf("Goal.title: want %q, got %v", "Promoted goal", liveGoals[0].Properties["title"])
	}
}

// ArchiveDraft

func TestArchiveDraft_MarksDraftArchived(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")

	draft, err := mgr.CreateDraft(context.Background(), "Archive me", "", "live")
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	archived, err := mgr.ArchiveDraft(context.Background(), draft.ID)
	if err != nil {
		t.Fatalf("ArchiveDraft: %v", err)
	}
	if archived.Status != codevaldagency.DraftStatusArchived {
		t.Errorf("Status: want %q, got %q", codevaldagency.DraftStatusArchived, archived.Status)
	}
}

func TestArchiveDraft_NotFound_ReturnsErrDraftNotFound(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	_, err := mgr.ArchiveDraft(context.Background(), "nonexistent")
	if !errors.Is(err, codevaldagency.ErrDraftNotFound) {
		t.Fatalf("expected ErrDraftNotFound, got %v", err)
	}
}

func TestArchiveDraft_AlreadyArchived_ReturnsErrDraftNotOpen(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")

	draft, err := mgr.CreateDraft(context.Background(), "desc", "", "live")
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if _, err := mgr.ArchiveDraft(context.Background(), draft.ID); err != nil {
		t.Fatalf("first ArchiveDraft: %v", err)
	}
	_, err = mgr.ArchiveDraft(context.Background(), draft.ID)
	if !errors.Is(err, codevaldagency.ErrDraftNotOpen) {
		t.Fatalf("expected ErrDraftNotOpen, got %v", err)
	}
}

func TestCreateDraft_FromDraft_CopiesSubEntities(t *testing.T) {
	t.Parallel()
	mgr, fdm := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")

	// Create source draft.
	src, err := mgr.CreateDraft(context.Background(), "Source draft", "", "live")
	if err != nil {
		t.Fatalf("CreateDraft source: %v", err)
	}
	// Add a DraftGoal in the source draft.
	fdm.addEntity(entitygraph.Entity{
		ID: "dg-src-001", AgencyID: testAgencyID, TypeID: "DraftGoal",
		Properties: map[string]any{
			"draft_id": src.ID,
			"title":    "Source goal",
		},
	})

	// Fork from that draft.
	dst, err := mgr.CreateDraft(context.Background(), "Forked draft", src.ID, "draft")
	if err != nil {
		t.Fatalf("CreateDraft fork: %v", err)
	}

	// Count DraftGoals belonging to the new draft.
	var forkedGoals int
	for _, e := range fdm.entities {
		if !e.Deleted && e.TypeID == "DraftGoal" && e.Properties["draft_id"] == dst.ID {
			forkedGoals++
		}
	}
	if forkedGoals != 1 {
		t.Fatalf("expected 1 forked DraftGoal, got %d", forkedGoals)
	}
}
