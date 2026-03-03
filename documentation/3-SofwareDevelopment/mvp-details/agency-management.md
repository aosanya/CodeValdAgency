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
