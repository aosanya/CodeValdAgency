package codevaldagency_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	codevaldagency "github.com/aosanya/CodeValdAgency"
)

// ── Fake Backend ────────────────────────────────────────────────────────────────────────────

// fakeBackend is an in-memory codevaldagency.Backend used for unit tests.
// It holds a single agency document, mirroring the single-agency-per-database model.
type fakeBackend struct {
	agency       *codevaldagency.Agency // nil until SetDetails is called
	snapshots    []codevaldagency.AgencySnapshot
	publications []codevaldagency.AgencyPublication
}

// fakePublisher records every Publish call so tests can assert events.
type fakePublisher struct {
	events []struct{ topic, id string }
}

func (fp *fakePublisher) Publish(_ context.Context, topic, agencyID string) error {
	fp.events = append(fp.events, struct{ topic, id string }{topic, agencyID})
	return nil
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{}
}

func (f *fakeBackend) SetDetails(_ context.Context, jsonStr string) (codevaldagency.Agency, error) {
	var raw struct {
		ID              string                       `json:"id"`
		Name            string                       `json:"name"`
		Mission         string                       `json:"mission"`
		Vision          string                       `json:"vision"`
		Status          string                       `json:"status"`
		ConfiguredRoles []codevaldagency.ConfiguredRole `json:"configured_roles"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return codevaldagency.Agency{}, codevaldagency.ErrInvalidJSON
	}
	if raw.ID == "" {
		return codevaldagency.Agency{}, codevaldagency.ErrInvalidJSON
	}
	a := codevaldagency.Agency{
		ID:              raw.ID,
		Name:            raw.Name,
		Mission:         raw.Mission,
		Vision:          raw.Vision,
		Status:          codevaldagency.AgencyLifecycle(raw.Status),
		ConfiguredRoles: raw.ConfiguredRoles,
	}
	f.agency = &a
	return a, nil
}

func (f *fakeBackend) Get(_ context.Context) (codevaldagency.Agency, error) {
	if f.agency == nil {
		return codevaldagency.Agency{}, codevaldagency.ErrAgencyNotFound
	}
	return *f.agency, nil
}

func (f *fakeBackend) Update(_ context.Context, req codevaldagency.UpdateAgencyRequest) (codevaldagency.Agency, error) {
	if f.agency == nil {
		return codevaldagency.Agency{}, codevaldagency.ErrAgencyNotFound
	}
	a := *f.agency
	if req.Name != "" {
		a.Name = req.Name
	}
	if req.Mission != "" {
		a.Mission = req.Mission
	}
	if req.Vision != "" {
		a.Vision = req.Vision
	}
	if req.Status != "" {
		a.Status = req.Status
	}
	if req.Goals != nil {
		a.Goals = req.Goals
	}
	if req.Workflows != nil {
		a.Workflows = req.Workflows
	}
	if req.ConfiguredRoles != nil {
		a.ConfiguredRoles = req.ConfiguredRoles
	}
	f.agency = &a
	return a, nil
}

func (f *fakeBackend) InsertSnapshot(_ context.Context, snap codevaldagency.AgencySnapshot) error {
	f.snapshots = append(f.snapshots, snap)
	return nil
}

func (f *fakeBackend) InsertPublication(_ context.Context, pub codevaldagency.AgencyPublication) error {
	f.publications = append(f.publications, pub)
	return nil
}

func (f *fakeBackend) GetPublication(_ context.Context, version int) (codevaldagency.AgencyPublication, error) {
	for _, p := range f.publications {
		if p.Version == version {
			return p, nil
		}
	}
	return codevaldagency.AgencyPublication{}, codevaldagency.ErrPublicationNotFound
}

func (f *fakeBackend) ListPublications(_ context.Context) ([]codevaldagency.AgencyPublication, error) {
	out := make([]codevaldagency.AgencyPublication, len(f.publications))
	copy(out, f.publications)
	return out, nil
}

func (f *fakeBackend) NextPublicationVersion(_ context.Context) (int, error) {
	return len(f.publications) + 1, nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────────────────

func mustNewManager(t *testing.T) (codevaldagency.AgencyManager, *fakeBackend) {
	t.Helper()
	fb := newFakeBackend()
	mgr, err := codevaldagency.NewAgencyManager(fb)
	if err != nil {
		t.Fatalf("NewAgencyManager: unexpected error: %v", err)
	}
	return mgr, fb
}

// mustSetupAgency calls SetAgencyDetails with a minimal valid JSON payload.
func mustSetupAgency(t *testing.T, mgr codevaldagency.AgencyManager, id, name string) codevaldagency.Agency {
	t.Helper()
	jsonStr := fmt.Sprintf(`{"id":%q,"name":%q,"status":"draft"}`, id, name)
	agency, err := mgr.SetAgencyDetails(context.Background(), jsonStr)
	if err != nil {
		t.Fatalf("SetAgencyDetails: %v", err)
	}
	return agency
}

// mustNewManagerWithPublisher constructs a manager that records publish events.
func mustNewManagerWithPublisher(t *testing.T) (codevaldagency.AgencyManager, *fakeBackend, *fakePublisher) {
	t.Helper()
	fb := newFakeBackend()
	fp := &fakePublisher{}
	mgr, err := codevaldagency.NewAgencyManager(fb, codevaldagency.WithPublisher(fp))
	if err != nil {
		t.Fatalf("NewAgencyManager: %v", err)
	}
	return mgr, fb, fp
}

// ── NewAgencyManager ─────────────────────────────────────────────────────────────────

func TestNewAgencyManager_NilBackend(t *testing.T) {
	_, err := codevaldagency.NewAgencyManager(nil)
	if err == nil {
		t.Fatal("expected error for nil backend, got nil")
	}
}

func TestNewAgencyManager_ValidBackend(t *testing.T) {
	mgr, _ := mustNewManager(t)
	if mgr == nil {
		t.Fatal("expected non-nil AgencyManager")
	}
}

// ── SetAgencyDetails ─────────────────────────────────────────────────────────────────

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
	if agency.ID != "agency-001" {
		t.Errorf("ID: want %q, got %q", "agency-001", agency.ID)
	}
	if agency.Name != "Alpha" {
		t.Errorf("Name: want %q, got %q", "Alpha", agency.Name)
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

// ── GetAgency ────────────────────────────────────────────────────────────────────────────

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

// ── UpdateAgency — lifecycle transitions ─────────────────────────────────────────────

func TestUpdateAgency_DraftToActive_Succeeds_WritesSnapshot(t *testing.T) {
	t.Parallel()
	mgr, fb := mustNewManager(t)
	set := mustSetupAgency(t, mgr, "agency-001", "Gamma")
	updated, err := mgr.UpdateAgency(context.Background(), codevaldagency.UpdateAgencyRequest{
		Status: codevaldagency.LifecycleActive,
	})
	if err != nil {
		t.Fatalf("UpdateAgency draft→active: %v", err)
	}
	if updated.Status != codevaldagency.LifecycleActive {
		t.Errorf("expected Status=active, got %q", updated.Status)
	}
	if len(fb.snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(fb.snapshots))
	}
	snap := fb.snapshots[0]
	if snap.AgencyID != set.ID {
		t.Errorf("snapshot AgencyID: want %q, got %q", set.ID, snap.AgencyID)
	}
	if snap.ID == "" {
		t.Error("snapshot ID must not be empty")
	}
	if snap.SnapshotAt.IsZero() {
		t.Error("snapshot SnapshotAt must not be zero")
	}
}

func TestUpdateAgency_ActiveToDraft_ReturnsErrInvalidLifecycleTransition(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Delta")
	_, err := mgr.UpdateAgency(context.Background(), codevaldagency.UpdateAgencyRequest{
		Status: codevaldagency.LifecycleActive,
	})
	if err != nil {
		t.Fatalf("draft→active: %v", err)
	}
	_, err = mgr.UpdateAgency(context.Background(), codevaldagency.UpdateAgencyRequest{
		Status: codevaldagency.LifecycleDraft,
	})
	if !errors.Is(err, codevaldagency.ErrInvalidLifecycleTransition) {
		t.Fatalf("expected ErrInvalidLifecycleTransition, got %v", err)
	}
}

func TestUpdateAgency_AchievedToAny_ReturnsErrInvalidLifecycleTransition(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Epsilon")
	_, _ = mgr.UpdateAgency(context.Background(), codevaldagency.UpdateAgencyRequest{Status: codevaldagency.LifecycleActive})
	_, _ = mgr.UpdateAgency(context.Background(), codevaldagency.UpdateAgencyRequest{Status: codevaldagency.LifecycleAchieved})
	for _, next := range []codevaldagency.AgencyLifecycle{
		codevaldagency.LifecycleDraft,
		codevaldagency.LifecycleActive,
		codevaldagency.LifecycleAchieved,
	} {
		_, err := mgr.UpdateAgency(context.Background(), codevaldagency.UpdateAgencyRequest{Status: next})
		if !errors.Is(err, codevaldagency.ErrInvalidLifecycleTransition) {
			t.Errorf("achieved→%q: expected ErrInvalidLifecycleTransition, got %v", next, err)
		}
	}
}

func TestUpdateAgency_NoStatusChange_DoesNotWriteSnapshot(t *testing.T) {
	t.Parallel()
	mgr, fb := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Zeta")
	_, err := mgr.UpdateAgency(context.Background(), codevaldagency.UpdateAgencyRequest{
		Name: "Zeta Updated",
	})
	if err != nil {
		t.Fatalf("UpdateAgency: %v", err)
	}
	if len(fb.snapshots) != 0 {
		t.Errorf("expected 0 snapshots for non-lifecycle update, got %d", len(fb.snapshots))
	}
}

// ── AgencyLifecycle.CanTransitionTo ───────────────────────────────────────────────

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

// ── PublishAgency ────────────────────────────────────────────────────────────────────────

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
	mgr, fb := mustNewManager(t)
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
	if len(fb.publications) != 1 {
		t.Fatalf("expected 1 stored publication, got %d", len(fb.publications))
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

func TestPublishAgency_SnapshotContainsAgencyState(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	set := mustSetupAgency(t, mgr, "agency-001", "Alpha")

	pub, err := mgr.PublishAgency(context.Background())
	if err != nil {
		t.Fatalf("PublishAgency: %v", err)
	}
	if pub.Agency.ID != set.ID {
		t.Errorf("Agency.ID: want %q, got %q", set.ID, pub.Agency.ID)
	}
	if pub.Agency.Name != set.Name {
		t.Errorf("Agency.Name: want %q, got %q", set.Name, pub.Agency.Name)
	}
}

func TestPublishAgency_OldPublicationUnchangedAfterEdit(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")

	pub1, err := mgr.PublishAgency(context.Background())
	if err != nil {
		t.Fatalf("first publish: %v", err)
	}

	// Edit the agency after publishing.
	mustSetupAgency(t, mgr, "agency-001", "Alpha Revised")

	// v1 snapshot should still reflect the original name.
	if pub1.Agency.Name != "Alpha" {
		t.Errorf("pub1.Agency.Name should be immutable; got %q, want %q", pub1.Agency.Name, "Alpha")
	}

	// v2 should capture the revised name.
	pub2, err := mgr.PublishAgency(context.Background())
	if err != nil {
		t.Fatalf("second publish: %v", err)
	}
	if pub2.Agency.Name != "Alpha Revised" {
		t.Errorf("pub2.Agency.Name: want %q, got %q", "Alpha Revised", pub2.Agency.Name)
	}
}

func TestPublishAgency_PublishesEvent(t *testing.T) {
	t.Parallel()
	mgr, _, fp := mustNewManagerWithPublisher(t)
	mustSetupAgency(t, mgr, "agency-001", "Alpha")
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
	if fp.events[0].id != "agency-001" {
		t.Errorf("agencyID: want %q, got %q", "agency-001", fp.events[0].id)
	}
}

// ── GetPublication ───────────────────────────────────────────────────────────────────────

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
	if got.Agency.ID != pub.Agency.ID {
		t.Errorf("Agency.ID: want %q, got %q", pub.Agency.ID, got.Agency.ID)
	}
}

// ── ListPublications ─────────────────────────────────────────────────────────────────────

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
