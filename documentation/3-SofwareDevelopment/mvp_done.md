# MVP Done — Completed Tasks

Completed tasks are removed from `mvp.md` and recorded here with their completion date.

| Task ID | Title | Completion Date | Branch | Notes |
|---------|-------|-----------------|--------|-------|
| MVP-AGENCY-001 | Library Scaffolding & Agency Model | 2026-03-18 | feature/AGENCY-001_library_scaffolding | Root package, AgencyManager interface, Agency model, lifecycle enforcement |
| MVP-AGENCY-002 | ArangoDB Backend | 2026-03-18 | feature/AGENCY-002_arangodb_backend | storage/arangodb: agency_details, agency_snapshots collections |
| MVP-AGENCY-003 | gRPC Service (AgencyService) | 2026-03-18 | feature/AGENCY-003_grpc_service | internal/server: all AgencyService RPCs wired to AgencyManager |
| MVP-AGENCY-004 | CodeValdCross Registration | 2026-03-18 | feature/AGENCY-004_cross_registration | internal/registrar: SharedLib heartbeat registrar; cross.agency.created publisher |
| MVP-AGENCY-005 | Unit & Integration Tests | 2026-03-18 | feature/AGENCY-005_integration_tests | agency_test.go, storage/arangodb/storage_test.go, internal/server/integration_test.go |
| MVP-AGENCY-006 | Service-Driven Route Registration | 2026-03-18 | feature/AGENCY-007_agency_publishing | Routes declared in registrar.agencyRoutes(); registered with Cross on every heartbeat |
| MVP-AGENCY-008-A | Models refactor — standalone entity types | 2026-03-18 | main (direct commits) | Goal, Workflow, WorkItem, ConfiguredRole, AgencySnapshot, AgencyPublication as standalone entity types in models.go |
| MVP-AGENCY-008-B | Pre-delivered schema (`schema.go`) | 2026-03-19 | feature/AGENCY-008-B_pre-delivered-schema | `DefaultAgencySchema()` returning `types.Schema` with 7 TypeDefinitions; AgencySnapshot + AgencyPublication marked Immutable with dedicated StorageCollection |
| MVP-AGENCY-008-B | Pre-delivered schema (`schema.go`) | 2026-03-19 | feature/AGENCY-008-B_pre-delivered-schema | `DefaultAgencySchema()` returning `types.Schema` with 7 TypeDefinitions; AgencySnapshot + AgencyPublication marked Immutable with dedicated StorageCollection |
