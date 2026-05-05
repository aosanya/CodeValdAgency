package server_test

import (
	"context"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	codevaldagency "github.com/aosanya/CodeValdAgency"
	pb "github.com/aosanya/CodeValdAgency/gen/go/codevaldagency/v1"
	"github.com/aosanya/CodeValdAgency/internal/server"
)

// ── Mock AgencyManager ────────────────────────────────────────────────────────────

// mockManager is a configurable stub of codevaldagency.AgencyManager.
type mockManager struct {
	setDetailsResult codevaldagency.Agency
	setDetailsErr    error
	getResult        codevaldagency.Agency
	getErr           error
	publishResult    codevaldagency.AgencyPublication
	publishErr       error
	getPubResult     codevaldagency.AgencyPublication
	getPubErr        error
	listPubResult    []codevaldagency.AgencyPublication
	listPubErr       error
	goalsResult      []codevaldagency.Goal
	goalsErr         error
	workflowsResult  []codevaldagency.Workflow
	workflowsErr     error
	rolesResult      []codevaldagency.ConfiguredRole
	rolesErr         error

	createDraftResult     codevaldagency.AgencyDraft
	createDraftErr        error
	getDraftResult        codevaldagency.AgencyDraft
	getDraftErr           error
	listDraftsResult      []codevaldagency.AgencyDraft
	listDraftsErr         error
	updateDraftDescResult codevaldagency.AgencyDraft
	updateDraftDescErr    error
	promoteDraftResult    codevaldagency.Agency
	promoteDraftErr       error
	archiveDraftResult    codevaldagency.AgencyDraft
	archiveDraftErr       error

	createRoleResult   codevaldagency.Role
	createRoleErr      error
	getRoleResult      codevaldagency.Role
	getRoleErr         error
	matchRolesResult   []codevaldagency.RoleMatch
	matchRolesErr      error
}

func (m *mockManager) SetAgencyDetails(_ context.Context, _ string) (codevaldagency.Agency, error) {
	return m.setDetailsResult, m.setDetailsErr
}
func (m *mockManager) GetAgency(_ context.Context) (codevaldagency.Agency, error) {
	return m.getResult, m.getErr
}
func (m *mockManager) CreateDraft(_ context.Context, _, _, _ string) (codevaldagency.AgencyDraft, error) {
	return m.createDraftResult, m.createDraftErr
}
func (m *mockManager) GetDraft(_ context.Context, _ string) (codevaldagency.AgencyDraft, error) {
	return m.getDraftResult, m.getDraftErr
}
func (m *mockManager) ListDrafts(_ context.Context) ([]codevaldagency.AgencyDraft, error) {
	return m.listDraftsResult, m.listDraftsErr
}
func (m *mockManager) UpdateDraftDescription(_ context.Context, _, _ string) (codevaldagency.AgencyDraft, error) {
	return m.updateDraftDescResult, m.updateDraftDescErr
}
func (m *mockManager) PromoteDraft(_ context.Context, _ string) (codevaldagency.Agency, error) {
	return m.promoteDraftResult, m.promoteDraftErr
}
func (m *mockManager) ArchiveDraft(_ context.Context, _ string) (codevaldagency.AgencyDraft, error) {
	return m.archiveDraftResult, m.archiveDraftErr
}
func (m *mockManager) PublishAgency(_ context.Context, _ string) (codevaldagency.AgencyPublication, error) {
	return m.publishResult, m.publishErr
}
func (m *mockManager) GetPublication(_ context.Context, _ int) (codevaldagency.AgencyPublication, error) {
	return m.getPublicationResult(), m.getPublicationErr()
}
func (m *mockManager) getPublicationResult() codevaldagency.AgencyPublication { return m.getPubResult }
func (m *mockManager) getPublicationErr() error                               { return m.getPubErr }
func (m *mockManager) ListPublications(_ context.Context) ([]codevaldagency.AgencyPublication, error) {
	return m.listPubResult, m.listPubErr
}
func (m *mockManager) UpdatePublicationStatus(_ context.Context, _ int, _ string) (codevaldagency.AgencyPublication, error) {
	return codevaldagency.AgencyPublication{}, nil
}
func (m *mockManager) GetGoals(_ context.Context) ([]codevaldagency.Goal, error) {
	return m.goalsResult, m.goalsErr
}
func (m *mockManager) GetWorkflows(_ context.Context) ([]codevaldagency.Workflow, error) {
	return m.workflowsResult, m.workflowsErr
}
func (m *mockManager) GetConfiguredRoles(_ context.Context) ([]codevaldagency.ConfiguredRole, error) {
	return m.rolesResult, m.rolesErr
}
func (m *mockManager) CreateRole(_ context.Context, _ codevaldagency.CreateRoleRequest) (codevaldagency.Role, error) {
	return m.createRoleResult, m.createRoleErr
}
func (m *mockManager) GetRole(_ context.Context, _ string) (codevaldagency.Role, error) {
	return m.getRoleResult, m.getRoleErr
}
func (m *mockManager) ListRoles(_ context.Context) ([]codevaldagency.Role, error) {
	return nil, nil
}
func (m *mockManager) UpdateRole(_ context.Context, _ string, _ codevaldagency.UpdateRoleRequest) (codevaldagency.Role, error) {
	return codevaldagency.Role{}, nil
}
func (m *mockManager) DeleteRole(_ context.Context, _ string) error {
	return nil
}
func (m *mockManager) AddContextSource(_ context.Context, _ string, _ codevaldagency.AddContextSourceRequest) (codevaldagency.ContextSource, error) {
	return codevaldagency.ContextSource{}, nil
}
func (m *mockManager) ListContextSources(_ context.Context, _ string) ([]codevaldagency.ContextSource, error) {
	return nil, nil
}
func (m *mockManager) RemoveContextSource(_ context.Context, _, _ string) error {
	return nil
}
func (m *mockManager) MatchRoles(_ context.Context, _, _ string) ([]codevaldagency.RoleMatch, error) {
	return m.matchRolesResult, m.matchRolesErr
}

// requireCode asserts that err is a gRPC status error with the expected code.
func requireCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected gRPC error with code %v, got nil", want)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}
	if st.Code() != want {
		t.Fatalf("expected code %v, got %v (msg: %s)", want, st.Code(), st.Message())
	}
}

// ── SetAgencyDetails ─────────────────────────────────────────────────────────────────

func TestServer_SetAgencyDetails_OK(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{setDetailsResult: codevaldagency.Agency{
		ID:   "a1",
		Name: "Alpha",
	}}
	srv := server.New(mgr, nil)
	got, err := srv.SetAgencyDetails(context.Background(), &pb.SetAgencyDetailsRequest{
		Json: `{"id":"a1","name":"Alpha","status":"draft"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetId() != "a1" {
		t.Errorf("ID: want %q, got %q", "a1", got.GetId())
	}
}

func TestServer_SetAgencyDetails_InvalidJSON_ReturnsInvalidArgument(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{setDetailsErr: codevaldagency.ErrInvalidJSON}
	srv := server.New(mgr, nil)
	_, err := srv.SetAgencyDetails(context.Background(), &pb.SetAgencyDetailsRequest{Json: "bad"})
	requireCode(t, err, codes.InvalidArgument)
}

func TestServer_SetAgencyDetails_BackendError_ReturnsInternal(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{setDetailsErr: fmt.Errorf("database failure")}
	srv := server.New(mgr, nil)
	_, err := srv.SetAgencyDetails(context.Background(), &pb.SetAgencyDetailsRequest{Json: `{"id":"a1"}`})
	requireCode(t, err, codes.Internal)
}

// ── GetAgency ────────────────────────────────────────────────────────────────────────────

func TestServer_GetAgency_OK(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{getResult: codevaldagency.Agency{ID: "a2", Name: "Beta"}}
	srv := server.New(mgr, nil)
	got, err := srv.GetAgency(context.Background(), &pb.GetAgencyRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetId() != "a2" {
		t.Errorf("ID: want %q, got %q", "a2", got.GetId())
	}
}

func TestServer_GetAgency_NotFound(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{getErr: codevaldagency.ErrAgencyNotFound}
	srv := server.New(mgr, nil)
	_, err := srv.GetAgency(context.Background(), &pb.GetAgencyRequest{})
	requireCode(t, err, codes.NotFound)
}

// ── Draft handlers ───────────────────────────────────────────────────────────────────────

func TestServer_CreateDraft_OK(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{
		createDraftResult: codevaldagency.AgencyDraft{
			ID:          "d1",
			AgencyID:    "a1",
			Description: "Draft one",
			Status:      codevaldagency.DraftStatusOpen,
		},
	}
	srv := server.New(mgr, nil)
	got, err := srv.CreateDraft(context.Background(), &pb.CreateDraftRequest{
		Description: "Draft one",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetId() != "d1" {
		t.Errorf("ID: want %q, got %q", "d1", got.GetId())
	}
	if got.GetStatus() != pb.AgencyDraftStatus_AGENCY_DRAFT_STATUS_OPEN {
		t.Errorf("Status: want OPEN, got %v", got.GetStatus())
	}
}

func TestServer_CreateDraft_AgencyNotFound_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{createDraftErr: codevaldagency.ErrAgencyNotFound}
	srv := server.New(mgr, nil)
	_, err := srv.CreateDraft(context.Background(), &pb.CreateDraftRequest{})
	requireCode(t, err, codes.NotFound)
}

func TestServer_GetDraft_OK(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{
		getDraftResult: codevaldagency.AgencyDraft{
			ID:     "d1",
			Status: codevaldagency.DraftStatusOpen,
		},
	}
	srv := server.New(mgr, nil)
	got, err := srv.GetDraft(context.Background(), &pb.GetDraftRequest{DraftId: "d1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetId() != "d1" {
		t.Errorf("ID: want %q, got %q", "d1", got.GetId())
	}
}

func TestServer_GetDraft_NotFound_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{getDraftErr: codevaldagency.ErrDraftNotFound}
	srv := server.New(mgr, nil)
	_, err := srv.GetDraft(context.Background(), &pb.GetDraftRequest{DraftId: "nonexistent"})
	requireCode(t, err, codes.NotFound)
}

func TestServer_ListDrafts_OK(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{
		listDraftsResult: []codevaldagency.AgencyDraft{
			{ID: "d1", Status: codevaldagency.DraftStatusOpen},
			{ID: "d2", Status: codevaldagency.DraftStatusArchived},
		},
	}
	srv := server.New(mgr, nil)
	got, err := srv.ListDrafts(context.Background(), &pb.ListDraftsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.GetDrafts()) != 2 {
		t.Fatalf("expected 2 drafts, got %d", len(got.GetDrafts()))
	}
}

func TestServer_UpdateDraftDescription_OK(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{
		updateDraftDescResult: codevaldagency.AgencyDraft{
			ID:          "d1",
			Description: "Updated",
			Status:      codevaldagency.DraftStatusOpen,
		},
	}
	srv := server.New(mgr, nil)
	got, err := srv.UpdateDraftDescription(context.Background(), &pb.UpdateDraftDescriptionRequest{
		DraftId:     "d1",
		Description: "Updated",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetDescription() != "Updated" {
		t.Errorf("Description: want %q, got %q", "Updated", got.GetDescription())
	}
}

func TestServer_UpdateDraftDescription_NotOpen_ReturnsFailedPrecondition(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{updateDraftDescErr: codevaldagency.ErrDraftNotOpen}
	srv := server.New(mgr, nil)
	_, err := srv.UpdateDraftDescription(context.Background(), &pb.UpdateDraftDescriptionRequest{
		DraftId: "d1",
	})
	requireCode(t, err, codes.FailedPrecondition)
}

func TestServer_PromoteDraft_OK(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{
		promoteDraftResult: codevaldagency.Agency{ID: "a1", Enabled: true},
	}
	srv := server.New(mgr, nil)
	got, err := srv.PromoteDraft(context.Background(), &pb.PromoteDraftRequest{DraftId: "d1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.GetEnabled() {
		t.Error("expected Enabled=true after PromoteDraft")
	}
}

func TestServer_PromoteDraft_NotOpen_ReturnsFailedPrecondition(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{promoteDraftErr: codevaldagency.ErrDraftNotOpen}
	srv := server.New(mgr, nil)
	_, err := srv.PromoteDraft(context.Background(), &pb.PromoteDraftRequest{DraftId: "d1"})
	requireCode(t, err, codes.FailedPrecondition)
}

func TestServer_ArchiveDraft_OK(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{
		archiveDraftResult: codevaldagency.AgencyDraft{
			ID:     "d1",
			Status: codevaldagency.DraftStatusArchived,
		},
	}
	srv := server.New(mgr, nil)
	got, err := srv.ArchiveDraft(context.Background(), &pb.ArchiveDraftRequest{DraftId: "d1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetStatus() != pb.AgencyDraftStatus_AGENCY_DRAFT_STATUS_ARCHIVED {
		t.Errorf("Status: want ARCHIVED, got %v", got.GetStatus())
	}
}

func TestServer_ArchiveDraft_NotOpen_ReturnsFailedPrecondition(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{archiveDraftErr: codevaldagency.ErrDraftNotOpen}
	srv := server.New(mgr, nil)
	_, err := srv.ArchiveDraft(context.Background(), &pb.ArchiveDraftRequest{DraftId: "d1"})
	requireCode(t, err, codes.FailedPrecondition)
}

// ── PublishAgency ───────────────────────────────────────────────────────────────────────

func TestServer_PublishAgency_OK(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{
		publishResult: codevaldagency.AgencyPublication{
			ID:       "pub-1",
			AgencyID: "a1",
			Version:  1,
			Tag:      "v1",
		},
	}
	srv := server.New(mgr, nil)
	got, err := srv.PublishAgency(context.Background(), &pb.PublishAgencyRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetVersion() != 1 {
		t.Errorf("Version: want 1, got %d", got.GetVersion())
	}
	if got.GetTag() != "v1" {
		t.Errorf("Tag: want %q, got %q", "v1", got.GetTag())
	}
}

func TestServer_PublishAgency_NotFound_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{publishErr: codevaldagency.ErrAgencyNotFound}
	srv := server.New(mgr, nil)
	_, err := srv.PublishAgency(context.Background(), &pb.PublishAgencyRequest{})
	requireCode(t, err, codes.NotFound)
}

// ── GetPublication ─────────────────────────────────────────────────────────────────────

func TestServer_GetPublication_OK(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{
		getPubResult: codevaldagency.AgencyPublication{
			ID:       "pub-1",
			AgencyID: "a1",
			Version:  1,
			Tag:      "v1",
		},
	}
	srv := server.New(mgr, nil)
	got, err := srv.GetPublication(context.Background(), &pb.GetPublicationRequest{Version: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetVersion() != 1 {
		t.Errorf("Version: want 1, got %d", got.GetVersion())
	}
}

func TestServer_GetPublication_NotFound_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{getPubErr: codevaldagency.ErrPublicationNotFound}
	srv := server.New(mgr, nil)
	_, err := srv.GetPublication(context.Background(), &pb.GetPublicationRequest{Version: 99})
	requireCode(t, err, codes.NotFound)
}

// ── ListPublications ───────────────────────────────────────────────────────────────────

func TestServer_ListPublications_OK(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{
		listPubResult: []codevaldagency.AgencyPublication{
			{ID: "pub-1", AgencyID: "a1", Version: 1, Tag: "v1"},
			{ID: "pub-2", AgencyID: "a1", Version: 2, Tag: "v2"},
		},
	}
	srv := server.New(mgr, nil)
	got, err := srv.ListPublications(context.Background(), &pb.ListPublicationsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.GetPublications()) != 2 {
		t.Fatalf("expected 2 publications, got %d", len(got.GetPublications()))
	}
	if got.GetPublications()[1].GetTag() != "v2" {
		t.Errorf("Tag: want %q, got %q", "v2", got.GetPublications()[1].GetTag())
	}
}

func TestServer_ListPublications_Empty_ReturnsEmptyList(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{listPubResult: nil}
	srv := server.New(mgr, nil)
	got, err := srv.ListPublications(context.Background(), &pb.ListPublicationsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.GetPublications()) != 0 {
		t.Errorf("expected empty list, got %d", len(got.GetPublications()))
	}
}

// ── GetGoals ────────────────────────────────────────────────────────────────────────────

func TestServer_GetGoals_OK(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{
		goalsResult: []codevaldagency.Goal{
			{ID: "g1", Title: "Goal One", Ordinality: 1},
			{ID: "g2", Title: "Goal Two", Ordinality: 2},
		},
	}
	srv := server.New(mgr, nil)
	got, err := srv.GetGoals(context.Background(), &pb.GetGoalsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.GetGoals()) != 2 {
		t.Fatalf("expected 2 goals, got %d", len(got.GetGoals()))
	}
	if got.GetGoals()[0].GetId() != "g1" {
		t.Errorf("Goal[0].ID: want %q, got %q", "g1", got.GetGoals()[0].GetId())
	}
}

func TestServer_GetGoals_Empty_ReturnsEmptyList(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{goalsResult: nil}
	srv := server.New(mgr, nil)
	got, err := srv.GetGoals(context.Background(), &pb.GetGoalsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.GetGoals()) != 0 {
		t.Errorf("expected empty list, got %d", len(got.GetGoals()))
	}
}

func TestServer_GetGoals_ManagerError_ReturnsInternal(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{goalsErr: fmt.Errorf("storage failure")}
	srv := server.New(mgr, nil)
	_, err := srv.GetGoals(context.Background(), &pb.GetGoalsRequest{})
	requireCode(t, err, codes.Internal)
}

// ── GetWorkflows ───────────────────────────────────────────────────────────────────────

func TestServer_GetWorkflows_OK(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{
		workflowsResult: []codevaldagency.Workflow{
			{
				ID:   "wf1",
				Name: "Workflow One",
			},
		},
	}
	srv := server.New(mgr, nil)
	got, err := srv.GetWorkflows(context.Background(), &pb.GetWorkflowsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.GetWorkflows()) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(got.GetWorkflows()))
	}
	if got.GetWorkflows()[0].GetId() != "wf1" {
		t.Errorf("Workflow[0].ID: want %q, got %q", "wf1", got.GetWorkflows()[0].GetId())
	}
}

func TestServer_GetWorkflows_Empty_ReturnsEmptyList(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{workflowsResult: nil}
	srv := server.New(mgr, nil)
	got, err := srv.GetWorkflows(context.Background(), &pb.GetWorkflowsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.GetWorkflows()) != 0 {
		t.Errorf("expected empty list, got %d", len(got.GetWorkflows()))
	}
}

func TestServer_GetWorkflows_ManagerError_ReturnsInternal(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{workflowsErr: fmt.Errorf("storage failure")}
	srv := server.New(mgr, nil)
	_, err := srv.GetWorkflows(context.Background(), &pb.GetWorkflowsRequest{})
	requireCode(t, err, codes.Internal)
}

// ── GetConfiguredRoles ─────────────────────────────────────────────────────────────────

func TestServer_GetConfiguredRoles_OK(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{
		rolesResult: []codevaldagency.ConfiguredRole{
			{Name: "analyst", ActorType: codevaldagency.ActorTypeHuman},
			{Name: "reviewer", ActorType: codevaldagency.ActorTypeAIAgent},
		},
	}
	srv := server.New(mgr, nil)
	got, err := srv.GetConfiguredRoles(context.Background(), &pb.GetConfiguredRolesRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.GetConfiguredRoles()) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(got.GetConfiguredRoles()))
	}
	if got.GetConfiguredRoles()[0].GetRole() != "analyst" {
		t.Errorf("Role[0]: want %q, got %q", "analyst", got.GetConfiguredRoles()[0].GetRole())
	}
}

func TestServer_GetConfiguredRoles_Empty_ReturnsEmptyList(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{rolesResult: nil}
	srv := server.New(mgr, nil)
	got, err := srv.GetConfiguredRoles(context.Background(), &pb.GetConfiguredRolesRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.GetConfiguredRoles()) != 0 {
		t.Errorf("expected empty list, got %d", len(got.GetConfiguredRoles()))
	}
}

func TestServer_GetConfiguredRoles_ManagerError_ReturnsInternal(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{rolesErr: fmt.Errorf("storage failure")}
	srv := server.New(mgr, nil)
	_, err := srv.GetConfiguredRoles(context.Background(), &pb.GetConfiguredRolesRequest{})
	requireCode(t, err, codes.Internal)
}
