// ops.go contains the Backend interface method implementations.
package arangodb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	driver "github.com/arangodb/go-driver"

	codevaldagency "github.com/aosanya/CodeValdAgency"
)

// SetDetails implements [codevaldagency.Backend].
// It parses the raw JSON, then writes it as the single agency document in the
// agency_details collection. On subsequent calls the existing document's _key
// is reused so the same record is always updated, regardless of the id value
// in the payload.
func (b *Backend) SetDetails(ctx context.Context, jsonStr string) (codevaldagency.Agency, error) {
	var raw struct {
		ID              string              `json:"id"`
		Name            string              `json:"name"`
		Mission         string              `json:"mission"`
		Vision          string              `json:"vision"`
		Status          string              `json:"status"`
		Goals           []goalDoc           `json:"goals"`
		Workflows       []workflowDoc       `json:"workflows"`
		ConfiguredRoles []configuredRoleDoc `json:"configured_roles"`
		CreatedAt       time.Time           `json:"created_at"`
		UpdatedAt       time.Time           `json:"updated_at"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return codevaldagency.Agency{}, fmt.Errorf("%w: %v", codevaldagency.ErrInvalidJSON, err)
	}
	if raw.ID == "" {
		return codevaldagency.Agency{}, fmt.Errorf("%w: \"id\" field is required", codevaldagency.ErrInvalidJSON)
	}

	doc := agencyDoc{
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

	allKeys, err := b.allAgencyKeys(ctx)
	if err != nil {
		return codevaldagency.Agency{}, fmt.Errorf("SetDetails: list keys: %w", err)
	}

	switch len(allKeys) {
	case 0:
		doc.Key = raw.ID
		if _, err = b.agencyDetails.CreateDocument(ctx, doc); err != nil {
			return codevaldagency.Agency{}, fmt.Errorf("SetDetails: create: %w", err)
		}
	case 1:
		doc.Key = allKeys[0]
		if _, err = b.agencyDetails.ReplaceDocument(ctx, allKeys[0], doc); err != nil {
			return codevaldagency.Agency{}, fmt.Errorf("SetDetails: replace: %w", err)
		}
	default:
		for _, k := range allKeys[1:] {
			if _, rmErr := b.agencyDetails.RemoveDocument(ctx, k); rmErr != nil && !driver.IsNotFound(rmErr) {
				return codevaldagency.Agency{}, fmt.Errorf("SetDetails: remove duplicate %q: %w", k, rmErr)
			}
		}
		doc.Key = allKeys[0]
		if _, err = b.agencyDetails.ReplaceDocument(ctx, allKeys[0], doc); err != nil {
			return codevaldagency.Agency{}, fmt.Errorf("SetDetails: replace after cleanup: %w", err)
		}
	}

	return fromAgencyDoc(doc.Key, doc), nil
}

func (b *Backend) allAgencyKeys(ctx context.Context) ([]string, error) {
	cursor, err := b.db.Query(ctx, `FOR doc IN agency_details RETURN doc._key`, nil)
	if err != nil {
		return nil, err
	}
	defer cursor.Close()
	var keys []string
	for cursor.HasMore() {
		var k string
		if _, err := cursor.ReadDocument(ctx, &k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, nil
}

// Get implements [codevaldagency.Backend].
func (b *Backend) Get(ctx context.Context) (codevaldagency.Agency, error) {
	cursor, err := b.db.Query(ctx, `FOR doc IN agency_details LIMIT 1 RETURN doc`, nil)
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
	if _, err = b.agencyDetails.ReplaceDocument(ctx, current.ID, doc); err != nil {
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
		ConfiguredRoles: toConfiguredRoleDocs(snap.ConfiguredRoles),
		SnapshotAt:      snap.SnapshotAt,
	}
	if _, err := b.snapshots.CreateDocument(ctx, doc); err != nil {
		return fmt.Errorf("InsertSnapshot: %w", err)
	}
	return nil
}

// InsertPublication implements [codevaldagency.Backend].
func (b *Backend) InsertPublication(ctx context.Context, pub codevaldagency.AgencyPublication) error {
	doc := publicationDoc{
		Key:         fmt.Sprintf("v%d", pub.Version),
		ID:          pub.ID,
		Version:     pub.Version,
		Tag:         pub.Tag,
		Agency:      toAgencyDoc(pub.Agency),
		PublishedAt: pub.PublishedAt,
	}
	if _, err := b.publications.CreateDocument(ctx, doc); err != nil {
		return fmt.Errorf("InsertPublication: %w", err)
	}
	return nil
}

// GetPublication implements [codevaldagency.Backend].
func (b *Backend) GetPublication(ctx context.Context, version int) (codevaldagency.AgencyPublication, error) {
	var doc publicationDoc
	if _, err := b.publications.ReadDocument(ctx, fmt.Sprintf("v%d", version), &doc); err != nil {
		if driver.IsNotFound(err) {
			return codevaldagency.AgencyPublication{}, codevaldagency.ErrPublicationNotFound
		}
		return codevaldagency.AgencyPublication{}, fmt.Errorf("GetPublication: %w", err)
	}
	return fromPublicationDoc(doc), nil
}

// ListPublications implements [codevaldagency.Backend].
// Returns all publications in ascending version order.
func (b *Backend) ListPublications(ctx context.Context) ([]codevaldagency.AgencyPublication, error) {
	q := `FOR doc IN agency_publications SORT doc.version ASC RETURN doc`
	cursor, err := b.db.Query(ctx, q, nil)
	if err != nil {
		return nil, fmt.Errorf("ListPublications: query: %w", err)
	}
	defer cursor.Close()
	var results []codevaldagency.AgencyPublication
	for cursor.HasMore() {
		var doc publicationDoc
		if _, err := cursor.ReadDocument(ctx, &doc); err != nil {
			return nil, fmt.Errorf("ListPublications: read: %w", err)
		}
		results = append(results, fromPublicationDoc(doc))
	}
	return results, nil
}

// NextPublicationVersion implements [codevaldagency.Backend].
// Returns MAX(version)+1, or 1 if no publications exist yet.
func (b *Backend) NextPublicationVersion(ctx context.Context) (int, error) {
	q := `RETURN MAX(FOR doc IN agency_publications RETURN doc.version)`
	cursor, err := b.db.Query(ctx, q, nil)
	if err != nil {
		return 0, fmt.Errorf("NextPublicationVersion: query: %w", err)
	}
	defer cursor.Close()
	var maxVersion *int
	if cursor.HasMore() {
		if _, err := cursor.ReadDocument(ctx, &maxVersion); err != nil {
			return 0, fmt.Errorf("NextPublicationVersion: read: %w", err)
		}
	}
	if maxVersion == nil {
		return 1, nil
	}
	return *maxVersion + 1, nil
}

// GetGoals implements [codevaldagency.Backend].
// Returns the Goals embedded in the agency document.
func (b *Backend) GetGoals(ctx context.Context) ([]codevaldagency.Goal, error) {
	agency, err := b.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetGoals: %w", err)
	}
	return agency.Goals, nil
}

// GetWorkflows implements [codevaldagency.Backend].
// Returns the Workflows (with their WorkItems) embedded in the agency document.
func (b *Backend) GetWorkflows(ctx context.Context) ([]codevaldagency.Workflow, error) {
	agency, err := b.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetWorkflows: %w", err)
	}
	return agency.Workflows, nil
}

// GetConfiguredRoles implements [codevaldagency.Backend].
// Returns the ConfiguredRoles embedded in the agency document.
func (b *Backend) GetConfiguredRoles(ctx context.Context) ([]codevaldagency.ConfiguredRole, error) {
	agency, err := b.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetConfiguredRoles: %w", err)
	}
	return agency.ConfiguredRoles, nil
}
