// Package arangodb implements the codevaldagency.Backend interface backed by
// ArangoDB. Agency documents are stored in the `agencies` collection and
// activation snapshots in the `agency_snapshots` collection.
//
// Use [NewBackend] to construct; pass the result to
// codevaldagency.NewAgencyManager.
package arangodb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	driver "github.com/arangodb/go-driver"

	codevaldagency "github.com/aosanya/CodeValdAgency"
	"github.com/aosanya/CodeValdSharedLib/arangoutil"
)

const (
	colAgencies  = "agency_details"
	colSnapshots = "agency_snapshots"
)

// Config holds the connection parameters for the ArangoDB backend.
type Config struct {
	// Endpoint is the ArangoDB HTTP endpoint (e.g. "http://localhost:8529").
	Endpoint string

	// Username is the ArangoDB username (default "root").
	Username string

	// Password is the ArangoDB password.
	Password string

	// Database is the ArangoDB database name (default "codevaldagency").
	Database string
}

// Backend is the ArangoDB implementation of [codevaldagency.Backend].
type Backend struct {
	db            driver.Database
	agencyDetails driver.Collection
	snapshots     driver.Collection
}

// NewBackend connects to ArangoDB, ensures both collections exist, and returns
// a ready-to-use [Backend].
func NewBackend(cfg Config) (*Backend, error) {
	if cfg.Endpoint == "" {
		cfg.Endpoint = "http://localhost:8529"
	}
	if cfg.Username == "" {
		cfg.Username = "root"
	}
	if cfg.Database == "" {
		cfg.Database = "codevaldagency"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := arangoutil.Connect(ctx, arangoutil.Config{
		Endpoint: cfg.Endpoint,
		Username: cfg.Username,
		Password: cfg.Password,
		Database: cfg.Database,
	})
	if err != nil {
		return nil, fmt.Errorf("arangodb: %w", err)
	}

	return newBackendFromDB(ctx, db)
}

// NewBackendFromDB constructs a [Backend] from an already-open
// [driver.Database]. It ensures both collections exist and returns a
// ready-to-use backend. This constructor is intended for tests that manage
// their own database lifecycle.
func NewBackendFromDB(db driver.Database) (*Backend, error) {
	if db == nil {
		return nil, fmt.Errorf("arangodb: NewBackendFromDB: database must not be nil")
	}
	return newBackendFromDB(context.Background(), db)
}

func newBackendFromDB(ctx context.Context, db driver.Database) (*Backend, error) {
	agencyDetails, err := ensureCollection(ctx, db, colAgencies)
	if err != nil {
		return nil, fmt.Errorf("arangodb: ensure %q: %w", colAgencies, err)
	}

	snapshots, err := ensureCollection(ctx, db, colSnapshots)
	if err != nil {
		return nil, fmt.Errorf("arangodb: ensure %q: %w", colSnapshots, err)
	}

	return &Backend{db: db, agencyDetails: agencyDetails, snapshots: snapshots}, nil
}

func ensureCollection(ctx context.Context, db driver.Database, name string) (driver.Collection, error) {
	exists, err := db.CollectionExists(ctx, name)
	if err != nil {
		return nil, err
	}
	if exists {
		return db.Collection(ctx, name)
	}
	col, err := db.CreateCollection(ctx, name, nil)
	if err != nil {
		// Another goroutine may have created it concurrently (TOCTOU).
		// Fall back to opening the existing collection.
		if driver.IsConflict(err) {
			return db.Collection(ctx, name)
		}
		return nil, err
	}
	return col, nil
}

// ── Document types ────────────────────────────────────────────────────────────

// roleAssignmentDoc is the JSON representation of a [codevaldagency.RoleAssignment].
type roleAssignmentDoc struct {
	Role string `json:"role"`
	RACI string `json:"raci"`
}

// workItemDoc is the JSON representation of a [codevaldagency.WorkItem].
type workItemDoc struct {
	ID          string              `json:"id"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Order       int                 `json:"order"`
	Parallel    bool                `json:"parallel"`
	GoalIDs     []string            `json:"goal_ids"`
	Assignments []roleAssignmentDoc `json:"assignments"`
}

// workflowDoc is the JSON representation of a [codevaldagency.Workflow].
type workflowDoc struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	WorkItems []workItemDoc `json:"work_items"`
}

// goalDoc is the JSON representation of a [codevaldagency.Goal].
type goalDoc struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Ordinality  int    `json:"ordinality"`
}

// agencyDoc is the ArangoDB document representation of a [codevaldagency.Agency].
type agencyDoc struct {
	Key             string        `json:"_key,omitempty"`
	Name            string        `json:"name"`
	Mission         string        `json:"mission"`
	Vision          string        `json:"vision"`
	Status          string        `json:"status"`
	Goals           []goalDoc     `json:"goals"`
	Workflows       []workflowDoc `json:"workflows"`
	ConfiguredRoles []string      `json:"configured_roles"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

// snapshotDoc is the ArangoDB document representation of a
// [codevaldagency.AgencySnapshot].
type snapshotDoc struct {
	Key             string        `json:"_key,omitempty"`
	AgencyID        string        `json:"agency_id"`
	Name            string        `json:"name"`
	Mission         string        `json:"mission"`
	Vision          string        `json:"vision"`
	Goals           []goalDoc     `json:"goals"`
	Workflows       []workflowDoc `json:"workflows"`
	ConfiguredRoles []string      `json:"configured_roles"`
	SnapshotAt      time.Time     `json:"snapshot_at"`
}

// ── Conversion helpers ────────────────────────────────────────────────────────

func toRoleAssignmentDocs(in []codevaldagency.RoleAssignment) []roleAssignmentDoc {
	out := make([]roleAssignmentDoc, len(in))
	for i, r := range in {
		out[i] = roleAssignmentDoc{Role: string(r.Role), RACI: string(r.RACI)}
	}
	return out
}

func fromRoleAssignmentDocs(in []roleAssignmentDoc) []codevaldagency.RoleAssignment {
	out := make([]codevaldagency.RoleAssignment, len(in))
	for i, r := range in {
		out[i] = codevaldagency.RoleAssignment{
			Role: codevaldagency.AgencyRole(r.Role),
			RACI: codevaldagency.RACILabel(r.RACI),
		}
	}
	return out
}

func toWorkItemDocs(in []codevaldagency.WorkItem) []workItemDoc {
	out := make([]workItemDoc, len(in))
	for i, w := range in {
		out[i] = workItemDoc{
			ID:          w.ID,
			Title:       w.Title,
			Description: w.Description,
			Order:       w.Order,
			Parallel:    w.Parallel,
			GoalIDs:     w.GoalIDs,
			Assignments: toRoleAssignmentDocs(w.Assignments),
		}
	}
	return out
}

func fromWorkItemDocs(in []workItemDoc) []codevaldagency.WorkItem {
	out := make([]codevaldagency.WorkItem, len(in))
	for i, w := range in {
		out[i] = codevaldagency.WorkItem{
			ID:          w.ID,
			Title:       w.Title,
			Description: w.Description,
			Order:       w.Order,
			Parallel:    w.Parallel,
			GoalIDs:     w.GoalIDs,
			Assignments: fromRoleAssignmentDocs(w.Assignments),
		}
	}
	return out
}

func toWorkflowDocs(in []codevaldagency.Workflow) []workflowDoc {
	out := make([]workflowDoc, len(in))
	for i, wf := range in {
		out[i] = workflowDoc{
			ID:        wf.ID,
			Name:      wf.Name,
			WorkItems: toWorkItemDocs(wf.WorkItems),
		}
	}
	return out
}

func fromWorkflowDocs(in []workflowDoc) []codevaldagency.Workflow {
	out := make([]codevaldagency.Workflow, len(in))
	for i, wf := range in {
		out[i] = codevaldagency.Workflow{
			ID:        wf.ID,
			Name:      wf.Name,
			WorkItems: fromWorkItemDocs(wf.WorkItems),
		}
	}
	return out
}

func toGoalDocs(in []codevaldagency.Goal) []goalDoc {
	out := make([]goalDoc, len(in))
	for i, g := range in {
		out[i] = goalDoc{
			ID:          g.ID,
			Title:       g.Title,
			Description: g.Description,
			Ordinality:  g.Ordinality,
		}
	}
	return out
}

func fromGoalDocs(in []goalDoc) []codevaldagency.Goal {
	out := make([]codevaldagency.Goal, len(in))
	for i, g := range in {
		out[i] = codevaldagency.Goal{
			ID:          g.ID,
			Title:       g.Title,
			Description: g.Description,
			Ordinality:  g.Ordinality,
		}
	}
	return out
}

func toAgencyDoc(a codevaldagency.Agency) agencyDoc {
	return agencyDoc{
		Key:             a.ID,
		Name:            a.Name,
		Mission:         a.Mission,
		Vision:          a.Vision,
		Status:          string(a.Status),
		Goals:           toGoalDocs(a.Goals),
		Workflows:       toWorkflowDocs(a.Workflows),
		ConfiguredRoles: a.ConfiguredRoles,
		CreatedAt:       a.CreatedAt,
		UpdatedAt:       a.UpdatedAt,
	}
}

func fromAgencyDoc(key string, doc agencyDoc) codevaldagency.Agency {
	return codevaldagency.Agency{
		ID:              key,
		Name:            doc.Name,
		Mission:         doc.Mission,
		Vision:          doc.Vision,
		Status:          codevaldagency.AgencyLifecycle(doc.Status),
		Goals:           fromGoalDocs(doc.Goals),
		Workflows:       fromWorkflowDocs(doc.Workflows),
		ConfiguredRoles: doc.ConfiguredRoles,
		CreatedAt:       doc.CreatedAt,
		UpdatedAt:       doc.UpdatedAt,
	}
}

// ── Backend interface implementation ─────────────────────────────────────────

// SetDetails implements [codevaldagency.Backend].
// It parses the raw JSON, builds an agencyDoc keyed by agency.id, then
// upserts (replace-or-create) the single document in the agency_details collection.
func (b *Backend) SetDetails(ctx context.Context, jsonStr string) (codevaldagency.Agency, error) {
	var raw struct {
		ID              string        `json:"id"`
		Name            string        `json:"name"`
		Mission         string        `json:"mission"`
		Vision          string        `json:"vision"`
		Status          string        `json:"status"`
		Goals           []goalDoc     `json:"goals"`
		Workflows       []workflowDoc `json:"workflows"`
		ConfiguredRoles []string      `json:"configured_roles"`
		CreatedAt       time.Time     `json:"created_at"`
		UpdatedAt       time.Time     `json:"updated_at"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return codevaldagency.Agency{}, codevaldagency.ErrInvalidJSON
	}
	if raw.ID == "" {
		return codevaldagency.Agency{}, codevaldagency.ErrInvalidJSON
	}

	doc := agencyDoc{
		Key:             raw.ID,
		Name:            raw.Name,
		Mission:         raw.Mission,
		Vision:          raw.Vision,
		Status:          raw.Status,
		Goals:           raw.Goals,
		Workflows:       raw.Workflows,
		ConfiguredRoles: raw.ConfiguredRoles,
		CreatedAt:       raw.CreatedAt,
		UpdatedAt:       raw.UpdatedAt,
	}

	// Upsert: try replace first; fall back to create on first write.
	_, err := b.agencyDetails.ReplaceDocument(ctx, doc.Key, doc)
	if err != nil {
		if driver.IsNotFound(err) {
			if _, err = b.agencyDetails.CreateDocument(ctx, doc); err != nil {
				return codevaldagency.Agency{}, fmt.Errorf("SetDetails: create: %w", err)
			}
		} else {
			return codevaldagency.Agency{}, fmt.Errorf("SetDetails: replace: %w", err)
		}
	}

	return fromAgencyDoc(doc.Key, doc), nil
}

// Get implements [codevaldagency.Backend].
// It retrieves the single agency document in the collection via an AQL query.
func (b *Backend) Get(ctx context.Context) (codevaldagency.Agency, error) {
	query := `FOR doc IN agency_details LIMIT 1 RETURN doc`
	cursor, err := b.db.Query(ctx, query, nil)
	if err != nil {
		return codevaldagency.Agency{}, fmt.Errorf("Get: query: %w", err)
	}
	defer cursor.Close()
	if !cursor.HasMore() {
		return codevaldagency.Agency{}, codevaldagency.ErrAgencyNotFound
	}
	var doc agencyDoc
	meta, err := cursor.ReadDocument(ctx, &doc)
	if err != nil {
		return codevaldagency.Agency{}, fmt.Errorf("Get: read: %w", err)
	}
	return fromAgencyDoc(meta.Key, doc), nil
}

// Update implements [codevaldagency.Backend].
func (b *Backend) Update(ctx context.Context, req codevaldagency.UpdateAgencyRequest) (codevaldagency.Agency, error) {
	current, err := b.Get(ctx)
	if err != nil {
		return codevaldagency.Agency{}, err
	}

	// Apply mutable fields from the request.
	if req.Name != "" {
		current.Name = req.Name
	}
	if req.Mission != "" {
		current.Mission = req.Mission
	}
	if req.Vision != "" {
		current.Vision = req.Vision
	}
	if req.Status != "" {
		current.Status = req.Status
	}
	if req.Goals != nil {
		current.Goals = req.Goals
	}
	if req.Workflows != nil {
		current.Workflows = req.Workflows
	}
	if req.ConfiguredRoles != nil {
		current.ConfiguredRoles = req.ConfiguredRoles
	}
	current.UpdatedAt = time.Now().UTC()

	doc := toAgencyDoc(current)
	_, err = b.agencyDetails.ReplaceDocument(ctx, current.ID, doc)
	if err != nil {
		if driver.IsNotFound(err) {
			return codevaldagency.Agency{}, codevaldagency.ErrAgencyNotFound
		}
		return codevaldagency.Agency{}, fmt.Errorf("Update: %w", err)
	}
	return current, nil
}


// InsertSnapshot implements [codevaldagency.Backend].
func (b *Backend) InsertSnapshot(ctx context.Context, snap codevaldagency.AgencySnapshot) error {
	doc := snapshotDoc{
		Key:             snap.ID,
		AgencyID:        snap.AgencyID,
		Name:            snap.Name,
		Mission:         snap.Mission,
		Vision:          snap.Vision,
		Goals:           toGoalDocs(snap.Goals),
		Workflows:       toWorkflowDocs(snap.Workflows),
		ConfiguredRoles: snap.ConfiguredRoles,
		SnapshotAt:      snap.SnapshotAt,
	}
	_, err := b.snapshots.CreateDocument(ctx, doc)
	if err != nil {
		return fmt.Errorf("InsertSnapshot: %w", err)
	}
	return nil
}

