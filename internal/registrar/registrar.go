// Package registrar provides the CodeValdAgency service registrar.
// It wraps the shared-library heartbeat registrar and additionally implements
// [codevaldagency.CrossPublisher] so the [AgencyManager] can notify
// CodeValdCross whenever an agency is successfully created.
package registrar

import (
	"context"
	"encoding/json"
	"log"
	"time"

	codevaldagency "github.com/aosanya/CodeValdAgency"
	egserver "github.com/aosanya/CodeValdSharedLib/entitygraph/server"
	"github.com/aosanya/CodeValdSharedLib/eventbus"
	sharedregistrar "github.com/aosanya/CodeValdSharedLib/registrar"
	"github.com/aosanya/CodeValdSharedLib/schemaroutes"
	"github.com/aosanya/CodeValdSharedLib/types"
)

// Registrar handles two responsibilities:
//  1. Sending periodic heartbeat registrations to CodeValdCross via the
//     shared-library registrar (Run / Close).
//  2. Implementing [codevaldagency.CrossPublisher] so that AgencyManager can
//     publish agency lifecycle events (e.g. cross.agency.published).
//
// Construct via [New]; start heartbeats by calling Run in a goroutine; stop
// by cancelling the context then calling Close.
type Registrar struct {
	heartbeat sharedregistrar.Registrar
}

// Compile-time assertion that *Registrar implements CrossPublisher.
var _ codevaldagency.CrossPublisher = (*Registrar)(nil)

// New constructs a Registrar that heartbeats to the CodeValdCross gRPC server
// at crossAddr and can publish agency lifecycle events.
//
//   - crossAddr    — host:port of the CodeValdCross gRPC server
//   - advertiseAddr — host:port that Cross dials back on
//   - pingInterval — heartbeat cadence; ≤ 0 means only the initial ping
//   - pingTimeout  — per-RPC timeout for each Register call
func New(
	crossAddr, advertiseAddr, agencyID string,
	pingInterval, pingTimeout time.Duration,
) (*Registrar, error) {
	routes := agencyRoutes()
	hb, err := sharedregistrar.New(
		crossAddr,
		advertiseAddr,
		agencyID,
		"codevaldagency",
		[]string{"cross.agency.published"},
		[]string{},
		routes,
		pingInterval,
		pingTimeout,
	)
	if err != nil {
		return nil, err
	}
	return &Registrar{heartbeat: hb}, nil
}

// Run starts the heartbeat loop, sending an immediate Register ping to
// CodeValdCross then repeating at the configured interval until ctx is
// cancelled. Must be called inside a goroutine.
func (r *Registrar) Run(ctx context.Context) {
	r.heartbeat.Run(ctx)
}

// Close releases the underlying gRPC connection used for heartbeats.
// Call after the context passed to Run has been cancelled.
func (r *Registrar) Close() {
	r.heartbeat.Close()
}

// SyncSubscriptions registers every enabled work plan's handler_service as a
// subscriber to its trigger_topic via Cross → PubSub. Duplicate
// (subscriber_service, topic_pattern) pairs are deduplicated before calling.
// Called on Agency startup and after every import or publish so subscriptions
// are active regardless of whether the handler services are running.
func (r *Registrar) SyncSubscriptions(ctx context.Context, agencyID string, plans []codevaldagency.WorkPlan) {
	seen := map[string]bool{}
	for _, wp := range plans {
		if !wp.Enabled || wp.HandlerService == "" || wp.TriggerTopic == "" {
			continue
		}
		key := wp.HandlerService + "\x00" + wp.TriggerTopic
		if seen[key] {
			continue
		}
		seen[key] = true
		if err := r.heartbeat.SubscribeTopic(ctx, agencyID, wp.HandlerService, wp.TriggerTopic); err != nil {
			log.Printf("registrar[codevaldagency]: SyncSubscriptions: subscribe %q → %q: %v", wp.HandlerService, wp.TriggerTopic, err)
		}
	}
}

// Publish implements [eventbus.Publisher].
// Marshals the event payload to JSON and forwards it to CodeValdCross via the
// OrchestratorService.Publish RPC, which routes it on to CodeValdPubSub.
// Errors are logged but not returned — the operation is already persisted.
func (r *Registrar) Publish(ctx context.Context, e eventbus.Event) error {
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		log.Printf("registrar[codevaldagency]: marshal payload for topic=%q: %v", e.Topic, err)
		payload = []byte("{}")
	}
	if err := r.heartbeat.Publish(ctx, e.AgencyID, e.Topic, "codevaldagency", string(payload)); err != nil {
		log.Printf("registrar[codevaldagency]: Publish topic=%q: %v", e.Topic, err)
	}
	return nil
}

// agencyRoutes returns all HTTP routes CodeValdAgency exposes via Cross.
//
// It combines:
//   - Static routes for the agency-level gRPC methods (SetAgencyDetails,
//     GetAgency, UpdateAgency, PublishAgency, GetPublication, ListPublications,
//     UpdatePublicationStatus).
//   - Dynamic entity CRUD routes generated from [codevaldagency.DefaultAgencySchema]
//     via a single [schemaroutes.RoutesFromSchema] call. DraftXxx types embed
//     their full sub-path in PathSegment (e.g. "drafts/{draftId}/goals"), so
//     they are naturally served at the correct nested URL with no extra logic.
func agencyRoutes() []types.RouteInfo {
	static := []types.RouteInfo{
		// POST /agency/{agencyId} — replace (or create) the full agency document.
		{
			Method:     "POST",
			Pattern:    "/agency/{agencyId}",
			Capability: "set_agency_details",
			GrpcMethod: "/codevaldagency.v1.AgencyService/SetAgencyDetails",
		},
		// GET /agency/{agencyId} — retrieve the agency.
		{
			Method:     "GET",
			Pattern:    "/agency/{agencyId}",
			Capability: "get_agency",
			GrpcMethod: "/codevaldagency.v1.AgencyService/GetAgency",
		},
		// PUT /agency/{agencyId} — apply incremental field edits.
		{
			Method:     "PUT",
			Pattern:    "/agency/{agencyId}",
			Capability: "update_agency",
			GrpcMethod: "/codevaldagency.v1.AgencyService/UpdateAgency",
		},
		// POST /agency/{agencyId}/publish — create an immutable versioned publication.
		{
			Method:     "POST",
			Pattern:    "/agency/{agencyId}/publish",
			Capability: "publish_agency",
			GrpcMethod: "/codevaldagency.v1.AgencyService/PublishAgency",
		},
		// POST /agency/{agencyId}/import — parse a raw agency.yaml body and
		// idempotently populate a draft in one shot.
		{
			Method:     "POST",
			Pattern:    "/agency/{agencyId}/import",
			Capability: "import_draft",
			GrpcMethod: "/codevaldagency.v1.AgencyService/ImportDraft",
		},
		// GET /agency/{agencyId}/publications — list all publications.
		{
			Method:     "GET",
			Pattern:    "/agency/{agencyId}/publications",
			Capability: "list_publications",
			GrpcMethod: "/codevaldagency.v1.AgencyService/ListPublications",
		},
		// GET /agency/{agencyId}/publications/{version} — get a specific publication.
		{
			Method:     "GET",
			Pattern:    "/agency/{agencyId}/publications/{version}",
			Capability: "get_publication",
			GrpcMethod: "/codevaldagency.v1.AgencyService/GetPublication",
			PathBindings: []types.PathBinding{
				{URLParam: "version", Field: "version"},
			},
		},
		// PUT /agency/{agencyId}/publications/{version}/status — update publication lifecycle status.
		{
			Method:     "PUT",
			Pattern:    "/agency/{agencyId}/publications/{version}/status",
			Capability: "update_publication_status",
			GrpcMethod: "/codevaldagency.v1.AgencyService/UpdatePublicationStatus",
			PathBindings: []types.PathBinding{
				{URLParam: "version", Field: "version"},
			},
		},

		// ── Live blueprint entity routes (read-only; written by PublishAgency) ─────
		// GET /agency/{agencyId}/goals — list promoted live goals.
		{
			Method:     "GET",
			Pattern:    "/agency/{agencyId}/goals",
			Capability: "get_goals",
			GrpcMethod: "/codevaldagency.v1.AgencyService/GetGoals",
		},
		// GET /agency/{agencyId}/workflows — list promoted live workflows.
		{
			Method:     "GET",
			Pattern:    "/agency/{agencyId}/workflows",
			Capability: "get_workflows",
			GrpcMethod: "/codevaldagency.v1.AgencyService/GetWorkflows",
		},
		// GET /agency/{agencyId}/configured-roles — list promoted live configured roles.
		{
			Method:     "GET",
			Pattern:    "/agency/{agencyId}/configured-roles",
			Capability: "get_configured_roles",
			GrpcMethod: "/codevaldagency.v1.AgencyService/GetConfiguredRoles",
		},
		// GET /agency/{agencyId}/work-items — list promoted live work items.
		{
			Method:     "GET",
			Pattern:    "/agency/{agencyId}/work-items",
			Capability: "get_work_items",
			GrpcMethod: "/codevaldagency.v1.AgencyService/GetWorkItems",
		},

		// ── RACI role routes ──────────────────────────────────────────────────────
		// POST /agency/{agencyId}/roles — create a new dispatch role.
		{
			Method:     "POST",
			Pattern:    "/agency/{agencyId}/roles",
			Capability: "create_role",
			GrpcMethod: "/codevaldagency.v1.AgencyService/CreateRole",
		},
		// GET /agency/{agencyId}/roles — list all dispatch roles.
		{
			Method:     "GET",
			Pattern:    "/agency/{agencyId}/roles",
			Capability: "list_roles",
			GrpcMethod: "/codevaldagency.v1.AgencyService/ListRoles",
		},
		// GET /agency/{agencyId}/roles/{roleId} — retrieve a single role.
		{
			Method:     "GET",
			Pattern:    "/agency/{agencyId}/roles/{roleId}",
			Capability: "get_role",
			GrpcMethod: "/codevaldagency.v1.AgencyService/GetRole",
			PathBindings: []types.PathBinding{
				{URLParam: "roleId", Field: "role_id"},
			},
		},
		// PUT /agency/{agencyId}/roles/{roleId} — update a role's fields.
		{
			Method:     "PUT",
			Pattern:    "/agency/{agencyId}/roles/{roleId}",
			Capability: "update_role",
			GrpcMethod: "/codevaldagency.v1.AgencyService/UpdateRole",
			PathBindings: []types.PathBinding{
				{URLParam: "roleId", Field: "role_id"},
			},
		},
		// DELETE /agency/{agencyId}/roles/{roleId} — delete a role.
		{
			Method:     "DELETE",
			Pattern:    "/agency/{agencyId}/roles/{roleId}",
			Capability: "delete_role",
			GrpcMethod: "/codevaldagency.v1.AgencyService/DeleteRole",
			PathBindings: []types.PathBinding{
				{URLParam: "roleId", Field: "role_id"},
			},
		},
		// POST /agency/{agencyId}/roles/{roleId}/context-sources — add a context source.
		{
			Method:     "POST",
			Pattern:    "/agency/{agencyId}/roles/{roleId}/context-sources",
			Capability: "add_context_source",
			GrpcMethod: "/codevaldagency.v1.AgencyService/AddContextSource",
			PathBindings: []types.PathBinding{
				{URLParam: "roleId", Field: "role_id"},
			},
		},
		// GET /agency/{agencyId}/roles/{roleId}/context-sources — list context sources.
		{
			Method:     "GET",
			Pattern:    "/agency/{agencyId}/roles/{roleId}/context-sources",
			Capability: "list_context_sources",
			GrpcMethod: "/codevaldagency.v1.AgencyService/ListContextSources",
			PathBindings: []types.PathBinding{
				{URLParam: "roleId", Field: "role_id"},
			},
		},
		// DELETE /agency/{agencyId}/roles/{roleId}/context-sources/{sourceId} — remove a context source.
		{
			Method:     "DELETE",
			Pattern:    "/agency/{agencyId}/roles/{roleId}/context-sources/{sourceId}",
			Capability: "remove_context_source",
			GrpcMethod: "/codevaldagency.v1.AgencyService/RemoveContextSource",
			PathBindings: []types.PathBinding{
				{URLParam: "roleId", Field: "role_id"},
				{URLParam: "sourceId", Field: "source_id"},
			},
		},

		// ── Draft routes — override dynamic entity CRUD for write operations ─────
		// POST /agency/{agencyId}/drafts — fork a new editable draft from live or
		// draft, deep-copying the relevant sub-entity graph. GET and single-draft
		// GET remain on the dynamic entity CRUD routes so the frontend's
		// ListEntitiesResponse format is preserved.
		{
			Method:     "POST",
			Pattern:    "/agency/{agencyId}/drafts",
			Capability: "create_draft",
			GrpcMethod: "/codevaldagency.v1.AgencyService/CreateDraft",
		},
		// PATCH /agency/{agencyId}/drafts/{draftId} — update draft description.
		{
			Method:     "PATCH",
			Pattern:    "/agency/{agencyId}/drafts/{draftId}",
			Capability: "update_draft_description",
			GrpcMethod: "/codevaldagency.v1.AgencyService/UpdateDraftDescription",
			PathBindings: []types.PathBinding{
				{URLParam: "draftId", Field: "draft_id"},
			},
		},

		// ── WorkPlan routes — override dynamic entity CRUD for write operations ─────
		// GET list and GET single remain on dynamic entity CRUD so the frontend's
		// ListEntitiesResponse format ({entities:[]}) is preserved.
		//
		// PUT uses UpdateWorkPlan for partial-update semantics (only non-zero fields
		// are written); entity CRUD UpdateEntity would require the full document.
		{
			Method:     "PUT",
			Pattern:    "/agency/{agencyId}/work-plans/{workPlanId}",
			Capability: "update_work_plan",
			GrpcMethod: "/codevaldagency.v1.AgencyService/UpdateWorkPlan",
			PathBindings: []types.PathBinding{
				{URLParam: "workPlanId", Field: "work_plan_id"},
			},
		},
		// DELETE /agency/{agencyId}/work-plans/{workPlanId} — delete a work plan.
		{
			Method:     "DELETE",
			Pattern:    "/agency/{agencyId}/work-plans/{workPlanId}",
			Capability: "delete_work_plan",
			GrpcMethod: "/codevaldagency.v1.AgencyService/DeleteWorkPlan",
			PathBindings: []types.PathBinding{
				{URLParam: "workPlanId", Field: "work_plan_id"},
			},
		},
	}

	// Dynamic routes: one CRUD set per TypeDefinition with a non-empty PathSegment.
	// DraftXxx types embed their full sub-path (e.g. "drafts/{draftId}/goals"),
	// so no extra splitting or grouping is needed here.
	dynamic := schemaroutes.RoutesFromSchema(
		codevaldagency.DefaultAgencySchema(),
		"/agency/{agencyId}",
		"agencyId",
		egserver.GRPCServicePath,
	)

	return append(static, dynamic...)
}
