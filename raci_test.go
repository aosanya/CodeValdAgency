package codevaldagency_test

import (
	"context"
	"errors"
	"testing"

	codevaldagency "github.com/aosanya/CodeValdAgency"
)

func ptrStr(s string) *string { return &s }

// mustSetupWorkPlan creates a role via CreateRole, failing the test on any error.
func mustSetupWorkPlan(t *testing.T, mgr codevaldagency.AgencyManager, topic string, enabled bool, ordinality int) codevaldagency.WorkPlan {
	t.Helper()
	role, err := mgr.CreateWorkPlan(context.Background(), codevaldagency.CreateWorkPlanRequest{
		Name:       "role-" + topic,
		TriggerTopic: topic,
		Enabled:    enabled,
		Ordinality: ordinality,
	})
	if err != nil {
		t.Fatalf("CreateRole(%q): %v", topic, err)
	}
	return role
}

// ── CreateRole ────────────────────────────────────────────────────────────────

func TestCreateRole_Valid_StoresEntityAndEdge(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")

	role, err := mgr.CreateWorkPlan(context.Background(), codevaldagency.CreateWorkPlanRequest{
		Name:             "dispatcher",
		TriggerTopic:       `work\.task\..*`,
		PayloadCondition: `"status":"open"`,
		Enabled:          true,
		Ordinality:       1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role.ID == "" {
		t.Error("expected non-empty role ID")
	}
	if role.Name != "dispatcher" {
		t.Errorf("Name: want %q, got %q", "dispatcher", role.Name)
	}
	if role.TriggerTopic != `work\.task\..*` {
		t.Errorf("TriggerTopic: want %q, got %q", `work\.task\..*`, role.TriggerTopic)
	}
	if !role.Enabled {
		t.Error("expected Enabled=true")
	}

	// Role should be retrievable.
	got, err := mgr.GetWorkPlan(context.Background(), role.ID)
	if err != nil {
		t.Fatalf("GetRole: %v", err)
	}
	if got.ID != role.ID {
		t.Errorf("GetRole ID: want %q, got %q", role.ID, got.ID)
	}
}

func TestCreateRole_InvalidEventTopic_ReturnsErrInvalidRegex(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")

	_, err := mgr.CreateWorkPlan(context.Background(), codevaldagency.CreateWorkPlanRequest{
		Name:       "bad",
		TriggerTopic: `[invalid`,
	})
	if !errors.Is(err, codevaldagency.ErrInvalidRegex) {
		t.Fatalf("expected ErrInvalidRegex, got %v", err)
	}
}

func TestCreateRole_InvalidPayloadCondition_ReturnsErrInvalidRegex(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")

	_, err := mgr.CreateWorkPlan(context.Background(), codevaldagency.CreateWorkPlanRequest{
		Name:             "bad",
		TriggerTopic:       `work\.task\..*`,
		PayloadCondition: `(unclosed`,
	})
	if !errors.Is(err, codevaldagency.ErrInvalidRegex) {
		t.Fatalf("expected ErrInvalidRegex, got %v", err)
	}
}

// ── GetRole ───────────────────────────────────────────────────────────────────

func TestGetRole_NotFound_ReturnsErrWorkPlanNotFound(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)

	_, err := mgr.GetWorkPlan(context.Background(), "no-such-id")
	if !errors.Is(err, codevaldagency.ErrWorkPlanNotFound) {
		t.Fatalf("expected ErrWorkPlanNotFound, got %v", err)
	}
}

func TestGetRole_RoundTrip(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")
	role := mustSetupWorkPlan(t, mgr, `work\.task\..*`, true, 5)

	got, err := mgr.GetWorkPlan(context.Background(), role.ID)
	if err != nil {
		t.Fatalf("GetRole: %v", err)
	}
	if got.Ordinality != 5 {
		t.Errorf("Ordinality: want 5, got %d", got.Ordinality)
	}
}

// ── ListRoles ─────────────────────────────────────────────────────────────────

func TestListRoles_EmptyBeforeAnyCreate(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)

	roles, err := mgr.ListWorkPlans(context.Background())
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	if len(roles) != 0 {
		t.Errorf("expected 0 roles, got %d", len(roles))
	}
}

func TestListRoles_ReturnsAll(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")
	mustSetupWorkPlan(t, mgr, `work\.task\..*`, true, 1)
	mustSetupWorkPlan(t, mgr, `git\.push\..*`, true, 2)

	roles, err := mgr.ListWorkPlans(context.Background())
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	if len(roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(roles))
	}
}

// ── UpdateRole ────────────────────────────────────────────────────────────────

func TestUpdateRole_NotFound_ReturnsErrWorkPlanNotFound(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)

	_, err := mgr.UpdateWorkPlan(context.Background(), "no-such-id", codevaldagency.UpdateWorkPlanRequest{
		TriggerTopic: ptrStr(`work\..*`),
	})
	if !errors.Is(err, codevaldagency.ErrWorkPlanNotFound) {
		t.Fatalf("expected ErrWorkPlanNotFound, got %v", err)
	}
}

func TestUpdateRole_InvalidRegex_ReturnsErrInvalidRegex(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")
	role := mustSetupWorkPlan(t, mgr, `work\..*`, true, 1)

	_, err := mgr.UpdateWorkPlan(context.Background(), role.ID, codevaldagency.UpdateWorkPlanRequest{
		TriggerTopic: ptrStr(`[bad`),
	})
	if !errors.Is(err, codevaldagency.ErrInvalidRegex) {
		t.Fatalf("expected ErrInvalidRegex, got %v", err)
	}
}

// ── DeleteRole ────────────────────────────────────────────────────────────────

func TestDeleteRole_NotFound_ReturnsErrWorkPlanNotFound(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)

	err := mgr.DeleteWorkPlan(context.Background(), "no-such-id")
	if !errors.Is(err, codevaldagency.ErrWorkPlanNotFound) {
		t.Fatalf("expected ErrWorkPlanNotFound, got %v", err)
	}
}

func TestDeleteRole_RemovesEntity(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")
	role := mustSetupWorkPlan(t, mgr, `work\..*`, true, 1)

	if err := mgr.DeleteWorkPlan(context.Background(), role.ID); err != nil {
		t.Fatalf("DeleteRole: %v", err)
	}
	_, err := mgr.GetWorkPlan(context.Background(), role.ID)
	if !errors.Is(err, codevaldagency.ErrWorkPlanNotFound) {
		t.Fatalf("expected ErrWorkPlanNotFound after delete, got %v", err)
	}
}

// ── MatchRoles ────────────────────────────────────────────────────────────────

func TestMatchRoles_ExactTopic_Matches(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")
	role := mustSetupWorkPlan(t, mgr, `work\.task\.status\.changed`, true, 1)

	matches, err := mgr.MatchWorkPlans(context.Background(), "work.task.status.changed", `{}`)
	if err != nil {
		t.Fatalf("MatchRoles: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].WorkPlan.ID != role.ID {
		t.Errorf("Role.ID: want %q, got %q", role.ID, matches[0].WorkPlan.ID)
	}
}

func TestMatchRoles_WildcardTopic_MatchesBoth(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")
	mustSetupWorkPlan(t, mgr, `work\.task\..*`, true, 1)

	topics := []string{"work.task.status.changed", "work.task.completed"}
	for _, topic := range topics {
		matches, err := mgr.MatchWorkPlans(context.Background(), topic, `{}`)
		if err != nil {
			t.Fatalf("MatchRoles(%q): %v", topic, err)
		}
		if len(matches) != 1 {
			t.Errorf("topic %q: expected 1 match, got %d", topic, len(matches))
		}
	}
}

func TestMatchRoles_NonMatchingTopic_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")
	mustSetupWorkPlan(t, mgr, `work\.task\..*`, true, 1)

	matches, err := mgr.MatchWorkPlans(context.Background(), "git.push.completed", `{}`)
	if err != nil {
		t.Fatalf("MatchRoles: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for non-matching topic, got %d", len(matches))
	}
}

func TestMatchRoles_PayloadCondition_FiltersByContent(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")

	// Role only fires when payload contains "To":"in_progress".
	_, err := mgr.CreateWorkPlan(context.Background(), codevaldagency.CreateWorkPlanRequest{
		Name:             "in-progress-role",
		TriggerTopic:       `work\.task\..*`,
		PayloadCondition: `"To":"in_progress"`,
		Enabled:          true,
		Ordinality:       1,
	})
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}

	// Matching payload.
	matches, err := mgr.MatchWorkPlans(context.Background(), "work.task.status.changed", `{"To":"in_progress"}`)
	if err != nil {
		t.Fatalf("MatchRoles (matching): %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("expected 1 match for matching payload, got %d", len(matches))
	}

	// Non-matching payload.
	matches, err = mgr.MatchWorkPlans(context.Background(), "work.task.status.changed", `{"To":"done"}`)
	if err != nil {
		t.Fatalf("MatchRoles (non-matching): %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for non-matching payload, got %d", len(matches))
	}
}

func TestMatchRoles_EmptyPayloadCondition_MatchesAll(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")
	mustSetupWorkPlan(t, mgr, `work\.task\..*`, true, 1) // PayloadCondition is empty

	payloads := []string{`{}`, `{"To":"done"}`, `{"anything":"goes"}`}
	for _, p := range payloads {
		matches, err := mgr.MatchWorkPlans(context.Background(), "work.task.status.changed", p)
		if err != nil {
			t.Fatalf("MatchRoles(%q): %v", p, err)
		}
		if len(matches) != 1 {
			t.Errorf("payload %q: expected 1 match (empty condition), got %d", p, len(matches))
		}
	}
}

func TestMatchRoles_DisabledRole_Excluded(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")
	mustSetupWorkPlan(t, mgr, `work\.task\..*`, false /* disabled */, 1)

	matches, err := mgr.MatchWorkPlans(context.Background(), "work.task.status.changed", `{}`)
	if err != nil {
		t.Fatalf("MatchRoles: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected disabled role to be excluded, got %d match(es)", len(matches))
	}
}

func TestMatchRoles_OrderedByOrdinality(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")
	// Create in reverse ordinality order.
	mustSetupWorkPlan(t, mgr, `work\.task\..*`, true, 30)
	mustSetupWorkPlan(t, mgr, `work\.task\..*`, true, 10)
	mustSetupWorkPlan(t, mgr, `work\.task\..*`, true, 20)

	matches, err := mgr.MatchWorkPlans(context.Background(), "work.task.status.changed", `{}`)
	if err != nil {
		t.Fatalf("MatchRoles: %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(matches))
	}
	for i := 1; i < len(matches); i++ {
		if matches[i].WorkPlan.Ordinality < matches[i-1].WorkPlan.Ordinality {
			t.Errorf("matches not ordered by ordinality: [%d]=%d > [%d]=%d",
				i-1, matches[i-1].WorkPlan.Ordinality, i, matches[i].WorkPlan.Ordinality)
		}
	}
}

// ── AddContextSource / ListContextSources ─────────────────────────────────────

func TestAddContextSource_Git_RoundTrip(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")
	role := mustSetupWorkPlan(t, mgr, `work\.task\..*`, true, 1)

	src, err := mgr.AddContextSource(context.Background(), role.ID, codevaldagency.AddContextSourceRequest{
		SourceType: codevaldagency.ContextSourceGit,
		Git: &codevaldagency.GitContextSourceConfig{
			Signals:    "authority,contributor",
			MaxResults: 20,
			MatchMode:  "OR",
			Cascade:    true,
			FileTypes:  ".go,.ts",
		},
	})
	if err != nil {
		t.Fatalf("AddContextSource(Git): %v", err)
	}
	if src.SourceType != codevaldagency.ContextSourceGit {
		t.Errorf("SourceType: want %q, got %q", codevaldagency.ContextSourceGit, src.SourceType)
	}
	if src.Git == nil {
		t.Fatal("expected non-nil Git config")
	}
	if src.Git.Signals != "authority,contributor" {
		t.Errorf("Signals: want %q, got %q", "authority,contributor", src.Git.Signals)
	}
	if src.Git.MaxResults != 20 {
		t.Errorf("MaxResults: want 20, got %d", src.Git.MaxResults)
	}
	if !src.Git.Cascade {
		t.Error("expected Cascade=true")
	}

	sources, err := mgr.ListContextSources(context.Background(), role.ID)
	if err != nil {
		t.Fatalf("ListContextSources: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0].ID != src.ID {
		t.Errorf("source ID mismatch: want %q, got %q", src.ID, sources[0].ID)
	}
}

func TestAddContextSource_AllThreeTypes_AllReturnedByList(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")
	role := mustSetupWorkPlan(t, mgr, `work\.task\..*`, true, 1)

	reqs := []codevaldagency.AddContextSourceRequest{
		{
			SourceType: codevaldagency.ContextSourceGit,
			Git:        &codevaldagency.GitContextSourceConfig{Signals: "authority"},
		},
		{
			SourceType: codevaldagency.ContextSourceComm,
			Comm:       &codevaldagency.CommContextSourceConfig{LookbackDays: 7, MaxResults: 10},
		},
		{
			SourceType: codevaldagency.ContextSourceWork,
			Work:       &codevaldagency.WorkContextSourceConfig{IncludeDescription: true, IncludeHistory: true},
		},
	}
	for _, req := range reqs {
		if _, err := mgr.AddContextSource(context.Background(), role.ID, req); err != nil {
			t.Fatalf("AddContextSource(%s): %v", req.SourceType, err)
		}
	}

	sources, err := mgr.ListContextSources(context.Background(), role.ID)
	if err != nil {
		t.Fatalf("ListContextSources: %v", err)
	}
	if len(sources) != 3 {
		t.Fatalf("expected 3 sources, got %d", len(sources))
	}

	typesSeen := make(map[codevaldagency.ContextSourceType]bool)
	for _, s := range sources {
		typesSeen[s.SourceType] = true
	}
	for _, want := range []codevaldagency.ContextSourceType{
		codevaldagency.ContextSourceGit,
		codevaldagency.ContextSourceComm,
		codevaldagency.ContextSourceWork,
	} {
		if !typesSeen[want] {
			t.Errorf("source type %q not found in ListContextSources result", want)
		}
	}
}

func TestAddContextSource_Comm_TypedFieldsRoundTrip(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")
	role := mustSetupWorkPlan(t, mgr, `work\.task\..*`, true, 1)

	src, err := mgr.AddContextSource(context.Background(), role.ID, codevaldagency.AddContextSourceRequest{
		SourceType: codevaldagency.ContextSourceComm,
		Comm:       &codevaldagency.CommContextSourceConfig{LookbackDays: 14, MaxResults: 5},
	})
	if err != nil {
		t.Fatalf("AddContextSource(Comm): %v", err)
	}
	if src.Comm == nil {
		t.Fatal("expected non-nil Comm config")
	}
	if src.Comm.LookbackDays != 14 {
		t.Errorf("LookbackDays: want 14, got %d", src.Comm.LookbackDays)
	}
	if src.Comm.MaxResults != 5 {
		t.Errorf("MaxResults: want 5, got %d", src.Comm.MaxResults)
	}
}

func TestAddContextSource_Work_TypedFieldsRoundTrip(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")
	role := mustSetupWorkPlan(t, mgr, `work\.task\..*`, true, 1)

	src, err := mgr.AddContextSource(context.Background(), role.ID, codevaldagency.AddContextSourceRequest{
		SourceType: codevaldagency.ContextSourceWork,
		Work:       &codevaldagency.WorkContextSourceConfig{IncludeDescription: true, IncludeHistory: false},
	})
	if err != nil {
		t.Fatalf("AddContextSource(Work): %v", err)
	}
	if src.Work == nil {
		t.Fatal("expected non-nil Work config")
	}
	if !src.Work.IncludeDescription {
		t.Error("expected IncludeDescription=true")
	}
	if src.Work.IncludeHistory {
		t.Error("expected IncludeHistory=false")
	}
}

// ── RemoveContextSource ───────────────────────────────────────────────────────

func TestRemoveContextSource_AbsentFromSubsequentList(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")
	role := mustSetupWorkPlan(t, mgr, `work\.task\..*`, true, 1)

	src, err := mgr.AddContextSource(context.Background(), role.ID, codevaldagency.AddContextSourceRequest{
		SourceType: codevaldagency.ContextSourceGit,
		Git:        &codevaldagency.GitContextSourceConfig{Signals: "authority"},
	})
	if err != nil {
		t.Fatalf("AddContextSource: %v", err)
	}

	if err := mgr.RemoveContextSource(context.Background(), role.ID, src.ID); err != nil {
		t.Fatalf("RemoveContextSource: %v", err)
	}

	sources, err := mgr.ListContextSources(context.Background(), role.ID)
	if err != nil {
		t.Fatalf("ListContextSources after remove: %v", err)
	}
	for _, s := range sources {
		if s.ID == src.ID {
			t.Errorf("removed source %q still present in ListContextSources", src.ID)
		}
	}
}

func TestRemoveContextSource_NotFound_ReturnsErrContextSourceNotFound(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")
	role := mustSetupWorkPlan(t, mgr, `work\.task\..*`, true, 1)

	err := mgr.RemoveContextSource(context.Background(), role.ID, "no-such-source")
	if !errors.Is(err, codevaldagency.ErrContextSourceNotFound) {
		t.Fatalf("expected ErrContextSourceNotFound, got %v", err)
	}
}

// ── ListContextSources ────────────────────────────────────────────────────────

func TestListContextSources_RoleNotFound_ReturnsErrWorkPlanNotFound(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)

	_, err := mgr.ListContextSources(context.Background(), "no-such-role")
	if !errors.Is(err, codevaldagency.ErrWorkPlanNotFound) {
		t.Fatalf("expected ErrWorkPlanNotFound, got %v", err)
	}
}

// ── MatchRoles returns ContextSources ─────────────────────────────────────────

func TestMatchRoles_ReturnsContextSources(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)
	mustSetupAgency(t, mgr, "a1", "Alpha")
	role := mustSetupWorkPlan(t, mgr, `work\.task\..*`, true, 1)

	if _, err := mgr.AddContextSource(context.Background(), role.ID, codevaldagency.AddContextSourceRequest{
		SourceType: codevaldagency.ContextSourceWork,
		Work:       &codevaldagency.WorkContextSourceConfig{IncludeDescription: true},
	}); err != nil {
		t.Fatalf("AddContextSource: %v", err)
	}

	matches, err := mgr.MatchWorkPlans(context.Background(), "work.task.status.changed", `{}`)
	if err != nil {
		t.Fatalf("MatchRoles: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if len(matches[0].ContextSources) != 1 {
		t.Errorf("expected 1 context source in match, got %d", len(matches[0].ContextSources))
	}
	if matches[0].ContextSources[0].SourceType != codevaldagency.ContextSourceWork {
		t.Errorf("SourceType: want %q, got %q", codevaldagency.ContextSourceWork, matches[0].ContextSources[0].SourceType)
	}
}
