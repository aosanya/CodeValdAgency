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
		fmt.Sprintf(`{"id":%q,"name":%q,"status":"draft"}`, id, name))
	if err != nil {
		t.Fatalf("SetAgencyDetails: %v", err)
	}
	return agency
}

// mustSeedActivationData inserts the minimum Goal + Workflow + WorkItem
// required to pass the draft -> active activation guard.
func mustSeedActivationData(fdm *fakeDataManager) {
	fdm.addEntity(entitygraph.Entity{
		ID: "seed-goal-001", AgencyID: testAgencyID, TypeID: "Goal",
		Properties: map[string]any{"title": "Seed Goal", "ordinality": 1},
	})
	fdm.addEntity(entitygraph.Entity{
		ID: "seed-wf-001", AgencyID: testAgencyID, TypeID: "Workflow",
		Properties: map[string]any{"name": "Seed Workflow"},
	})
	fdm.addEntity(entitygraph.Entity{
		ID: "seed-wi-001", AgencyID: testAgencyID, TypeID: "WorkItem",
		Properties: map[string]any{"title": "Seed WorkItem", "order": 1},
	})
	fdm.addRelationship(entitygraph.Relationship{
		ID: "seed-rel-001", AgencyID: testAgencyID,
		Name: "has_work_item", FromID: "seed-wf-001", ToID: "seed-wi-001",
	})
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

// UpdateAgency lifecycle transitions

func TestUpdateAgency_DraftToActive_Succeeds_WritesSnapshot(t *testing.T) {
	t.Parallel()
	mgr, fdm := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Gamma")
	mustSeedActivationData(fdm)

	updated, err := mgr.UpdateAgency(context.Background(), codevaldagency.UpdateAgencyRequest{
		Status: codevaldagency.LifecycleActive,
	})
	if err != nil {
		t.Fatalf("UpdateAgency draft->active: %v", err)
	}
	if updated.Status != codevaldagency.LifecycleActive {
		t.Errorf("expected Status=active, got %q", updated.Status)
	}
	snaps := fdm.entitiesByType("AgencySnapshot")
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot entity, got %d", len(snaps))
	}
	if snaps[0].ID == "" {
		t.Error("snapshot entity ID must not be empty")
	}
}

func TestUpdateAgency_ActiveToDraft_ReturnsErrInvalidLifecycleTransition(t *testing.T) {
	t.Parallel()
	mgr, fdm := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Delta")
	mustSeedActivationData(fdm)

	if _, err := mgr.UpdateAgency(context.Background(), codevaldagency.UpdateAgencyRequest{
		Status: codevaldagency.LifecycleActive,
	}); err != nil {
		t.Fatalf("draft->active: %v", err)
	}
	_, err := mgr.UpdateAgency(context.Background(), codevaldagency.UpdateAgencyRequest{
		Status: codevaldagency.LifecycleDraft,
	})
	if !errors.Is(err, codevaldagency.ErrInvalidLifecycleTransition) {
		t.Fatalf("expected ErrInvalidLifecycleTransition, got %v", err)
	}
}

func TestUpdateAgency_AchievedToAny_ReturnsErrInvalidLifecycleTransition(t *testing.T) {
	t.Parallel()
	mgr, fdm := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Epsilon")
	mustSeedActivationData(fdm)

	if _, err := mgr.UpdateAgency(context.Background(), codevaldagency.UpdateAgencyRequest{
		Status: codevaldagency.LifecycleActive,
	}); err != nil {
		t.Fatalf("draft->active: %v", err)
	}
	if _, err := mgr.UpdateAgency(context.Background(), codevaldagency.UpdateAgencyRequest{
		Status: codevaldagency.LifecycleAchieved,
	}); err != nil {
		t.Fatalf("active->achieved: %v", err)
	}

	for _, next := range []codevaldagency.AgencyLifecycle{
		codevaldagency.LifecycleDraft,
		codevaldagency.LifecycleActive,
		codevaldagency.LifecycleAchieved,
	} {
		_, err := mgr.UpdateAgency(context.Background(), codevaldagency.UpdateAgencyRequest{Status: next})
		if !errors.Is(err, codevaldagency.ErrInvalidLifecycleTransition) {
			t.Errorf("achieved->%q: expected ErrInvalidLifecycleTransition, got %v", next, err)
		}
	}
}

func TestUpdateAgency_NoStatusChange_DoesNotWriteSnapshot(t *testing.T) {
	t.Parallel()
	mgr, fdm := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Zeta")
	_, err := mgr.UpdateAgency(context.Background(), codevaldagency.UpdateAgencyRequest{Name: "Zeta Updated"})
	if err != nil {
		t.Fatalf("UpdateAgency: %v", err)
	}
	if snaps := fdm.entitiesByType("AgencySnapshot"); len(snaps) != 0 {
		t.Errorf("expected 0 snapshots for non-lifecycle update, got %d", len(snaps))
	}
}

func TestUpdateAgency_DraftToActive_MissingGoal_ReturnsErrInvalidAgency(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Eta")
	_, err := mgr.UpdateAgency(context.Background(), codevaldagency.UpdateAgencyRequest{
		Status: codevaldagency.LifecycleActive,
	})
	if !errors.Is(err, codevaldagency.ErrInvalidAgency) {
		t.Fatalf("expected ErrInvalidAgency (no goals), got %v", err)
	}
}

// AgencyLifecycle.CanTransitionTo

func TestAgencyLifecycle_CanTransitionTo(t *testing.T) {
	tests := []struct {
		from    codevaldagency.AgencyLifecycle
		to      codevaldagency.AgencyLifecycle
		allowed bool
	}{
		{codevaldagency.LifecycleDraft, codevaldagency.LifecycleActive, true},
		{codevaldagency.LifecycleDraft, codevaldagency.LifecycleAchieved, false},
		{codevaldagency.LifecycleDraft, codevaldagency.LifecycleDraft, false},
		{codevaldagency.LifecycleActive, codevaldagency.LifecycleAchieved, true},
		{codevaldagency.LifecycleActive, codevaldagency.LifecycleDraft, false},
		{codevaldagency.LifecycleActive, codevaldagency.LifecycleActive, false},
		{codevaldagency.LifecycleAchieved, codevaldagency.LifecycleDraft, false},
		{codevaldagency.LifecycleAchieved, codevaldagency.LifecycleActive, false},
		{codevaldagency.LifecycleAchieved, codevaldagency.LifecycleAchieved, false},
	}
	for _, tt := range tests {
		got := tt.from.CanTransitionTo(tt.to)
		if got != tt.allowed {
			t.Errorf("%q.CanTransitionTo(%q): got %v, want %v", tt.from, tt.to, got, tt.allowed)
		}
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

func TestPublishAgency_DoesNotChangeAgencyStatus(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")

	if _, err := mgr.PublishAgency(context.Background()); err != nil {
		t.Fatalf("PublishAgency: %v", err)
	}
	agency, err := mgr.GetAgency(context.Background())
	if err != nil {
		t.Fatalf("GetAgency: %v", err)
	}
	if agency.Status != codevaldagency.LifecycleDraft {
		t.Errorf("Status: want %q (unchanged), got %q", codevaldagency.LifecycleDraft, agency.Status)
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
