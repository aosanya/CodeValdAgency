// Package registrar provides the CodeValdAgency service registrar.
// It wraps the shared-library heartbeat registrar and additionally implements
// [codevaldagency.CrossPublisher] so the [AgencyManager] can notify
// CodeValdCross whenever an agency is successfully created.
package registrar

import (
	"context"
	"log"
	"time"

	codevaldagency "github.com/aosanya/CodeValdAgency"
	egserver "github.com/aosanya/CodeValdSharedLib/entitygraph/server"
	sharedregistrar "github.com/aosanya/CodeValdSharedLib/registrar"
	"github.com/aosanya/CodeValdSharedLib/schemaroutes"
	"github.com/aosanya/CodeValdSharedLib/types"
)

// Registrar handles two responsibilities:
//  1. Sending periodic heartbeat registrations to CodeValdCross via the
//     shared-library registrar (Run / Close).
//  2. Implementing [codevaldagency.CrossPublisher] so that AgencyManager can
//     fire "cross.agency.created" events on every successful CreateAgency call.
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
		[]string{"cross.agency.created", "cross.agency.published"},
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

// Publish implements [codevaldagency.CrossPublisher].
// It fires a best-effort notification for topic and agencyID.
// Currently logs the event; a future iteration will call a Cross Publish RPC
// once CodeValdCross exposes one. Errors are always nil — the agency has
// already been persisted and its creation must not be rolled back.
func (r *Registrar) Publish(ctx context.Context, topic string, agencyID string) error {
	log.Printf("registrar: publish topic=%q agencyID=%q", topic, agencyID)
	// TODO(CROSS-007): call OrchestratorService.Publish RPC when available.
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
