// Package codevaldagency provides agency lifecycle management for the CodeVald
// platform. It exposes [AgencyManager] — the single interface for writing,
// reading, and updating the one agency that lives in this database.
//
// Usage:
//
//	dm, sm, err := arangodb.New(db)
//	mgr := codevaldagency.NewAgencyManager(dm, sm, publisher, agencyID)
//	agency, err := mgr.SetAgencyDetails(ctx, `{"id":"agency-001","name":"Alpha"}`)
package codevaldagency

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
)

// AgencyManager is the primary interface for agency lifecycle management.
// gRPC handlers hold this interface — never the concrete type.
//
// Implementations must be safe for concurrent use.
type AgencyManager interface {
	// SetAgencyDetails replaces the full agency document from a raw JSON string.
	// The JSON must include a non-empty "id" field; all other fields are optional.
	// Returns [ErrInvalidJSON] if the payload cannot be parsed or id is missing.
	// Lifecycle validation is NOT applied — any status value is written as-is.
	// Publishes "cross.agency.created" after every successful write.
	SetAgencyDetails(ctx context.Context, jsonStr string) (Agency, error)

	// GetAgency retrieves the single agency for this database.
	// Returns [ErrAgencyNotFound] if no agency entity exists yet.
	GetAgency(ctx context.Context) (Agency, error)

	// UpdateAgency applies incremental field edits with lifecycle validation.
	// Lifecycle transitions are validated — returns [ErrInvalidLifecycleTransition]
	// if the new status is not reachable from the current status.
	// On draft → active, a guard checks ≥1 Goal and ≥1 Workflow with ≥1 WorkItem;
	// returns [ErrInvalidAgency] if violated. An immutable [AgencySnapshot] entity
	// is written as a side-effect of the draft → active transition.
	// Returns [ErrAgencyNotFound] if no agency entity exists yet.
	UpdateAgency(ctx context.Context, req UpdateAgencyRequest) (Agency, error)

	// GetGoals returns all Goal entities linked to this Agency.
	GetGoals(ctx context.Context) ([]Goal, error)

	// GetWorkflows returns all Workflow entities linked to this Agency,
	// each populated with its ordered WorkItem entities.
	GetWorkflows(ctx context.Context) ([]Workflow, error)

	// GetConfiguredRoles returns all ConfiguredRole entities linked to this Agency.
	GetConfiguredRoles(ctx context.Context) ([]ConfiguredRole, error)

	// PublishAgency creates an immutable versioned publication of the current
	// agency state. The agency [Status] is NOT changed by this operation.
	// Version is auto-incremented from the last publication (starts at 1).
	// Publishes "cross.agency.published" after every successful write.
	// Returns [ErrAgencyNotFound] if no agency entity exists yet.
	PublishAgency(ctx context.Context) (AgencyPublication, error)

	// GetPublication retrieves a single publication by its version number.
	// Returns [ErrPublicationNotFound] if no publication with that version exists.
	GetPublication(ctx context.Context, version int) (AgencyPublication, error)

	// ListPublications returns all publications for this agency in ascending
	// version order.
	ListPublications(ctx context.Context) ([]AgencyPublication, error)
}

// AgencySchemaManager manages schema versions for the Agency entity graph.
// It is a type alias for [entitygraph.SchemaManager] from CodeValdSharedLib.
type AgencySchemaManager = entitygraph.SchemaManager

// CrossPublisher publishes agency lifecycle events to CodeValdCross.
// Implementations must be safe for concurrent use. A nil CrossPublisher is
// valid — publish calls are silently skipped.
type CrossPublisher interface {
	// Publish delivers an event for the given topic and agencyID to
	// CodeValdCross. Errors are non-fatal: implementations should log and
	// return nil for best-effort delivery.
	Publish(ctx context.Context, topic string, agencyID string) error
}

// agencyManager is the concrete implementation of [AgencyManager].
// It wraps [entitygraph.DataManager] to expose agency-specific convenience
// methods. All storage operations go through dm; no bespoke Backend interface
// is used.
type agencyManager struct {
	dm        entitygraph.DataManager // graph CRUD — injected by cmd/main.go
	sm        AgencySchemaManager     // schema versioning — injected by cmd/main.go
	publisher CrossPublisher          // optional; nil = skip event publishing
	agencyID  string                  // the single agency ID for this database
}

// NewAgencyManager constructs an [AgencyManager] backed by the given
// [entitygraph.DataManager] and [AgencySchemaManager].
// agencyID is the single agency scoped to this database; it is passed to every
// DataManager call as the scope key.
// pub may be nil — cross-service events are skipped when no publisher is set.
func NewAgencyManager(
	dm entitygraph.DataManager,
	sm AgencySchemaManager,
	pub CrossPublisher,
	agencyID string,
) AgencyManager {
	return &agencyManager{dm: dm, sm: sm, publisher: pub, agencyID: agencyID}
}

// ── SetAgencyDetails ──────────────────────────────────────────────────────────

// SetAgencyDetails parses the JSON payload and upserts the root Agency entity.
// If no Agency entity exists yet it calls CreateEntity; otherwise UpdateEntity.
// Publishes "cross.agency.created" on every successful write.
func (m *agencyManager) SetAgencyDetails(ctx context.Context, jsonStr string) (Agency, error) {
	var raw struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Mission string `json:"mission"`
		Vision  string `json:"vision"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return Agency{}, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}
	if raw.ID == "" {
		return Agency{}, fmt.Errorf("%w: \"id\" field is required", ErrInvalidJSON)
	}

	props := map[string]any{
		"name":    raw.Name,
		"mission": raw.Mission,
		"vision":  raw.Vision,
		"status":  raw.Status,
	}

	// Check whether an Agency entity already exists.
	existing, err := m.listAgencyEntities(ctx)
	if err != nil {
		return Agency{}, fmt.Errorf("SetAgencyDetails: list: %w", err)
	}

	var entity entitygraph.Entity
	if len(existing) == 0 {
		entity, err = m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
			AgencyID:   m.agencyID,
			TypeID:     "Agency",
			Properties: props,
		})
	} else {
		entity, err = m.dm.UpdateEntity(ctx, m.agencyID, existing[0].ID, entitygraph.UpdateEntityRequest{
			Properties: props,
		})
	}
	if err != nil {
		return Agency{}, fmt.Errorf("SetAgencyDetails: %w", err)
	}

	agency := entityToAgency(entity)

	if m.publisher != nil {
		_ = m.publisher.Publish(ctx, "cross.agency.created", agency.ID)
	}
	return agency, nil
}

// ── GetAgency ─────────────────────────────────────────────────────────────────

// GetAgency returns the single Agency entity stored in this database.
// Returns [ErrAgencyNotFound] if no Agency entity exists yet.
func (m *agencyManager) GetAgency(ctx context.Context) (Agency, error) {
	entities, err := m.listAgencyEntities(ctx)
	if err != nil {
		return Agency{}, fmt.Errorf("GetAgency: %w", err)
	}
	if len(entities) == 0 {
		return Agency{}, ErrAgencyNotFound
	}
	return entityToAgency(entities[0]), nil
}

// ── UpdateAgency ──────────────────────────────────────────────────────────────

// UpdateAgency applies lifecycle-validated partial updates to the Agency entity.
// On draft → active it enforces ≥1 Goal and ≥1 Workflow-with-WorkItem guard,
// then writes an immutable [AgencySnapshot] entity as a side-effect.
func (m *agencyManager) UpdateAgency(ctx context.Context, req UpdateAgencyRequest) (Agency, error) {
	current, err := m.GetAgency(ctx)
	if err != nil {
		return Agency{}, err
	}

	if req.Status != "" {
		if current.Status == LifecycleAchieved {
			return Agency{}, ErrInvalidLifecycleTransition
		}
		if req.Status != current.Status {
			if !current.Status.CanTransitionTo(req.Status) {
				return Agency{}, ErrInvalidLifecycleTransition
			}
			if current.Status == LifecycleDraft && req.Status == LifecycleActive {
				if err := m.checkActivationGuards(ctx); err != nil {
					return Agency{}, err
				}
				if sErr := m.writeSnapshot(ctx, current.ID); sErr != nil {
					// Non-fatal — log and continue.
					_ = sErr
				}
			}
		}
	}

	props := map[string]any{}
	if req.Name != "" {
		props["name"] = req.Name
	}
	if req.Mission != "" {
		props["mission"] = req.Mission
	}
	if req.Vision != "" {
		props["vision"] = req.Vision
	}
	if req.Status != "" {
		props["status"] = string(req.Status)
	}

	entities, err := m.listAgencyEntities(ctx)
	if err != nil || len(entities) == 0 {
		return Agency{}, ErrAgencyNotFound
	}

	updated, err := m.dm.UpdateEntity(ctx, m.agencyID, entities[0].ID, entitygraph.UpdateEntityRequest{
		Properties: props,
	})
	if err != nil {
		return Agency{}, fmt.Errorf("UpdateAgency: %w", err)
	}
	return entityToAgency(updated), nil
}

// ── GetGoals ─────────────────────────────────────────────────────────────────

// GetGoals returns all Goal entities linked to this Agency via has_goal edges.
func (m *agencyManager) GetGoals(ctx context.Context) ([]Goal, error) {
	entities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: m.agencyID,
		TypeID:   "Goal",
	})
	if err != nil {
		return nil, fmt.Errorf("GetGoals: %w", err)
	}
	goals := make([]Goal, len(entities))
	for i, e := range entities {
		goals[i] = entityToGoal(e)
	}
	return goals, nil
}

// ── GetWorkflows ──────────────────────────────────────────────────────────────

// GetWorkflows returns all Workflow entities linked to this Agency, each
// populated with its ordered WorkItem entities (fetched via has_work_item edges).
func (m *agencyManager) GetWorkflows(ctx context.Context) ([]Workflow, error) {
	wfEntities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: m.agencyID,
		TypeID:   "Workflow",
	})
	if err != nil {
		return nil, fmt.Errorf("GetWorkflows: %w", err)
	}

	wiEntities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: m.agencyID,
		TypeID:   "WorkItem",
	})
	if err != nil {
		return nil, fmt.Errorf("GetWorkflows: list work items: %w", err)
	}

	// Build a relationship map: workflowID → []WorkItem via has_work_item edges.
	rels, err := m.dm.ListRelationships(ctx, entitygraph.RelationshipFilter{
		AgencyID: m.agencyID,
		Name:     "has_work_item",
	})
	if err != nil {
		return nil, fmt.Errorf("GetWorkflows: list relationships: %w", err)
	}

	wiByID := make(map[string]entitygraph.Entity, len(wiEntities))
	for _, wi := range wiEntities {
		wiByID[wi.ID] = wi
	}

	wfItems := make(map[string][]WorkItem)
	for _, rel := range rels {
		if wi, ok := wiByID[rel.ToID]; ok {
			wfItems[rel.FromID] = append(wfItems[rel.FromID], entityToWorkItem(wi))
		}
	}

	workflows := make([]Workflow, len(wfEntities))
	for i, e := range wfEntities {
		wf := entityToWorkflow(e)
		wf.WorkItems = wfItems[e.ID]
		workflows[i] = wf
	}
	return workflows, nil
}

// ── GetConfiguredRoles ────────────────────────────────────────────────────────

// GetConfiguredRoles returns all ConfiguredRole entities linked to this Agency.
func (m *agencyManager) GetConfiguredRoles(ctx context.Context) ([]ConfiguredRole, error) {
	entities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: m.agencyID,
		TypeID:   "ConfiguredRole",
	})
	if err != nil {
		return nil, fmt.Errorf("GetConfiguredRoles: %w", err)
	}
	roles := make([]ConfiguredRole, len(entities))
	for i, e := range entities {
		roles[i] = entityToConfiguredRole(e)
	}
	return roles, nil
}

// ── PublishAgency ─────────────────────────────────────────────────────────────

// PublishAgency creates an immutable [AgencyPublication] entity with an
// auto-incremented version. Publishes "cross.agency.published" on success.
func (m *agencyManager) PublishAgency(ctx context.Context) (AgencyPublication, error) {
	agency, err := m.GetAgency(ctx)
	if err != nil {
		return AgencyPublication{}, err
	}

	version, err := m.nextPublicationVersion(ctx)
	if err != nil {
		return AgencyPublication{}, fmt.Errorf("PublishAgency: next version: %w", err)
	}

	now := time.Now().UTC()
	entity, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: m.agencyID,
		TypeID:   "AgencyPublication",
		Properties: map[string]any{
			"version":      version,
			"tag":          fmt.Sprintf("v%d", version),
			"published_at": now.Format(time.RFC3339),
		},
	})
	if err != nil {
		return AgencyPublication{}, fmt.Errorf("PublishAgency: create entity: %w", err)
	}

	pub := AgencyPublication{
		ID:          entity.ID,
		Agency:      agency,
		Version:     version,
		Tag:         fmt.Sprintf("v%d", version),
		PublishedAt: now,
	}

	if m.publisher != nil {
		_ = m.publisher.Publish(ctx, "cross.agency.published", agency.ID)
	}
	return pub, nil
}

// ── GetPublication / ListPublications ─────────────────────────────────────────

// GetPublication retrieves a publication by version number.
// Returns [ErrPublicationNotFound] if no match exists.
func (m *agencyManager) GetPublication(ctx context.Context, version int) (AgencyPublication, error) {
	pubs, err := m.ListPublications(ctx)
	if err != nil {
		return AgencyPublication{}, err
	}
	for _, p := range pubs {
		if p.Version == version {
			return p, nil
		}
	}
	return AgencyPublication{}, ErrPublicationNotFound
}

// ListPublications returns all publications in ascending version order.
func (m *agencyManager) ListPublications(ctx context.Context) ([]AgencyPublication, error) {
	entities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: m.agencyID,
		TypeID:   "AgencyPublication",
	})
	if err != nil {
		return nil, fmt.Errorf("ListPublications: %w", err)
	}

	agency, err := m.GetAgency(ctx)
	if err != nil {
		// Best-effort — return publications without embedded Agency.
		agency = Agency{}
	}

	pubs := make([]AgencyPublication, 0, len(entities))
	for _, e := range entities {
		pub := entityToPublication(e, agency)
		pubs = append(pubs, pub)
	}
	// Sort ascending by version.
	for i := 0; i < len(pubs); i++ {
		for j := i + 1; j < len(pubs); j++ {
			if pubs[j].Version < pubs[i].Version {
				pubs[i], pubs[j] = pubs[j], pubs[i]
			}
		}
	}
	return pubs, nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// listAgencyEntities returns all Agency-typed entities for this database.
func (m *agencyManager) listAgencyEntities(ctx context.Context) ([]entitygraph.Entity, error) {
	return m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: m.agencyID,
		TypeID:   "Agency",
	})
}

// checkActivationGuards enforces the draft → active preconditions:
// ≥1 Goal entity and ≥1 Workflow with ≥1 WorkItem.
func (m *agencyManager) checkActivationGuards(ctx context.Context) error {
	goals, err := m.GetGoals(ctx)
	if err != nil {
		return fmt.Errorf("activation guard: goals: %w", err)
	}
	if len(goals) == 0 {
		return fmt.Errorf("%w: agency must have at least one Goal before activating", ErrInvalidAgency)
	}

	workflows, err := m.GetWorkflows(ctx)
	if err != nil {
		return fmt.Errorf("activation guard: workflows: %w", err)
	}
	hasWorkItem := false
	for _, wf := range workflows {
		if len(wf.WorkItems) > 0 {
			hasWorkItem = true
			break
		}
	}
	if !hasWorkItem {
		return fmt.Errorf("%w: agency must have at least one Workflow with a WorkItem before activating", ErrInvalidAgency)
	}
	return nil
}

// writeSnapshot creates an immutable AgencySnapshot entity.
func (m *agencyManager) writeSnapshot(ctx context.Context, agencyID string) error {
	_, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: m.agencyID,
		TypeID:   "AgencySnapshot",
		Properties: map[string]any{
			"snapshot_at": time.Now().UTC().Format(time.RFC3339),
			"agency_id":   agencyID,
		},
	})
	return err
}

// nextPublicationVersion returns MAX(version)+1 across existing publications,
// or 1 if none exist.
func (m *agencyManager) nextPublicationVersion(ctx context.Context) (int, error) {
	entities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: m.agencyID,
		TypeID:   "AgencyPublication",
	})
	if err != nil {
		return 0, err
	}
	max := 0
	for _, e := range entities {
		if v, ok := e.Properties["version"]; ok {
			switch vv := v.(type) {
			case int:
				if vv > max {
					max = vv
				}
			case float64:
				if int(vv) > max {
					max = int(vv)
				}
			}
		}
	}
	return max + 1, nil
}

// ── Entity → Domain converters ────────────────────────────────────────────────

func entityToAgency(e entitygraph.Entity) Agency {
	p := e.Properties
	return Agency{
		ID:        e.ID,
		Name:      strProp(p, "name"),
		Mission:   strProp(p, "mission"),
		Vision:    strProp(p, "vision"),
		Status:    AgencyLifecycle(strProp(p, "status")),
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}

func entityToGoal(e entitygraph.Entity) Goal {
	p := e.Properties
	ord := 0
	if v, ok := p["ordinality"]; ok {
		switch vv := v.(type) {
		case int:
			ord = vv
		case float64:
			ord = int(vv)
		}
	}
	return Goal{
		ID:          e.ID,
		Title:       strProp(p, "title"),
		Description: strProp(p, "description"),
		Ordinality:  ord,
	}
}

func entityToWorkflow(e entitygraph.Entity) Workflow {
	return Workflow{
		ID:   e.ID,
		Name: strProp(e.Properties, "name"),
	}
}

func entityToWorkItem(e entitygraph.Entity) WorkItem {
	p := e.Properties
	order := 0
	if v, ok := p["order"]; ok {
		switch vv := v.(type) {
		case int:
			order = vv
		case float64:
			order = int(vv)
		}
	}
	parallel := false
	if v, ok := p["parallel"]; ok {
		if b, ok := v.(bool); ok {
			parallel = b
		}
	}
	return WorkItem{
		ID:          e.ID,
		Title:       strProp(p, "title"),
		Description: strProp(p, "description"),
		Order:       order,
		Parallel:    parallel,
	}
}

func entityToConfiguredRole(e entitygraph.Entity) ConfiguredRole {
	p := e.Properties
	return ConfiguredRole{
		Role:      AgencyRole(strProp(p, "name")),
		ActorType: ActorType(strProp(p, "actor_type")),
	}
}

func entityToPublication(e entitygraph.Entity, agency Agency) AgencyPublication {
	p := e.Properties
	version := 0
	if v, ok := p["version"]; ok {
		switch vv := v.(type) {
		case int:
			version = vv
		case float64:
			version = int(vv)
		}
	}
	var publishedAt time.Time
	if v, ok := p["published_at"]; ok {
		if s, ok := v.(string); ok {
			publishedAt, _ = time.Parse(time.RFC3339, s)
		}
	}
	return AgencyPublication{
		ID:          e.ID,
		Agency:      agency,
		Version:     version,
		Tag:         strProp(p, "tag"),
		PublishedAt: publishedAt,
	}
}

func strProp(props map[string]any, key string) string {
	if v, ok := props[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// newID returns a random UUID v4 string using crypto/rand.
func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
