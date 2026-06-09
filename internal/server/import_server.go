// Package server — import_server.go
// Implements the ImportDraft RPC on Server. It parses a raw agency.yaml (or
// agency.json) body that CodeValdCross wraps into ImportDraftRequest.body,
// then performs the full import in one transaction-free sequence:
//
//  1. Set agency details (name, mission, vision, code).
//  2. Create or reuse an open AgencyDraft entity.
//  3. Upsert all DraftConfiguredRole entities.
//  4. Upsert all DraftGoal entities.
//  5. For each DraftWorkflow: upsert the workflow, then its scoped
//     DraftInstruction entities, then each DraftWorkItem (with its own
//     DraftInstruction and DraftDeliverable entities).
//  6. Upsert all WorkPlan entities (live, not draft-scoped).
//
// All entity writes go through entitygraph.DataManager.UpsertEntity — the same
// idempotency path used by EntityService.CreateEntity. Re-running the import
// updates existing entities rather than creating duplicates.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	codevaldagency "github.com/aosanya/CodeValdAgency"
	pb "github.com/aosanya/CodeValdAgency/gen/go/codevaldagency/v1"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"
)

// ── YAML schema types ─────────────────────────────────────────────────────────

type importAgencyYAML struct {
	Agency          importAgencySpec      `yaml:"agency"`
	ConfiguredRoles []importRoleSpec      `yaml:"configured_roles"`
	Goals           []importGoalSpec      `yaml:"goals"`
	Workflows       []importWorkflowSpec  `yaml:"workflows"`
	WorkPlans       []importWorkPlanSpec  `yaml:"work_plans"`
	AIConfig        importAIConfigSpec    `yaml:"ai_config"`
	EventFlows      interface{}            `yaml:"event_flows"`
}

type importAIConfigSpec struct {
	Providers []importAIProviderSpec `yaml:"providers"`
	Agents    []importAIAgentSpec    `yaml:"agents"`
}

type importAIProviderSpec struct {
	Code          string `yaml:"code"`
	Name          string `yaml:"name"`
	ProviderType  string `yaml:"provider_type"`
	APIKeyEnv     string `yaml:"api_key_env"`
	BaseURL       string `yaml:"base_url"`
	ProviderRoute string `yaml:"provider_route"`
}

type importAIAgentSpec struct {
	Code               string  `yaml:"code"`
	Name               string  `yaml:"name"`
	ProviderCode       string  `yaml:"provider_code"`
	Model              string  `yaml:"model"`
	SystemPrompt       string  `yaml:"system_prompt"`
	Temperature        float64 `yaml:"temperature"`
	MaxTokens          int     `yaml:"max_tokens"`
	SessionMaxSeconds  int     `yaml:"session_max_seconds"`
	SessionMaxTokens   int     `yaml:"session_max_tokens"`
	SessionMaxSessions int     `yaml:"session_max_sessions"`
}

type importAgencySpec struct {
	Code    string `yaml:"code"`
	Name    string `yaml:"name"`
	Mission string `yaml:"mission"`
	Vision  string `yaml:"vision"`
	// DefaultFailurePipelineBudget is the per-WorkflowRun recovery-activation
	// cap for runs under this agency. Zero/unset falls back to the
	// WORKFLOW_RUN_FAILURE_BUDGET env default. See FEAT-20260602-007.
	DefaultFailurePipelineBudget int `yaml:"default_failure_pipeline_budget"`
}

type importRoleSpec struct {
	Code        string `yaml:"code"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	ActorType   string `yaml:"actor_type"`
	Ordinality  int    `yaml:"ordinality"`
}

type importGoalSpec struct {
	Code        string `yaml:"code"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Ordinality  int    `yaml:"ordinality"`
}

type importWorkflowSpec struct {
	Code         string                  `yaml:"code"`
	Name         string                  `yaml:"name"`
	Description  string                  `yaml:"description"`
	Ordinality   int                     `yaml:"ordinality"`
	Instructions []importInstructionSpec `yaml:"instructions"`
	WorkItems    []importWorkItemSpec    `yaml:"work_items"`
	// EventFlows accepts the per-workflow { name, steps: [...] } block bundled
	// into agency.json from flows_<workflow.code>.json by the caller. yaml.v3
	// decodes it as interface{}; the importer re-marshals to a JSON string
	// before storing on the DraftWorkflow entity. See FEAT-20260609-002.
	EventFlows interface{} `yaml:"event_flows" json:"event_flows"`
}

type importInstructionSpec struct {
	Code       string `yaml:"code"`
	Content    string `yaml:"content"`
	Ordinality int    `yaml:"ordinality"`
}

type importWorkItemSpec struct {
	Code          string                  `yaml:"code"`
	Title         string                  `yaml:"title"`
	Description   string                  `yaml:"description"`
	Ordinality    int                     `yaml:"ordinality"`
	// AssignedRoles is the plural form used in agency.json (e.g. ["developer"]).
	// The first entry is stored as assigned_role on the DraftWorkItem entity.
	AssignedRoles []string                `yaml:"assigned_roles" json:"assigned_roles"`
	Prompt        string                  `yaml:"prompt"`
	Instructions  []importInstructionSpec `yaml:"instructions"`
	Deliverables  []importDeliverableSpec `yaml:"deliverables"`
}

type importDeliverableSpec struct {
	Code         string `yaml:"code"`
	Title        string `yaml:"title"`
	Description  string `yaml:"description"`
	Ordinality   int    `yaml:"ordinality"`
	Blocking     bool   `yaml:"blocking"`
	// ReviewerRole is the ConfiguredRole code that can waive a rejected result
	// (e.g. "domain-expert"). Stored as reviewer_role_code on DraftDeliverable.
	ReviewerRole string `yaml:"reviewer_role" json:"reviewer_role"`
}

type importWorkPlanSpec struct {
	Code             string `yaml:"code"`
	Name             string `yaml:"name"`
	Description      string `yaml:"description"`
	TriggerTopic     string `yaml:"trigger_topic"`
	PayloadCondition string `yaml:"payload_condition"`
	Instructions     string `yaml:"instructions"`
	AgentID          string `yaml:"agent_id"`
	// AgentCode is a symbolic reference resolved by CodeValdAI on startup.
	// It identifies which agent in ai_config should handle this work plan.
	AgentCode      string      `yaml:"agent_code"`
	HandlerService string      `yaml:"handler_service"`
	FunctionCode   string      `yaml:"function_code"`
	// FunctionParams accepts both a plain string and a JSON/YAML object.
	// When it is an object (as in agency.json), it is serialised back to a
	// JSON string before being stored in the WorkPlan entity.
	FunctionParams interface{} `yaml:"function_params" json:"function_params"`
	Enabled        bool   `yaml:"enabled"`
	Ordinality     int    `yaml:"ordinality"`
	// SuccessEvent/FailureEvent/OnFailurePipeline implement FEAT-20260602-005
	// — the synthesized-success failure-pipeline contract. See
	// CodeValdCross/.../mvp-details/FEAT-20260602-005_failure_pipelines_synthesized_success.md.
	SuccessEvent      string `yaml:"success_event"`
	FailureEvent      string `yaml:"failure_event"`
	OnFailurePipeline string `yaml:"on_failure_pipeline"`
	// StepTimeout is the per-step inactivity duration (e.g. "10m"). See FEAT-20260602-006.
	StepTimeout string `yaml:"step_timeout"`
	// ReviewStepType / ReviewTriggerTopic / ReviewSuccessTopic / ReviewFailureTopic
	// implement FEAT-20260605-002 — the review gate that checks AcceptanceCriteria
	// results before advancing to the next WorkPlan step.
	ReviewStepType     string `yaml:"review_step_type"`
	ReviewTriggerTopic string `yaml:"review_trigger_topic"`
	ReviewSuccessTopic string `yaml:"review_success_topic"`
	ReviewFailureTopic string `yaml:"review_failure_topic"`
}

// ── RPC handler ───────────────────────────────────────────────────────────────

// ImportDraft implements pb.AgencyServiceServer.
// It accepts a raw YAML (or JSON) body delivered in req.GetBody() by the
// CodeValdCross HTTP proxy and idempotently populates a draft.
func (s *Server) ImportDraft(ctx context.Context, req *pb.ImportDraftRequest) (*pb.ImportDraftResponse, error) {
	rawBody := req.GetBody()
	log.Printf("[ImportDraft] received body len=%d", len(rawBody))
	if strings.TrimSpace(rawBody) == "" {
		log.Printf("[ImportDraft] body is empty — returning InvalidArgument")
		return nil, status.Error(codes.InvalidArgument, "ImportDraft: body is empty")
	}

	var spec importAgencyYAML
	if err := yaml.Unmarshal([]byte(rawBody), &spec); err != nil {
		log.Printf("[ImportDraft] yaml.Unmarshal failed (%v) — trying JSON fallback", err)
		if jsonErr := json.Unmarshal([]byte(rawBody), &spec); jsonErr != nil {
			log.Printf("[ImportDraft] json.Unmarshal also failed: %v", jsonErr)
			return nil, status.Errorf(codes.InvalidArgument, "ImportDraft: parse body as YAML: %v", err)
		}
		log.Printf("[ImportDraft] parsed as JSON fallback")
	} else {
		log.Printf("[ImportDraft] parsed as YAML")
	}

	log.Printf("[ImportDraft] parsed spec: agency.code=%q roles=%d goals=%d workflows=%d",
		spec.Agency.Code, len(spec.ConfiguredRoles), len(spec.Goals), len(spec.Workflows))

	if spec.Agency.Code == "" {
		log.Printf("[ImportDraft] agency.code is empty — returning InvalidArgument")
		return nil, status.Error(codes.InvalidArgument, "ImportDraft: agency.code is required")
	}

	agencyID := spec.Agency.Code

	// 1. Set agency details.
	log.Printf("[ImportDraft] %s: setting agency details", agencyID)
	if err := s.importSetDetails(ctx, agencyID, spec.Agency, spec.EventFlows, req.GetAutoPromote()); err != nil {
		log.Printf("[ImportDraft] %s: set details failed: %v", agencyID, err)
		if errors.Is(err, codevaldagency.ErrAgencyReadOnly) {
			return nil, status.Errorf(codes.FailedPrecondition,
				"ImportDraft %s: agency is published; pass auto_promote=true to auto-create and promote a draft", agencyID)
		}
		return nil, status.Errorf(codes.Internal, "ImportDraft %s: set details: %v", agencyID, err)
	}

	// 2. Ensure open draft.
	log.Printf("[ImportDraft] %s: ensuring draft", agencyID)
	draftID, err := s.importEnsureDraft(ctx, agencyID, spec.Agency)
	if err != nil {
		log.Printf("[ImportDraft] %s: ensure draft failed: %v", agencyID, err)
		return nil, status.Errorf(codes.Internal, "ImportDraft %s: ensure draft: %v", agencyID, err)
	}
	log.Printf("[ImportDraft] %s: draftID=%s", agencyID, draftID)

	// 3. Configured roles.
	log.Printf("[ImportDraft] %s: upserting %d configured roles", agencyID, len(spec.ConfiguredRoles))
	for _, r := range spec.ConfiguredRoles {
		log.Printf("[ImportDraft] %s: upsert role code=%s", agencyID, r.Code)
		props := map[string]any{
			"draft_ref_code": draftID,
			"code":           r.Code,
			"name":           r.Name,
			"description":    clean(r.Description),
			"actor_type":     r.ActorType,
			"ordinality":     r.Ordinality,
		}
		if err := s.importUpsert(ctx, agencyID, "DraftConfiguredRole", props); err != nil {
			log.Printf("[ImportDraft] %s: role %s upsert failed: %v", agencyID, r.Code, err)
			return nil, status.Errorf(codes.Internal, "ImportDraft %s: role %s: %v", agencyID, r.Code, err)
		}
	}

	// 4. Goals.
	log.Printf("[ImportDraft] %s: upserting %d goals", agencyID, len(spec.Goals))
	for _, g := range spec.Goals {
		log.Printf("[ImportDraft] %s: upsert goal code=%s", agencyID, g.Code)
		props := map[string]any{
			"draft_ref_code": draftID,
			"code":           g.Code,
			"title":          clean(g.Title),
			"description":    clean(g.Description),
			"ordinality":     g.Ordinality,
		}
		if err := s.importUpsert(ctx, agencyID, "DraftGoal", props); err != nil {
			log.Printf("[ImportDraft] %s: goal %s upsert failed: %v", agencyID, g.Code, err)
			return nil, status.Errorf(codes.Internal, "ImportDraft %s: goal %s: %v", agencyID, g.Code, err)
		}
	}

	// 5. Workflows.
	log.Printf("[ImportDraft] %s: upserting %d workflows", agencyID, len(spec.Workflows))
	perWorkflowFlowsSeen := 0
	for _, wf := range spec.Workflows {
		log.Printf("[ImportDraft] %s: upsert workflow code=%s workItems=%d", agencyID, wf.Code, len(wf.WorkItems))
		wfProps := map[string]any{
			"draft_ref_code": draftID,
			"code":           wf.Code,
			"name":           wf.Name,
			"description":    clean(wf.Description),
			"ordinality":     wf.Ordinality,
		}
		// Per-workflow event_flows (FEAT-20260609-002): caller bundles
		// flows_<workflow.code>.json into wf.event_flows. Re-marshal to a JSON
		// string for storage — same pattern as the agency-level event_flows in
		// importSetDetails.
		if wf.EventFlows != nil {
			b, merr := json.Marshal(wf.EventFlows)
			if merr == nil {
				wfProps["event_flows"] = string(b)
				perWorkflowFlowsSeen++
			} else {
				log.Printf("[ImportDraft] %s: workflow %s event_flows marshal failed: %v", agencyID, wf.Code, merr)
			}
		}
		wfID, err := s.importUpsertID(ctx, agencyID, "DraftWorkflow", wfProps)
		if err != nil {
			log.Printf("[ImportDraft] %s: workflow %s upsert failed: %v", agencyID, wf.Code, err)
			return nil, status.Errorf(codes.Internal, "ImportDraft %s: workflow %s: %v", agencyID, wf.Code, err)
		}
		log.Printf("[ImportDraft] %s: workflow %s -> id=%s", agencyID, wf.Code, wfID)

		// Workflow-scoped instructions.
		for _, inst := range wf.Instructions {
			log.Printf("[ImportDraft] %s: upsert workflow instruction code=%s", agencyID, inst.Code)
			if err := s.importUpsert(ctx, agencyID, "DraftInstruction", map[string]any{
				"draft_ref_code":          draftID,
				"code":                    inst.Code,
				"draft_workflow_ref_code": wfID,
				"content":                 clean(inst.Content),
				"ordinality":              inst.Ordinality,
			}); err != nil {
				log.Printf("[ImportDraft] %s: workflow %s instruction %s upsert failed: %v", agencyID, wf.Code, inst.Code, err)
				return nil, status.Errorf(codes.Internal, "ImportDraft %s: workflow %s instruction %s: %v", agencyID, wf.Code, inst.Code, err)
			}
		}

		// Work items.
		for _, wi := range wf.WorkItems {
			log.Printf("[ImportDraft] %s: upsert work item code=%s deliverables=%d", agencyID, wi.Code, len(wi.Deliverables))
			wiProps := map[string]any{
				"draft_ref_code":          draftID,
				"code":                    wi.Code,
				"draft_workflow_ref_code": wfID,
				"title":                   clean(wi.Title),
				"description":             clean(wi.Description),
				"ordinality":              wi.Ordinality,
				"prompt":                  clean(wi.Prompt),
			}
			if len(wi.AssignedRoles) > 0 {
				wiProps["assigned_role"] = wi.AssignedRoles[0]
			}
			wiID, err := s.importUpsertID(ctx, agencyID, "DraftWorkItem", wiProps)
			if err != nil {
				log.Printf("[ImportDraft] %s: work item %s upsert failed: %v", agencyID, wi.Code, err)
				return nil, status.Errorf(codes.Internal, "ImportDraft %s: work item %s: %v", agencyID, wi.Code, err)
			}
			log.Printf("[ImportDraft] %s: work item %s -> id=%s", agencyID, wi.Code, wiID)

			// Work-item-scoped instructions.
			for _, inst := range wi.Instructions {
				log.Printf("[ImportDraft] %s: upsert work item instruction code=%s", agencyID, inst.Code)
				if err := s.importUpsert(ctx, agencyID, "DraftInstruction", map[string]any{
					"draft_ref_code":           draftID,
					"code":                     inst.Code,
					"draft_work_item_ref_code": wiID,
					"content":                  clean(inst.Content),
					"ordinality":               inst.Ordinality,
				}); err != nil {
					log.Printf("[ImportDraft] %s: work item %s instruction %s upsert failed: %v", agencyID, wi.Code, inst.Code, err)
					return nil, status.Errorf(codes.Internal, "ImportDraft %s: work item %s instruction %s: %v", agencyID, wi.Code, inst.Code, err)
				}
			}

			// Deliverables.
			for _, d := range wi.Deliverables {
				log.Printf("[ImportDraft] %s: upsert deliverable code=%s", agencyID, d.Code)
				deliverableProps := map[string]any{
					"draft_ref_code":           draftID,
					"code":                     d.Code,
					"draft_work_item_ref_code": wiID,
					"title":                    clean(d.Title),
					"description":              clean(d.Description),
					"ordinality":               d.Ordinality,
					"blocking":                 d.Blocking,
				}
				if d.ReviewerRole != "" {
					deliverableProps["reviewer_role_code"] = d.ReviewerRole
				}
				if err := s.importUpsert(ctx, agencyID, "DraftDeliverable", deliverableProps); err != nil {
					log.Printf("[ImportDraft] %s: work item %s deliverable %s upsert failed: %v", agencyID, wi.Code, d.Code, err)
					return nil, status.Errorf(codes.Internal, "ImportDraft %s: work item %s deliverable %s: %v", agencyID, wi.Code, d.Code, err)
				}
			}
		}
	}
	// Deprecation warning (FEAT-20260609-002): top-level event_flows is the legacy
	// monolithic form; per-workflow event_flows (one block per workflow) is the
	// supported convention. Warn when callers are still on the legacy-only path.
	if perWorkflowFlowsSeen == 0 && spec.EventFlows != nil {
		log.Printf("[ImportDraft] %s: DEPRECATED top-level event_flows used without per-workflow blocks; bundle flows_<workflow.code>.json into each workflow's event_flows field (FEAT-20260609-002)", agencyID)
	}

	// 6. Work plans (live entities, not draft-scoped).
	log.Printf("[ImportDraft] %s: upserting %d work plans", agencyID, len(spec.WorkPlans))
	for _, wp := range spec.WorkPlans {
		log.Printf("[ImportDraft] %s: upsert work plan code=%s", agencyID, wp.Code)
		// Serialise FunctionParams: agency.json sends a JSON object; store as string.
		var fpStr string
		switch v := wp.FunctionParams.(type) {
		case string:
			fpStr = v
		case nil:
			// nothing
		default:
			if b, err := json.Marshal(v); err == nil {
				fpStr = string(b)
			}
		}
		props := map[string]any{
			"code":                wp.Code,
			"name":                wp.Name,
			"description":         clean(wp.Description),
			"trigger_topic":       wp.TriggerTopic,
			"payload_condition":   clean(wp.PayloadCondition),
			"instructions":        clean(wp.Instructions),
			"agent_id":            clean(wp.AgentID),
			"agent_code":          clean(wp.AgentCode),
			"handler_service":     clean(wp.HandlerService),
			"function_code":       clean(wp.FunctionCode),
			"function_params":     fpStr,
			"enabled":             wp.Enabled,
			"ordinality":          wp.Ordinality,
			"success_event":        clean(wp.SuccessEvent),
			"failure_event":        clean(wp.FailureEvent),
			"on_failure_pipeline":  clean(wp.OnFailurePipeline),
			"step_timeout":         clean(wp.StepTimeout),
			"review_step_type":     clean(wp.ReviewStepType),
			"review_trigger_topic": clean(wp.ReviewTriggerTopic),
			"review_success_topic": clean(wp.ReviewSuccessTopic),
			"review_failure_topic": clean(wp.ReviewFailureTopic),
		}
		if err := s.importUpsert(ctx, agencyID, "WorkPlan", props); err != nil {
			log.Printf("[ImportDraft] %s: work plan %s upsert failed: %v", agencyID, wp.Code, err)
			return nil, status.Errorf(codes.Internal, "ImportDraft %s: work plan %s: %v", agencyID, wp.Code, err)
		}
	}

	// 7. AI Providers (live entities, not draft-scoped).
	log.Printf("[ImportDraft] %s: upserting %d ai_config providers", agencyID, len(spec.AIConfig.Providers))
	for _, p := range spec.AIConfig.Providers {
		log.Printf("[ImportDraft] %s: upsert ai provider code=%s", agencyID, p.Code)
		if err := s.importUpsert(ctx, agencyID, "AIProvider", map[string]any{
			"code":           p.Code,
			"name":           p.Name,
			"provider_type":  p.ProviderType,
			"api_key_env":    p.APIKeyEnv,
			"base_url":       p.BaseURL,
			"provider_route": p.ProviderRoute,
		}); err != nil {
			log.Printf("[ImportDraft] %s: ai provider %s upsert failed: %v", agencyID, p.Code, err)
			return nil, status.Errorf(codes.Internal, "ImportDraft %s: ai_provider %s: %v", agencyID, p.Code, err)
		}
	}

	// 8. AI Agents (live entities, not draft-scoped).
	log.Printf("[ImportDraft] %s: upserting %d ai_config agents", agencyID, len(spec.AIConfig.Agents))
	for _, a := range spec.AIConfig.Agents {
		log.Printf("[ImportDraft] %s: upsert ai agent code=%s", agencyID, a.Code)
		if err := s.importUpsert(ctx, agencyID, "AIAgent", map[string]any{
			"code":                 a.Code,
			"name":                 a.Name,
			"provider_code":        a.ProviderCode,
			"model":                a.Model,
			"system_prompt":        clean(a.SystemPrompt),
			"temperature":          a.Temperature,
			"max_tokens":           a.MaxTokens,
			"session_max_seconds":  a.SessionMaxSeconds,
			"session_max_tokens":   a.SessionMaxTokens,
			"session_max_sessions": a.SessionMaxSessions,
		}); err != nil {
			log.Printf("[ImportDraft] %s: ai agent %s upsert failed: %v", agencyID, a.Code, err)
			return nil, status.Errorf(codes.Internal, "ImportDraft %s: ai_agent %s: %v", agencyID, a.Code, err)
		}
	}

	// 9. Auto-promote the draft when requested (FEAT-20260609-003). Required for
	// re-import against an already-published agency; an optional convenience for
	// unpublished agencies (one-shot import + promote).
	promoted := false
	if req.GetAutoPromote() {
		log.Printf("[ImportDraft] %s: auto-promoting draft %s", agencyID, draftID)
		if _, perr := s.mgr.PromoteDraft(ctx, draftID); perr != nil {
			log.Printf("[ImportDraft] %s: auto-promote draft %s failed: %v", agencyID, draftID, perr)
			return nil, status.Errorf(codes.Internal, "ImportDraft %s: auto-promote draft %s: %v", agencyID, draftID, perr)
		}
		promoted = true
		log.Printf("[ImportDraft] %s: auto-promoted draft %s", agencyID, draftID)
	}

	log.Printf("[ImportDraft] %s: done draftID=%s promoted=%v", agencyID, draftID, promoted)
	go s.syncSubscriptions(context.Background())
	return &pb.ImportDraftResponse{AgencyId: agencyID, DraftId: draftID, Promoted: promoted}, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// importSetDetails updates the root agency document.
// For published agencies (ErrAgencyReadOnly), structural fields are skipped but
// event_flows metadata is refreshed via SetAgencyEventFlows. When autoPromote
// is true (FEAT-20260609-003), ErrAgencyReadOnly is swallowed after the
// event_flows refresh so the caller can continue into the draft pipeline; when
// autoPromote is false the ErrAgencyReadOnly is propagated so the caller can
// surface a clean FailedPrecondition.
func (s *Server) importSetDetails(ctx context.Context, agencyID string, a importAgencySpec, eventFlows interface{}, autoPromote bool) error {
	// Marshal event_flows back to a JSON string for storage (yaml.v3 decodes it
	// as interface{}, so we re-encode to get the raw JSON blob).
	var eventFlowsJSON string
	if eventFlows != nil {
		b, merr := json.Marshal(eventFlows)
		if merr == nil {
			eventFlowsJSON = string(b)
		}
	}

	body, err := json.Marshal(map[string]any{
		"id":                              agencyID,
		"name":                            a.Name,
		"mission":                         clean(a.Mission),
		"vision":                          clean(a.Vision),
		"code":                            a.Code,
		"default_failure_pipeline_budget": a.DefaultFailurePipelineBudget,
		"event_flows":                     eventFlowsJSON,
	})
	if err != nil {
		return fmt.Errorf("importSetDetails: marshal: %w", err)
	}
	_, err = s.mgr.SetAgencyDetails(ctx, string(body))
	if err == nil {
		return nil
	}
	// Published agencies block structural field updates. event_flows is
	// display-only metadata and can always be refreshed even after publish.
	if errors.Is(err, codevaldagency.ErrAgencyReadOnly) {
		if eventFlowsJSON != "" {
			if _, efErr := s.mgr.SetAgencyEventFlows(ctx, eventFlowsJSON); efErr != nil {
				return fmt.Errorf("importSetDetails: refresh event_flows on published agency: %w", efErr)
			}
		}
		if autoPromote {
			// Caller will continue into the draft pipeline and call PromoteDraft.
			// Agency-level structural fields (mission, vision, etc.) are preserved
			// as-is on the published agency; only sub-entities and event_flows
			// participate in the re-import.
			log.Printf("[importSetDetails] %s: published; auto_promote=true — structural fields preserved, draft pipeline will run", agencyID)
			return nil
		}
		return err
	}
	return err
}

// importEnsureDraft returns the entity ID of an open AgencyDraft for agencyID.
// It reuses an existing open draft (matched by code, then by status alone) or
// creates a fresh one.
func (s *Server) importEnsureDraft(ctx context.Context, agencyID string, a importAgencySpec) (string, error) {
	// List existing drafts via DataManager.
	existing, err := s.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: agencyID,
		TypeID:   "AgencyDraft",
	})
	if err != nil {
		return "", fmt.Errorf("importEnsureDraft: list drafts: %w", err)
	}

	// Prefer an open draft with matching code.
	for _, e := range existing {
		code, _ := e.Properties["code"].(string)
		st, _ := e.Properties["status"].(string)
		if code == a.Code && strings.EqualFold(st, "open") {
			return e.ID, nil
		}
	}
	// Fall back to any open draft.
	for _, e := range existing {
		st, _ := e.Properties["status"].(string)
		if strings.EqualFold(st, "open") {
			return e.ID, nil
		}
	}

	// No open draft found — create a fresh one. Use CreateEntity (not UpsertEntity)
	// so the UniqueKey match on code never resets a promoted draft back to open.
	entity, err := s.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: agencyID,
		TypeID:   "AgencyDraft",
		Properties: map[string]any{
			"code":             a.Code,
			"description":      a.Name,
			"status":           "open",
			"forked_from_type": "import",
		},
	})
	if err != nil {
		return "", fmt.Errorf("importEnsureDraft: create: %w", err)
	}
	return entity.ID, nil
}

// importUpsert upserts a single entity via UpsertEntity (idempotent by UniqueKey).
func (s *Server) importUpsert(ctx context.Context, agencyID, typeID string, props map[string]any) error {
	_, err := s.dm.UpsertEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID:   agencyID,
		TypeID:     typeID,
		Properties: props,
	})
	return err
}

// importUpsertID upserts an entity and returns its ID.
func (s *Server) importUpsertID(ctx context.Context, agencyID, typeID string, props map[string]any) (string, error) {
	entity, err := s.dm.UpsertEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID:   agencyID,
		TypeID:     typeID,
		Properties: props,
	})
	if err != nil {
		return "", err
	}
	return entity.ID, nil
}

// clean collapses multi-line YAML scalars (block/folded/literal style) into a
// single whitespace-normalised line.
func clean(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
