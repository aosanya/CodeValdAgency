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
}

type importAgencySpec struct {
	Code    string `yaml:"code"`
	Name    string `yaml:"name"`
	Mission string `yaml:"mission"`
	Vision  string `yaml:"vision"`
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
}

type importInstructionSpec struct {
	Code       string `yaml:"code"`
	Content    string `yaml:"content"`
	Ordinality int    `yaml:"ordinality"`
}

type importWorkItemSpec struct {
	Code         string                  `yaml:"code"`
	Title        string                  `yaml:"title"`
	Description  string                  `yaml:"description"`
	Ordinality   int                     `yaml:"ordinality"`
	AssignedRole string                  `yaml:"assigned_role"`
	Prompt       string                  `yaml:"prompt"`
	Instructions []importInstructionSpec `yaml:"instructions"`
	Deliverables []importDeliverableSpec `yaml:"deliverables"`
}

type importDeliverableSpec struct {
	Code        string `yaml:"code"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Ordinality  int    `yaml:"ordinality"`
	Blocking    bool   `yaml:"blocking"`
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
	AgentCode      string `yaml:"agent_code"`
	HandlerService string `yaml:"handler_service"`
	Enabled        bool   `yaml:"enabled"`
	Ordinality     int    `yaml:"ordinality"`
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

	// 1. Set agency details — skip if the agency is already published.
	log.Printf("[ImportDraft] %s: setting agency details", agencyID)
	if err := s.importSetDetails(ctx, agencyID, spec.Agency); err != nil {
		if errors.Is(err, codevaldagency.ErrAgencyReadOnly) {
			log.Printf("[ImportDraft] %s: agency already published — skipping details update", agencyID)
		} else {
			log.Printf("[ImportDraft] %s: set details failed: %v", agencyID, err)
			return nil, status.Errorf(codes.Internal, "ImportDraft %s: set details: %v", agencyID, err)
		}
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
	for _, wf := range spec.Workflows {
		log.Printf("[ImportDraft] %s: upsert workflow code=%s workItems=%d", agencyID, wf.Code, len(wf.WorkItems))
		wfID, err := s.importUpsertID(ctx, agencyID, "DraftWorkflow", map[string]any{
			"draft_ref_code": draftID,
			"code":           wf.Code,
			"name":           wf.Name,
			"description":    clean(wf.Description),
			"ordinality":     wf.Ordinality,
		})
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
			if wi.AssignedRole != "" {
				wiProps["assigned_role"] = wi.AssignedRole
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
				if err := s.importUpsert(ctx, agencyID, "DraftDeliverable", map[string]any{
					"draft_ref_code":           draftID,
					"code":                     d.Code,
					"draft_work_item_ref_code": wiID,
					"title":                    clean(d.Title),
					"description":              clean(d.Description),
					"ordinality":               d.Ordinality,
					"blocking":                 d.Blocking,
				}); err != nil {
					log.Printf("[ImportDraft] %s: work item %s deliverable %s upsert failed: %v", agencyID, wi.Code, d.Code, err)
					return nil, status.Errorf(codes.Internal, "ImportDraft %s: work item %s deliverable %s: %v", agencyID, wi.Code, d.Code, err)
				}
			}
		}
	}

	// 6. Work plans (live entities, not draft-scoped).
	log.Printf("[ImportDraft] %s: upserting %d work plans", agencyID, len(spec.WorkPlans))
	for _, wp := range spec.WorkPlans {
		log.Printf("[ImportDraft] %s: upsert work plan code=%s", agencyID, wp.Code)
		props := map[string]any{
			"code":              wp.Code,
			"name":              wp.Name,
			"description":       clean(wp.Description),
			"trigger_topic":     wp.TriggerTopic,
			"payload_condition": clean(wp.PayloadCondition),
			"instructions":      clean(wp.Instructions),
			"agent_id":          clean(wp.AgentID),
			"handler_service":   clean(wp.HandlerService),
			"enabled":           wp.Enabled,
			"ordinality":        wp.Ordinality,
		}
		if err := s.importUpsert(ctx, agencyID, "WorkPlan", props); err != nil {
			log.Printf("[ImportDraft] %s: work plan %s upsert failed: %v", agencyID, wp.Code, err)
			return nil, status.Errorf(codes.Internal, "ImportDraft %s: work plan %s: %v", agencyID, wp.Code, err)
		}
	}

	log.Printf("[ImportDraft] %s: done draftID=%s", agencyID, draftID)
	go s.syncSubscriptions(context.Background())
	return &pb.ImportDraftResponse{AgencyId: agencyID, DraftId: draftID}, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// importSetDetails updates the root agency document.
func (s *Server) importSetDetails(ctx context.Context, agencyID string, a importAgencySpec) error {
	body, err := json.Marshal(map[string]any{
		"id":      agencyID,
		"name":    a.Name,
		"mission": clean(a.Mission),
		"vision":  clean(a.Vision),
		"code":    a.Code,
	})
	if err != nil {
		return fmt.Errorf("importSetDetails: marshal: %w", err)
	}
	_, err = s.mgr.SetAgencyDetails(ctx, string(body))
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

	// Create a fresh draft entity.
	entity, err := s.dm.UpsertEntity(ctx, entitygraph.CreateEntityRequest{
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
