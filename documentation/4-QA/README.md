# CodeValdAgency — QA & Testing

## Overview

The service has **89 Go tests** spread across four files. Tests fall into three
tiers; choose the right tier for the behaviour under verification.

| Tier | File | Count | Lines | Dependencies |
|---|---|---|---|---|
| Unit (manager) | [`agency_test.go`](../../agency_test.go) | 40 | 996 | None — `fakeDataManager` |
| Unit (handlers) | [`internal/server/server_test.go`](../../internal/server/server_test.go) | 31 | 564 | None — `fakeDataManager` |
| Integration (storage) | [`storage/arangodb/storage_test.go`](../../storage/arangodb/storage_test.go) | 15 | 605 | Real ArangoDB |
| Integration (gRPC + ArangoDB) | [`internal/server/integration_test.go`](../../internal/server/integration_test.go) | 3 | 211 | Real ArangoDB + in-process gRPC server |

Run them all with `go test -race ./...` from the repo root.

---

## 1. Tier Selection

Use this decision tree when adding a test:

```
Does the behaviour live in agencyManager (drafts, publish, validation)?
  └─► agency_test.go (fakeDataManager)

Does it live in the gRPC handler layer (proto ↔ domain conversion, error
mapping, request validation before reaching the manager)?
  └─► internal/server/server_test.go (fakeDataManager)

Does the test exercise an AQL query, edge collection, immutable-type guard,
or schema-collection round-trip?
  └─► storage/arangodb/storage_test.go (real ArangoDB)

Does it span the full stack (gRPC client → handler → manager → ArangoDB) or
verify the registrar/heartbeat behaviour end-to-end?
  └─► internal/server/integration_test.go (real ArangoDB)
```

If a test fits in two places, prefer the smaller one (faster, no Arango
dependency). Reach for integration tests only when the bug couldn't have been
caught at the unit tier.

---

## 2. The `fakeDataManager`

Defined in [`agency_test.go`](../../agency_test.go) and re-used by
`internal/server/server_test.go`. It implements every method on
`entitygraph.DataManager` against an in-memory map keyed by entity ID:

- `entities[id] entitygraph.Entity`
- `relationships[id] entitygraph.Relationship`
- `nextID()` — monotonic counter

What it deliberately **does not** simulate:

- Schema validation (no required-field checks, no cardinality enforcement)
- Immutable-type guards (`UpdateEntity` on an `AgencySnapshot` succeeds in the fake; the real ArangoDB backend rejects it)
- Edge cycles, traversal queries, or AQL-specific behaviour (`TraverseGraph` returns an empty result)

When a test needs any of those, it must move to the integration tier.

---

## 3. Integration Test Gate

Both integration files start with the same skip pattern:

```go
endpoint := os.Getenv("AGENCY_ARANGO_ENDPOINT")
if endpoint == "" {
    t.Skip("AGENCY_ARANGO_ENDPOINT not set; skipping integration test")
}
```

Effect:

- `go test ./...` on a developer laptop with no ArangoDB exits 0 — only the
  unit tests ran.
- CI sets `AGENCY_ARANGO_ENDPOINT` (and friends) and the same `go test ./...`
  invocation now exercises 18 ArangoDB-backed tests on top of the 71 unit
  tests.
- See [architecture-configuration.md §2](../2-SoftwareDesignAndArchitecture/architecture-configuration.md#2-integration-test-only-variables)
  for the full env-var list.

Each integration test creates a uniquely-named scratch database and tears it
down on completion, so parallel runs and re-runs do not collide.

---

## 4. Coverage Map

### Manager (`agencyManager`)

| Behaviour | Covered in |
|---|---|
| `SetAgencyDetails` — parse, upsert, `ErrInvalidJSON`, `ErrAgencyReadOnly` | `agency_test.go` |
| `GetAgency` / `GetGoals` / `GetWorkflows` / `GetConfiguredRoles` | `agency_test.go` |
| All six draft methods (`CreateDraft`/`GetDraft`/`ListDrafts`/`UpdateDraftDescription`/`PromoteDraft`/`ArchiveDraft`) | `agency_test.go` |
| `PromoteDraft` deep-copy + snapshot side-effect | `agency_test.go` |
| `PublishAgency` version increment, content-hash idempotency, `ErrNoChangesDetected` | `agency_test.go` |
| `UpdatePublicationStatus` allowed/disallowed transitions | `agency_test.go` |

### gRPC handlers (`internal/server/server.go`)

| Behaviour | Covered in |
|---|---|
| Domain error → gRPC code mapping | `server_test.go` |
| Proto ↔ domain field conversion (drafts, publications, statuses) | `server_test.go` |
| Request validation before manager call | `server_test.go` |

### Storage (`storage/arangodb/storage.go`)

| Behaviour | Covered in |
|---|---|
| `ensureCollection` + `ensureGraph` idempotency | `storage_test.go` |
| Entity CRUD with immutable-type rejection | `storage_test.go` |
| Edge `_from` / `_to` integrity, `ListRelationships` filters | `storage_test.go` |
| Schema versioning (`SetSchema` auto-increment, `ListSchemaVersions`) | `storage_test.go` |
| Soft-delete preservation of created/updated timestamps | `storage_test.go` |

### End-to-end

| Behaviour | Covered in |
|---|---|
| gRPC client → handler → manager → ArangoDB happy path for `SetAgencyDetails` + `GetAgency` | `integration_test.go` |
| Cross registrar startup with `CROSS_GRPC_ADDR` unset (no panic, no goroutine leak) | `integration_test.go` |

---

## 5. Acceptance Criteria

The current API surface — and therefore the acceptance bar for a green PR:

| Scenario | Expected |
|---|---|
| `SetAgencyDetails` with valid JSON | Returns the stored `Agency`; publishes `cross.agency.created` |
| `SetAgencyDetails` with malformed JSON | `INVALID_ARGUMENT` (`ErrInvalidJSON`) |
| `SetAgencyDetails` after a successful `PromoteDraft` | `FAILED_PRECONDITION` (`ErrAgencyReadOnly`) |
| `GetAgency` on empty database | `NOT_FOUND` (`ErrAgencyNotFound`) |
| `CreateDraft` from live or open draft | Returns new `AgencyDraft` with `Status = "open"` |
| `CreateDraft` from promoted/archived draft | `FAILED_PRECONDITION` (`ErrDraftNotOpen`) |
| `PromoteDraft` | Replaces live sub-graph; sets `Agency.Enabled = true`; writes `AgencySnapshot`; publishes `cross.agency.promoted` |
| `PublishAgency(draftID)` re-using an unchanged draft | `FAILED_PRECONDITION` (`ErrNoChangesDetected`) |
| `UpdatePublicationStatus(v, "active")` from `draft` | Succeeds; the immutable `AgencyPublication` entity is **not** mutated; the linked `AgencyPublicationStatus` entity flips |
| Cross registration with `CROSS_GRPC_ADDR` unreachable | Service stays up; loop retries silently |

---

## 6. Related Documentation

| Section | Link |
|---|---|
| Requirements | [../1-SoftwareRequirements/README.md](../1-SoftwareRequirements/README.md) |
| Architecture | [../2-SoftwareDesignAndArchitecture/README.md](../2-SoftwareDesignAndArchitecture/README.md) |
| Configuration | [../2-SoftwareDesignAndArchitecture/architecture-configuration.md](../2-SoftwareDesignAndArchitecture/architecture-configuration.md) |
| Lifecycle, flows & errors | [../2-SoftwareDesignAndArchitecture/architecture-flows.md](../2-SoftwareDesignAndArchitecture/architecture-flows.md) |
| Development | [../3-SofwareDevelopment/README.md](../3-SofwareDevelopment/README.md) |
