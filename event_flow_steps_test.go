package codevaldagency_test

import (
	"context"
	"errors"
	"testing"

	codevaldagency "github.com/aosanya/CodeValdAgency"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
)

// TestLookupFlowStep_ByHandlerCode verifies the most common lookup path:
// Work resolves "this AgentRun came from handler X" by handler_code.
func TestLookupFlowStep_ByHandlerCode(t *testing.T) {
	t.Parallel()
	mgr, dm := mustNewManager(t)
	seedSteps(dm, "test-agency",
		stepEntity("planning:1.1", "planning", "1.1", "planner-assigned-handler", "task.assigned", "codevaldai"),
		stepEntity("planning:1.1.1.2", "planning", "1.1.1.2", "developer-assigned-handler", "task.request-decompose", "codevaldai"),
	)

	got, err := mgr.LookupFlowStep(context.Background(), codevaldagency.EventFlowStepLookup{
		HandlerCode: "developer-assigned-handler",
	})
	if err != nil {
		t.Fatalf("LookupFlowStep returned error: %v", err)
	}
	if got.HandlerCode != "developer-assigned-handler" || got.Step != "1.1.1.2" {
		t.Errorf("wrong step matched: %+v", got)
	}
}

// TestLookupFlowStep_ByTriggerAndConsumer verifies the dispatcher lookup —
// answering "for an event on topic T routed to service S, which step runs?"
func TestLookupFlowStep_ByTriggerAndConsumer(t *testing.T) {
	t.Parallel()
	mgr, dm := mustNewManager(t)
	seedSteps(dm, "test-agency",
		stepEntity("planning:1.1", "planning", "1.1", "planner-assigned-handler", "task.assigned", "codevaldai"),
		stepEntity("planning:1.1.1.2.1", "planning", "1.1.1.2.1", "", "task.todo", "codevaldwork"),
	)

	got, err := mgr.LookupFlowStep(context.Background(), codevaldagency.EventFlowStepLookup{
		TriggerTopic: "task.todo",
		Consumer:     "codevaldwork",
	})
	if err != nil {
		t.Fatalf("LookupFlowStep returned error: %v", err)
	}
	if got.Step != "1.1.1.2.1" || got.Consumer != "codevaldwork" {
		t.Errorf("wrong step matched: %+v", got)
	}
}

// TestLookupFlowStep_NotFound asserts NotFound when no step matches.
func TestLookupFlowStep_NotFound(t *testing.T) {
	t.Parallel()
	mgr, dm := mustNewManager(t)
	seedSteps(dm, "test-agency",
		stepEntity("planning:1.1", "planning", "1.1", "planner-assigned-handler", "task.assigned", "codevaldai"),
	)

	_, err := mgr.LookupFlowStep(context.Background(), codevaldagency.EventFlowStepLookup{
		HandlerCode: "does-not-exist",
	})
	if !errors.Is(err, codevaldagency.ErrEventFlowStepNotFound) {
		t.Errorf("want ErrEventFlowStepNotFound; got %v", err)
	}
}

// TestLookupFlowStep_InvalidLookup asserts InvalidArgument when no lookup key
// is populated.
func TestLookupFlowStep_InvalidLookup(t *testing.T) {
	t.Parallel()
	mgr, _ := mustNewManager(t)

	_, err := mgr.LookupFlowStep(context.Background(), codevaldagency.EventFlowStepLookup{})
	if !errors.Is(err, codevaldagency.ErrInvalidLookup) {
		t.Errorf("want ErrInvalidLookup; got %v", err)
	}

	// trigger_topic alone (without consumer) is also invalid — the (topic, consumer)
	// lookup requires both halves.
	_, err = mgr.LookupFlowStep(context.Background(), codevaldagency.EventFlowStepLookup{
		TriggerTopic: "task.assigned",
	})
	if !errors.Is(err, codevaldagency.ErrInvalidLookup) {
		t.Errorf("trigger_topic-only lookup should be invalid; got %v", err)
	}
}

// TestListEventFlowSteps_FilteredByWorkflow asserts the workflow filter and
// the (workflow_code, ordinality, step) sort order.
func TestListEventFlowSteps_FilteredByWorkflow(t *testing.T) {
	t.Parallel()
	mgr, dm := mustNewManager(t)
	seedSteps(dm, "test-agency",
		stepEntity("planning:1", "planning", "1", "", "", ""),
		stepEntity("planning:1.1", "planning", "1.1", "planner-assigned-handler", "task.assigned", "codevaldai"),
		stepEntity("merge:1", "merge", "1", "merge-handler", "branch.merged", "codevaldgit"),
	)

	all, err := mgr.ListEventFlowSteps(context.Background(), "")
	if err != nil {
		t.Fatalf("ListEventFlowSteps(empty) returned error: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("ListEventFlowSteps(empty): want 3 steps, got %d", len(all))
	}

	planning, err := mgr.ListEventFlowSteps(context.Background(), "planning")
	if err != nil {
		t.Fatalf("ListEventFlowSteps(planning) returned error: %v", err)
	}
	if len(planning) != 2 {
		t.Fatalf("ListEventFlowSteps(planning): want 2 steps, got %d", len(planning))
	}
	// (workflow_code, ordinality, step) ASC — "1" before "1.1".
	if planning[0].Step != "1" || planning[1].Step != "1.1" {
		t.Errorf("sort order broken: got steps %q, %q", planning[0].Step, planning[1].Step)
	}
}

// ── test helpers ───────────────────────────────────────────────────────────

func stepEntity(code, workflowCode, step, handlerCode, triggerTopic, consumer string) entitygraph.Entity {
	return entitygraph.Entity{
		TypeID: "EventFlowStep",
		Properties: map[string]any{
			"code":          code,
			"workflow_code": workflowCode,
			"step":          step,
			"handler_code":  handlerCode,
			"trigger_topic": triggerTopic,
			"consumer":      consumer,
		},
	}
}

func seedSteps(dm *fakeDataManager, agencyID string, steps ...entitygraph.Entity) {
	for _, s := range steps {
		s.AgencyID = agencyID
		s.ID = dm.nextID()
		dm.addEntity(s)
	}
}
