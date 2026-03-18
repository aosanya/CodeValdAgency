// docs.go contains ArangoDB document types and domain↔document conversion
// helpers for the arangodb package.
package arangodb

import (
	"time"

	codevaldagency "github.com/aosanya/CodeValdAgency"
)

// ── Document types ────────────────────────────────────────────────────────────

type roleAssignmentDoc struct {
	Role string `json:"role"`
	RACI string `json:"raci"`
}

type configuredRoleDoc struct {
	Role      string `json:"role"`
	ActorType string `json:"actor_type"`
}

type workItemDoc struct {
	ID          string              `json:"id"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Order       int                 `json:"order"`
	Parallel    bool                `json:"parallel"`
	GoalIDs     []string            `json:"goal_ids"`
	Assignments []roleAssignmentDoc `json:"assignments"`
}

type workflowDoc struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	WorkItems []workItemDoc `json:"work_items"`
}

type goalDoc struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Ordinality  int    `json:"ordinality"`
}

type agencyDoc struct {
	Key             string              `json:"_key,omitempty"`
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

type snapshotDoc struct {
	Key             string              `json:"_key,omitempty"`
	AgencyID        string              `json:"agency_id"`
	Name            string              `json:"name"`
	Mission         string              `json:"mission"`
	Vision          string              `json:"vision"`
	Goals           []goalDoc           `json:"goals"`
	Workflows       []workflowDoc       `json:"workflows"`
	ConfiguredRoles []configuredRoleDoc `json:"configured_roles"`
	SnapshotAt      time.Time           `json:"snapshot_at"`
}

// publicationDoc is the ArangoDB document representation of a
// [codevaldagency.AgencyPublication]. The _key is "v{version}".
type publicationDoc struct {
	Key         string    `json:"_key,omitempty"`
	ID          string    `json:"id"`
	Version     int       `json:"version"`
	Tag         string    `json:"tag"`
	Agency      agencyDoc `json:"agency"`
	PublishedAt time.Time `json:"published_at"`
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
		out[i] = workflowDoc{ID: wf.ID, Name: wf.Name, WorkItems: toWorkItemDocs(wf.WorkItems)}
	}
	return out
}

func fromWorkflowDocs(in []workflowDoc) []codevaldagency.Workflow {
	out := make([]codevaldagency.Workflow, len(in))
	for i, wf := range in {
		out[i] = codevaldagency.Workflow{ID: wf.ID, Name: wf.Name, WorkItems: fromWorkItemDocs(wf.WorkItems)}
	}
	return out
}

func toGoalDocs(in []codevaldagency.Goal) []goalDoc {
	out := make([]goalDoc, len(in))
	for i, g := range in {
		out[i] = goalDoc{ID: g.ID, Title: g.Title, Description: g.Description, Ordinality: g.Ordinality}
	}
	return out
}

func fromGoalDocs(in []goalDoc) []codevaldagency.Goal {
	out := make([]codevaldagency.Goal, len(in))
	for i, g := range in {
		out[i] = codevaldagency.Goal{ID: g.ID, Title: g.Title, Description: g.Description, Ordinality: g.Ordinality}
	}
	return out
}

func toConfiguredRoleDocs(in []codevaldagency.ConfiguredRole) []configuredRoleDoc {
	out := make([]configuredRoleDoc, len(in))
	for i, r := range in {
		out[i] = configuredRoleDoc{Role: string(r.Role), ActorType: string(r.ActorType)}
	}
	return out
}

func fromConfiguredRoleDocs(in []configuredRoleDoc) []codevaldagency.ConfiguredRole {
	out := make([]codevaldagency.ConfiguredRole, len(in))
	for i, r := range in {
		out[i] = codevaldagency.ConfiguredRole{
			Role:      codevaldagency.AgencyRole(r.Role),
			ActorType: codevaldagency.ActorType(r.ActorType),
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
		ConfiguredRoles: toConfiguredRoleDocs(a.ConfiguredRoles),
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
		ConfiguredRoles: fromConfiguredRoleDocs(doc.ConfiguredRoles),
		CreatedAt:       doc.CreatedAt,
		UpdatedAt:       doc.UpdatedAt,
	}
}

func fromPublicationDoc(doc publicationDoc) codevaldagency.AgencyPublication {
	agency := fromAgencyDoc(doc.Agency.Key, doc.Agency)
	return codevaldagency.AgencyPublication{
		ID:          doc.ID,
		Agency:      agency,
		Version:     doc.Version,
		Tag:         doc.Tag,
		PublishedAt: doc.PublishedAt,
	}
}
