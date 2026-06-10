// Package server — import_event_flow_steps.go
//
// Projects the opaque event_flows JSON blob attached to each Workflow into
// per-step DraftEventFlowStep entities. The blob remains on the DraftWorkflow
// for round-trip / debugging; the per-step entities are the queryable form
// used by downstream services (BUG-20260610-002).
//
// One DraftEventFlowStep per `steps[*]` entry. The entity carries the same
// fields the source flow file declares — step number, trigger topic,
// consumer, handler code, emits_topics, on-error emits — flattened from the
// nested action / on-error objects into top-level properties.
//
// Promotion to live EventFlowStep is handled by PromoteDraft alongside the
// rest of the draft sub-graph.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// importEventFlowSteps parses eventFlowsJSON (the opaque
// { flows: [ { name, steps: [...] } ] } block) and upserts one
// DraftEventFlowStep entity per step. Returns the number of steps upserted.
//
// eventFlowsJSON is the JSON-encoded value of wf.EventFlows; the caller has
// already validated it parses as JSON via json.Marshal earlier in the import.
func (s *Server) importEventFlowSteps(ctx context.Context, agencyID, draftID, draftWorkflowRefCode, workflowCode, eventFlowsJSON string) (int, error) {
	var doc importFlowDoc
	if err := json.Unmarshal([]byte(eventFlowsJSON), &doc); err != nil {
		return 0, fmt.Errorf("unmarshal event_flows: %w", err)
	}

	count := 0
	for _, flow := range doc.Flows {
		// flow.Name often matches workflowCode (e.g. "planning"), but the JSON
		// authoritatively declares it — prefer the JSON value when set.
		flowName := flow.Name
		if flowName == "" {
			flowName = workflowCode
		}
		for _, step := range flow.Steps {
			props := flowStepProperties(draftID, draftWorkflowRefCode, flowName, step)
			if err := s.importUpsert(ctx, agencyID, "DraftEventFlowStep", props); err != nil {
				return count, fmt.Errorf("upsert DraftEventFlowStep %s: %w", props["code"], err)
			}
			count++
		}
	}
	return count, nil
}

// flowStepProperties flattens one importFlowStep into the property map
// expected by DraftEventFlowStep. The deterministic code "<flow>:<step>"
// is the UniqueKey enforcement key, so re-imports update in place.
func flowStepProperties(draftID, draftWorkflowRefCode, flowName string, step importFlowStep) map[string]any {
	props := map[string]any{
		"draft_ref_code":          draftID,
		"draft_workflow_ref_code": draftWorkflowRefCode,
		"workflow_code":           flowName,
		"code":                    flowName + ":" + step.Step,
		"step":                    step.Step,
		"name":                    step.Name,
		"description":             strings.TrimSpace(step.Description),
		"step_type":               step.Type,
		"trigger_topic":           step.Trigger,
		"trigger_publisher":       step.TriggerPublisher,
		"consumer":                step.Consumer,
	}
	// emits_topics / handler live under nested action / on-error blocks in the
	// flow file; surface them as flat properties so downstream code never has
	// to re-parse the original JSON.
	if step.Action != nil {
		props["handler_code"] = step.Action.Handler
		if len(step.Action.EmitsTopics) > 0 {
			props["emits_topics"] = strings.Join(step.Action.EmitsTopics, ",")
		}
	}
	if step.OnError != nil && len(step.OnError.EmitsTopics) > 0 {
		props["on_error_emits_topics"] = strings.Join(step.OnError.EmitsTopics, ",")
	}
	// Start steps declare emits_topics at the top level (entry-point events)
	// instead of inside an action block. Preserve that when present.
	if props["emits_topics"] == nil && len(step.EmitsTopics) > 0 {
		props["emits_topics"] = strings.Join(step.EmitsTopics, ",")
	}
	return props
}

// ── JSON schema types for the event_flows blob ───────────────────────────────

type importFlowDoc struct {
	Flows []importFlow `json:"flows"`
}

type importFlow struct {
	Name  string           `json:"name"`
	Steps []importFlowStep `json:"steps"`
}

// importFlowStep mirrors the shape declared in FLOWS_FORMAT.md.
// All fields are optional except `step` and `name`; not every step has a
// trigger, consumer, action, or on-error block.
type importFlowStep struct {
	Step             string           `json:"step"`
	Name             string           `json:"name"`
	Type             string           `json:"type"`
	Description      string           `json:"description"`
	Trigger          string           `json:"trigger"`
	TriggerPublisher string           `json:"trigger_publisher"`
	Consumer         string           `json:"consumer"`
	EmitsTopics      []string         `json:"emits_topics"`
	Action           *importFlowBlock `json:"action,omitempty"`
	OnError          *importFlowBlock `json:"on-error,omitempty"`
}

type importFlowBlock struct {
	Handler     string   `json:"handler"`
	EmitsTopics []string `json:"emits_topics"`
}
