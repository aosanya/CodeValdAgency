// Package codevaldagency — event_flow_steps.go
//
// Manager-side implementation of [AgencyManager.LookupFlowStep] and
// [AgencyManager.ListEventFlowSteps] (BUG-20260610-002). These methods query
// the live EventFlowStep entities produced by ImportDraft+PromoteDraft and
// give downstream services (CodeValdWork, CodeValdAI) a structured view of
// the active publication's flow steps.
//
// The "active publication" is implicit in the entity store: this service hosts
// exactly one agency (per CLAUDE.md), and PromoteDraft deletes the previous
// publication's EventFlowStep entities before creating the new generation, so
// every live EventFlowStep is by definition part of the currently-promoted
// publication.
package codevaldagency

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
)

// LookupFlowStep finds a single live EventFlowStep matching one of the lookup
// keys carried by lookup. Returns ErrEventFlowStepNotFound if no step matches,
// or ErrInvalidLookup if none of the keys are populated.
//
// Lookup precedence — alternate keys, evaluated independently:
//   1. lookup.HandlerCode (most common — Work resolves "this AgentRun came
//      from handler X — what step is that?").
//   2. lookup.TriggerTopic + lookup.Consumer (dispatchers asking "for an event
//      on topic T routed to service S, which step runs?").
//   3. lookup.Code (deterministic "<workflow>:<step>" identifier).
//
// At least one shape must be set; combinations are not mixed. If more than
// one shape is populated, HandlerCode wins, then (TriggerTopic, Consumer),
// then Code.
func (m *agencyManager) LookupFlowStep(ctx context.Context, lookup EventFlowStepLookup) (EventFlowStep, error) {
	if lookup.HandlerCode == "" && lookup.Code == "" &&
		(lookup.TriggerTopic == "" || lookup.Consumer == "") {
		return EventFlowStep{}, ErrInvalidLookup
	}

	all, err := m.listLiveEventFlowSteps(ctx)
	if err != nil {
		return EventFlowStep{}, fmt.Errorf("LookupFlowStep: %w", err)
	}

	for _, s := range all {
		switch {
		case lookup.HandlerCode != "" && s.HandlerCode == lookup.HandlerCode:
			return s, nil
		case lookup.TriggerTopic != "" && lookup.Consumer != "" &&
			s.TriggerTopic == lookup.TriggerTopic && s.Consumer == lookup.Consumer:
			return s, nil
		case lookup.Code != "" && s.Code == lookup.Code:
			return s, nil
		}
	}
	return EventFlowStep{}, ErrEventFlowStepNotFound
}

// ListEventFlowSteps returns every live EventFlowStep in the active publication,
// optionally filtered to a single workflow by code (empty matches all).
// Steps are returned in (workflow_code, ordinality, step) ascending order so
// callers see a deterministic traversal.
func (m *agencyManager) ListEventFlowSteps(ctx context.Context, workflowCode string) ([]EventFlowStep, error) {
	all, err := m.listLiveEventFlowSteps(ctx)
	if err != nil {
		return nil, fmt.Errorf("ListEventFlowSteps: %w", err)
	}
	if workflowCode == "" {
		sortEventFlowSteps(all)
		return all, nil
	}
	out := all[:0]
	for _, s := range all {
		if s.WorkflowCode == workflowCode {
			out = append(out, s)
		}
	}
	sortEventFlowSteps(out)
	return out, nil
}

// listLiveEventFlowSteps fetches every EventFlowStep entity for this agency
// from storage and projects each entity into the EventFlowStep struct.
func (m *agencyManager) listLiveEventFlowSteps(ctx context.Context) ([]EventFlowStep, error) {
	entities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: m.agencyID,
		TypeID:   "EventFlowStep",
	})
	if err != nil {
		return nil, fmt.Errorf("list EventFlowStep: %w", err)
	}
	out := make([]EventFlowStep, 0, len(entities))
	for _, e := range entities {
		out = append(out, eventFlowStepFromEntity(e))
	}
	return out, nil
}

// eventFlowStepFromEntity decodes a stored entity into the Go value type.
func eventFlowStepFromEntity(e entitygraph.Entity) EventFlowStep {
	return EventFlowStep{
		ID:                 e.ID,
		Code:               strProperty(e, "code"),
		WorkflowCode:       strProperty(e, "workflow_code"),
		Step:               strProperty(e, "step"),
		Name:               strProperty(e, "name"),
		Description:        strProperty(e, "description"),
		StepType:           strProperty(e, "step_type"),
		TriggerTopic:       strProperty(e, "trigger_topic"),
		TriggerPublisher:   strProperty(e, "trigger_publisher"),
		Consumer:           strProperty(e, "consumer"),
		HandlerCode:        strProperty(e, "handler_code"),
		EmitsTopics:        strProperty(e, "emits_topics"),
		OnErrorEmitsTopics: strProperty(e, "on_error_emits_topics"),
		Ordinality:         intProperty(e, "ordinality"),
	}
}

// sortEventFlowSteps sorts in place by (WorkflowCode, Ordinality, Step) ASC.
// Step comparison falls back to lexical when ordinality ties, which matches
// the natural "1" < "1.1" < "1.1.1" ordering of dotted step numbers.
func sortEventFlowSteps(steps []EventFlowStep) {
	sort.SliceStable(steps, func(i, j int) bool {
		if steps[i].WorkflowCode != steps[j].WorkflowCode {
			return steps[i].WorkflowCode < steps[j].WorkflowCode
		}
		if steps[i].Ordinality != steps[j].Ordinality {
			return steps[i].Ordinality < steps[j].Ordinality
		}
		return steps[i].Step < steps[j].Step
	})
}

// strProperty pulls a string-typed property from an entity, defaulting to "".
func strProperty(e entitygraph.Entity, key string) string {
	if v, ok := e.Properties[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// intProperty pulls an integer-typed property from an entity, defaulting to 0.
// Handles both the native int representation and the float64 ArangoDB returns
// for JSON-decoded number properties.
func intProperty(e entitygraph.Entity, key string) int {
	switch v := e.Properties[key].(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}
