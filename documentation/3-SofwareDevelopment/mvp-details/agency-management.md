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
`storage/arangodb/storage.go` provides the ArangoDB implementation of that
interface. The agency-specific collections used today are:

| Collection | Purpose |
|---|---|
| `agency_entities` | All live entities (Agency, Goal, Workflow, WorkItem, ConfiguredRole, Instruction, Deliverable) |
| `agency_draft_entities` | Draft copies of the same types, scoped by `draft_id` |
| `agency_drafts` | `AgencyDraft` root entities |
| `agency_relationships` | Edge collection — spans both vertex collections |
| `agency_schemas` | Pre-delivered schema versions seeded on startup |
| `agency_snapshots` | Immutable promotion snapshots (written on `PromoteDraft`) |
| `agency_publications` | Immutable versioned publications |

There is no `agency_details` collection. Snapshots are written by
`PromoteDraft`, not by any lifecycle transition.

See [architecture-storage.md](../../2-SoftwareDesignAndArchitecture/architecture-storage.md)
for the canonical schema layout.

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
- `PromoteDraft` writes a row to `agency_snapshots`
- `SetAgencyDetails` after a successful `PromoteDraft` → `FAILED_PRECONDITION` (`ErrAgencyReadOnly`)
- `PromoteDraft` on an already-promoted or archived draft → `FAILED_PRECONDITION` (`ErrDraftNotOpen`)
- `GetAgency` on empty database → `NOT_FOUND`


---

## MVP-AGENCY-007 — Agency Publishing & Version Tagging

**Status**: 🔲 Not Started  
**Branch**: `feature/AGENCY-007_agency_publishing`

### Goal

Introduce an explicit **publish** operation that takes a point-in-time snapshot of
the current agency and tags it with an auto-incrementing version number (`v1`, `v2`, …).

Publishing is **entirely independent of the agency lifecycle**. The agency always
remains in `draft` and can be freely edited before and after any publish. There is
no `active` or `achieved` transition involved. The only thing a publish does is
capture and version the current state.

### Key Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Agency status | Always stays `draft` | Lifecycle transitions are not the publish mechanism |
| Version scheme | Auto-incrementing integer rendered as `"v1"`, `"v2"`, … | Simple, deterministic, human-readable |
| Immutability | Publications are write-once; no update or delete | Audit integrity — every published version is permanent |
| Storage | New `agency_publications` collection in ArangoDB | Separate from `agency_snapshots` (which is lifecycle-audit only) |
| Event | Publishes `cross.agency.published` after every successful publish | Allows downstream services to react to a new version |
| Version resolution | Backend reads MAX(version) for the agency and increments atomically | Prevents version gaps or duplicates under concurrent calls |

### New Model

```go
// AgencyPublication is an immutable, versioned snapshot of an [Agency]
// created by an explicit publish action. The agency status is not changed.
// Publications are written once and never updated or deleted.
type AgencyPublication struct {
    // ID is the unique identifier for this publication (UUID).
    ID string

    // AgencyID is the ID of the agency this publication belongs to.
    AgencyID string

    // Version is the auto-incrementing publication number (1, 2, 3, …).
    Version int

    // Tag is the human-readable version label, e.g. "v1", "v2".
    Tag string

    Name            string
    Mission         string
    Vision          string
    Goals           []Goal
    Workflows       []Workflow
    ConfiguredRoles []ConfiguredRole

    // PublishedAt is the exact time this publication was created.
    PublishedAt time.Time
}
```

### AgencyManager Interface Addition

```go
type AgencyManager interface {
    // ... existing methods ...

    // PublishAgency creates an immutable versioned publication of the current
    // agency state. The agency status is NOT changed — it always remains draft.
    // Version is auto-incremented from the last publication for this agency
    // (or starts at 1 if no prior publication exists).
    // Publishes "cross.agency.published" after every successful write.
    PublishAgency(ctx context.Context) (AgencyPublication, error)

    // GetPublication retrieves a single publication by its version number.
    // Returns ErrPublicationNotFound if no publication with that version exists.
    GetPublication(ctx context.Context, version int) (AgencyPublication, error)

    // ListPublications returns all publications for this agency in ascending
    // version order.
    ListPublications(ctx context.Context) ([]AgencyPublication, error)
}
```

### New Error

```go
// ErrPublicationNotFound is returned when the requested agency publication
// does not exist.
var ErrPublicationNotFound = errors.New("agency publication not found")
```

### Backend Interface Addition

```go
type Backend interface {
    // ... existing methods ...

    // InsertPublication writes a new AgencyPublication to the
    // agency_publications collection.
    InsertPublication(ctx context.Context, pub AgencyPublication) error

    // GetPublication retrieves a publication by its version number.
    // Returns ErrPublicationNotFound if no match exists.
    GetPublication(ctx context.Context, version int) (AgencyPublication, error)

    // ListPublications returns all publications in ascending version order.
    ListPublications(ctx context.Context) ([]AgencyPublication, error)

    // NextPublicationVersion returns the next auto-increment version number
    // (MAX(version) + 1, or 1 if no publications exist).
    NextPublicationVersion(ctx context.Context) (int, error)
}
```

### ArangoDB Collection

| Collection | Key Pattern | Purpose |
|---|---|---|
| `agency_publications` | `{agencyID}_v{version}` | Immutable versioned snapshots |

**Indexes**: persistent index on `(agency_id, version)` with `unique: true`.

### Proto Additions

```protobuf
// PublishAgency creates an immutable versioned publication of the current
// agency state. The agency status is NOT changed.
rpc PublishAgency(PublishAgencyRequest) returns (AgencyPublication);

// GetPublication retrieves a single publication by version number.
// Error: NOT_FOUND if no publication with that version exists.
rpc GetPublication(GetPublicationRequest) returns (AgencyPublication);

// ListPublications returns all publications in ascending version order.
rpc ListPublications(ListPublicationsRequest) returns (ListPublicationsResponse);

message PublishAgencyRequest {}

message GetPublicationRequest {
  int32 version = 1;
}

message ListPublicationsRequest {}

message ListPublicationsResponse {
  repeated AgencyPublication publications = 1;
}

message AgencyPublication {
  string id          = 1;
  string agency_id   = 2;
  int32  version     = 3;
  string tag         = 4; // e.g. "v1", "v2"
  string name        = 5;
  string mission     = 6;
  string vision      = 7;
  repeated Goal              goals            = 8;
  repeated Workflow          workflows        = 9;
  repeated ConfiguredRole    configured_roles = 10;
  google.protobuf.Timestamp  published_at     = 11;
}
```

### Cross Route Declarations

Declared in `internal/registrar/registrar.go` alongside the existing agency routes:

| Method | Pattern | Capability | gRPC Method |
|---|---|---|---|
| `POST` | `/agency/publish` | `publish_agency` | `AgencyService/PublishAgency` |
| `GET` | `/agency/publications` | `list_publications` | `AgencyService/ListPublications` |
| `GET` | `/agency/publications/{version}` | `get_publication` | `AgencyService/GetPublication` |

**PathBindings**: `{version}` → gRPC field `version`.

### Cross Pub/Sub

| Direction | Topic | Trigger |
|---|---|---|
| Produces | `cross.agency.published` | After every successful `PublishAgency` |

### Files to Create/Modify

| File | Change |
|---|---|
| `models.go` | Add `AgencyPublication` struct |
| `errors.go` | Add `ErrPublicationNotFound` |
| `agency.go` | Add `PublishAgency`, `GetPublication`, `ListPublications` to `AgencyManager`; add `InsertPublication`, `GetPublication`, `ListPublications`, `NextPublicationVersion` to `Backend` |
| `proto/codevaldagency/v1/agency.proto` | Add three new RPCs and `AgencyPublication` message |
| `internal/server/server.go` | Implement the three new RPC handlers |
| `storage/arangodb/storage.go` | Implement the four new Backend methods; create `agency_publications` collection |
| `internal/registrar/registrar.go` | Add three new route declarations |

### Error Mapping

| Domain Error | gRPC Code | Trigger |
|---|---|---|
| `ErrPublicationNotFound` | `NOT_FOUND` | `GetPublication` |
| `ErrAgencyNotFound` | `NOT_FOUND` | `PublishAgency` (agency must exist first) |

### Acceptance Tests

- `PublishAgency` on a non-existent agency → `ErrAgencyNotFound`
- `PublishAgency` called once → returns `AgencyPublication` with `Version=1`, `Tag="v1"`
- `PublishAgency` called twice → second publication has `Version=2`, `Tag="v2"`
- `PublishAgency` does NOT change the agency `Status` field — it remains `draft`
- Agency can be edited (`SetAgencyDetails` / `UpdateAgency`) after a publish — the old publication is unchanged
- `GetPublication(version=1)` after two publishes → returns the first (older) snapshot, not the current state
- `GetPublication` for non-existent version → `ErrPublicationNotFound`
- `ListPublications` → returns publications in ascending version order
- `cross.agency.published` is published once per successful `PublishAgency` call
- `POST /agency/publish` proxied through CodeValdCross → 200 with `AgencyPublication` JSON
