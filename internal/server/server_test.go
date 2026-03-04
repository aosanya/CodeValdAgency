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
	setDetailsResult   codevaldagency.Agency
	setDetailsErr      error
	getResult          codevaldagency.Agency
	getErr             error
	updateResult       codevaldagency.Agency
	updateErr          error
	publishResult      codevaldagency.AgencyPublication
	publishErr         error
	getPubResult       codevaldagency.AgencyPublication
	getPubErr          error
	listPubResult      []codevaldagency.AgencyPublication
	listPubErr         error
}

func (m *mockManager) SetAgencyDetails(_ context.Context, _ string) (codevaldagency.Agency, error) {
	return m.setDetailsResult, m.setDetailsErr
}
func (m *mockManager) GetAgency(_ context.Context) (codevaldagency.Agency, error) {
	return m.getResult, m.getErr
}
func (m *mockManager) UpdateAgency(_ context.Context, _ codevaldagency.UpdateAgencyRequest) (codevaldagency.Agency, error) {
	return m.updateResult, m.updateErr
}
func (m *mockManager) PublishAgency(_ context.Context) (codevaldagency.AgencyPublication, error) {
	return m.publishResult, m.publishErr
}
func (m *mockManager) GetPublication(_ context.Context, _ int) (codevaldagency.AgencyPublication, error) {
	return m.getPublicationResult(), m.getPublicationErr()
}
func (m *mockManager) getPublicationResult() codevaldagency.AgencyPublication { return m.getPubResult }
func (m *mockManager) getPublicationErr() error                                { return m.getPubErr }
func (m *mockManager) ListPublications(_ context.Context) ([]codevaldagency.AgencyPublication, error) {
	return m.listPubResult, m.listPubErr
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
	srv := server.New(mgr)
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
	srv := server.New(mgr)
	_, err := srv.SetAgencyDetails(context.Background(), &pb.SetAgencyDetailsRequest{Json: "bad"})
	requireCode(t, err, codes.InvalidArgument)
}

func TestServer_SetAgencyDetails_BackendError_ReturnsInternal(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{setDetailsErr: fmt.Errorf("database failure")}
	srv := server.New(mgr)
	_, err := srv.SetAgencyDetails(context.Background(), &pb.SetAgencyDetailsRequest{Json: `{"id":"a1"}`})
	requireCode(t, err, codes.Internal)
}

// ── GetAgency ────────────────────────────────────────────────────────────────────────────

func TestServer_GetAgency_OK(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{getResult: codevaldagency.Agency{ID: "a2", Name: "Beta"}}
	srv := server.New(mgr)
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
	srv := server.New(mgr)
	_, err := srv.GetAgency(context.Background(), &pb.GetAgencyRequest{})
	requireCode(t, err, codes.NotFound)
}

// ── UpdateAgency ───────────────────────────────────────────────────────────────────────────

func TestServer_UpdateAgency_OK(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{updateResult: codevaldagency.Agency{
		ID:     "a3",
		Status: codevaldagency.LifecycleActive,
	}}
	srv := server.New(mgr)
	got, err := srv.UpdateAgency(context.Background(), &pb.UpdateAgencyRequest{
		Status: pb.AgencyLifecycle_AGENCY_LIFECYCLE_ACTIVE,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetStatus() != pb.AgencyLifecycle_AGENCY_LIFECYCLE_ACTIVE {
		t.Errorf("Status: want ACTIVE, got %v", got.GetStatus())
	}
}

func TestServer_UpdateAgency_InvalidTransition_ReturnsFailedPrecondition(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{updateErr: codevaldagency.ErrInvalidLifecycleTransition}
	srv := server.New(mgr)
	_, err := srv.UpdateAgency(context.Background(), &pb.UpdateAgencyRequest{
		Status: pb.AgencyLifecycle_AGENCY_LIFECYCLE_DRAFT,
	})
	requireCode(t, err, codes.FailedPrecondition)
}

func TestServer_UpdateAgency_NotFound(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{updateErr: codevaldagency.ErrAgencyNotFound}
	srv := server.New(mgr)
	_, err := srv.UpdateAgency(context.Background(), &pb.UpdateAgencyRequest{})
	requireCode(t, err, codes.NotFound)
}

// ── PublishAgency ───────────────────────────────────────────────────────────────────────

func TestServer_PublishAgency_OK(t *testing.T) {
	t.Parallel()
	agency := codevaldagency.Agency{ID: "a1", Name: "Alpha", Status: codevaldagency.LifecycleDraft}
	mgr := &mockManager{
		publishResult: codevaldagency.AgencyPublication{
			ID:      "pub-1",
			Agency:  agency,
			Version: 1,
			Tag:     "v1",
		},
	}
	srv := server.New(mgr)
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
	if got.GetAgency().GetId() != "a1" {
		t.Errorf("Agency.ID: want %q, got %q", "a1", got.GetAgency().GetId())
	}
}

func TestServer_PublishAgency_NotFound_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{publishErr: codevaldagency.ErrAgencyNotFound}
	srv := server.New(mgr)
	_, err := srv.PublishAgency(context.Background(), &pb.PublishAgencyRequest{})
	requireCode(t, err, codes.NotFound)
}

// ── GetPublication ─────────────────────────────────────────────────────────────────────

func TestServer_GetPublication_OK(t *testing.T) {
	t.Parallel()
	agency := codevaldagency.Agency{ID: "a1", Name: "Alpha", Status: codevaldagency.LifecycleDraft}
	mgr := &mockManager{
		getPubResult: codevaldagency.AgencyPublication{
			ID:      "pub-1",
			Agency:  agency,
			Version: 1,
			Tag:     "v1",
		},
	}
	srv := server.New(mgr)
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
	srv := server.New(mgr)
	_, err := srv.GetPublication(context.Background(), &pb.GetPublicationRequest{Version: 99})
	requireCode(t, err, codes.NotFound)
}

// ── ListPublications ───────────────────────────────────────────────────────────────────

func TestServer_ListPublications_OK(t *testing.T) {
	t.Parallel()
	agency := codevaldagency.Agency{ID: "a1", Name: "Alpha"}
	mgr := &mockManager{
		listPubResult: []codevaldagency.AgencyPublication{
			{ID: "pub-1", Agency: agency, Version: 1, Tag: "v1"},
			{ID: "pub-2", Agency: agency, Version: 2, Tag: "v2"},
		},
	}
	srv := server.New(mgr)
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
	srv := server.New(mgr)
	got, err := srv.ListPublications(context.Background(), &pb.ListPublicationsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.GetPublications()) != 0 {
		t.Errorf("expected empty list, got %d", len(got.GetPublications()))
	}
}
