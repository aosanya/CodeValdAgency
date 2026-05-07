// Package codevaldagency provides agency lifecycle management for the CodeVald
// platform. It exposes [AgencyManager] — the single interface for writing,
// reading, and managing the one agency that lives in this database, including
// draft-based version management via [AgencyManager.CreateDraft] and
// [AgencyManager.PromoteDraft].
//
// Usage:
//
//	dm, sm, err := arangodb.New(db)
//	mgr := codevaldagency.NewAgencyManager(dm, sm, publisher, agencyID)
//	agency, err := mgr.SetAgencyDetails(ctx, `{"id":"agency-001","name":"Alpha"}`)
package codevaldagency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	"github.com/aosanya/CodeValdSharedLib/eventbus"
)

// AgencyManager is the primary interface for agency lifecycle management.
// gRPC handlers hold this interface — never the concrete type.
//
// Implementations must be safe for concurrent use.
type AgencyManager interface {
	// SetAgencyDetails replaces the full agency document from a raw JSON string.
	// The JSON must include a non-empty "id" field; all other fields are optional.
	// Returns [ErrInvalidJSON] if the payload cannot be parsed or id is missing.
	// Returns [ErrAgencyReadOnly] if the live agency has already been published
	// (i.e. at least one draft has been promoted). Once published, only
	// [CreateDraft] + [PromoteDraft] may mutate the agency state.
	SetAgencyDetails(ctx context.Context, jsonStr string) (Agency, error)

	// GetAgency retrieves the single agency for this database.
	// Returns [ErrAgencyNotFound] if no agency entity exists yet.
	GetAgency(ctx context.Context) (Agency, error)

	// GetGoals returns all live Goal entities linked to this Agency.
	GetGoals(ctx context.Context) ([]Goal, error)

	// GetWorkflows returns all live Workflow entities linked to this Agency.
	GetWorkflows(ctx context.Context) ([]Workflow, error)

	// GetConfiguredRoles returns all live ConfiguredRole entities linked to this Agency.
	GetConfiguredRoles(ctx context.Context) ([]ConfiguredRole, error)

	// PublishAgency creates an immutable versioned publication of the current
	// agency state, linked to the given draftID. Version is auto-incremented
	// from the last publication (starts at 1).
	// Returns [ErrAgencyNotFound] if no agency entity exists.
	// Returns [ErrDraftNotFound] if draftID is provided but not found.
	// Returns [ErrNoChangesDetected] if draftID is non-empty and the draft's
	// content hash matches an existing publication.
	// Publishes "cross.agency.published" after every successful write.
	PublishAgency(ctx context.Context, draftID string) (AgencyPublication, error)

	// GetPublication retrieves a single publication by its version number.
	// Returns [ErrPublicationNotFound] if no publication with that version exists.
	GetPublication(ctx context.Context, version int) (AgencyPublication, error)

	// ListPublications returns all publications for this agency in ascending
	// version order.
	ListPublications(ctx context.Context) ([]AgencyPublication, error)

	// UpdatePublicationStatus transitions a publication to a new status.
	// Allowed transitions: draft → active, active → archived.
	// Returns [ErrPublicationNotFound] if no publication with that version exists.
	// Returns [ErrInvalidPublicationStatus] if the transition is not permitted.
	UpdatePublicationStatus(ctx context.Context, version int, status string) (AgencyPublication, error)

	// CreateDraft creates a new [AgencyDraft] by deep-copying the sub-entity
	// graph from the source identified by forkedFromID and forkedFromType.
	// forkedFromType must be "live" (fork from current live agency) or "draft"
	// (fork from an existing open draft identified by forkedFromID).
	// Returns [ErrAgencyNotFound] if no agency entity exists yet.
	// Returns [ErrDraftNotFound] if forkedFromType=="draft" and the source draft
	// does not exist.
	// Returns [ErrDraftNotOpen] if the source draft is not in the "open" state.
	CreateDraft(ctx context.Context, description, forkedFromID, forkedFromType string) (AgencyDraft, error)

	// GetDraft retrieves a single [AgencyDraft] by its ID.
	// Returns [ErrDraftNotFound] if no draft with that ID exists.
	GetDraft(ctx context.Context, draftID string) (AgencyDraft, error)

	// ListDrafts returns all [AgencyDraft] entities for this agency in creation
	// order (oldest first).
	ListDrafts(ctx context.Context) ([]AgencyDraft, error)

	// UpdateDraftDescription replaces the human-readable description on an open
	// draft. Returns [ErrDraftNotFound] if the draft does not exist.
	// Returns [ErrDraftNotOpen] if the draft is already promoted or archived.
	UpdateDraftDescription(ctx context.Context, draftID, description string) (AgencyDraft, error)

	// PromoteDraft copies all draft sub-entities into the live agency graph,
	// replaces the previous live sub-entities, sets Agency.Enabled = true, marks
	// the draft as "promoted", and writes an [AgencySnapshot].
	// Returns [ErrDraftNotFound] if the draft does not exist.
	// Returns [ErrDraftNotOpen] if the draft is already promoted or archived.
	// Publishes "cross.agency.promoted" on success.
	PromoteDraft(ctx context.Context, draftID string) (Agency, error)

	// ArchiveDraft marks an open draft as "archived" without promoting it.
	// Returns [ErrDraftNotFound] if the draft does not exist.
	// Returns [ErrDraftNotOpen] if the draft is already promoted or archived.
	ArchiveDraft(ctx context.Context, draftID string) (AgencyDraft, error)

	// ── WorkPlan dispatch methods ─────────────────────────────────────────────

	// CreateWorkPlan stores a new [WorkPlan] entity and links it to the Agency via
	// has_work_plan / belongs_to_agency edges.
	// Returns [ErrInvalidRegex] if TriggerTopic or PayloadCondition is not a valid
	// Go regular expression.
	CreateWorkPlan(ctx context.Context, req CreateWorkPlanRequest) (WorkPlan, error)

	// GetWorkPlan retrieves a single [WorkPlan] by its entity ID.
	// Returns [ErrWorkPlanNotFound] if no WorkPlan entity with that ID exists.
	GetWorkPlan(ctx context.Context, workPlanID string) (WorkPlan, error)

	// ListWorkPlans returns all [WorkPlan] entities linked to this Agency.
	ListWorkPlans(ctx context.Context) ([]WorkPlan, error)

	// UpdateWorkPlan applies req to the [WorkPlan] identified by workPlanID.
	// Returns [ErrWorkPlanNotFound] if no WorkPlan entity with that ID exists.
	// Returns [ErrInvalidRegex] if TriggerTopic or PayloadCondition is not a valid
	// Go regular expression.
	UpdateWorkPlan(ctx context.Context, workPlanID string, req UpdateWorkPlanRequest) (WorkPlan, error)

	// DeleteWorkPlan removes the [WorkPlan] entity identified by workPlanID.
	// Returns [ErrWorkPlanNotFound] if no WorkPlan entity with that ID exists.
	DeleteWorkPlan(ctx context.Context, workPlanID string) error

	// AddContextSource creates a typed [ContextSource] entity and links it to
	// the [WorkPlan] identified by workPlanID via has_context_source / belongs_to_work_plan edges.
	// Returns [ErrWorkPlanNotFound] if no WorkPlan entity with that ID exists.
	AddContextSource(ctx context.Context, workPlanID string, req AddContextSourceRequest) (ContextSource, error)

	// ListContextSources returns all [ContextSource] entities linked to the
	// [WorkPlan] identified by workPlanID.
	// Returns [ErrWorkPlanNotFound] if no WorkPlan entity with that ID exists.
	ListContextSources(ctx context.Context, workPlanID string) ([]ContextSource, error)

	// RemoveContextSource deletes the [ContextSource] entity identified by
	// sourceID and removes its has_context_source edge from the owning WorkPlan.
	// Returns [ErrContextSourceNotFound] if no such entity exists.
	RemoveContextSource(ctx context.Context, workPlanID, sourceID string) error

	// MatchWorkPlans evaluates topic and payload against all enabled [WorkPlan] entities.
	// TriggerTopic is matched as a Go regex against topic; if PayloadCondition is
	// non-empty it is also matched against the raw payload string. Returns all
	// matching work plans with their linked [ContextSource] entities, ordered by
	// WorkPlan.Ordinality ascending.
	MatchWorkPlans(ctx context.Context, topic, payload string) ([]WorkPlanMatch, error)
}

// AgencySchemaManager manages schema versions for the Agency entity graph.
// It is a type alias for [entitygraph.SchemaManager] from CodeValdSharedLib.
type AgencySchemaManager = entitygraph.SchemaManager

// CrossPublisher is the historical name for the event-publishing contract
// CodeValdAgency callers inject. As of MVP-WORK-014 it is a type alias for
// [eventbus.Publisher] — the SharedLib package that unifies the publish
// contract across CodeValdAgency, CodeValdComm, CodeValdDT, and CodeValdWork.
//
// New callers should refer to [eventbus.Publisher] directly; this alias
// remains for source compatibility.
type CrossPublisher = eventbus.Publisher

// agencyManager is the concrete implementation of [AgencyManager].
// It wraps [entitygraph.DataManager] to expose agency-specific convenience
// methods. All storage operations go through dm; no bespoke Backend interface
// is used.
type agencyManager struct {
	dm        entitygraph.DataManager // graph CRUD — injected by cmd/main.go
	sm        AgencySchemaManager     // schema versioning — injected by cmd/main.go
	publisher eventbus.Publisher      // optional; nil = skip event publishing
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
	pub eventbus.Publisher,
	agencyID string,
) AgencyManager {
	return &agencyManager{dm: dm, sm: sm, publisher: pub, agencyID: agencyID}
}

// ── SetAgencyDetails ──────────────────────────────────────────────────────────

// SetAgencyDetails parses the JSON payload and upserts the root Agency entity.
// If no Agency entity exists yet it calls CreateEntity; otherwise UpdateEntity
// (only when the agency has not yet been published — Agency.Enabled == false).
// Returns [ErrAgencyReadOnly] when the live agency is already published.
func (m *agencyManager) SetAgencyDetails(ctx context.Context, jsonStr string) (Agency, error) {
	var raw struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Mission string `json:"mission"`
		Vision  string `json:"vision"`
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
		// Reject edits once the agency has been published.
		if boolProp(existing[0].Properties, "enabled") {
			return Agency{}, ErrAgencyReadOnly
		}
		entity, err = m.dm.UpdateEntity(ctx, m.agencyID, existing[0].ID, entitygraph.UpdateEntityRequest{
			Properties: props,
		})
	}
	if err != nil {
		return Agency{}, fmt.Errorf("SetAgencyDetails: %w", err)
	}

	return entityToAgency(entity), nil
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

// GetWorkflows returns all Workflow entities linked to this Agency.
func (m *agencyManager) GetWorkflows(ctx context.Context) ([]Workflow, error) {
	wfEntities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: m.agencyID,
		TypeID:   "Workflow",
	})
	if err != nil {
		return nil, fmt.Errorf("GetWorkflows: %w", err)
	}

	workflows := make([]Workflow, len(wfEntities))
	for i, e := range wfEntities {
		workflows[i] = entityToWorkflow(e)
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
// auto-incremented version linked to the given draftID, then creates a linked
// mutable [AgencyPublicationStatus] entity seeded at status "draft".
// When draftID is non-empty, the draft's content is hashed and compared
// against all existing publications — [ErrNoChangesDetected] is returned if
// the hash matches, preventing a re-publish with no new content.
// Publishes "cross.agency.published" on success.
// Returns [ErrAgencyNotFound] if no agency entity exists yet.
func (m *agencyManager) PublishAgency(ctx context.Context, draftID string) (AgencyPublication, error) {
	agency, err := m.GetAgency(ctx)
	if err != nil {
		return AgencyPublication{}, err
	}

	version, err := m.nextPublicationVersion(ctx)
	if err != nil {
		return AgencyPublication{}, fmt.Errorf("PublishAgency: next version: %w", err)
	}

	// Guard: when a draftID is supplied, compute the content hash of the draft
	// and reject if any existing publication already has the same hash.
	var contentHash string
	if draftID != "" {
		h, err := m.draftContentHash(ctx, draftID)
		if err != nil {
			return AgencyPublication{}, fmt.Errorf("PublishAgency: %w", err)
		}
		contentHash = h
		existing, err := m.ListPublications(ctx)
		if err != nil {
			return AgencyPublication{}, fmt.Errorf("PublishAgency: %w", err)
		}
		for _, p := range existing {
			if p.ContentHash != "" && p.ContentHash == contentHash {
				return AgencyPublication{}, fmt.Errorf("PublishAgency: %w", ErrNoChangesDetected)
			}
		}
	}

	now := time.Now().UTC()
	entity, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: m.agencyID,
		TypeID:   "AgencyPublication",
		Properties: map[string]any{
			"version":      version,
			"tag":          fmt.Sprintf("v%d", version),
			"published_at": now.Format(time.RFC3339),
			"draft_id":     draftID,
			"content_hash": contentHash,
		},
	})
	if err != nil {
		return AgencyPublication{}, fmt.Errorf("PublishAgency: create entity: %w", err)
	}

	// Create the mutable status node for this publication.
	statusEntity, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID: m.agencyID,
		TypeID:   "AgencyPublicationStatus",
		Properties: map[string]any{
			"status": "draft",
		},
	})
	if err != nil {
		return AgencyPublication{}, fmt.Errorf("PublishAgency: create status entity: %w", err)
	}

	// Link publication → status via has_status edge.
	_, err = m.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID: m.agencyID,
		Name:     "has_status",
		FromID:   entity.ID,
		ToID:     statusEntity.ID,
	})
	if err != nil {
		return AgencyPublication{}, fmt.Errorf("PublishAgency: link status entity: %w", err)
	}

	pub := AgencyPublication{
		ID:          entity.ID,
		AgencyID:    m.agencyID,
		DraftID:     draftID,
		ContentHash: contentHash,
		Version:     version,
		Tag:         fmt.Sprintf("v%d", version),
		PublishedAt: now,
		Status:      "draft",
	}

	eventbus.SafePublish(ctx, m.publisher, eventbus.Event{
		Topic:    "cross.agency.published",
		AgencyID: agency.ID,
	})
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
// Each publication's Status field is populated from its linked
// [AgencyPublicationStatus] entity.
func (m *agencyManager) ListPublications(ctx context.Context) ([]AgencyPublication, error) {
	entities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: m.agencyID,
		TypeID:   "AgencyPublication",
	})
	if err != nil {
		return nil, fmt.Errorf("ListPublications: %w", err)
	}

	// Build pubID → status map from the mutable status entities.
	statusMap, err := m.pubStatusMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("ListPublications: %w", err)
	}

	pubs := make([]AgencyPublication, 0, len(entities))
	for _, e := range entities {
		pub := entityToPublication(e, statusMap[e.ID])
		pub.AgencyID = m.agencyID
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

// pubStatusMap fetches all has_status relationships for this agency and
// returns a map from AgencyPublication entity ID to its current status string.
func (m *agencyManager) pubStatusMap(ctx context.Context) (map[string]string, error) {
	rels, err := m.dm.ListRelationships(ctx, entitygraph.RelationshipFilter{
		AgencyID: m.agencyID,
		Name:     "has_status",
	})
	if err != nil {
		return nil, fmt.Errorf("pubStatusMap: list rels: %w", err)
	}

	statusEntities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: m.agencyID,
		TypeID:   "AgencyPublicationStatus",
	})
	if err != nil {
		return nil, fmt.Errorf("pubStatusMap: list status entities: %w", err)
	}

	// Index status entities by their ID.
	statusByID := make(map[string]string, len(statusEntities))
	for _, se := range statusEntities {
		statusByID[se.ID] = strProp(se.Properties, "status")
	}

	// Map publicationID → status using the has_status edges.
	m2 := make(map[string]string, len(rels))
	for _, r := range rels {
		if s, ok := statusByID[r.ToID]; ok {
			m2[r.FromID] = s
		}
	}
	return m2, nil
}

// ── UpdatePublicationStatus ───────────────────────────────────────────────────

// canTransitionPublicationStatus reports whether transitioning from current
// to next is a valid publication lifecycle move.
//
// Allowed transitions:
//
//	draft  → active
//	active → archived
func canTransitionPublicationStatus(current, next string) bool {
	switch current {
	case "draft":
		return next == "active"
	case "active":
		return next == "archived"
	default:
		return false // archived is terminal
	}
}

// UpdatePublicationStatus transitions the publication at the given version to
// a new status. Allowed: draft → active, active → archived.
// Returns [ErrPublicationNotFound] if no matching version exists.
// Returns [ErrInvalidPublicationStatus] if the transition is not permitted.
//
// Status is stored in the mutable [AgencyPublicationStatus] entity linked via
// the has_status edge; the immutable [AgencyPublication] entity is never
// updated.
func (m *agencyManager) UpdatePublicationStatus(ctx context.Context, version int, status string) (AgencyPublication, error) {
	pub, err := m.GetPublication(ctx, version)
	if err != nil {
		return AgencyPublication{}, err
	}

	if !canTransitionPublicationStatus(pub.Status, status) {
		return AgencyPublication{}, fmt.Errorf(
			"UpdatePublicationStatus v%d: %w (%s → %s)",
			version, ErrInvalidPublicationStatus, pub.Status, status,
		)
	}

	// Locate the mutable AgencyPublicationStatus entity via has_status edge.
	statusEntityID, err := m.findStatusEntityID(ctx, pub.ID)
	if err != nil {
		return AgencyPublication{}, fmt.Errorf("UpdatePublicationStatus v%d: %w", version, err)
	}

	_, err = m.dm.UpdateEntity(ctx, m.agencyID, statusEntityID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{"status": status},
	})
	if err != nil {
		return AgencyPublication{}, fmt.Errorf("UpdatePublicationStatus v%d: update status entity: %w", version, err)
	}

	pub.Status = status
	return pub, nil
}

// findStatusEntityID returns the ID of the [AgencyPublicationStatus] entity
// linked to the given publication entity via the has_status edge.
// Returns [ErrPublicationNotFound] if the edge or entity is missing.
func (m *agencyManager) findStatusEntityID(ctx context.Context, pubEntityID string) (string, error) {
	rels, err := m.dm.ListRelationships(ctx, entitygraph.RelationshipFilter{
		AgencyID: m.agencyID,
		Name:     "has_status",
		FromID:   pubEntityID,
	})
	if err != nil {
		return "", fmt.Errorf("findStatusEntityID: %w", err)
	}
	if len(rels) == 0 {
		return "", fmt.Errorf("findStatusEntityID: %w", ErrPublicationNotFound)
	}
	return rels[0].ToID, nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// listAgencyEntities returns all Agency-typed entities for this database.
func (m *agencyManager) listAgencyEntities(ctx context.Context) ([]entitygraph.Entity, error) {
	return m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID: m.agencyID,
		TypeID:   "Agency",
	})
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

// draftContentHash computes a deterministic SHA-256 fingerprint of the
// logical content of the given draft. It collects DraftGoal, DraftWorkflow,
// DraftWorkItem, and DraftConfiguredRole entities whose draft_ref_code property
// equals draftID, excludes infrastructure properties (draft_ref_code,
// draft_workflow_ref_code, created_at, updated_at), JSON-encodes each entity's
// remaining properties together with its type, sorts the resulting strings for
// determinism, and returns the hex-encoded SHA-256 of the joined output.
//
// Two drafts whose sub-entity logical content is identical produce the same
// hash regardless of entity creation order or internal IDs.
func (m *agencyManager) draftContentHash(ctx context.Context, draftID string) (string, error) {
	listByDraft := func(typeID string) ([]entitygraph.Entity, error) {
		all, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
			AgencyID: m.agencyID,
			TypeID:   typeID,
		})
		if err != nil {
			return nil, err
		}
		var out []entitygraph.Entity
		for _, e := range all {
			if strProp(e.Properties, "draft_ref_code") == draftID {
				out = append(out, e)
			}
		}
		return out, nil
	}

	var records []string
	for _, typeID := range []string{"DraftGoal", "DraftWorkflow", "DraftWorkItem", "DraftConfiguredRole"} {
		entities, err := listByDraft(typeID)
		if err != nil {
			return "", fmt.Errorf("draftContentHash %s: %w", typeID, err)
		}
		for _, e := range entities {
			props := copyPropsExcluding(e.Properties,
				"draft_ref_code", "draft_workflow_ref_code", "created_at", "updated_at")
			b, err := json.Marshal(map[string]any{"type": typeID, "props": props})
			if err != nil {
				return "", fmt.Errorf("draftContentHash marshal: %w", err)
			}
			records = append(records, string(b))
		}
	}
	sort.Strings(records)
	sum := sha256.Sum256([]byte(strings.Join(records, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

// ── Entity → Domain converters ────────────────────────────────────────────────

func entityToAgency(e entitygraph.Entity) Agency {
	p := e.Properties
	return Agency{
		ID:        e.ID,
		Name:      strProp(p, "name"),
		Mission:   strProp(p, "mission"),
		Vision:    strProp(p, "vision"),
		Enabled:   boolProp(p, "enabled"),
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
	return Workflow{
		ID:          e.ID,
		Name:        strProp(p, "name"),
		Description: strProp(p, "description"),
		Ordinality:  ord,
	}
}

func entityToConfiguredRole(e entitygraph.Entity) ConfiguredRole {
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
	return ConfiguredRole{
		ID:          e.ID,
		Name:        strProp(p, "name"),
		Description: strProp(p, "description"),
		ActorType:   ActorType(strProp(p, "actor_type")),
		Ordinality:  ord,
	}
}

// entityToPublication converts a raw [entitygraph.Entity] of type
// "AgencyPublication" to a domain [AgencyPublication]. The status is sourced
// from the linked [AgencyPublicationStatus] entity and must be supplied by the
// caller — it is not stored on the immutable publication entity itself.
func entityToPublication(e entitygraph.Entity, status string) AgencyPublication {
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
		Version:     version,
		Tag:         strProp(p, "tag"),
		DraftID:     strProp(p, "draft_id"),
		ContentHash: strProp(p, "content_hash"),
		PublishedAt: publishedAt,
		Status:      status,
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

// boolProp returns the bool value of key in props, defaulting to false.
func boolProp(props map[string]any, key string) bool {
	if v, ok := props[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// copyProps returns a shallow copy of src.
func copyProps(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// copyPropsExcluding returns a shallow copy of src with the given keys omitted.
func copyPropsExcluding(src map[string]any, excludeKeys ...string) map[string]any {
	exclude := make(map[string]struct{}, len(excludeKeys))
	for _, k := range excludeKeys {
		exclude[k] = struct{}{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		if _, skip := exclude[k]; !skip {
			dst[k] = v
		}
	}
	return dst
}

// entityToDraft converts a raw [entitygraph.Entity] of type "AgencyDraft" to a
// domain [AgencyDraft].
func entityToDraft(e entitygraph.Entity) AgencyDraft {
	p := e.Properties
	return AgencyDraft{
		ID:             e.ID,
		AgencyID:       e.AgencyID,
		Description:    strProp(p, "description"),
		ForkedFromID:   strProp(p, "forked_from_ref_code"),
		ForkedFromType: strProp(p, "forked_from_type"),
		Status:         AgencyDraftStatus(strProp(p, "status")),
		CreatedAt:      e.CreatedAt,
		UpdatedAt:      e.UpdatedAt,
	}
}
