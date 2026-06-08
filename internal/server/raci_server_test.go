package server_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"

	codevaldagency "github.com/aosanya/CodeValdAgency"
	pb "github.com/aosanya/CodeValdAgency/gen/go/codevaldagency/v1"
	"github.com/aosanya/CodeValdAgency/internal/server"
)

// ── CreateWorkPlan ────────────────────────────────────────────────────────────

func TestServer_CreateWorkPlan_OK(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{
		createWorkPlanResult: codevaldagency.WorkPlan{
			ID:           "wp-1",
			AgencyID:     "a1",
			Name:         "dispatcher",
			TriggerTopic: `work\.task\..*`,
			Enabled:      true,
			Ordinality:   1,
		},
	}
	srv := server.New(mgr, nil, nil)
	got, err := srv.CreateWorkPlan(context.Background(), &pb.CreateWorkPlanRequest{
		Name:         "dispatcher",
		TriggerTopic: `work\.task\..*`,
		Enabled:      true,
		Ordinality:   1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetId() != "wp-1" {
		t.Errorf("ID: want %q, got %q", "wp-1", got.GetId())
	}
	if got.GetName() != "dispatcher" {
		t.Errorf("Name: want %q, got %q", "dispatcher", got.GetName())
	}
	if !got.GetEnabled() {
		t.Error("expected Enabled=true")
	}
}

func TestServer_CreateWorkPlan_InvalidRegex_ReturnsInvalidArgument(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{createWorkPlanErr: codevaldagency.ErrInvalidRegex}
	srv := server.New(mgr, nil, nil)
	_, err := srv.CreateWorkPlan(context.Background(), &pb.CreateWorkPlanRequest{
		Name:         "bad",
		TriggerTopic: `[invalid`,
	})
	requireCode(t, err, codes.InvalidArgument)
}

func TestServer_GetWorkPlan_NotFound_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{getWorkPlanErr: codevaldagency.ErrWorkPlanNotFound}
	srv := server.New(mgr, nil, nil)
	_, err := srv.GetWorkPlan(context.Background(), &pb.GetWorkPlanRequest{WorkPlanId: "missing"})
	requireCode(t, err, codes.NotFound)
}

// ── MatchWorkPlans ────────────────────────────────────────────────────────────

func TestServer_MatchWorkPlans_ReturnsMatchesWithSources(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{
		matchWorkPlansResult: []codevaldagency.WorkPlanMatch{
			{
				WorkPlan: codevaldagency.WorkPlan{
					ID:           "wp-1",
					Name:         "dispatcher",
					TriggerTopic: `work\.task\..*`,
					Enabled:      true,
					Ordinality:   1,
				},
				ContextSources: []codevaldagency.ContextSource{
					{
						ID:         "src-1",
						WorkPlanID: "wp-1",
						SourceType: codevaldagency.ContextSourceGit,
						Git: &codevaldagency.GitContextSourceConfig{
							Signals:    "commit,pr",
							MaxResults: 10,
						},
					},
				},
			},
		},
	}
	srv := server.New(mgr, nil, nil)
	resp, err := srv.MatchWorkPlans(context.Background(), &pb.MatchWorkPlansRequest{
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
	if m.GetWorkPlan().GetId() != "wp-1" {
		t.Errorf("match work plan ID: want %q, got %q", "wp-1", m.GetWorkPlan().GetId())
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

// ── ReviewStep fields round-trip ──────────────────────────────────────────────

func TestServer_CreateWorkPlan_ReviewStepFields_RoundTrip(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{
		createWorkPlanResult: codevaldagency.WorkPlan{
			ID:                 "wp-review",
			Name:               "implement-task",
			ReviewStepType:     "ai_review",
			ReviewTriggerTopic: "work.task.completed",
			ReviewSuccessTopic: "work.review.passed",
			ReviewFailureTopic: "work.review.failed",
		},
	}
	srv := server.New(mgr, nil, nil)
	got, err := srv.CreateWorkPlan(context.Background(), &pb.CreateWorkPlanRequest{
		Name:               "implement-task",
		ReviewStepType:     "ai_review",
		ReviewTriggerTopic: "work.task.completed",
		ReviewSuccessTopic: "work.review.passed",
		ReviewFailureTopic: "work.review.failed",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetReviewStepType() != "ai_review" {
		t.Errorf("ReviewStepType: want %q, got %q", "ai_review", got.GetReviewStepType())
	}
	if got.GetReviewTriggerTopic() != "work.task.completed" {
		t.Errorf("ReviewTriggerTopic: want %q, got %q", "work.task.completed", got.GetReviewTriggerTopic())
	}
	if got.GetReviewSuccessTopic() != "work.review.passed" {
		t.Errorf("ReviewSuccessTopic: want %q, got %q", "work.review.passed", got.GetReviewSuccessTopic())
	}
	if got.GetReviewFailureTopic() != "work.review.failed" {
		t.Errorf("ReviewFailureTopic: want %q, got %q", "work.review.failed", got.GetReviewFailureTopic())
	}
}

func TestServer_CreateWorkPlan_NoReviewStep_FieldsEmpty(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{
		createWorkPlanResult: codevaldagency.WorkPlan{ID: "wp-plain", Name: "plain"},
	}
	srv := server.New(mgr, nil, nil)
	got, err := srv.CreateWorkPlan(context.Background(), &pb.CreateWorkPlanRequest{Name: "plain"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetReviewStepType() != "" {
		t.Errorf("expected empty ReviewStepType, got %q", got.GetReviewStepType())
	}
}

func TestServer_MatchWorkPlans_Empty_ReturnsEmptyList(t *testing.T) {
	t.Parallel()
	mgr := &mockManager{matchWorkPlansResult: nil}
	srv := server.New(mgr, nil, nil)
	resp, err := srv.MatchWorkPlans(context.Background(), &pb.MatchWorkPlansRequest{
		Topic: "unknown.topic",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetMatches()) != 0 {
		t.Errorf("expected 0 matches, got %d", len(resp.GetMatches()))
	}
}
