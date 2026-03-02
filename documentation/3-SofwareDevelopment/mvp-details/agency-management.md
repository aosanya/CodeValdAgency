# Agency Management — Implementation Details

## MVP-AGENCY-001 — Library Scaffolding & Agency Model

**Status**: 🔲 Not Started  
**Branch**: `feature/AGENCY-001_library_scaffolding`

### Goal

Scaffold the Go module with the `AgencyManager` interface, `Agency` domain type, lifecycle enforcement, and exported errors.

### Files to Create/Modify

| File | Purpose |
|---|---|
| `go.mod` | Module declaration (`github.com/aosanya/CodeValdAgency`) |
| `agency.go` | `AgencyManager` interface, `Backend` interface, `agencyManager` implementation |
| `models.go` | `Agency`, `Goal`, `Workflow`, `WorkItem`, `RoleAssignment`, `AgencyRole`, `RACILabel`, `AgencyLifecycle`, `AgencySnapshot`, request/filter types |
| `errors.go` | `ErrAgencyNotFound`, `ErrAgencyAlreadyExists`, `ErrInvalidLifecycleTransition`, `ErrInvalidAgency` |

### Lifecycle Transition Rules

| From | To | Allowed |
|---|---|---|
| `draft` | `active` | ✅ |
| `active` | `achieved` | ✅ |
| `active` | `draft` | ❌ |
| `achieved` | anything | ❌ (terminal) |

### Acceptance Tests

- `CreateAgency` with empty `Name` returns `ErrInvalidAgency`
- `UpdateAgency` with `active → draft` returns `ErrInvalidLifecycleTransition`
- `UpdateAgency` with `draft → active` succeeds and triggers snapshot write
- `UpdateAgency` on an `achieved` agency returns `ErrInvalidLifecycleTransition`
- `NewAgencyManager(nil)` returns an error

---

## MVP-AGENCY-002 — ArangoDB Backend

**Status**: 🔲 Not Started  
**Branch**: `feature/AGENCY-002_arangodb_backend`

### Goal

Implement `codevaldagency.Backend` backed by ArangoDB. Agencies are stored in the `agencies` collection, keyed by agency ID. Activation snapshots are written to `agency_snapshots`.

### Files to Create/Modify

| File | Purpose |
|---|---|
| `storage/arangodb/storage.go` | `ArangoBackend` implementing `codevaldagency.Backend` |
| `storage/arangodb/storage_test.go` | Integration tests (skip when `AGENCY_ARANGO_ENDPOINT` not set) |

### Key Behaviours

- `Insert` with `_key = agency.ID` — returns `ErrAgencyAlreadyExists` on conflict
- `Get` returns `ErrAgencyNotFound` if key missing
- `Update` — full document replace with refreshed `updated_at`; validates lifecycle transition before write
- `Delete` after existence check
- `List` with optional `lifecycle` and `name` filters via AQL
- `InsertSnapshot` writes to `agency_snapshots` collection on `draft → active` transition

### Acceptance Tests

- Create an agency and read it back — all fields match
- Create two agencies and list both
- Delete an agency — subsequent `GetAgency` returns `ErrAgencyNotFound`
- Get a non-existent agency — returns `ErrAgencyNotFound`
- `InsertSnapshot` on `draft → active` — snapshot is retrievable with the same agency ID

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

### Error Mapping

| Domain Error | gRPC Code |
|---|---|
| `ErrAgencyNotFound` | `NOT_FOUND` |
| `ErrAgencyAlreadyExists` | `ALREADY_EXISTS` |
| `ErrInvalidLifecycleTransition` | `FAILED_PRECONDITION` |
| `ErrInvalidAgency` | `INVALID_ARGUMENT` |

### Acceptance Tests

- `CreateAgency` RPC returns `ALREADY_EXISTS` when agency ID is duplicate
- `UpdateAgency` RPC returns `FAILED_PRECONDITION` on invalid lifecycle transition
- `DeleteAgency` RPC returns `NOT_FOUND` for unknown agency
- `ListAgencies` RPC with lifecycle filter only returns matching agencies

---

## MVP-AGENCY-004 — CodeValdCross Registration

**Status**: 🔲 Not Started  
**Branch**: `feature/AGENCY-004_cross_registration`

### Goal

Register with CodeValdCross on startup and send periodic heartbeats. Publish `cross.agency.created` after every successful `CreateAgency`.

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
- `cross.agency.created` is published once per successful `CreateAgency` call

---

## MVP-AGENCY-005 — Unit & Integration Tests

**Status**: 🔲 Not Started  
**Branch**: `feature/AGENCY-005_integration_tests`

### Goal

End-to-end tests covering the full gRPC + ArangoDB stack using a real ArangoDB instance. Tests skip when `AGENCY_ARANGO_ENDPOINT` is not set.

### Test Matrix

- Create → Get round-trip
- Create → Update (valid lifecycle transition `draft → active`)
- Create → Update (invalid transition → `FAILED_PRECONDITION`)
- Create → Update `active → achieved` (terminal; subsequent update → `FAILED_PRECONDITION`)
- Create → Delete → Get (`NOT_FOUND`)
- Create multiple → List with lifecycle filter
- Create with duplicate ID → `ALREADY_EXISTS`
- `draft → active` transition → snapshot exists in `agency_snapshots`
