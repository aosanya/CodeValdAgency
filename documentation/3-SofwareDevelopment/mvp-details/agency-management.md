# Agency Management — Implementation Details

## Design Decision: Single-Agency Database

Each CodeValdAgency database instance holds **exactly one agency**. There is no
listing, no multi-tenancy, and no deletion. The `agency_details` collection always
contains a single document keyed by the agency's own ID.

`SetAgencyDetails` is the authoritative write path. It accepts a full JSON
representation of the agency, validates structure, and upserts the document.
`UpdateAgency` remains for incremental field edits with lifecycle-guarded transitions.

---

## MVP-AGENCY-001 — Library Scaffolding & Agency Model

**Status**: 🔲 Not Started  
**Branch**: `feature/AGENCY-001_library_scaffolding`

### Goal

Scaffold the Go module with the `AgencyManager` interface, `Agency` domain type,
lifecycle enforcement, and exported errors.

### Files to Create/Modify

| File | Purpose |
|---|---|
| `go.mod` | Module declaration (`github.com/aosanya/CodeValdAgency`) |
| `agency.go` | `AgencyManager` interface, `Backend` interface, `agencyManager` implementation |
| `models.go` | `Agency`, `Goal`, `Workflow`, `WorkItem`, `RoleAssignment`, `AgencyRole`, `RACILabel`, `AgencyLifecycle`, `AgencySnapshot`, `UpdateAgencyRequest` |
| `errors.go` | `ErrAgencyNotFound`, `ErrInvalidLifecycleTransition`, `ErrInvalidAgency`, `ErrInvalidJSON` |

### AgencyManager Interface

```go
type AgencyManager interface {
    // SetAgencyDetails replaces the full agency document from raw JSON.
    // Returns ErrInvalidJSON (→ INVALID_ARGUMENT) if the payload cannot be
    // parsed or if the id field is missing. Lifecycle validation is NOT applied.
    // Publishes cross.agency.created after every successful write.
    SetAgencyDetails(ctx context.Context, jsonStr string) (Agency, error)

    // GetAgency retrieves the single agency by its ID.
    // Returns ErrAgencyNotFound if no agency document exists yet.
    GetAgency(ctx context.Context, agencyID string) (Agency, error)

    // UpdateAgency applies incremental field edits with lifecycle validation.
    // Returns ErrInvalidLifecycleTransition on invalid status change.
    // Returns ErrAgencyNotFound if the agency does not exist.
    UpdateAgency(ctx context.Context, agencyID string, req UpdateAgencyRequest) (Agency, error)
}
```

### Lifecycle Transition Rules (UpdateAgency only)

| From | To | Allowed |
|---|---|---|
| `draft` | `active` | ✅ |
| `active` | `achieved` | ✅ |
| `active` | `draft` | ❌ |
| `achieved` | anything | ❌ (terminal) |

> `SetAgencyDetails` bypasses lifecycle validation entirely — any status value
> in the JSON is written as-is.

### Acceptance Tests

- `SetAgencyDetails` with invalid JSON returns `ErrInvalidJSON`
- `SetAgencyDetails` with missing `id` field returns `ErrInvalidJSON`
- `SetAgencyDetails` with valid JSON returns the stored agency
- `GetAgency` after `SetAgencyDetails` returns matching data
- `SetAgencyDetails` called twice replaces the document
- `UpdateAgency` with `active → draft` returns `ErrInvalidLifecycleTransition`
- `UpdateAgency` with `draft → active` succeeds and triggers snapshot write
- `UpdateAgency` on an `achieved` agency returns `ErrInvalidLifecycleTransition`
- `NewAgencyManager(nil)` returns an error

---

## MVP-AGENCY-002 — ArangoDB Backend

**Status**: 🔲 Not Started  
**Branch**: `feature/AGENCY-002_arangodb_backend`

### Goal

Implement `codevaldagency.Backend` backed by ArangoDB. The single agency document
is stored in the `agency_details` collection, keyed by agency ID. Activation
snapshots are written to `agency_snapshots`.

### Files to Create/Modify

| File | Purpose |
|---|---|
| `storage/arangodb/storage.go` | `Backend` implementing `codevaldagency.Backend` |
| `storage/arangodb/storage_test.go` | Integration tests (skip when `AGENCY_ARANGO_ENDPOINT` not set) |

### Backend Interface

```go
type Backend interface {
    // SetDetails parses the raw JSON and upserts the agency document at
    // _key = agency.id in the agency_details collection.
    // Returns ErrInvalidJSON if the JSON is malformed or id is missing.
    SetDetails(ctx context.Context, jsonStr string) (Agency, error)

    // Get retrieves the agency document by its ID.
    // Returns ErrAgencyNotFound if the document does not exist.
    Get(ctx context.Context, agencyID string) (Agency, error)

    // Update applies a partial field merge and returns the updated agency.
    // Returns ErrAgencyNotFound if the document does not exist.
    Update(ctx context.Context, agencyID string, req UpdateAgencyRequest) (Agency, error)

    // InsertSnapshot writes an immutable activation snapshot to agency_snapshots.
    InsertSnapshot(ctx context.Context, snapshot AgencySnapshot) error
}
```

### Collections

| Collection | Purpose |
|---|---|
| `agency_details` | Single agency document (keyed by agency ID) |
| `agency_snapshots` | Immutable activation snapshots |

### Key Behaviours

- `SetDetails` upserts using `_key = agency.id` — creates on first call, replaces on subsequent calls
- `Get` returns `ErrAgencyNotFound` if key is missing
- `Update` — partial field merge with refreshed `updated_at`
- `InsertSnapshot` writes to `agency_snapshots` on `draft → active` transition

### Acceptance Tests

- `SetDetails` with valid JSON → document upserted; `Get` returns same data
- `SetDetails` with invalid JSON → `ErrInvalidJSON`
- `SetDetails` called twice → second call replaces document; `Get` returns latest
- `Get` on non-existent agency → `ErrAgencyNotFound`
- `Update` on non-existent agency → `ErrAgencyNotFound`
- `InsertSnapshot` → snapshot is retrievable with the same agency ID

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

```protobuf
service AgencyService {
  // SetAgencyDetails replaces the full agency document from a JSON string.
  // Error: INVALID_ARGUMENT if the JSON is malformed or id is missing.
  rpc SetAgencyDetails(SetAgencyDetailsRequest) returns (Agency);

  // GetAgency retrieves the single agency by its ID.
  // Error: NOT_FOUND if no agency document exists.
  rpc GetAgency(GetAgencyRequest) returns (Agency);

  // UpdateAgency applies incremental field edits with lifecycle validation.
  // Error: FAILED_PRECONDITION on invalid lifecycle transition.
  // Error: NOT_FOUND if the agency does not exist.
  rpc UpdateAgency(UpdateAgencyRequest) returns (Agency);
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
| `ErrAgencyNotFound` | `NOT_FOUND` | `GetAgency`, `UpdateAgency` |
| `ErrInvalidLifecycleTransition` | `FAILED_PRECONDITION` | `UpdateAgency` |
| `ErrInvalidAgency` | `INVALID_ARGUMENT` | `UpdateAgency` (empty name) |
| `ErrInvalidJSON` | `INVALID_ARGUMENT` | `SetAgencyDetails` (bad JSON) |

### Acceptance Tests

- `SetAgencyDetails` RPC with valid JSON → returns populated `Agency`
- `SetAgencyDetails` RPC with invalid JSON → `INVALID_ARGUMENT`
- `UpdateAgency` RPC returns `FAILED_PRECONDITION` on invalid lifecycle transition
- `GetAgency` RPC returns `NOT_FOUND` for unknown agency

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
- `SetAgencyDetails` called twice → `GetAgency` returns latest data
- `SetAgencyDetails` with invalid JSON → `INVALID_ARGUMENT`
- `SetAgencyDetails` → `UpdateAgency` (valid lifecycle transition `draft → active`)
- `SetAgencyDetails` → `UpdateAgency` (invalid transition → `FAILED_PRECONDITION`)
- `SetAgencyDetails` → `UpdateAgency` `draft → active` → `active → achieved`
  (terminal; subsequent update → `FAILED_PRECONDITION`)
- `GetAgency` on empty database → `NOT_FOUND`
- `draft → active` transition via `UpdateAgency` → snapshot exists in `agency_snapshots`


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
