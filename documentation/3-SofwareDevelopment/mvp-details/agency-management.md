# Agency Management — Implementation Details

## Design Decision: Single-Agency Database

Each CodeValdAgency database instance holds **exactly one agency**. There is no
listing, no multi-tenancy, and no deletion.

`SetAgencyDetails` is the **bootstrap-only** write path: it creates or updates
the root `Agency` entity *until the first draft has been promoted*. After that,
the live agency is read-only — direct calls to `SetAgencyDetails` return
`ErrAgencyReadOnly` and all further changes flow through `CreateDraft` +
`PromoteDraft` (see [agency-drafts.md](agency-drafts.md)).

> **Historical note.** Tasks MVP-AGENCY-001 → 005 below were originally written
> against an `AgencyLifecycle` (`draft → active → achieved`) model with a
> separate `Backend` interface. Both were superseded:
> - `AgencyLifecycle` was replaced by `Enabled bool` + draft-based authoring in MVP-AGENCY-009.
> - The bespoke `Backend` interface was replaced by `entitygraph.DataManager` in MVP-AGENCY-008.
>
> Sections below have been trimmed to the parts that still match the shipped
> code; the lifecycle-transition rules, the `UpdateAgency` RPC, and the
> `agency_details` collection are gone.

---

## MVP-AGENCY-001 — Library Scaffolding & Agency Model

**Status**: ✅ Done — see [mvp_done.md](../mvp_done.md)
**Branch**: `feature/AGENCY-001_library_scaffolding`

### Goal

Scaffold the Go module with the `AgencyManager` interface, `Agency` domain
type, and exported errors. The shipped surface (post MVP-AGENCY-008/009) is
the canonical reference — see [agency.go](../../../agency.go),
[models.go](../../../models.go), [errors.go](../../../errors.go).

### Files

| File | Purpose |
|---|---|
| `go.mod` | Module declaration (`github.com/aosanya/CodeValdAgency`) |
| `agency.go` | `AgencyManager` interface + `agencyManager` implementation (wraps `entitygraph.DataManager` — no bespoke `Backend`) |
| `drafts.go` | Draft-specific manager methods (added in MVP-AGENCY-009) |
| `models.go` | `Agency`, `Goal`, `Workflow`, `WorkItem`, `ConfiguredRole`, `AgencyDraft`, `AgencyDraftStatus`, `AgencySnapshot`, `AgencyPublication` |
| `errors.go` | `ErrAgencyNotFound`, `ErrAgencyNotPublished`, `ErrAgencyReadOnly`, `ErrInvalidAgency`, `ErrInvalidJSON`, `ErrDraftNotFound`, `ErrDraftNotOpen`, `ErrPublicationNotFound`, `ErrInvalidPublicationStatus`, `ErrNoChangesDetected` |

### AgencyManager Interface (shipped)

```go
type AgencyManager interface {
    // SetAgencyDetails — bootstrap-only write path.
    // Returns ErrInvalidJSON if the payload cannot be parsed or "id" is missing.
    // Returns ErrAgencyReadOnly once the live agency has been published.
    // Publishes cross.agency.created on success.
    SetAgencyDetails(ctx context.Context, jsonStr string) (Agency, error)

    // GetAgency retrieves the single live agency.
    // Returns ErrAgencyNotFound if no agency entity exists yet.
    GetAgency(ctx context.Context) (Agency, error)

    // Convenience accessors over the live sub-graph.
    GetGoals(ctx context.Context) ([]Goal, error)
    GetWorkflows(ctx context.Context) ([]Workflow, error)
    GetConfiguredRoles(ctx context.Context) ([]ConfiguredRole, error)

    // Drafts — see agency-drafts.md
    CreateDraft(ctx context.Context, description, forkedFromID, forkedFromType string) (AgencyDraft, error)
    GetDraft(ctx context.Context, draftID string) (AgencyDraft, error)
    ListDrafts(ctx context.Context) ([]AgencyDraft, error)
    UpdateDraftDescription(ctx context.Context, draftID, description string) (AgencyDraft, error)
    PromoteDraft(ctx context.Context, draftID string) (Agency, error)
    ArchiveDraft(ctx context.Context, draftID string) (AgencyDraft, error)

    // Publications — see "MVP-AGENCY-007" section below.
    PublishAgency(ctx context.Context, draftID string) (AgencyPublication, error)
    GetPublication(ctx context.Context, version int) (AgencyPublication, error)
    ListPublications(ctx context.Context) ([]AgencyPublication, error)
    UpdatePublicationStatus(ctx context.Context, version int, status string) (AgencyPublication, error)
}
```

There is **no `UpdateAgency` method** — incremental field edits on a published
agency must go through a draft.

### Acceptance Tests (current)

- `SetAgencyDetails` with invalid JSON returns `ErrInvalidJSON`
- `SetAgencyDetails` with missing `id` field returns `ErrInvalidJSON`
- `SetAgencyDetails` with valid JSON returns the stored agency
- `GetAgency` after `SetAgencyDetails` returns matching data
- `SetAgencyDetails` called twice replaces the document while no draft has been promoted
- `SetAgencyDetails` on a published agency (`Enabled == true`) returns `ErrAgencyReadOnly`
- Direct mutations against a published agency return `ErrAgencyReadOnly`

---

## MVP-AGENCY-002 — ArangoDB Backend

**Status**: ✅ Done — superseded by MVP-AGENCY-008-D
**Branch**: `feature/AGENCY-002_arangodb_backend`

The bespoke `Backend` interface from this task was retired in MVP-AGENCY-008.
Storage now goes through `entitygraph.DataManager` from CodeValdSharedLib;
`storage/arangodb/storage.go` is a thin adapter that fixes the agency-specific
collection and graph names.

| Collection | Holds |
|---|---|
| `agency_entities` | Live entities of every non-draft type — Agency, Goal, Workflow, WorkItem, ConfiguredRole, Instruction, Deliverable, DeliverableResult, ContentRef, AgencySnapshot, AgencyPublication, AgencyPublicationStatus |
| `agency_drafts` | `AgencyDraft` root entities (routed by `TypeDefinition.StorageCollection`) |
| `agency_draft_entities` | Draft sub-entities — dedicated types `DraftGoal`, `DraftWorkflow`, `DraftWorkItem`, `DraftConfiguredRole`, `DraftInstruction`, `DraftDeliverable`, `DraftDeliverableResult` (each routed by `StorageCollection`) |
| `agency_relationships` | Edge collection — spans all vertex collections via full ArangoDB document handles |
| `agency_schemas_draft` | In-progress schema versions managed by `SchemaManager` |
| `agency_schemas_published` | Published, versioned schema documents seeded on startup |

There is no `agency_details` collection (retired with the bespoke `Backend`).
There are no separate `agency_snapshots` or `agency_publications` collections —
those are `TypeID`s within `agency_entities`. Snapshots are written by
`PromoteDraft`, not by any lifecycle transition.

See [architecture-storage.md](../../2-SoftwareDesignAndArchitecture/architecture-storage.md)
for document shapes, indexes, and the named graph definition.

---

## MVP-AGENCY-003 — gRPC Service (AgencyService)

**Status**: 🔲 Not Started  
**Branch**: `feature/AGENCY-003_grpc_service`

### Goal

Generate proto stubs and implement the `AgencyService` gRPC handler in `internal/server/`.

### Files to Create/Modify

| File | Purpose |
|---|---|
| `proto/codevaldagency/v1/agency.proto` | RPC and message definitions |
| `internal/server/server.go` | Handler implementations |
| `internal/server/errors.go` | Domain error → gRPC status code mapping |
| `cmd/main.go` | Binary wiring |

### Proto Service

The full `AgencyService` definition (drafts + publications + convenience
accessors) is documented in
[architecture-flows.md §6](../../2-SoftwareDesignAndArchitecture/architecture-flows.md#6-grpc-service-definition).
The bootstrap-only RPCs introduced by this task are:

```protobuf
service AgencyService {
  // SetAgencyDetails replaces the full agency document from a JSON string.
  // Error: INVALID_ARGUMENT if the JSON is malformed or id is missing.
  // Error: FAILED_PRECONDITION (ErrAgencyReadOnly) once the live agency is published.
  rpc SetAgencyDetails(SetAgencyDetailsRequest) returns (Agency);

  // GetAgency retrieves the single live agency.
  // Error: NOT_FOUND if no agency entity exists.
  rpc GetAgency(GetAgencyRequest) returns (Agency);
}

message SetAgencyDetailsRequest {
  // json is the full agency document serialised as a JSON string.
  // Must include a non-empty "id" field.
  string json = 1;
}
```

### Error Mapping

| Domain Error | gRPC Code | Trigger |
|---|---|---|
| `ErrAgencyNotFound` | `NOT_FOUND` | `GetAgency` |
| `ErrAgencyReadOnly` | `FAILED_PRECONDITION` | `SetAgencyDetails` after first promotion |
| `ErrInvalidAgency` | `INVALID_ARGUMENT` | `SetAgencyDetails` (missing required field) |
| `ErrInvalidJSON` | `INVALID_ARGUMENT` | `SetAgencyDetails` (bad JSON) |

> Full error → gRPC code mapping for drafts, publications, and EntityService
> errors lives in
> [architecture-flows.md §8](../../2-SoftwareDesignAndArchitecture/architecture-flows.md#8-error-types).

### Acceptance Tests

- `SetAgencyDetails` RPC with valid JSON → returns populated `Agency`
- `SetAgencyDetails` RPC with invalid JSON → `INVALID_ARGUMENT`
- `SetAgencyDetails` RPC on a published agency → `FAILED_PRECONDITION`
- `GetAgency` RPC returns `NOT_FOUND` when no agency entity exists

---

## MVP-AGENCY-004 — CodeValdCross Registration

**Status**: 🔲 Not Started  
**Branch**: `feature/AGENCY-004_cross_registration`

### Goal

Register with CodeValdCross on startup and send periodic heartbeats. Publish
`cross.agency.created` after every successful `SetAgencyDetails`.

### Files to Create/Modify

| File | Purpose |
|---|---|
| `internal/registrar/registrar.go` | `Registrar` struct, `New`, `Run`, `Close`, `ping` |

### Topics Declared

| Direction | Topic |
|---|---|
| Produces | `cross.agency.created` |

### Acceptance Tests

- When `CROSS_GRPC_ADDR` is unset, server starts without error and skips registration
- When `CROSS_GRPC_ADDR` is set but unreachable, server continues running (non-fatal)
- Registrar sends heartbeat at configured interval
- `cross.agency.created` is published once per successful `SetAgencyDetails` call

---

## MVP-AGENCY-005 — Unit & Integration Tests

**Status**: 🔲 Not Started  
**Branch**: `feature/AGENCY-005_integration_tests`

### Goal

End-to-end tests covering the full gRPC + ArangoDB stack using a real ArangoDB
instance. Tests skip when `AGENCY_ARANGO_ENDPOINT` is not set.

### Test Matrix

- `SetAgencyDetails` with valid JSON → `GetAgency` returns same data
- `SetAgencyDetails` called twice (pre-promotion) → `GetAgency` returns latest data
- `SetAgencyDetails` with invalid JSON → `INVALID_ARGUMENT`
- `SetAgencyDetails` → `CreateDraft` → edit sub-graph → `PromoteDraft` → `GetAgency` returns the promoted state and `Enabled == true`
- `PromoteDraft` writes an `AgencySnapshot` entity into `agency_entities`
- `SetAgencyDetails` after a successful `PromoteDraft` → `FAILED_PRECONDITION` (`ErrAgencyReadOnly`)
- `PromoteDraft` on an already-promoted or archived draft → `FAILED_PRECONDITION` (`ErrDraftNotOpen`)
- `GetAgency` on empty database → `NOT_FOUND`


---

## MVP-AGENCY-007 — Agency Publishing & Version Tagging

**Status**: 🚀 In Progress
**Branch**: `feature/AGENCY-007_agency_publishing`

### Goal

Provide an explicit **publish** operation that records an immutable, versioned
snapshot of the agency. Publishing is orthogonal to whether the agency is
enabled and orthogonal to draft status — it is a separate audit trail of
versions that have been released to consumers.

### Key Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Coupling to drafts | A publish is optionally tagged with the `draftID` it captured | Lets `PublishAgency` short-circuit re-publishes via content hashing |
| Idempotency | `ContentHash` of the draft sub-graph; matches an existing publication ⇒ `ErrNoChangesDetected` | Prevents accidental duplicate releases of identical content |
| Version scheme | Auto-incrementing integer rendered as `"v1"`, `"v2"`, … | Simple, deterministic, human-readable |
| Immutability | The `AgencyPublication` entity is write-once | Audit integrity — every published version is permanent |
| Mutable status | A separate `AgencyPublicationStatus` entity, linked via `has_status` | Lets ops transition `draft → active → archived` without mutating the publication record |
| Storage | `agency_publications` entries in `agency_entities`, status in same collection linked via `agency_relationships` edge | Same entitygraph storage model as the rest of the schema |
| Event | `cross.agency.published` after every successful publish | Downstream services react to a new version |

### Model (shipped — see [models.go](../../../models.go))

```go
type AgencyPublication struct {
    ID          string
    AgencyID    string
    DraftID     string    // optional — links to the AgencyDraft that was captured
    ContentHash string    // SHA-256 of the draft's sub-graph; "" when no draftID
    Version     int       // 1, 2, 3, …
    Tag         string    // "v1", "v2", …
    PublishedAt time.Time
    Status      string    // "draft" | "active" | "archived" — sourced from AgencyPublicationStatus
}
```

### AgencyManager Methods (shipped)

```go
// PublishAgency captures the current agency state into a new immutable
// publication. When draftID is non-empty, the draft's content hash is
// compared against existing publications and ErrNoChangesDetected is
// returned on a match.
PublishAgency(ctx context.Context, draftID string) (AgencyPublication, error)

GetPublication(ctx context.Context, version int) (AgencyPublication, error)
ListPublications(ctx context.Context) ([]AgencyPublication, error)

// UpdatePublicationStatus transitions the linked AgencyPublicationStatus.
// Allowed: draft → active, active → archived. archived is terminal.
UpdatePublicationStatus(ctx context.Context, version int, status string) (AgencyPublication, error)
```

### Errors

```go
ErrPublicationNotFound       // GetPublication / UpdatePublicationStatus on unknown version
ErrInvalidPublicationStatus  // UpdatePublicationStatus with disallowed transition
ErrNoChangesDetected         // PublishAgency when draft content hash matches an existing publication
```

### Proto Additions (shipped)

```protobuf
rpc PublishAgency           (PublishAgencyRequest)           returns (AgencyPublication);
rpc GetPublication          (GetPublicationRequest)          returns (AgencyPublication);
rpc ListPublications        (ListPublicationsRequest)        returns (ListPublicationsResponse);
rpc UpdatePublicationStatus (UpdatePublicationStatusRequest) returns (AgencyPublication);

message PublishAgencyRequest {
  // Optional. When set, the publication is linked to this draft and the
  // draft's content hash is checked against existing publications.
  string draft_id = 1;
}

message UpdatePublicationStatusRequest {
  int32  version = 1;
  string status  = 2; // "active" | "archived"
}
```

### Cross Route Declarations

Listed in full in
[architecture-flows.md §7](../../2-SoftwareDesignAndArchitecture/architecture-flows.md#7-codevaldcross-registration).
The publication-specific routes:

| Method | Pattern | gRPC Method |
|---|---|---|
| `POST` | `/agency/{agencyId}/publish` | `AgencyService/PublishAgency` |
| `GET`  | `/agency/{agencyId}/publications` | `AgencyService/ListPublications` |
| `GET`  | `/agency/{agencyId}/publications/{version}` | `AgencyService/GetPublication` |
| `PUT`  | `/agency/{agencyId}/publications/{version}/status` | `AgencyService/UpdatePublicationStatus` |

### Cross Pub/Sub

| Direction | Topic | Trigger |
|---|---|---|
| Produces | `cross.agency.published` | After every successful `PublishAgency` |

### Error Mapping

| Domain Error | gRPC Code | Trigger |
|---|---|---|
| `ErrAgencyNotFound` | `NOT_FOUND` | `PublishAgency` (no live agency yet) |
| `ErrPublicationNotFound` | `NOT_FOUND` | `GetPublication`, `UpdatePublicationStatus` |
| `ErrDraftNotFound` | `NOT_FOUND` | `PublishAgency` with unknown `draftID` |
| `ErrNoChangesDetected` | `FAILED_PRECONDITION` | `PublishAgency` re-using an already-published draft hash |
| `ErrInvalidPublicationStatus` | `FAILED_PRECONDITION` | `UpdatePublicationStatus` with disallowed transition |

### Acceptance Tests

- `PublishAgency` with no live agency → `ErrAgencyNotFound`
- `PublishAgency()` once → `Version=1`, `Tag="v1"`, `Status="draft"`
- `PublishAgency()` twice → second publication has `Version=2`, `Tag="v2"`
- `PublishAgency(draftID)` records the `DraftID` and a non-empty `ContentHash`
- `PublishAgency(draftID)` re-using an unchanged draft → `ErrNoChangesDetected`
- `GetPublication(1)` after two publishes returns the first snapshot, unchanged
- `GetPublication` for an unknown version → `ErrPublicationNotFound`
- `ListPublications` returns publications in ascending version order
- `UpdatePublicationStatus(v, "active")` from `draft` succeeds; from `archived` → `ErrInvalidPublicationStatus`
- `UpdatePublicationStatus(v, "archived")` from `active` succeeds; from `draft` → `ErrInvalidPublicationStatus`
- `cross.agency.published` is published once per successful `PublishAgency` call
- `POST /agency/{agencyId}/publish` proxied through CodeValdCross returns 200 with `AgencyPublication` JSON
