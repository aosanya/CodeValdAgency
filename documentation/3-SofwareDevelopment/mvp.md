# MVP — CodeValdAgency

## Goal

Deliver a production-ready agency lifecycle management gRPC microservice with ArangoDB persistence and CodeValdCross registration.

---

## MVP Scope

The MVP delivers:
1. The `AgencyManager` Go interface (convenience-method facade over `entitygraph.DataManager`) and its `agencyManager` implementation
2. The `Agency` domain model with `Enabled` flag and read-only enforcement once published (edits flow through `AgencyDraft` + `PromoteDraft`)
3. An ArangoDB `entitygraph.DataManager` + `entitygraph.SchemaManager` implementation with `agency_` prefixed collections
4. A pre-delivered agency schema (TypeDefinitions for Agency, Goal, Workflow, WorkItem, ConfiguredRole, AgencySnapshot, AgencyPublication) seeded on first use
5. An `AgencyService` gRPC service exposing all CRUD + convenience operations
6. CodeValdCross heartbeat registration and `agency.created` / `agency.published` event publishing
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
| **MVP-AGENCY-008 — EntityGraph Refactor** | | | |
| MVP-AGENCY-008-A | Models refactor — standalone entity types | ✅ Done | ~~SHAREDLIB-010~~ ✅ |
| MVP-AGENCY-008-B | Pre-delivered schema (`schema.go`) | ✅ Done | ~~MVP-AGENCY-008-A~~ ✅ |
| MVP-AGENCY-008-C | `AgencyManager` refactor — entitygraph wrapper | ✅ Done | MVP-AGENCY-008-A |
| MVP-AGENCY-008-D | ArangoDB storage split (`entities.go`, `relationships.go`, `schemaops.go`) | ✅ Done | MVP-AGENCY-008-C |
| MVP-AGENCY-008-E | Proto + gRPC handlers (`GetGoals`, `GetWorkflows`, `GetConfiguredRoles`) | ✅ Done | MVP-AGENCY-008-C |
| MVP-AGENCY-008-F | `cmd/main.go` wiring — inject DataManager, seed schema on startup | ✅ Done | MVP-AGENCY-008-D, MVP-AGENCY-008-E |
| MVP-AGENCY-008-G | Test rewrite — `fakeDataManager` replacing `fakeBackend` | ✅ Done | MVP-AGENCY-008-D |
| MVP-AGENCY-008-H | Type-specific URL params — `EntityIDParam` on all schema types; adopt `schemaroutes.RoutesFromSchema` in registrar | ✅ Done | MVP-AGENCY-008-B, ~~SHAREDLIB-011~~ 🚀 |
| **MVP-AGENCY-009 — Agency Drafts** | | | |
| MVP-AGENCY-009-A | Models & errors update — `AgencyDraft`, `AgencyDraftStatus`, remove `AgencyLifecycle`, add `enabled` to `Agency`, add `ErrAgencyNotPublished / ErrDraftNotFound / ErrDraftNotOpen / ErrAgencyReadOnly` | ✅ Done | — |
| MVP-AGENCY-009-B | Schema update — `AgencyDraft` TypeDefinition, update `Agency` / `Goal` / `Workflow` / `ConfiguredRole` TypeDefinitions | ✅ Done | ~~MVP-AGENCY-009-A~~ ✅ |
| MVP-AGENCY-009-C | `AgencyManager` draft methods — `CreateDraft`, `GetDraft`, `ListDrafts`, `UpdateDraftDescription`, `PromoteDraft`, `ArchiveDraft`; enforce `ErrAgencyReadOnly` on direct edits | ✅ Done | ~~MVP-AGENCY-009-A~~ ✅ |
| MVP-AGENCY-009-D | ArangoDB storage — `agency_drafts` (root) and `agency_draft_entities` (sub-types `DraftGoal`/`DraftWorkflow`/…) routed via `TypeDefinition.StorageCollection`; fork deep-copy logic in `PromoteDraft` | ✅ Done | ~~MVP-AGENCY-009-C~~ ✅ |
| MVP-AGENCY-009-E | gRPC handlers — new RPCs for all draft operations; update `Agency` proto message | ✅ Done | ~~MVP-AGENCY-009-C~~ ✅ |
| MVP-AGENCY-009-F | Tests — full acceptance test suite for all draft flows | ✅ Done | ~~MVP-AGENCY-009-C~~ ✅, ~~MVP-AGENCY-009-D~~ ✅ |

| **MVP-AGENCY-010 — RACI Schema** | | | |
| MVP-AGENCY-010 | RACI Schema: Role + ContextSource entity types | ✅ Done | — |
| MVP-AGENCY-011 | RACI AgencyManager: CRUD + MatchRoles query | ✅ Done | MVP-AGENCY-010 |
| MVP-AGENCY-012 | RACI gRPC RPCs | ✅ Done | MVP-AGENCY-011 |
| MVP-AGENCY-013 | RACI Cross Registration & Route Declarations | ✅ Done | MVP-AGENCY-012 |
| MVP-AGENCY-014 | RACI Unit & Integration Tests | ✅ Done | MVP-AGENCY-011, MVP-AGENCY-012 |
| FEAT-20260605-002 | WorkPlan review step type — `review_step_type`, `review_trigger_topic`, `review_success_topic`, `review_failure_topic` fields on WorkPlan; gates task progression on AcceptanceCriteria results | 📋 Not Started | FEAT-20260605-001 (Work) |
| FEAT-20260608-001 | `EventFlowBranch` model — branches[] field on EventFlow; schema.go comment; agency.json utility-app-builder updated | ✅ Done | — |
| FEAT-20260608-002 | `dev-agency-flowchart` skill — render branches[] as multiple Mermaid edges with condition labels | 🚀 In Progress | FEAT-20260608-001 |

---

## Success Criteria

- [ ] `go build ./...` succeeds
- [ ] `go test -race ./...` all pass
- [ ] `go vet ./...` shows 0 issues
- [ ] All `AgencyService` RPCs work end-to-end with ArangoDB
- [ ] CodeValdCross registration fires on startup and repeats on heartbeat
- [ ] `PromoteDraft` writes an `AgencySnapshot` entity to `agency_entities`
- [ ] Direct edits to a published agency return `ErrAgencyReadOnly` → `FAILED_PRECONDITION`
- [ ] Invalid draft-status transitions (`promoted`/`archived` → anything) return `ErrDraftNotOpen` → `FAILED_PRECONDITION`
- [ ] `agency.created` is published after every successful `SetAgencyDetails`
- [ ] Routes declared in `RegisterRequest` and proxied via CodeValdCross dynamic proxy
- [ ] `PublishAgency` creates an immutable versioned publication (`v1`, `v2`, …) without touching agency status
- [ ] `agency.published` is fired after every successful publish
- [ ] `POST /agency/publish` is proxied through CodeValdCross
- [ ] `AgencyManager` is a convenience facade over `entitygraph.DataManager` — no bespoke `Backend` interface
- [ ] Storage uses `agency_entities`, `agency_drafts`, `agency_draft_entities`, `agency_relationships` (edge), `agency_schemas_draft`, and `agency_schemas_published` collections (snapshots and publications live as `TypeID`s within `agency_entities`)
- [ ] Pre-delivered schema is seeded into `agency_schemas_published` on first use (idempotent)
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
feature/AGENCY-008-A_models_refactor
feature/AGENCY-008-B_pre_delivered_schema
feature/AGENCY-008-C_agencymanager_entitygraph_wrapper
feature/AGENCY-008-D_arangodb_storage_split
feature/AGENCY-008-E_grpc_goals_workflows_roles
feature/AGENCY-008-F_cmd_wiring_schema_seed
feature/AGENCY-008-G_test_rewrite
feature/AGENCY-009-A_agency_draft_models
feature/AGENCY-009-B_agency_draft_schema
feature/AGENCY-009-C_agency_draft_manager
feature/AGENCY-009-D_agency_draft_storage
feature/AGENCY-009-E_agency_draft_grpc
feature/AGENCY-009-F_agency_draft_tests
```
