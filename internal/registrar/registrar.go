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
	crossv1 "github.com/aosanya/CodeValdSharedLib/gen/go/codevaldcross/v1"
	sharedregistrar "github.com/aosanya/CodeValdSharedLib/registrar"
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
	crossAddr, advertiseAddr string,
	pingInterval, pingTimeout time.Duration,
) (*Registrar, error) {
	routes := agencyRoutes()
	hb, err := sharedregistrar.New(
		crossAddr,
		advertiseAddr,
		"", // CodeValdAgency is not scoped to a single agency
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

// agencyRoutes returns the HTTP routes that CodeValdAgency exposes via Cross.
// All routes are prefixed with /agency/{agencyId} so that the service name
// disambiguates agency endpoints from other services, and the Cross dynamic
// proxy can extract the agency ID from the path to resolve the correct
// service instance via ConnForAgency.
func agencyRoutes() []*crossv1.RouteDeclaration {
	return []*crossv1.RouteDeclaration{
		// POST /agency/{agencyId} — replace (or create) the full agency document from a JSON body.
		// Body: {"json": "<agency-document-as-JSON-string>"}
		// agencyId is used by Cross for routing only — SetAgencyDetailsRequest has no agency_id field.
		{
			Method:     "POST",
			Pattern:    "/agency/{agencyId}",
			Capability: "set_agency_details",
			GrpcMethod: "/codevaldagency.v1.AgencyService/SetAgencyDetails",
		},
		// GET /agency/{agencyId} — retrieve the single agency for this database.
		{
			Method:     "GET",
			Pattern:    "/agency/{agencyId}",
			Capability: "get_agency",
			GrpcMethod: "/codevaldagency.v1.AgencyService/GetAgency",
		},
		// PUT /agency/{agencyId} — apply incremental field edits with lifecycle validation.
		{
			Method:     "PUT",
			Pattern:    "/agency/{agencyId}",
			Capability: "update_agency",
			GrpcMethod: "/codevaldagency.v1.AgencyService/UpdateAgency",
		},
		// POST /agency/{agencyId}/publish — create an immutable versioned publication of the current agency.
		{
			Method:     "POST",
			Pattern:    "/agency/{agencyId}/publish",
			Capability: "publish_agency",
			GrpcMethod: "/codevaldagency.v1.AgencyService/PublishAgency",
		},
		// GET /agency/{agencyId}/publications — list all publications in ascending version order.
		{
			Method:     "GET",
			Pattern:    "/agency/{agencyId}/publications",
			Capability: "list_publications",
			GrpcMethod: "/codevaldagency.v1.AgencyService/ListPublications",
		},
		// GET /agency/{agencyId}/publications/{version} — retrieve a specific publication by version.
		{
			Method:     "GET",
			Pattern:    "/agency/{agencyId}/publications/{version}",
			Capability: "get_publication",
			GrpcMethod: "/codevaldagency.v1.AgencyService/GetPublication",
			PathBindings: []*crossv1.PathBinding{
				{UrlParam: "version", Field: "version"},
			},
		},
	}
}
