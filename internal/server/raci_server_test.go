package server_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"

	codevaldagency "github.com/aosanya/CodeValdAgency"
	pb "github.com/aosanya/CodeValdAgency/gen/go/codevaldagency/v1"
	"github.com/aosanya/CodeValdAgency/internal/server"
)

// ── CreateRole ────────────────────────────────────────────────────────────────

func TestServer_CreateRole_OK(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{
		createRoleResult: codevaldagency.Role{
			ID:         "role-1",
			AgencyID:   "a1",
			Name:       "dispatcher",
			EventTopic: `work\.task\..*`,
			Enabled:    true,
			Ordinality: 1,
		},
	}
	srv := server.New(mgr, nil)
	got, err := srv.CreateRole(context.Background(), &pb.CreateRoleRequest{
		Name:       "dispatcher",
		EventTopic: `work\.task\..*`,
		Enabled:    true,
		Ordinality: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetId() != "role-1" {
		t.Errorf("ID: want %q, got %q", "role-1", got.GetId())
	}
	if got.GetName() != "dispatcher" {
		t.Errorf("Name: want %q, got %q", "dispatcher", got.GetName())
	}
	if !got.GetEnabled() {
		t.Error("expected Enabled=true")
	}
}

func TestServer_CreateRole_InvalidRegex_ReturnsInvalidArgument(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{createRoleErr: codevaldagency.ErrInvalidRegex}
	srv := server.New(mgr, nil)
	_, err := srv.CreateRole(context.Background(), &pb.CreateRoleRequest{
		Name:       "bad",
		EventTopic: `[invalid`,
	})
	requireCode(t, err, codes.InvalidArgument)
}

func TestServer_GetRole_NotFound_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{getRoleErr: codevaldagency.ErrRoleNotFound}
	srv := server.New(mgr, nil)
	_, err := srv.GetRole(context.Background(), &pb.GetRoleRequest{RoleId: "missing"})
	requireCode(t, err, codes.NotFound)
}

// ── MatchRoles ────────────────────────────────────────────────────────────────

func TestServer_MatchRoles_ReturnsMatchesWithSources(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{
		matchRolesResult: []codevaldagency.RoleMatch{
			{
				Role: codevaldagency.Role{
					ID:         "role-1",
					Name:       "dispatcher",
					EventTopic: `work\.task\..*`,
					Enabled:    true,
					Ordinality: 1,
				},
				ContextSources: []codevaldagency.ContextSource{
					{
						ID:         "src-1",
						RoleID:     "role-1",
						SourceType: codevaldagency.ContextSourceGit,
						Git: &codevaldagency.GitContextSourceConfig{
							Signals:    []string{"commit", "pr"},
							MaxResults: 10,
						},
					},
				},
			},
		},
	}
	srv := server.New(mgr, nil)
	resp, err := srv.MatchRoles(context.Background(), &pb.MatchRolesRequest{
		Topic:   "work.task.status.changed",
		Payload: `{"status":"open"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetMatches()) != 1 {
		t.Fatalf("expected 1 match, got %d", len(resp.GetMatches()))
	}
	m := resp.GetMatches()[0]
	if m.GetRole().GetId() != "role-1" {
		t.Errorf("match role ID: want %q, got %q", "role-1", m.GetRole().GetId())
	}
	if len(m.GetContextSources()) != 1 {
		t.Fatalf("expected 1 context source, got %d", len(m.GetContextSources()))
	}
	src := m.GetContextSources()[0]
	if src.GetId() != "src-1" {
		t.Errorf("source ID: want %q, got %q", "src-1", src.GetId())
	}
	if src.GetSourceType() != string(codevaldagency.ContextSourceGit) {
		t.Errorf("source type: want %q, got %q", codevaldagency.ContextSourceGit, src.GetSourceType())
	}
	if src.GetGit() == nil {
		t.Fatal("expected non-nil Git config")
	}
	if src.GetGit().GetMaxResults() != 10 {
		t.Errorf("MaxResults: want 10, got %d", src.GetGit().GetMaxResults())
	}
}

func TestServer_MatchRoles_Empty_ReturnsEmptyList(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{matchRolesResult: nil}
	srv := server.New(mgr, nil)
	resp, err := srv.MatchRoles(context.Background(), &pb.MatchRolesRequest{
		Topic: "unknown.topic",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetMatches()) != 0 {
		t.Errorf("expected 0 matches, got %d", len(resp.GetMatches()))
	}
}
