# Agency Drafts — Implementation Details

> Task group: **MVP-AGENCY-009**
> Index: [mvp.md](../mvp.md) | [README.md](README.md)

---

## Overview

Agency Drafts allow multiple versions of an agency to be developed in parallel.
A draft is a **full deep-copy** of the live agency graph. The live agency is
**read-only**; all configuration changes flow through a draft. When a draft is
promoted it replaces the live agency entirely.

---

## Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Draft is a full copy | Yes — entire agency graph deep-copied on fork | Each draft is independently editable; no shared mutable state between versions |
| Live agency mutability | Read-only once published | All configuration changes go through drafts; prevents divergent edits |
| Promotion model | Draft replaces live; previous live is discarded | Simple replacement model; no merge complexity in v1 |
| Other open drafts on promotion | Remain open | User may still promote them; they are aware their base may be stale |
| Fork source | Live agency **or** another open draft | Maximum flexibility |
| Draft description | Required human-readable field | Needed to distinguish open drafts (e.g. "Q2 restructure", "experimental v2") |
| Agency lifecycle | Removed — replaced by `enabled`/`disabled` | `LifecycleDraft` was ambiguous with the new drafts concept; lifecycle is no longer needed on the live entity |
| First access before any promotion | Returns `ErrAgencyNotPublished` | No live agency exists until the first draft is promoted |
| Draft status lifecycle | `open → promoted \| archived` | `promoted` = became live; `archived` = soft-discarded; both are terminal |

---

## Changed: `Agency` Entity

`AgencyLifecycle` and its `status` field are **removed** from the live `Agency`
entity. The live agency gains a simple `enabled` boolean instead.

```go
// Agency is the live, published version of the agency.
// It is read-only — all edits go through an AgencyDraft.
// GetAgency returns ErrAgencyNotPublished until the first draft is promoted.
type Agency struct {
    ID        string
    Name      string
    Mission   string
    Vision    string
    Enabled   bool      // replaces AgencyLifecycle.Status
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

`AgencyLifecycle`, `CanTransitionTo`, and all `Lifecycle*` constants are
**deleted**. The activation-snapshot guard (`checkActivationGuards`,
`writeSnapshot`) is also removed — snapshots are now a side-effect of
`PromoteDraft`.

---

## New Type: `AgencyDraftStatus`

```go
type AgencyDraftStatus string

const (
    // DraftStatusOpen means the draft is being edited and can be promoted.
    DraftStatusOpen AgencyDraftStatus = "open"

    // DraftStatusPromoted is a terminal state: this draft became the live agency.
    DraftStatusPromoted AgencyDraftStatus = "promoted"

    // DraftStatusArchived is a terminal state: this draft was soft-discarded.
    DraftStatusArchived AgencyDraftStatus = "archived"
)
```

`CanTransitionTo` on `AgencyDraftStatus`:

| From | To | Allowed |
|---|---|---|
| `open` | `promoted` | ✅ |
| `open` | `archived` | ✅ |
| `promoted` | anything | ❌ terminal |
| `archived` | anything | ❌ terminal |

---

## New Type: `AgencyDraft`

```go
// AgencyDraft is a mutable, versioned copy of the agency graph.
// It holds the same sub-graph structure as a live Agency:
// Goals, Workflows, WorkItems, ConfiguredRoles, Instructions, Deliverables.
type AgencyDraft struct {
    ID             string
    Description    string            // required; human-readable label
    ForkedFromID   string            // ID of the live Agency or source AgencyDraft
    ForkedFromType string            // "live" or "draft"
    Status         AgencyDraftStatus
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

---

## New Errors

```go
// ErrAgencyNotPublished is returned by GetAgency when no draft has been
// promoted yet and no live agency exists.
var ErrAgencyNotPublished = errors.New("agency not published: promote a draft first")

// ErrDraftNotFound is returned when no AgencyDraft with the requested ID exists.
var ErrDraftNotFound = errors.New("agency draft not found")

// ErrDraftNotOpen is returned when an operation that requires an open draft
// (edit, promote, archive) is attempted on a promoted or archived draft.
var ErrDraftNotOpen = errors.New("agency draft is not open")

// ErrAgencyReadOnly is returned when a caller attempts to mutate the live
// agency directly. Use CreateDraft + PromoteDraft instead.
var ErrAgencyReadOnly = errors.New("live agency is read-only: use a draft to make changes")
```

---

## New `AgencyManager` Methods

```go
// CreateDraft forks the agency graph into a new AgencyDraft.
// forkedFromType must be "live" or "draft".
// If "live":  forks from the current live Agency; returns ErrAgencyNotPublished if none exists.
// If "draft": forks from the source AgencyDraft; returns ErrDraftNotFound if it does not exist,
//             returns ErrDraftNotOpen if the source draft is not open.
// description must be non-empty.
// The new draft status is "open". CreatedAt and UpdatedAt are server-stamped.
CreateDraft(ctx context.Context, description, forkedFromID, forkedFromType string) (AgencyDraft, error)

// GetDraft retrieves a single draft by its ID.
// Returns ErrDraftNotFound if no draft with that ID exists.
GetDraft(ctx context.Context, draftID string) (AgencyDraft, error)

// ListDrafts returns all drafts for this agency in descending creation order.
ListDrafts(ctx context.Context) ([]AgencyDraft, error)

// UpdateDraftDescription updates the human-readable description of an open draft.
// Returns ErrDraftNotFound if the draft does not exist.
// Returns ErrDraftNotOpen if the draft is not open.
UpdateDraftDescription(ctx context.Context, draftID, description string) (AgencyDraft, error)

// PromoteDraft replaces the live agency with the full graph of the given draft.
// The draft status transitions from "open" to "promoted".
// Other open drafts are unaffected (remain open; may still be promoted later).
// Publishes "cross.agency.promoted" on success.
// Returns ErrDraftNotFound if the draft does not exist.
// Returns ErrDraftNotOpen if the draft is not open.
PromoteDraft(ctx context.Context, draftID string) (Agency, error)

// ArchiveDraft soft-discards an open draft.
// The draft status transitions from "open" to "archived".
// Returns ErrDraftNotFound if the draft does not exist.
// Returns ErrDraftNotOpen if the draft is not open.
ArchiveDraft(ctx context.Context, draftID string) (AgencyDraft, error)
```

**Existing write methods** (`SetAgencyDetails`, `UpdateAgency`) return
`ErrAgencyReadOnly` once a live agency exists. `SetAgencyDetails` remains the
bootstrap path for populating the very first draft when no live agency exists.

---

## Fork Deep-Copy Scope

`CreateDraft` deep-copies the following entity types from the source:

| Entity type | Copied |
|---|---|
| Agency scalar fields (`name`, `mission`, `vision`, `enabled`) | ✅ |
| All Goals | ✅ |
| All Workflows | ✅ |
| All WorkItems (with Workflow assignment) | ✅ |
| All ConfiguredRoles | ✅ |
| All Instructions (with Workflow/WorkItem assignments) | ✅ |
| All Deliverables (with WorkItem assignments) | ✅ |
| AgencySnapshot | ❌ runtime record — not configuration |
| AgencyPublication / AgencyPublicationStatus | ❌ publication history — not configuration |
| DeliverableResult | ❌ runtime record — not configuration |
| ContentRef | ❌ runtime artifact pointer — not configuration |

---

## Schema Changes

### New TypeDefinition: `AgencyDraft`

- **Mutable**
- Properties: `description`(req), `status`(req), `forked_from_id`, `forked_from_type`, `created_at`, `updated_at`
- Relationships mirror the live `Agency` sub-graph:
  - `has_goal` → `Goal` (ToMany, Inverse: `belongs_to_draft`)
  - `has_workflow` → `Workflow` (ToMany, Inverse: `belongs_to_draft`)
  - `has_configured_role` → `ConfiguredRole` (ToMany, Inverse: `belongs_to_draft`)
- Inverse from Agency: `belongs_to_agency` (ToMany=false, Required=true)

The live `Agency` entity holds a **`has_draft` (ToMany)** edge to every draft ever
created for it — open, promoted, and archived. There is no single
"active draft" pointer. Callers retrieve open drafts via `ListDrafts` filtered
by `status=open`. This keeps the graph model simple and the draft history
complete.

### Updated TypeDefinition: `Agency`

- Remove `status` property (was `AgencyLifecycle`)
- Add `enabled` boolean property
- Add relationship: `has_draft` → `AgencyDraft` (ToMany=true, Inverse: `belongs_to_agency`)

### Updated TypeDefinitions: `Goal`, `Workflow`, `ConfiguredRole`

Each gains an optional inverse relationship `belongs_to_draft` → `AgencyDraft`
(ToMany=false, Required=false) alongside the existing `belongs_to_agency`.
This follows the same multi-parent pattern used by `Instruction`.

---

## Recommended Workflow

```
1. SetAgencyDetails (first-time bootstrap)
        │
        └─► CreateDraft (description="initial", forkedFromType="live")
                │
                └─► [edit Goals / Workflows / WorkItems / Roles via EntityService]
                        │
                        └─► PromoteDraft
                                │  draft.status → "promoted" (terminal)
                                │  live Agency updated
                                │  AgencySnapshot written
                                │
                        want more changes?
                                │
                                └─► CreateDraft (forkedFromType="live")
                                        │
                                        └─► [edit] → PromoteDraft → repeat
```

The promoted draft is always terminal — it is the immutable record linking
"what was live" to the `AgencyPublication` history. The publication chain
(`PublishAgency`) provides the externally versioned snapshot; the draft chain
provides the internal authoring audit trail.

---

## Storage

**Decision: Option B — separate `agency_draft_entities` collection.**

Draft sub-entities are stored in a **dedicated `agency_draft_entities` document
collection**, completely isolated from the live `agency_entities` collection.
The single `agency_relationships` edge collection spans both vertex collections
natively (ArangoDB full document handles: `_from: "agency_draft_entities/<uuid>"`).

| Collection | Holds |
|---|---|
| `agency_entities` | Live entity types only (Agency, Goal, Workflow, WorkItem, ConfiguredRole, Instruction, Deliverable) |
| `agency_draft_entities` | Draft copies of the same types, scoped by `draft_id` |
| `agency_drafts` | `AgencyDraft` root entities (description, status, fork metadata) |
| `agency_relationships` | All edges — live-to-live, draft-to-draft, and agency-to-draft root |

Draft entities carry a `draft_id` property (not `agency_id`) as their primary
scope key. `agency_id` is still present for cross-cutting queries.

```json
// agency_draft_entities/{key}
{
  "_key":      "<uuid>",
  "type_id":   "Goal",
  "draft_id":  "<draftID>",
  "agency_id": "<agencyID>",
  "properties": { "title": "Reduce onboarding time", "ordinality": 1 },
  "created_at": "...",
  "updated_at": "..."
}
```

### Why this is the right model

- **Zero contamination** — `agency_entities` exclusively holds live data; no filter can accidentally leak draft entities into a live context.
- **`PromoteDraft` is a clean copy** — read from `agency_draft_entities` where `draft_id=X`, write to `agency_entities`. No field patching.
- **`ArchiveDraft` cleanup** — delete all `agency_draft_entities` documents where `draft_id=X` in a single AQL statement.
- **No second edge collection** — ArangoDB edge `_from`/`_to` use full handles, so `agency_relationships` works across both vertex collections.

---

## Pub/Sub Events

| Topic | Trigger |
|---|---|
| `cross.agency.promoted` | `PromoteDraft` — a draft became the live agency |
| `cross.agency.draft.created` | `CreateDraft` — a new draft was forked |
| `cross.agency.draft.archived` | `ArchiveDraft` — a draft was soft-discarded |

---

## MVP-AGENCY-009-A — Models & Errors Update

**Status**: 🔲 Not Started
**Branch**: `feature/AGENCY-009-A_agency_draft_models`

### Files to Modify

| File | Change |
|---|---|
| `models.go` | Remove `AgencyLifecycle`, `LifecycleDraft/Active/Achieved`, `CanTransitionTo`. Add `AgencyDraftStatus`, `DraftStatus*` constants, `AgencyDraft`. Update `Agency` (remove `Status AgencyLifecycle`, add `Enabled bool`). |
| `errors.go` | Add `ErrAgencyNotPublished`, `ErrDraftNotFound`, `ErrDraftNotOpen`, `ErrAgencyReadOnly`. |

### Acceptance Tests

- `AgencyDraftStatus.CanTransitionTo` — all valid and invalid transitions
- `AgencyDraft` struct fields present and zero-valued correctly

---

## MVP-AGENCY-009-B — Schema Update

**Status**: 🔲 Not Started
**Branch**: `feature/AGENCY-009-B_agency_draft_schema`
**Depends on**: MVP-AGENCY-009-A

### Files to Modify

| File | Change |
|---|---|
| `schema.go` | Add `AgencyDraft` TypeDefinition. Update `Agency` TypeDefinition (remove `status`, add `enabled`, add `has_draft` relationship). Update `Goal`, `Workflow`, `ConfiguredRole` TypeDefinitions (add `belongs_to_draft` optional inverse). |

---

## MVP-AGENCY-009-C — AgencyManager Draft Methods

**Status**: 🔲 Not Started
**Branch**: `feature/AGENCY-009-C_agency_draft_manager`
**Depends on**: MVP-AGENCY-009-A

### Files to Modify

| File | Change |
|---|---|
| `agency.go` | Add `CreateDraft`, `GetDraft`, `ListDrafts`, `UpdateDraftDescription`, `PromoteDraft`, `ArchiveDraft` to `AgencyManager` interface and `agencyManager` implementation. Update `SetAgencyDetails` and `UpdateAgency` to return `ErrAgencyReadOnly` when a live agency exists. Remove `checkActivationGuards` and `writeSnapshot`. |

---

## MVP-AGENCY-009-D — ArangoDB Storage

**Status**: 🔲 Not Started
**Branch**: `feature/AGENCY-009-D_agency_draft_storage`
**Depends on**: MVP-AGENCY-009-C

### Files to Modify

| File | Change |
|---|---|
| `storage/arangodb/storage.go` | Add `ensureCollection` calls for `agency_drafts` and `agency_draft_entities`. Update `ensureGraph` to add `agency_draft_entities` as a vertex collection. |
| `storage/arangodb/docs.go` | Add `agencyDraftDoc` ↔ `AgencyDraft` and `agencyDraftEntityDoc` ↔ entity conversion helpers. |
| `storage/arangodb/entities.go` | Add `CreateDraftEntity`, `ListDraftEntities`, `UpdateDraftEntity`, `DeleteDraftEntity` — target `agency_draft_entities`, scoped by `draft_id`. |

Key implementation: `PromoteDraft` deep-copy — AQL reads all documents from
`agency_draft_entities` where `draft_id=X`, inserts them into `agency_entities`,
then re-creates all edges in `agency_relationships` under the live `agency_id` scope.

---

## MVP-AGENCY-009-E — gRPC Handlers

**Status**: 🔲 Not Started
**Branch**: `feature/AGENCY-009-E_agency_draft_grpc`
**Depends on**: MVP-AGENCY-009-C

### New RPCs (to be added to `agency.proto`)

| RPC | Request | Response |
|---|---|---|
| `CreateDraft` | `CreateDraftRequest` | `AgencyDraft` |
| `GetDraft` | `GetDraftRequest` | `AgencyDraft` |
| `ListDrafts` | `ListDraftsRequest` | `ListDraftsResponse` |
| `UpdateDraftDescription` | `UpdateDraftDescriptionRequest` | `AgencyDraft` |
| `PromoteDraft` | `PromoteDraftRequest` | `Agency` |
| `ArchiveDraft` | `ArchiveDraftRequest` | `AgencyDraft` |

---

## MVP-AGENCY-009-F — Tests

**Status**: 🔲 Not Started
**Branch**: `feature/AGENCY-009-F_agency_draft_tests`
**Depends on**: MVP-AGENCY-009-C, MVP-AGENCY-009-D

### Acceptance Tests

- `CreateDraft` from live agency succeeds; returns draft with `status=open`
- `CreateDraft` from another open draft succeeds; `forked_from_type="draft"`
- `CreateDraft` from promoted/archived draft returns `ErrDraftNotOpen`
- `CreateDraft` when no live agency and `forkedFromType="live"` returns `ErrAgencyNotPublished`
- `GetDraft` returns `ErrDraftNotFound` for unknown ID
- `ListDrafts` returns all drafts in descending creation order
- `UpdateDraftDescription` on promoted draft returns `ErrDraftNotOpen`
- `PromoteDraft` replaces live agency; draft becomes `promoted`
- `PromoteDraft` does not affect other open drafts
- `PromoteDraft` on archived draft returns `ErrDraftNotOpen`
- `ArchiveDraft` sets status to `archived`; subsequent `PromoteDraft` returns `ErrDraftNotOpen`
- `GetAgency` before any promotion returns `ErrAgencyNotPublished`
- `SetAgencyDetails` / `UpdateAgency` on a live agency return `ErrAgencyReadOnly`
