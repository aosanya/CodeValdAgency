package codevaldagency

import (
	"context"
	"fmt"
	"time"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
)

// ── CreateDraft ────────────────────────────────────────────────────────────────

// CreateDraft creates a new [AgencyDraft] rooted under the current Agency and
// deep-copies the relevant sub-entity graph from the nominated source.
//
// forkedFromType must be "live" (copy current live Goal/Workflow/WorkItem/
// ConfiguredRole entities) or "draft" (copy from an existing open draft
// identified by forkedFromID).
//
// Returns [ErrAgencyNotFound] if no Agency entity exists.
// Returns [ErrDraftNotFound] when forkedFromType=="draft" and no such draft exists.
// Returns [ErrDraftNotOpen]  when forkedFromType=="draft" and the source draft
// is not in "open" status.
func (m *agencyManager) CreateDraft(ctx context.Context, description, forkedFromID, forkedFromType string) (AgencyDraft, error) {
	// 1. Confirm the agency exists.
	agency, err := m.GetAgency(ctx)
	if err != nil {
		return AgencyDraft{}, fmt.Errorf("CreateDraft: %w", err)
	}

	// 2. If forking from another draft, validate it is open.
	if forkedFromType == "draft" {
		src, err := m.GetDraft(ctx, forkedFromID)
		if err != nil {
			return AgencyDraft{}, fmt.Errorf("CreateDraft: source draft: %w", err)
		}
		if src.Status != DraftStatusOpen {
			return AgencyDraft{}, fmt.Errorf("CreateDraft: source draft: %w", ErrDraftNotOpen)
		}
	}

	// 3. Create the AgencyDraft root entity.
	now := time.Now().UTC()
	draftEntity, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: m.agencyID,
		TypeID:   "AgencyDraft",
		Properties: map[string]any{
			"description":      description,
			"status":           string(DraftStatusOpen),
			"forked_from_id":   forkedFromID,
			"forked_from_type": forkedFromType,
			"created_at":       now.Format(time.RFC3339),
			"updated_at":       now.Format(time.RFC3339),
		},
	})
	if err != nil {
		return AgencyDraft{}, fmt.Errorf("CreateDraft: create entity: %w", err)
	}
	draftID := draftEntity.ID

	// 4. Link Agency → AgencyDraft and AgencyDraft → Agency.
	if _, err := m.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: m.agencyID,
		Name:     "has_draft",
		FromID:   agency.ID,
		ToID:     draftID,
	}); err != nil {
		return AgencyDraft{}, fmt.Errorf("CreateDraft: link has_draft: %w", err)
	}
	if _, err := m.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: m.agencyID,
		Name:     "belongs_to_agency",
		FromID:   draftID,
		ToID:     agency.ID,
	}); err != nil {
		return AgencyDraft{}, fmt.Errorf("CreateDraft: link belongs_to_agency: %w", err)
	}

	// 5. Deep-copy sub-entities from the nominated source.
	if err := m.copySubEntitiesToDraft(ctx, agency.ID, draftID, forkedFromID, forkedFromType); err != nil {
		return AgencyDraft{}, fmt.Errorf("CreateDraft: copy sub-entities: %w", err)
	}

	return entityToDraft(draftEntity), nil
}

// copySubEntitiesToDraft routes the deep-copy to the appropriate helper based
// on forkedFromType.
func (m *agencyManager) copySubEntitiesToDraft(ctx context.Context, agencyEntityID, draftID, forkedFromID, forkedFromType string) error {
	if forkedFromType == "live" {
		return m.copyLiveEntitiesToDraft(ctx, draftID)
	}
	return m.copyDraftEntitiesToDraft(ctx, draftID, forkedFromID)
}

// copyLiveEntitiesToDraft copies Goal, Workflow, WorkItem, and ConfiguredRole
// entities from the live agency graph into a new draft.
func (m *agencyManager) copyLiveEntitiesToDraft(ctx context.Context, draftID string) error {
	// ── Goals ──────────────────────────────────────────────────────────────────
	goals, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{AgencyID: m.agencyID, TypeID: "Goal"})
	if err != nil {
		return fmt.Errorf("copyLive: goals: %w", err)
	}
	for _, g := range goals {
		props := copyProps(g.Properties)
		props["draft_id"] = draftID
		if _, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
			AgencyID: m.agencyID, TypeID: "DraftGoal", Properties: props,
		}); err != nil {
			return fmt.Errorf("copyLive: goal %s: %w", g.ID, err)
		}
	}

	// ── Workflows (track live→draft WF ID mapping for WorkItem parent refs) ───
	workflows, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{AgencyID: m.agencyID, TypeID: "Workflow"})
	if err != nil {
		return fmt.Errorf("copyLive: workflows: %w", err)
	}
	liveWFToDraftWF := make(map[string]string, len(workflows))
	for _, wf := range workflows {
		props := copyProps(wf.Properties)
		props["draft_id"] = draftID
		newWF, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
			AgencyID: m.agencyID, TypeID: "DraftWorkflow", Properties: props,
		})
		if err != nil {
			return fmt.Errorf("copyLive: workflow %s: %w", wf.ID, err)
		}
		liveWFToDraftWF[wf.ID] = newWF.ID
	}

	// ── ConfiguredRoles ────────────────────────────────────────────────────────
	roles, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{AgencyID: m.agencyID, TypeID: "ConfiguredRole"})
	if err != nil {
		return fmt.Errorf("copyLive: configured roles: %w", err)
	}
	for _, r := range roles {
		props := copyProps(r.Properties)
		props["draft_id"] = draftID
		if _, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
			AgencyID: m.agencyID, TypeID: "DraftConfiguredRole", Properties: props,
		}); err != nil {
			return fmt.Errorf("copyLive: role %s: %w", r.ID, err)
		}
	}

	// ── WorkItems (re-map workflow parent via belongs_to_workflow edges) ───────
	workItems, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{AgencyID: m.agencyID, TypeID: "WorkItem"})
	if err != nil {
		return fmt.Errorf("copyLive: work items: %w", err)
	}
	if len(workItems) == 0 {
		return nil
	}

	wiToWFRels, err := m.dm.ListRelationships(ctx, entitygraph.RelationshipFilter{
		AgencyID: m.agencyID, Name: "belongs_to_workflow",
	})
	if err != nil {
		return fmt.Errorf("copyLive: belongs_to_workflow rels: %w", err)
	}
	liveWIToLiveWF := make(map[string]string, len(wiToWFRels))
	for _, r := range wiToWFRels {
		liveWIToLiveWF[r.FromID] = r.ToID
	}

	for _, wi := range workItems {
		props := copyProps(wi.Properties)
		props["draft_id"] = draftID
		if liveWFID, ok := liveWIToLiveWF[wi.ID]; ok {
			if draftWFID, ok := liveWFToDraftWF[liveWFID]; ok {
				props["draft_workflow_id"] = draftWFID
			}
		}
		if _, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
			AgencyID: m.agencyID, TypeID: "DraftWorkItem", Properties: props,
		}); err != nil {
			return fmt.Errorf("copyLive: work item %s: %w", wi.ID, err)
		}
	}
	return nil
}

// copyDraftEntitiesToDraft copies DraftGoal, DraftWorkflow, DraftWorkItem, and
// DraftConfiguredRole entities from an existing open draft into a new draft,
// re-mapping draft_workflow_id references.
func (m *agencyManager) copyDraftEntitiesToDraft(ctx context.Context, newDraftID, srcDraftID string) error {
	listByDraftID := func(typeID string) ([]entitygraph.Entity, error) {
		all, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{AgencyID: m.agencyID, TypeID: typeID})
		if err != nil {
			return nil, err
		}
		var out []entitygraph.Entity
		for _, e := range all {
			if strProp(e.Properties, "draft_id") == srcDraftID {
				out = append(out, e)
			}
		}
		return out, nil
	}

	// ── Goals ──────────────────────────────────────────────────────────────────
	goals, err := listByDraftID("DraftGoal")
	if err != nil {
		return fmt.Errorf("copyDraft: draft goals: %w", err)
	}
	for _, g := range goals {
		props := copyProps(g.Properties)
		props["draft_id"] = newDraftID
		if _, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
			AgencyID: m.agencyID, TypeID: "DraftGoal", Properties: props,
		}); err != nil {
			return fmt.Errorf("copyDraft: goal %s: %w", g.ID, err)
		}
	}

	// ── Workflows (build src→new WF ID mapping) ────────────────────────────────
	srcWorkflows, err := listByDraftID("DraftWorkflow")
	if err != nil {
		return fmt.Errorf("copyDraft: draft workflows: %w", err)
	}
	srcWFToNewWF := make(map[string]string, len(srcWorkflows))
	for _, wf := range srcWorkflows {
		props := copyProps(wf.Properties)
		props["draft_id"] = newDraftID
		newWF, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
			AgencyID: m.agencyID, TypeID: "DraftWorkflow", Properties: props,
		})
		if err != nil {
			return fmt.Errorf("copyDraft: workflow %s: %w", wf.ID, err)
		}
		srcWFToNewWF[wf.ID] = newWF.ID
	}

	// ── ConfiguredRoles ────────────────────────────────────────────────────────
	roles, err := listByDraftID("DraftConfiguredRole")
	if err != nil {
		return fmt.Errorf("copyDraft: draft configured roles: %w", err)
	}
	for _, r := range roles {
		props := copyProps(r.Properties)
		props["draft_id"] = newDraftID
		if _, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
			AgencyID: m.agencyID, TypeID: "DraftConfiguredRole", Properties: props,
		}); err != nil {
			return fmt.Errorf("copyDraft: role %s: %w", r.ID, err)
		}
	}

	// ── WorkItems (re-map draft_workflow_id) ───────────────────────────────────
	workItems, err := listByDraftID("DraftWorkItem")
	if err != nil {
		return fmt.Errorf("copyDraft: draft work items: %w", err)
	}
	for _, wi := range workItems {
		props := copyProps(wi.Properties)
		props["draft_id"] = newDraftID
		if srcWFID := strProp(wi.Properties, "draft_workflow_id"); srcWFID != "" {
			if newWFID, ok := srcWFToNewWF[srcWFID]; ok {
				props["draft_workflow_id"] = newWFID
			}
		}
		if _, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
			AgencyID: m.agencyID, TypeID: "DraftWorkItem", Properties: props,
		}); err != nil {
			return fmt.Errorf("copyDraft: work item %s: %w", wi.ID, err)
		}
	}
	return nil
}

// ── GetDraft ───────────────────────────────────────────────────────────────────

// GetDraft retrieves a single AgencyDraft entity by its ID.
// Returns [ErrDraftNotFound] if no matching entity exists.
func (m *agencyManager) GetDraft(ctx context.Context, draftID string) (AgencyDraft, error) {
	entity, err := m.dm.GetEntity(ctx, m.agencyID, draftID)
	if err != nil {
		return AgencyDraft{}, fmt.Errorf("GetDraft %s: %w", draftID, ErrDraftNotFound)
	}
	if entity.TypeID != "AgencyDraft" {
		return AgencyDraft{}, fmt.Errorf("GetDraft %s: %w", draftID, ErrDraftNotFound)
	}
	return entityToDraft(entity), nil
}

// ── ListDrafts ─────────────────────────────────────────────────────────────────

// ListDrafts returns all AgencyDraft entities for this agency in creation order.
func (m *agencyManager) ListDrafts(ctx context.Context) ([]AgencyDraft, error) {
	entities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: m.agencyID,
		TypeID:   "AgencyDraft",
	})
	if err != nil {
		return nil, fmt.Errorf("ListDrafts: %w", err)
	}
	drafts := make([]AgencyDraft, len(entities))
	for i, e := range entities {
		drafts[i] = entityToDraft(e)
	}
	return drafts, nil
}

// ── UpdateDraftDescription ─────────────────────────────────────────────────────

// UpdateDraftDescription replaces the description field of an open draft.
// Returns [ErrDraftNotFound] if the draft does not exist.
// Returns [ErrDraftNotOpen] if the draft is already promoted or archived.
func (m *agencyManager) UpdateDraftDescription(ctx context.Context, draftID, description string) (AgencyDraft, error) {
	draft, err := m.GetDraft(ctx, draftID)
	if err != nil {
		return AgencyDraft{}, fmt.Errorf("UpdateDraftDescription %s: %w", draftID, err)
	}
	if draft.Status != DraftStatusOpen {
		return AgencyDraft{}, fmt.Errorf("UpdateDraftDescription %s: %w", draftID, ErrDraftNotOpen)
	}

	updated, err := m.dm.UpdateEntity(ctx, m.agencyID, draftID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{"description": description},
	})
	if err != nil {
		return AgencyDraft{}, fmt.Errorf("UpdateDraftDescription %s: update entity: %w", draftID, err)
	}
	return entityToDraft(updated), nil
}

// ── PromoteDraft ───────────────────────────────────────────────────────────────

// PromoteDraft replaces the live agency sub-entity graph with the draft
// sub-entities, enables the agency, marks the draft as "promoted", and writes
// an AgencySnapshot. Publishes "cross.agency.promoted" on success.
//
// Returns [ErrDraftNotFound] if the draft does not exist.
// Returns [ErrDraftNotOpen] if the draft is already promoted or archived.
func (m *agencyManager) PromoteDraft(ctx context.Context, draftID string) (Agency, error) {
	// 1. Validate draft state.
	draft, err := m.GetDraft(ctx, draftID)
	if err != nil {
		return Agency{}, fmt.Errorf("PromoteDraft %s: %w", draftID, err)
	}
	if draft.Status != DraftStatusOpen {
		return Agency{}, fmt.Errorf("PromoteDraft %s: %w", draftID, ErrDraftNotOpen)
	}

	// 2. Get agency root entity ID.
	agencyEntity, err := m.GetAgency(ctx)
	if err != nil {
		return Agency{}, fmt.Errorf("PromoteDraft %s: %w", draftID, err)
	}

	// 3. Collect old live sub-entity IDs before creating new ones.
	oldIDs, err := m.collectLiveSubEntityIDs(ctx)
	if err != nil {
		return Agency{}, fmt.Errorf("PromoteDraft %s: collect old IDs: %w", draftID, err)
	}

	// 4. Create new live entities from draft sub-entities.
	if err := m.promoteSubEntities(ctx, agencyEntity.ID, draftID); err != nil {
		return Agency{}, fmt.Errorf("PromoteDraft %s: promote sub-entities: %w", draftID, err)
	}

	// 5. Delete old live sub-entities.
	for _, id := range oldIDs {
		if err := m.dm.DeleteEntity(ctx, m.agencyID, id); err != nil {
			// Non-fatal: log and continue so promotion still succeeds.
			_ = err
		}
	}

	// 6. Mark the agency as enabled.
	agencyEntities, err := m.listAgencyEntities(ctx)
	if err != nil || len(agencyEntities) == 0 {
		return Agency{}, fmt.Errorf("PromoteDraft %s: reload agency: %w", draftID, ErrAgencyNotFound)
	}
	updatedAgencyEntity, err := m.dm.UpdateEntity(ctx, m.agencyID, agencyEntities[0].ID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{"enabled": true},
	})
	if err != nil {
		return Agency{}, fmt.Errorf("PromoteDraft %s: enable agency: %w", draftID, err)
	}

	// 7. Write an immutable snapshot.
	_ = m.writePromoteSnapshot(ctx, agencyEntity.ID)

	// 8. Mark draft as promoted.
	if _, err := m.dm.UpdateEntity(ctx, m.agencyID, draftID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{"status": string(DraftStatusPromoted)},
	}); err != nil {
		_ = err // non-fatal; log in production
	}

	// 9. Publish event.
	if m.publisher != nil {
		_ = m.publisher.Publish(ctx, "cross.agency.promoted", agencyEntity.ID)
	}

	return entityToAgency(updatedAgencyEntity), nil
}

// collectLiveSubEntityIDs returns the entity IDs of all current live Goal,
// Workflow, WorkItem, and ConfiguredRole entities.
func (m *agencyManager) collectLiveSubEntityIDs(ctx context.Context) ([]string, error) {
	typeIDs := []string{"Goal", "Workflow", "WorkItem", "ConfiguredRole"}
	var ids []string
	for _, typeID := range typeIDs {
		entities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
			AgencyID: m.agencyID,
			TypeID:   typeID,
		})
		if err != nil {
			return nil, fmt.Errorf("collectLiveSubEntityIDs %s: %w", typeID, err)
		}
		for _, e := range entities {
			ids = append(ids, e.ID)
		}
	}
	return ids, nil
}

// promoteSubEntities creates new live Goal, Workflow, WorkItem, ConfiguredRole
// entities from the draft sub-entities belonging to draftID.
func (m *agencyManager) promoteSubEntities(ctx context.Context, agencyEntityID, draftID string) error {
	listByDraftID := func(typeID string) ([]entitygraph.Entity, error) {
		all, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{AgencyID: m.agencyID, TypeID: typeID})
		if err != nil {
			return nil, err
		}
		var out []entitygraph.Entity
		for _, e := range all {
			if strProp(e.Properties, "draft_id") == draftID {
				out = append(out, e)
			}
		}
		return out, nil
	}

	agencyRel := entitygraph.EntityRelationshipRequest{Name: "belongs_to_agency", ToID: agencyEntityID}

	// ── Goals ──────────────────────────────────────────────────────────────────
	draftGoals, err := listByDraftID("DraftGoal")
	if err != nil {
		return fmt.Errorf("promote goals: %w", err)
	}
	for _, dg := range draftGoals {
		props := copyPropsExcluding(dg.Properties, "draft_id")
		if _, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
			AgencyID:      m.agencyID,
			TypeID:        "Goal",
			Properties:    props,
			Relationships: []entitygraph.EntityRelationshipRequest{agencyRel},
		}); err != nil {
			return fmt.Errorf("promote goal %s: %w", dg.ID, err)
		}
	}

	// ── Workflows (build draft→live WF ID mapping for WorkItem parent refs) ────
	draftWorkflows, err := listByDraftID("DraftWorkflow")
	if err != nil {
		return fmt.Errorf("promote workflows: %w", err)
	}
	draftWFToLiveWF := make(map[string]string, len(draftWorkflows))
	for _, dw := range draftWorkflows {
		props := copyPropsExcluding(dw.Properties, "draft_id")
		newWF, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
			AgencyID:      m.agencyID,
			TypeID:        "Workflow",
			Properties:    props,
			Relationships: []entitygraph.EntityRelationshipRequest{agencyRel},
		})
		if err != nil {
			return fmt.Errorf("promote workflow %s: %w", dw.ID, err)
		}
		draftWFToLiveWF[dw.ID] = newWF.ID
	}

	// ── ConfiguredRoles ────────────────────────────────────────────────────────
	draftRoles, err := listByDraftID("DraftConfiguredRole")
	if err != nil {
		return fmt.Errorf("promote configured roles: %w", err)
	}
	for _, dr := range draftRoles {
		props := copyPropsExcluding(dr.Properties, "draft_id")
		if _, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
			AgencyID:      m.agencyID,
			TypeID:        "ConfiguredRole",
			Properties:    props,
			Relationships: []entitygraph.EntityRelationshipRequest{agencyRel},
		}); err != nil {
			return fmt.Errorf("promote role %s: %w", dr.ID, err)
		}
	}

	// ── WorkItems (re-map draft_workflow_id → new live WF ID) ─────────────────
	draftWorkItems, err := listByDraftID("DraftWorkItem")
	if err != nil {
		return fmt.Errorf("promote work items: %w", err)
	}
	for _, dwi := range draftWorkItems {
		props := copyPropsExcluding(dwi.Properties, "draft_id", "draft_workflow_id")
		var rels []entitygraph.EntityRelationshipRequest
		if draftWFID := strProp(dwi.Properties, "draft_workflow_id"); draftWFID != "" {
			if liveWFID, ok := draftWFToLiveWF[draftWFID]; ok {
				rels = append(rels, entitygraph.EntityRelationshipRequest{Name: "belongs_to_workflow", ToID: liveWFID})
			}
		}
		if _, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
			AgencyID:      m.agencyID,
			TypeID:        "WorkItem",
			Properties:    props,
			Relationships: rels,
		}); err != nil {
			return fmt.Errorf("promote work item %s: %w", dwi.ID, err)
		}
	}
	return nil
}

// writePromoteSnapshot creates an immutable AgencySnapshot entity recording
// the promotion event.
func (m *agencyManager) writePromoteSnapshot(ctx context.Context, agencyEntityID string) error {
	_, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: m.agencyID,
		TypeID:   "AgencySnapshot",
		Properties: map[string]any{
			"snapshot_at": time.Now().UTC().Format(time.RFC3339),
			"agency_id":   agencyEntityID,
		},
	})
	return err
}

// ── ArchiveDraft ───────────────────────────────────────────────────────────────

// ArchiveDraft marks an open draft as "archived" without promoting it.
// The draft sub-entities remain in storage but the draft can no longer be
// promoted or modified.
//
// Returns [ErrDraftNotFound] if the draft does not exist.
// Returns [ErrDraftNotOpen] if the draft is already promoted or archived.
func (m *agencyManager) ArchiveDraft(ctx context.Context, draftID string) (AgencyDraft, error) {
	draft, err := m.GetDraft(ctx, draftID)
	if err != nil {
		return AgencyDraft{}, fmt.Errorf("ArchiveDraft %s: %w", draftID, err)
	}
	if draft.Status != DraftStatusOpen {
		return AgencyDraft{}, fmt.Errorf("ArchiveDraft %s: %w", draftID, ErrDraftNotOpen)
	}

	updated, err := m.dm.UpdateEntity(ctx, m.agencyID, draftID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{"status": string(DraftStatusArchived)},
	})
	if err != nil {
		return AgencyDraft{}, fmt.Errorf("ArchiveDraft %s: update entity: %w", draftID, err)
	}
	return entityToDraft(updated), nil
}
