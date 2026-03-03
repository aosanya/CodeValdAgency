// Package codevaldagency provides agency lifecycle management for the CodeVald
// platform. It exposes [AgencyManager] — the single interface for writing,
// reading, and updating the one agency that lives in this database.
//
// Usage:
//
//	b, err := arangodb.NewBackend(arangodb.Config{...})
//	mgr, err := codevaldagency.NewAgencyManager(b)
//	agency, err := mgr.SetAgencyDetails(ctx, `{"id":"agency-001","name":"Alpha"}`)
package codevaldagency

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"
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

	// GetAgency retrieves the single agency by its ID.
	// Returns [ErrAgencyNotFound] if no agency document exists yet.
	GetAgency(ctx context.Context, agencyID string) (Agency, error)

	// UpdateAgency applies incremental field edits with lifecycle validation.
	// Lifecycle transitions are validated — returns [ErrInvalidLifecycleTransition]
	// if the new status is not reachable from the current status.
	// When the transition is draft → active, a snapshot is written to the backend
	// before the update is applied.
	// Returns [ErrAgencyNotFound] if the agency does not exist.
	UpdateAgency(ctx context.Context, agencyID string, req UpdateAgencyRequest) (Agency, error)
}

// Backend is the storage abstraction injected into [AgencyManager].
// cmd/main.go constructs the chosen implementation (e.g. arangodb.NewBackend)
// and passes it to [NewAgencyManager]. The root package never imports any
// storage driver directly.
type Backend interface {
	// SetDetails parses the raw JSON and upserts the agency document at
	// _key = agency.id in the agency_details collection.
	// Returns [ErrInvalidJSON] if the JSON is malformed or the id field is missing.
	SetDetails(ctx context.Context, jsonStr string) (Agency, error)

	// Get retrieves the agency by its ID.
	// Returns [ErrAgencyNotFound] if no matching document exists.
	Get(ctx context.Context, agencyID string) (Agency, error)

	// Update applies a partial field merge and returns the updated agency.
	// Returns [ErrAgencyNotFound] if the agency does not exist.
	Update(ctx context.Context, agencyID string, req UpdateAgencyRequest) (Agency, error)

	// InsertSnapshot writes an immutable point-in-time copy of an agency to
	// the agency_snapshots collection. Called by [AgencyManager.UpdateAgency]
	// immediately before a draft → active transition is committed.
	InsertSnapshot(ctx context.Context, snapshot AgencySnapshot) error
}

// CrossPublisher publishes agency lifecycle events to CodeValdCross.
// Implementations must be safe for concurrent use. A nil CrossPublisher is
// valid — publish calls are silently skipped.
type CrossPublisher interface {
	// Publish delivers an event for the given topic and agencyID to
	// CodeValdCross. Errors are non-fatal: implementations should log and
	// return nil for best-effort delivery.
	Publish(ctx context.Context, topic string, agencyID string) error
}

// AgencyManagerOption is a functional option for [NewAgencyManager].
type AgencyManagerOption func(*agencyManager)

// WithPublisher attaches a [CrossPublisher] to the [AgencyManager].
// When provided, [AgencyManager.SetAgencyDetails] calls Publish with
// "cross.agency.created" after every successful write.
func WithPublisher(p CrossPublisher) AgencyManagerOption {
	return func(m *agencyManager) {
		m.publisher = p
	}
}

// agencyManager is the concrete implementation of [AgencyManager].
// It delegates all storage operations to the injected [Backend].
type agencyManager struct {
	backend   Backend
	publisher CrossPublisher // optional; nil = skip event publishing
}

// NewAgencyManager constructs an [AgencyManager] backed by the given [Backend].
// Use storage/arangodb.NewBackend to obtain a Backend, then pass it here.
// Pass [WithPublisher] to enable cross-service event publishing.
// Returns an error if b is nil.
func NewAgencyManager(b Backend, opts ...AgencyManagerOption) (AgencyManager, error) {
	if b == nil {
		return nil, fmt.Errorf("NewAgencyManager: backend must not be nil")
	}
	m := &agencyManager{backend: b}
	for _, opt := range opts {
		opt(m)
	}
	return m, nil
}

// SetAgencyDetails delegates to [Backend.SetDetails] and publishes
// "cross.agency.created" on every successful write.
func (m *agencyManager) SetAgencyDetails(ctx context.Context, jsonStr string) (Agency, error) {
	agency, err := m.backend.SetDetails(ctx, jsonStr)
	if err != nil {
		return Agency{}, err
	}
	// Best-effort publish — a publish error does not roll back the write.
	if m.publisher != nil {
		if pErr := m.publisher.Publish(ctx, "cross.agency.created", agency.ID); pErr != nil {
			_ = pErr
		}
	}
	return agency, nil
}

// GetAgency delegates to [Backend.Get].
func (m *agencyManager) GetAgency(ctx context.Context, agencyID string) (Agency, error) {
	return m.backend.Get(ctx, agencyID)
}

// UpdateAgency validates the lifecycle transition (if Status is changing),
// writes an activation snapshot on draft → active, and delegates to
// [Backend.Update].
func (m *agencyManager) UpdateAgency(ctx context.Context, agencyID string, req UpdateAgencyRequest) (Agency, error) {
	current, err := m.backend.Get(ctx, agencyID)
	if err != nil {
		return Agency{}, err
	}

	// Validate lifecycle when a status is explicitly provided.
	if req.Status != "" {
		// achieved is a terminal state — no further status changes are permitted,
		// even setting the same value.
		if current.Status == LifecycleAchieved {
			return Agency{}, ErrInvalidLifecycleTransition
		}

		if req.Status != current.Status {
			if !current.Status.CanTransitionTo(req.Status) {
				return Agency{}, ErrInvalidLifecycleTransition
			}

			// Write an activation snapshot before committing the draft → active
			// transition. This is an immutable audit record.
			if current.Status == LifecycleDraft && req.Status == LifecycleActive {
				snapshot := AgencySnapshot{
					ID:              newID(),
					AgencyID:        current.ID,
					Name:            current.Name,
					Mission:         current.Mission,
					Vision:          current.Vision,
					Goals:           current.Goals,
					Workflows:       current.Workflows,
					ConfiguredRoles: current.ConfiguredRoles,
					SnapshotAt:      time.Now().UTC(),
				}
				if err := m.backend.InsertSnapshot(ctx, snapshot); err != nil {
					return Agency{}, fmt.Errorf("UpdateAgency: write activation snapshot: %w", err)
				}
			}
		}
	}

	return m.backend.Update(ctx, agencyID, req)
}


// newID returns a random UUID v4 string using crypto/rand.
// Format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
