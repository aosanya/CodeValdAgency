# MVP — CodeValdAgency

## Goal

Deliver a production-ready agency lifecycle management gRPC microservice with ArangoDB persistence and CodeValdCross registration.

---

## MVP Scope

The MVP delivers:
1. The `AgencyManager` Go interface (convenience-method facade over `entitygraph.DataManager`) and its `agencyManager` implementation
2. The `Agency` domain model with lifecycle enforcement (`draft → active → achieved`)
3. An ArangoDB `entitygraph.DataManager` + `entitygraph.SchemaManager` implementation with `agency_` prefixed collections
4. A pre-delivered agency schema (TypeDefinitions for Agency, Goal, Workflow, WorkItem, ConfiguredRole, AgencySnapshot, AgencyPublication) seeded on first use
5. An `AgencyService` gRPC service exposing all CRUD + convenience operations
6. CodeValdCross heartbeat registration and `cross.agency.created` / `cross.agency.published` event publishing
7. Integration tests for all gRPC operations

---

## Task List

| Task ID | Title | Status | Depends On |
|---|---|---|---|
| MVP-AGENCY-001 | Library Scaffolding & Agency Model | ✅ Done | — |
| MVP-AGENCY-002 | ArangoDB Backend | ✅ Done | MVP-AGENCY-001 |
| MVP-AGENCY-003 | gRPC Service (AgencyService) | ✅ Done | MVP-AGENCY-001 |
| MVP-AGENCY-004 | CodeValdCross Registration | ✅ Done | MVP-AGENCY-003 |
| MVP-AGENCY-005 | Unit & Integration Tests | ✅ Done | MVP-AGENCY-001, MVP-AGENCY-002 |
| ~~MVP-AGENCY-006~~ ✅ | Service-Driven Route Registration | ✅ Done | MVP-AGENCY-003, CROSS-007 |
| MVP-AGENCY-007 | Agency Publishing & Version Tagging | 🚀 In Progress | MVP-AGENCY-003, ~~MVP-AGENCY-006~~ ✅ |
| MVP-AGENCY-008 | EntityGraph Refactor | 🚀 In Progress | ~~SHAREDLIB-010~~ (SharedLib branch) |

---

## Success Criteria

- [ ] `go build ./...` succeeds
- [ ] `go test -race ./...` all pass
- [ ] `go vet ./...` shows 0 issues
- [ ] All `AgencyService` RPCs work end-to-end with ArangoDB
- [ ] CodeValdCross registration fires on startup and repeats on heartbeat
- [ ] `draft → active` transition writes a snapshot to `agency_snapshots`
- [ ] Invalid lifecycle transitions return `FAILED_PRECONDITION` from gRPC
- [ ] `cross.agency.created` is published after every successful `SetAgencyDetails`
- [ ] Routes declared in `RegisterRequest` and proxied via CodeValdCross dynamic proxy
- [ ] `PublishAgency` creates an immutable versioned publication (`v1`, `v2`, …) without touching agency status
- [ ] `cross.agency.published` is fired after every successful publish
- [ ] `POST /agency/publish` is proxied through CodeValdCross
- [ ] `AgencyManager` is a convenience facade over `entitygraph.DataManager` — no bespoke `Backend` interface
- [ ] Storage uses `agency_entities`, `agency_relationships` (edge), `agency_schemas`, `agency_snapshots`, `agency_publications` collections
- [ ] Pre-delivered schema is seeded into `agency_schemas` on first use (idempotent)
- [ ] `GetWorkflows`, `GetGoals`, `GetConfiguredRoles` convenience methods work end-to-end

---

## Branch Naming

```
feature/AGENCY-001_library_scaffolding
feature/AGENCY-002_arangodb_backend
feature/AGENCY-003_grpc_service
feature/AGENCY-004_cross_registration
feature/AGENCY-005_integration_tests
feature/AGENCY-006_service_driven_route_registration
feature/AGENCY-007_agency_publishing
feature/AGENCY-008_entitygraph_refactor
```
