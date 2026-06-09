package codevaldagency

import (
	"context"
	"fmt"
	"regexp"
	"sort"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
)

// ── CreateWorkPlan ────────────────────────────────────────────────────────────

// CreateWorkPlan stores a new WorkPlan entity and links it to the Agency via
// has_work_plan / belongs_to_agency edges.
func (m *agencyManager) CreateWorkPlan(ctx context.Context, req CreateWorkPlanRequest) (WorkPlan, error) {
	if err := validateWorkPlanRegex(req.TriggerTopic, req.PayloadCondition); err != nil {
		return WorkPlan{}, err
	}

	agency, err := m.GetAgency(ctx)
	if err != nil {
		return WorkPlan{}, fmt.Errorf("CreateWorkPlan: %w", err)
	}

	entity, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: m.agencyID,
		TypeID:   "WorkPlan",
		Properties: map[string]any{
			"name":                req.Name,
			"description":         req.Description,
			"trigger_topic":       req.TriggerTopic,
			"payload_condition":   req.PayloadCondition,
			"instructions":        req.Instructions,
			"agent_id":            req.AgentID,
			"agent_code":          req.AgentCode,
			"function_code":       req.FunctionCode,
			"function_params":     req.FunctionParams,
			"handler_service":     req.HandlerService,
			"enabled":             req.Enabled,
			"ordinality":          req.Ordinality,
			"success_event":        req.SuccessEvent,
			"failure_event":        req.FailureEvent,
			"on_failure_pipeline":  req.OnFailurePipeline,
			"step_timeout":         req.StepTimeout,
			"review_step_type":     req.ReviewStepType,
			"review_trigger_topic": req.ReviewTriggerTopic,
			"review_success_topic": req.ReviewSuccessTopic,
			"review_failure_topic": req.ReviewFailureTopic,
		},
	})
	if err != nil {
		return WorkPlan{}, fmt.Errorf("CreateWorkPlan: create entity: %w", err)
	}

	if _, err := m.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: m.agencyID,
		Name:     "has_work_plan",
		FromID:   agency.ID,
		ToID:     entity.ID,
	}); err != nil {
		return WorkPlan{}, fmt.Errorf("CreateWorkPlan: link has_work_plan: %w", err)
	}
	if _, err := m.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: m.agencyID,
		Name:     "belongs_to_agency",
		FromID:   entity.ID,
		ToID:     agency.ID,
	}); err != nil {
		return WorkPlan{}, fmt.Errorf("CreateWorkPlan: link belongs_to_agency: %w", err)
	}

	return entityToWorkPlan(entity, m.agencyID), nil
}

// ── GetWorkPlan ───────────────────────────────────────────────────────────────

// GetWorkPlan retrieves a single WorkPlan by its entity ID.
// Returns [ErrWorkPlanNotFound] if no WorkPlan entity with that ID exists.
func (m *agencyManager) GetWorkPlan(ctx context.Context, workPlanID string) (WorkPlan, error) {
	entity, err := m.dm.GetEntity(ctx, m.agencyID, workPlanID)
	if err != nil || entity.TypeID != "WorkPlan" {
		return WorkPlan{}, fmt.Errorf("GetWorkPlan %s: %w", workPlanID, ErrWorkPlanNotFound)
	}
	return entityToWorkPlan(entity, m.agencyID), nil
}

// ── ListWorkPlans ─────────────────────────────────────────────────────────────

// ListWorkPlans returns all WorkPlan entities linked to this Agency.
func (m *agencyManager) ListWorkPlans(ctx context.Context) ([]WorkPlan, error) {
	entities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: m.agencyID,
		TypeID:   "WorkPlan",
	})
	if err != nil {
		return nil, fmt.Errorf("ListWorkPlans: %w", err)
	}
	plans := make([]WorkPlan, len(entities))
	for i, e := range entities {
		plans[i] = entityToWorkPlan(e, m.agencyID)
	}
	return plans, nil
}

// ── UpdateWorkPlan ────────────────────────────────────────────────────────────

// UpdateWorkPlan applies req to the WorkPlan identified by workPlanID.
// Returns [ErrWorkPlanNotFound] if no WorkPlan entity with that ID exists.
// Returns [ErrInvalidRegex] if TriggerTopic or PayloadCondition is not a valid Go regexp.
func (m *agencyManager) UpdateWorkPlan(ctx context.Context, workPlanID string, req UpdateWorkPlanRequest) (WorkPlan, error) {
	topic := ""
	if req.TriggerTopic != nil {
		topic = *req.TriggerTopic
	}
	cond := ""
	if req.PayloadCondition != nil {
		cond = *req.PayloadCondition
	}
	if err := validateWorkPlanRegex(topic, cond); err != nil {
		return WorkPlan{}, err
	}
	if _, err := m.GetWorkPlan(ctx, workPlanID); err != nil {
		return WorkPlan{}, err
	}

	props := map[string]any{}
	if req.Name != nil {
		props["name"] = *req.Name
	}
	if req.Description != nil {
		props["description"] = *req.Description
	}
	if req.TriggerTopic != nil {
		props["trigger_topic"] = *req.TriggerTopic
	}
	if req.PayloadCondition != nil {
		props["payload_condition"] = *req.PayloadCondition
	}
	if req.Instructions != nil {
		props["instructions"] = *req.Instructions
	}
	if req.AgentID != nil {
		props["agent_id"] = *req.AgentID
	}
	if req.AgentCode != nil {
		props["agent_code"] = *req.AgentCode
	}
	if req.FunctionCode != nil {
		props["function_code"] = *req.FunctionCode
	}
	if req.FunctionParams != nil {
		props["function_params"] = *req.FunctionParams
	}
	if req.HandlerService != nil {
		props["handler_service"] = *req.HandlerService
	}
	if req.Enabled != nil {
		props["enabled"] = *req.Enabled
	}
	if req.Ordinality != nil {
		props["ordinality"] = *req.Ordinality
	}
	if req.SuccessEvent != nil {
		props["success_event"] = *req.SuccessEvent
	}
	if req.FailureEvent != nil {
		props["failure_event"] = *req.FailureEvent
	}
	if req.OnFailurePipeline != nil {
		props["on_failure_pipeline"] = *req.OnFailurePipeline
	}
	if req.StepTimeout != nil {
		props["step_timeout"] = *req.StepTimeout
	}
	if req.ReviewStepType != nil {
		props["review_step_type"] = *req.ReviewStepType
	}
	if req.ReviewTriggerTopic != nil {
		props["review_trigger_topic"] = *req.ReviewTriggerTopic
	}
	if req.ReviewSuccessTopic != nil {
		props["review_success_topic"] = *req.ReviewSuccessTopic
	}
	if req.ReviewFailureTopic != nil {
		props["review_failure_topic"] = *req.ReviewFailureTopic
	}

	updated, err := m.dm.UpdateEntity(ctx, m.agencyID, workPlanID, entitygraph.UpdateEntityRequest{
		Properties: props,
	})
	if err != nil {
		return WorkPlan{}, fmt.Errorf("UpdateWorkPlan %s: %w", workPlanID, err)
	}
	return entityToWorkPlan(updated, m.agencyID), nil
}

// ── DeleteWorkPlan ────────────────────────────────────────────────────────────

// DeleteWorkPlan removes the WorkPlan entity identified by workPlanID.
// Returns [ErrWorkPlanNotFound] if no WorkPlan entity with that ID exists.
func (m *agencyManager) DeleteWorkPlan(ctx context.Context, workPlanID string) error {
	if _, err := m.GetWorkPlan(ctx, workPlanID); err != nil {
		return err
	}
	if err := m.dm.DeleteEntity(ctx, m.agencyID, workPlanID); err != nil {
		return fmt.Errorf("DeleteWorkPlan %s: %w", workPlanID, err)
	}
	return nil
}

// ── AddContextSource ──────────────────────────────────────────────────────────

// AddContextSource creates a typed ContextSource entity and links it to the
// WorkPlan identified by workPlanID via has_context_source / belongs_to_work_plan edges.
// Returns [ErrWorkPlanNotFound] if no WorkPlan entity with that ID exists.
func (m *agencyManager) AddContextSource(ctx context.Context, workPlanID string, req AddContextSourceRequest) (ContextSource, error) {
	if _, err := m.GetWorkPlan(ctx, workPlanID); err != nil {
		return ContextSource{}, err
	}

	typeID, props := contextSourceTypeIDAndProps(req)

	entity, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID:   m.agencyID,
		TypeID:     typeID,
		Properties: props,
	})
	if err != nil {
		return ContextSource{}, fmt.Errorf("AddContextSource: create entity: %w", err)
	}

	if _, err := m.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: m.agencyID,
		Name:     "has_context_source",
		FromID:   workPlanID,
		ToID:     entity.ID,
	}); err != nil {
		return ContextSource{}, fmt.Errorf("AddContextSource: link has_context_source: %w", err)
	}
	if _, err := m.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: m.agencyID,
		Name:     "belongs_to_work_plan",
		FromID:   entity.ID,
		ToID:     workPlanID,
	}); err != nil {
		return ContextSource{}, fmt.Errorf("AddContextSource: link belongs_to_work_plan: %w", err)
	}

	return entityToContextSource(entity, workPlanID), nil
}

// ── ListContextSources ────────────────────────────────────────────────────────

// ListContextSources returns all ContextSource entities linked to the WorkPlan
// identified by workPlanID via has_context_source edges.
// Returns [ErrWorkPlanNotFound] if no WorkPlan entity with that ID exists.
func (m *agencyManager) ListContextSources(ctx context.Context, workPlanID string) ([]ContextSource, error) {
	if _, err := m.GetWorkPlan(ctx, workPlanID); err != nil {
		return nil, err
	}

	rels, err := m.dm.ListRelationships(ctx, entitygraph.RelationshipFilter{
		AgencyID: m.agencyID,
		Name:     "has_context_source",
		FromID:   workPlanID,
	})
	if err != nil {
		return nil, fmt.Errorf("ListContextSources: list rels: %w", err)
	}

	sources := make([]ContextSource, 0, len(rels))
	for _, rel := range rels {
		entity, err := m.dm.GetEntity(ctx, m.agencyID, rel.ToID)
		if err != nil {
			continue // entity may have been concurrently deleted
		}
		sources = append(sources, entityToContextSource(entity, workPlanID))
	}
	return sources, nil
}

// ── RemoveContextSource ───────────────────────────────────────────────────────

// RemoveContextSource deletes the ContextSource entity identified by sourceID
// and removes its has_context_source edge from the owning WorkPlan.
// Returns [ErrContextSourceNotFound] if no such entity exists.
func (m *agencyManager) RemoveContextSource(ctx context.Context, workPlanID, sourceID string) error {
	entity, err := m.dm.GetEntity(ctx, m.agencyID, sourceID)
	if err != nil {
		return fmt.Errorf("RemoveContextSource %s: %w", sourceID, ErrContextSourceNotFound)
	}
	if !isContextSourceTypeID(entity.TypeID) {
		return fmt.Errorf("RemoveContextSource %s: %w", sourceID, ErrContextSourceNotFound)
	}

	if err := m.dm.DeleteEntity(ctx, m.agencyID, sourceID); err != nil {
		return fmt.Errorf("RemoveContextSource %s: delete entity: %w", sourceID, err)
	}
	return nil
}

// ── MatchWorkPlans ────────────────────────────────────────────────────────────

// MatchWorkPlans evaluates topic and payload against all enabled WorkPlan entities.
// TriggerTopic is compiled as a Go regex and matched against topic; if
// PayloadCondition is non-empty it is also matched against payload.
// Returns all matching work plans with their ContextSource entities, ordered by
// WorkPlan.Ordinality ascending.
func (m *agencyManager) MatchWorkPlans(ctx context.Context, topic, payload string) ([]WorkPlanMatch, error) {
	plans, err := m.ListWorkPlans(ctx)
	if err != nil {
		return nil, fmt.Errorf("MatchWorkPlans: %w", err)
	}

	var matches []WorkPlanMatch
	for _, plan := range plans {
		if !plan.Enabled {
			continue
		}
		topicRe, err := regexp.Compile(plan.TriggerTopic)
		if err != nil {
			continue // skip malformed regex — should not happen post-validation
		}
		if !topicRe.MatchString(topic) {
			continue
		}
		if plan.PayloadCondition != "" {
			payloadRe, err := regexp.Compile(plan.PayloadCondition)
			if err != nil {
				continue
			}
			if !payloadRe.MatchString(payload) {
				continue
			}
		}

		sources, err := m.ListContextSources(ctx, plan.ID)
		if err != nil {
			return nil, fmt.Errorf("MatchWorkPlans: sources for work plan %s: %w", plan.ID, err)
		}
		matches = append(matches, WorkPlanMatch{WorkPlan: plan, ContextSources: sources})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].WorkPlan.Ordinality < matches[j].WorkPlan.Ordinality
	})
	return matches, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// validateWorkPlanRegex compiles triggerTopic and (if non-empty) payloadCondition
// as Go regular expressions, returning [ErrInvalidRegex] on the first failure.
func validateWorkPlanRegex(triggerTopic, payloadCondition string) error {
	if _, err := regexp.Compile(triggerTopic); err != nil {
		return fmt.Errorf("%w: trigger_topic: %v", ErrInvalidRegex, err)
	}
	if payloadCondition != "" {
		if _, err := regexp.Compile(payloadCondition); err != nil {
			return fmt.Errorf("%w: payload_condition: %v", ErrInvalidRegex, err)
		}
	}
	return nil
}

// contextSourceTypeIDAndProps returns the TypeID and property map for the given
// AddContextSourceRequest.
func contextSourceTypeIDAndProps(req AddContextSourceRequest) (string, map[string]any) {
	switch req.SourceType {
	case ContextSourceGit:
		g := req.Git
		if g == nil {
			g = &GitContextSourceConfig{}
		}
		return "GitContextSource", map[string]any{
			"signals":     g.Signals,
			"max_results": g.MaxResults,
			"match_mode":  g.MatchMode,
			"cascade":     g.Cascade,
			"file_types":  g.FileTypes,
		}
	case ContextSourceComm:
		c := req.Comm
		if c == nil {
			c = &CommContextSourceConfig{}
		}
		return "CommContextSource", map[string]any{
			"lookback_days": c.LookbackDays,
			"max_results":   c.MaxResults,
		}
	default: // ContextSourceWork
		w := req.Work
		if w == nil {
			w = &WorkContextSourceConfig{}
		}
		return "WorkContextSource", map[string]any{
			"include_description": w.IncludeDescription,
			"include_history":     w.IncludeHistory,
		}
	}
}

// isContextSourceTypeID reports whether typeID is one of the three context
// source type identifiers.
func isContextSourceTypeID(typeID string) bool {
	return typeID == "GitContextSource" || typeID == "CommContextSource" || typeID == "WorkContextSource"
}

// entityToWorkPlan converts a raw [entitygraph.Entity] of type "WorkPlan" to a
// domain [WorkPlan].
func entityToWorkPlan(e entitygraph.Entity, agencyID string) WorkPlan {
	p := e.Properties
	return WorkPlan{
		ID:                e.ID,
		AgencyID:          agencyID,
		Code:              strProp(p, "code"),
		Name:              strProp(p, "name"),
		Description:       strProp(p, "description"),
		TriggerTopic:      strProp(p, "trigger_topic"),
		PayloadCondition:  strProp(p, "payload_condition"),
		Instructions:      strProp(p, "instructions"),
		AgentID:           strProp(p, "agent_id"),
		AgentCode:         strProp(p, "agent_code"),
		FunctionCode:      strProp(p, "function_code"),
		FunctionParams:    strProp(p, "function_params"),
		HandlerService:    strProp(p, "handler_service"),
		Enabled:           boolProp(p, "enabled"),
		Ordinality:        intProp(p, "ordinality"),
		SuccessEvent:       strProp(p, "success_event"),
		FailureEvent:       strProp(p, "failure_event"),
		OnFailurePipeline:  strProp(p, "on_failure_pipeline"),
		StepTimeout:        strProp(p, "step_timeout"),
		ReviewStepType:     strProp(p, "review_step_type"),
		ReviewTriggerTopic: strProp(p, "review_trigger_topic"),
		ReviewSuccessTopic: strProp(p, "review_success_topic"),
		ReviewFailureTopic: strProp(p, "review_failure_topic"),
	}
}

// entityToContextSource converts a raw [entitygraph.Entity] of a context
// source type to a domain [ContextSource]. The TypeID determines which typed
// config struct is populated.
func entityToContextSource(e entitygraph.Entity, workPlanID string) ContextSource {
	p := e.Properties
	cs := ContextSource{
		ID:         e.ID,
		WorkPlanID: workPlanID,
		SourceType: ContextSourceType(e.TypeID),
	}
	switch e.TypeID {
	case "GitContextSource":
		cs.Git = &GitContextSourceConfig{
			Signals:    strProp(p, "signals"),
			MaxResults: intProp(p, "max_results"),
			MatchMode:  strProp(p, "match_mode"),
			Cascade:    boolProp(p, "cascade"),
			FileTypes:  strProp(p, "file_types"),
		}
	case "CommContextSource":
		cs.Comm = &CommContextSourceConfig{
			LookbackDays: intProp(p, "lookback_days"),
			MaxResults:   intProp(p, "max_results"),
		}
	case "WorkContextSource":
		cs.Work = &WorkContextSourceConfig{
			IncludeDescription: boolProp(p, "include_description"),
			IncludeHistory:     boolProp(p, "include_history"),
		}
	}
	return cs
}

// intProp returns the int value of key in props, defaulting to 0.
func intProp(props map[string]any, key string) int {
	if v, ok := props[key]; ok {
		switch vv := v.(type) {
		case int:
			return vv
		case float64:
			return int(vv)
		}
	}
	return 0
}
