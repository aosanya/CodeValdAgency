package codevaldagency_test

import (
	"context"
	"errors"
	"testing"
	"time"

	codevaldagency "github.com/aosanya/CodeValdAgency"
)

// ── Fake Backend ──────────────────────────────────────────────────────────────

// fakeBackend is an in-memory codevaldagency.Backend used for unit tests.
type fakeBackend struct {
	agencies  map[string]codevaldagency.Agency
	snapshots []codevaldagency.AgencySnapshot
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		agencies: make(map[string]codevaldagency.Agency),
	}
}

func (f *fakeBackend) Insert(_ context.Context, req codevaldagency.CreateAgencyRequest) (codevaldagency.Agency, error) {
	// Generate a deterministic key from the name for test predictability.
	id := "agency-" + req.Name
	if _, exists := f.agencies[id]; exists {
		return codevaldagency.Agency{}, codevaldagency.ErrAgencyAlreadyExists
	}
	now := time.Now().UTC()
	a := codevaldagency.Agency{
		ID:        id,
		Name:      req.Name,
		Mission:   req.Mission,
		Vision:    req.Vision,
		Status:    codevaldagency.LifecycleDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}
	f.agencies[id] = a
	return a, nil
}

func (f *fakeBackend) Get(_ context.Context, agencyID string) (codevaldagency.Agency, error) {
	a, ok := f.agencies[agencyID]
	if !ok {
		return codevaldagency.Agency{}, codevaldagency.ErrAgencyNotFound
	}
	return a, nil
}

func (f *fakeBackend) Update(_ context.Context, agencyID string, req codevaldagency.UpdateAgencyRequest) (codevaldagency.Agency, error) {
	a, ok := f.agencies[agencyID]
	if !ok {
		return codevaldagency.Agency{}, codevaldagency.ErrAgencyNotFound
	}
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
	a.UpdatedAt = time.Now().UTC()
	f.agencies[agencyID] = a
	return a, nil
}

func (f *fakeBackend) Delete(_ context.Context, agencyID string) error {
	if _, ok := f.agencies[agencyID]; !ok {
		return codevaldagency.ErrAgencyNotFound
	}
	delete(f.agencies, agencyID)
	return nil
}

func (f *fakeBackend) List(_ context.Context, filter codevaldagency.AgencyFilter) ([]codevaldagency.Agency, error) {
	var out []codevaldagency.Agency
	for _, a := range f.agencies {
		if filter.Status != "" && a.Status != filter.Status {
			continue
		}
		out = append(out, a)
	}
	if out == nil {
		out = []codevaldagency.Agency{}
	}
	return out, nil
}

func (f *fakeBackend) InsertSnapshot(_ context.Context, snap codevaldagency.AgencySnapshot) error {
	f.snapshots = append(f.snapshots, snap)
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func mustNewManager(t *testing.T) (codevaldagency.AgencyManager, *fakeBackend) {
	t.Helper()
	fb := newFakeBackend()
	mgr, err := codevaldagency.NewAgencyManager(fb)
	if err != nil {
		t.Fatalf("NewAgencyManager: unexpected error: %v", err)
	}
	return mgr, fb
}

// ── NewAgencyManager ──────────────────────────────────────────────────────────

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

// ── CreateAgency ──────────────────────────────────────────────────────────────

func TestCreateAgency_EmptyName_ReturnsErrInvalidAgency(t *testing.T) {
	mgr, _ := mustNewManager(t)
	_, err := mgr.CreateAgency(context.Background(), codevaldagency.CreateAgencyRequest{
		Name: "",
	})
	if !errors.Is(err, codevaldagency.ErrInvalidAgency) {
		t.Fatalf("expected ErrInvalidAgency, got %v", err)
	}
}

func TestCreateAgency_ValidRequest_ReturnsDraftAgency(t *testing.T) {
	mgr, _ := mustNewManager(t)
	agency, err := mgr.CreateAgency(context.Background(), codevaldagency.CreateAgencyRequest{
		Name:    "Alpha",
		Mission: "Build great software",
		Vision:  "A world of automated excellence",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agency.Name != "Alpha" {
		t.Errorf("expected Name=Alpha, got %q", agency.Name)
	}
	if agency.Status != codevaldagency.LifecycleDraft {
		t.Errorf("expected Status=draft, got %q", agency.Status)
	}
	if agency.ID == "" {
		t.Error("expected non-empty ID")
	}
}

// ── GetAgency ─────────────────────────────────────────────────────────────────

func TestGetAgency_NotFound(t *testing.T) {
	mgr, _ := mustNewManager(t)
	_, err := mgr.GetAgency(context.Background(), "nonexistent")
	if !errors.Is(err, codevaldagency.ErrAgencyNotFound) {
		t.Fatalf("expected ErrAgencyNotFound, got %v", err)
	}
}

func TestGetAgency_RoundTrip(t *testing.T) {
	mgr, _ := mustNewManager(t)
	created, err := mgr.CreateAgency(context.Background(), codevaldagency.CreateAgencyRequest{Name: "Beta"})
	if err != nil {
		t.Fatalf("CreateAgency: %v", err)
	}
	got, err := mgr.GetAgency(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetAgency: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID mismatch: want %q, got %q", created.ID, got.ID)
	}
	if got.Name != created.Name {
		t.Errorf("Name mismatch: want %q, got %q", created.Name, got.Name)
	}
}

// ── UpdateAgency — lifecycle transitions ─────────────────────────────────────

func TestUpdateAgency_DraftToActive_Succeeds_WritesSnapshot(t *testing.T) {
	mgr, fb := mustNewManager(t)

	created, err := mgr.CreateAgency(context.Background(), codevaldagency.CreateAgencyRequest{Name: "Gamma"})
	if err != nil {
		t.Fatalf("CreateAgency: %v", err)
	}

	updated, err := mgr.UpdateAgency(context.Background(), created.ID, codevaldagency.UpdateAgencyRequest{
		Status: codevaldagency.LifecycleActive,
	})
	if err != nil {
		t.Fatalf("UpdateAgency draft→active: %v", err)
	}
	if updated.Status != codevaldagency.LifecycleActive {
		t.Errorf("expected Status=active, got %q", updated.Status)
	}

	// Verify a snapshot was written.
	if len(fb.snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(fb.snapshots))
	}
	snap := fb.snapshots[0]
	if snap.AgencyID != created.ID {
		t.Errorf("snapshot AgencyID: want %q, got %q", created.ID, snap.AgencyID)
	}
	if snap.ID == "" {
		t.Error("snapshot ID must not be empty")
	}
	if snap.SnapshotAt.IsZero() {
		t.Error("snapshot SnapshotAt must not be zero")
	}
}

func TestUpdateAgency_ActiveToDraft_ReturnsErrInvalidLifecycleTransition(t *testing.T) {
	mgr, _ := mustNewManager(t)

	created, err := mgr.CreateAgency(context.Background(), codevaldagency.CreateAgencyRequest{Name: "Delta"})
	if err != nil {
		t.Fatalf("CreateAgency: %v", err)
	}
	// Move to active first.
	_, err = mgr.UpdateAgency(context.Background(), created.ID, codevaldagency.UpdateAgencyRequest{
		Status: codevaldagency.LifecycleActive,
	})
	if err != nil {
		t.Fatalf("draft→active: %v", err)
	}

	// Attempting active → draft must fail.
	_, err = mgr.UpdateAgency(context.Background(), created.ID, codevaldagency.UpdateAgencyRequest{
		Status: codevaldagency.LifecycleDraft,
	})
	if !errors.Is(err, codevaldagency.ErrInvalidLifecycleTransition) {
		t.Fatalf("expected ErrInvalidLifecycleTransition, got %v", err)
	}
}

func TestUpdateAgency_AchievedToAny_ReturnsErrInvalidLifecycleTransition(t *testing.T) {
	mgr, _ := mustNewManager(t)

	created, err := mgr.CreateAgency(context.Background(), codevaldagency.CreateAgencyRequest{Name: "Epsilon"})
	if err != nil {
		t.Fatalf("CreateAgency: %v", err)
	}
	// draft → active
	_, err = mgr.UpdateAgency(context.Background(), created.ID, codevaldagency.UpdateAgencyRequest{Status: codevaldagency.LifecycleActive})
	if err != nil {
		t.Fatalf("draft→active: %v", err)
	}
	// active → achieved
	_, err = mgr.UpdateAgency(context.Background(), created.ID, codevaldagency.UpdateAgencyRequest{Status: codevaldagency.LifecycleAchieved})
	if err != nil {
		t.Fatalf("active→achieved: %v", err)
	}

	// Any further transition from achieved must fail.
	for _, next := range []codevaldagency.AgencyLifecycle{
		codevaldagency.LifecycleDraft,
		codevaldagency.LifecycleActive,
		codevaldagency.LifecycleAchieved,
	} {
		_, err = mgr.UpdateAgency(context.Background(), created.ID, codevaldagency.UpdateAgencyRequest{Status: next})
		if !errors.Is(err, codevaldagency.ErrInvalidLifecycleTransition) {
			t.Errorf("achieved→%q: expected ErrInvalidLifecycleTransition, got %v", next, err)
		}
	}
}

func TestUpdateAgency_NoStatusChange_DoesNotWriteSnapshot(t *testing.T) {
	mgr, fb := mustNewManager(t)

	created, err := mgr.CreateAgency(context.Background(), codevaldagency.CreateAgencyRequest{Name: "Zeta"})
	if err != nil {
		t.Fatalf("CreateAgency: %v", err)
	}
	// Update name only — no lifecycle change.
	_, err = mgr.UpdateAgency(context.Background(), created.ID, codevaldagency.UpdateAgencyRequest{
		Name: "Zeta Updated",
	})
	if err != nil {
		t.Fatalf("UpdateAgency: %v", err)
	}
	if len(fb.snapshots) != 0 {
		t.Errorf("expected 0 snapshots for non-lifecycle update, got %d", len(fb.snapshots))
	}
}

// ── DeleteAgency ──────────────────────────────────────────────────────────────

func TestDeleteAgency_NotFound(t *testing.T) {
	mgr, _ := mustNewManager(t)
	err := mgr.DeleteAgency(context.Background(), "nonexistent")
	if !errors.Is(err, codevaldagency.ErrAgencyNotFound) {
		t.Fatalf("expected ErrAgencyNotFound, got %v", err)
	}
}

func TestDeleteAgency_ThenGet_ReturnsNotFound(t *testing.T) {
	mgr, _ := mustNewManager(t)

	created, err := mgr.CreateAgency(context.Background(), codevaldagency.CreateAgencyRequest{Name: "Eta"})
	if err != nil {
		t.Fatalf("CreateAgency: %v", err)
	}
	if err := mgr.DeleteAgency(context.Background(), created.ID); err != nil {
		t.Fatalf("DeleteAgency: %v", err)
	}
	_, err = mgr.GetAgency(context.Background(), created.ID)
	if !errors.Is(err, codevaldagency.ErrAgencyNotFound) {
		t.Fatalf("expected ErrAgencyNotFound after delete, got %v", err)
	}
}

// ── ListAgencies ──────────────────────────────────────────────────────────────

func TestListAgencies_EmptyFilter_ReturnsAll(t *testing.T) {
	mgr, _ := mustNewManager(t)

	names := []string{"Theta", "Iota", "Kappa"}
	for _, n := range names {
		if _, err := mgr.CreateAgency(context.Background(), codevaldagency.CreateAgencyRequest{Name: n}); err != nil {
			t.Fatalf("CreateAgency %q: %v", n, err)
		}
	}

	list, err := mgr.ListAgencies(context.Background(), codevaldagency.AgencyFilter{})
	if err != nil {
		t.Fatalf("ListAgencies: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 agencies, got %d", len(list))
	}
}

func TestListAgencies_StatusFilter(t *testing.T) {
	mgr, _ := mustNewManager(t)

	// Create two draft agencies.
	a1, _ := mgr.CreateAgency(context.Background(), codevaldagency.CreateAgencyRequest{Name: "Lambda"})
	_, _ = mgr.CreateAgency(context.Background(), codevaldagency.CreateAgencyRequest{Name: "Mu"})

	// Activate one.
	_, err := mgr.UpdateAgency(context.Background(), a1.ID, codevaldagency.UpdateAgencyRequest{
		Status: codevaldagency.LifecycleActive,
	})
	if err != nil {
		t.Fatalf("UpdateAgency: %v", err)
	}

	active, err := mgr.ListAgencies(context.Background(), codevaldagency.AgencyFilter{Status: codevaldagency.LifecycleActive})
	if err != nil {
		t.Fatalf("ListAgencies: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("expected 1 active agency, got %d", len(active))
	}
	if active[0].ID != a1.ID {
		t.Errorf("unexpected active agency ID: %q", active[0].ID)
	}
}

// ── AgencyLifecycle.CanTransitionTo ──────────────────────────────────────────

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
