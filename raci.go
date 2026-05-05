package codevaldagency

import (
	"context"
	"fmt"
	"regexp"
	"sort"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
)

// ── CreateRole ────────────────────────────────────────────────────────────────

// CreateRole stores a new Role entity and links it to the Agency via
// has_role / belongs_to_agency edges.
func (m *agencyManager) CreateRole(ctx context.Context, req CreateRoleRequest) (Role, error) {
	if err := validateRoleRegexes(req.EventTopic, req.PayloadCondition); err != nil {
		return Role{}, err
	}

	agency, err := m.GetAgency(ctx)
	if err != nil {
		return Role{}, fmt.Errorf("CreateRole: %w", err)
	}

	entity, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: m.agencyID,
		TypeID:   "Role",
		Properties: map[string]any{
			"name":              req.Name,
			"description":       req.Description,
			"event_topic":       req.EventTopic,
			"payload_condition": req.PayloadCondition,
			"instructions":      req.Instructions,
			"agent_id":          req.AgentID,
			"enabled":           req.Enabled,
			"ordinality":        req.Ordinality,
		},
	})
	if err != nil {
		return Role{}, fmt.Errorf("CreateRole: create entity: %w", err)
	}

	if _, err := m.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: m.agencyID,
		Name:     "has_role",
		FromID:   agency.ID,
		ToID:     entity.ID,
	}); err != nil {
		return Role{}, fmt.Errorf("CreateRole: link has_role: %w", err)
	}
	if _, err := m.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: m.agencyID,
		Name:     "belongs_to_agency",
		FromID:   entity.ID,
		ToID:     agency.ID,
	}); err != nil {
		return Role{}, fmt.Errorf("CreateRole: link belongs_to_agency: %w", err)
	}

	return entityToRole(entity, m.agencyID), nil
}

// ── GetRole ───────────────────────────────────────────────────────────────────

// GetRole retrieves a single Role by its entity ID.
// Returns [ErrRoleNotFound] if no Role entity with that ID exists.
func (m *agencyManager) GetRole(ctx context.Context, roleID string) (Role, error) {
	entity, err := m.dm.GetEntity(ctx, m.agencyID, roleID)
	if err != nil || entity.TypeID != "Role" {
		return Role{}, fmt.Errorf("GetRole %s: %w", roleID, ErrRoleNotFound)
	}
	return entityToRole(entity, m.agencyID), nil
}

// ── ListRoles ─────────────────────────────────────────────────────────────────

// ListRoles returns all Role entities linked to this Agency.
func (m *agencyManager) ListRoles(ctx context.Context) ([]Role, error) {
	entities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: m.agencyID,
		TypeID:   "Role",
	})
	if err != nil {
		return nil, fmt.Errorf("ListRoles: %w", err)
	}
	roles := make([]Role, len(entities))
	for i, e := range entities {
		roles[i] = entityToRole(e, m.agencyID)
	}
	return roles, nil
}

// ── UpdateRole ────────────────────────────────────────────────────────────────

// UpdateRole applies req to the Role identified by roleID.
// Returns [ErrRoleNotFound] if no Role entity with that ID exists.
// Returns [ErrInvalidRegex] if EventTopic or PayloadCondition is not a valid Go regexp.
func (m *agencyManager) UpdateRole(ctx context.Context, roleID string, req UpdateRoleRequest) (Role, error) {
	if err := validateRoleRegexes(req.EventTopic, req.PayloadCondition); err != nil {
		return Role{}, err
	}
	if _, err := m.GetRole(ctx, roleID); err != nil {
		return Role{}, err
	}

	updated, err := m.dm.UpdateEntity(ctx, m.agencyID, roleID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{
			"name":              req.Name,
			"description":       req.Description,
			"event_topic":       req.EventTopic,
			"payload_condition": req.PayloadCondition,
			"instructions":      req.Instructions,
			"agent_id":          req.AgentID,
			"enabled":           req.Enabled,
			"ordinality":        req.Ordinality,
		},
	})
	if err != nil {
		return Role{}, fmt.Errorf("UpdateRole %s: %w", roleID, err)
	}
	return entityToRole(updated, m.agencyID), nil
}

// ── DeleteRole ────────────────────────────────────────────────────────────────

// DeleteRole removes the Role entity identified by roleID.
// Returns [ErrRoleNotFound] if no Role entity with that ID exists.
func (m *agencyManager) DeleteRole(ctx context.Context, roleID string) error {
	if _, err := m.GetRole(ctx, roleID); err != nil {
		return err
	}
	if err := m.dm.DeleteEntity(ctx, m.agencyID, roleID); err != nil {
		return fmt.Errorf("DeleteRole %s: %w", roleID, err)
	}
	return nil
}

// ── AddContextSource ──────────────────────────────────────────────────────────

// AddContextSource creates a typed ContextSource entity and links it to the
// Role identified by roleID via has_context_source / belongs_to_role edges.
// Returns [ErrRoleNotFound] if no Role entity with that ID exists.
func (m *agencyManager) AddContextSource(ctx context.Context, roleID string, req AddContextSourceRequest) (ContextSource, error) {
	if _, err := m.GetRole(ctx, roleID); err != nil {
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
		FromID:   roleID,
		ToID:     entity.ID,
	}); err != nil {
		return ContextSource{}, fmt.Errorf("AddContextSource: link has_context_source: %w", err)
	}
	if _, err := m.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: m.agencyID,
		Name:     "belongs_to_role",
		FromID:   entity.ID,
		ToID:     roleID,
	}); err != nil {
		return ContextSource{}, fmt.Errorf("AddContextSource: link belongs_to_role: %w", err)
	}

	return entityToContextSource(entity, roleID), nil
}

// ── ListContextSources ────────────────────────────────────────────────────────

// ListContextSources returns all ContextSource entities linked to the Role
// identified by roleID via has_context_source edges.
// Returns [ErrRoleNotFound] if no Role entity with that ID exists.
func (m *agencyManager) ListContextSources(ctx context.Context, roleID string) ([]ContextSource, error) {
	if _, err := m.GetRole(ctx, roleID); err != nil {
		return nil, err
	}

	rels, err := m.dm.ListRelationships(ctx, entitygraph.RelationshipFilter{
		AgencyID: m.agencyID,
		Name:     "has_context_source",
		FromID:   roleID,
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
		sources = append(sources, entityToContextSource(entity, roleID))
	}
	return sources, nil
}

// ── RemoveContextSource ───────────────────────────────────────────────────────

// RemoveContextSource deletes the ContextSource entity identified by sourceID
// and removes its has_context_source edge from the owning Role.
// Returns [ErrContextSourceNotFound] if no such entity exists.
func (m *agencyManager) RemoveContextSource(ctx context.Context, roleID, sourceID string) error {
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

// ── MatchRoles ────────────────────────────────────────────────────────────────

// MatchRoles evaluates topic and payload against all enabled Role entities.
// For each role, EventTopic is compiled as a Go regex and matched against
// topic; if PayloadCondition is non-empty it is also matched against payload.
// Returns all matching roles with their ContextSource entities, ordered by
// Role.Ordinality ascending.
func (m *agencyManager) MatchRoles(ctx context.Context, topic, payload string) ([]RoleMatch, error) {
	roles, err := m.ListRoles(ctx)
	if err != nil {
		return nil, fmt.Errorf("MatchRoles: %w", err)
	}

	var matches []RoleMatch
	for _, role := range roles {
		if !role.Enabled {
			continue
		}
		topicRe, err := regexp.Compile(role.EventTopic)
		if err != nil {
			continue // skip malformed regex — should not happen post-validation
		}
		if !topicRe.MatchString(topic) {
			continue
		}
		if role.PayloadCondition != "" {
			payloadRe, err := regexp.Compile(role.PayloadCondition)
			if err != nil {
				continue
			}
			if !payloadRe.MatchString(payload) {
				continue
			}
		}

		sources, err := m.ListContextSources(ctx, role.ID)
		if err != nil {
			return nil, fmt.Errorf("MatchRoles: sources for role %s: %w", role.ID, err)
		}
		matches = append(matches, RoleMatch{Role: role, ContextSources: sources})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Role.Ordinality < matches[j].Role.Ordinality
	})
	return matches, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// validateRoleRegexes compiles eventTopic and (if non-empty) payloadCondition
// as Go regular expressions, returning [ErrInvalidRegex] on the first failure.
func validateRoleRegexes(eventTopic, payloadCondition string) error {
	if _, err := regexp.Compile(eventTopic); err != nil {
		return fmt.Errorf("%w: event_topic: %v", ErrInvalidRegex, err)
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

// entityToRole converts a raw [entitygraph.Entity] of type "Role" to a domain
// [Role].
func entityToRole(e entitygraph.Entity, agencyID string) Role {
	p := e.Properties
	return Role{
		ID:               e.ID,
		AgencyID:         agencyID,
		Name:             strProp(p, "name"),
		Description:      strProp(p, "description"),
		EventTopic:       strProp(p, "event_topic"),
		PayloadCondition: strProp(p, "payload_condition"),
		Instructions:     strProp(p, "instructions"),
		AgentID:          strProp(p, "agent_id"),
		Enabled:          boolProp(p, "enabled"),
		Ordinality:       intProp(p, "ordinality"),
	}
}

// entityToContextSource converts a raw [entitygraph.Entity] of a context
// source type to a domain [ContextSource]. The TypeID determines which typed
// config struct is populated.
func entityToContextSource(e entitygraph.Entity, roleID string) ContextSource {
	p := e.Properties
	cs := ContextSource{
		ID:         e.ID,
		RoleID:     roleID,
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
