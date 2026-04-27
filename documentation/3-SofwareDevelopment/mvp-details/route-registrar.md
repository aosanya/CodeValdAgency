# Service-Driven Route Registration

Topics: HTTP Routing · Route Registrar · CodeValdCross Integration

---

## MVP-AGENCY-006 — Service-Driven Route Registration

### Overview

Declare CodeValdAgency HTTP routes in the `RegisterRequest` sent to CodeValdCross so that
the dynamic proxy can forward requests to this service without Cross hardcoding any
agency-specific handler logic.

This task mirrors the pattern established by **MVP-WORK-006** and **GIT-011**, where
each service owns its route declarations and CodeValdCross acts purely as a proxy.

---

### Dependencies

- **MVP-AGENCY-003** (gRPC service — `AgencyServiceClient` is available)
- **CROSS-007** must be implemented (provides the `server.Route` type and dynamic-proxy
  infrastructure in CodeValdCross)

---

### Routes to Declare

The original AGENCY-006 task declared three routes (`SetAgencyDetails`,
`GetAgency`, `UpdateAgency`) and the publishing additions from AGENCY-007.
The `UpdateAgency` route was removed in AGENCY-009 — direct edits to a
published agency are no longer permitted; changes flow through drafts. The
authoritative, current route list lives in
[architecture-flows.md §7](../../2-SoftwareDesignAndArchitecture/architecture-flows.md#7-codevaldcross-registration);
the headline static routes are:

| Method | Pattern | gRPC Method |
|--------|---------|-------------|
| `POST` | `/agency/{agencyId}` | `SetAgencyDetails` |
| `GET`  | `/agency/{agencyId}` | `GetAgency` |
| `POST` | `/agency/{agencyId}/drafts` | `CreateDraft` |
| `GET`  | `/agency/{agencyId}/drafts` | `ListDrafts` |
| `GET`  | `/agency/{agencyId}/drafts/{draftId}` | `GetDraft` |
| `PUT`  | `/agency/{agencyId}/drafts/{draftId}` | `UpdateDraftDescription` |
| `POST` | `/agency/{agencyId}/drafts/{draftId}/promote` | `PromoteDraft` |
| `POST` | `/agency/{agencyId}/drafts/{draftId}/archive` | `ArchiveDraft` |
| `POST` | `/agency/{agencyId}/publish` | `PublishAgency` |
| `GET`  | `/agency/{agencyId}/publications` | `ListPublications` |
| `GET`  | `/agency/{agencyId}/publications/{version}` | `GetPublication` |
| `PUT`  | `/agency/{agencyId}/publications/{version}/status` | `UpdatePublicationStatus` |

EntityService CRUD routes for each schema type (Goal, Workflow, etc.) are
generated dynamically by `schemaroutes.RoutesFromSchema` and registered
alongside the static set on every heartbeat.

---

### Acceptance Criteria

#### `internal/registrar/registrar.go` (updated)

- [x] `RegisterRequest` includes a `DeclaredRoutes` field listing all routes
  above, each with `Method`, `Pattern`, `GrpcMethod`, and `PathBindings`
- [x] `PathBindings` maps `{draftId}` → gRPC field `draft_id` for draft routes
  and `{version}` → gRPC field `version` for publication routes

#### CodeValdCross (no changes to CodeValdAgency repo)

- [ ] The dynamic proxy in CodeValdCross forwards the route patterns to the
  registered `codevaldagency` service without any hardcoded handler

---

### What Does NOT Change in CodeValdAgency

The proto definitions, gRPC server, generated stubs, and `AgencyService`
implementation are untouched. Route registration is purely additive to the
`RegisterRequest` payload.

---

### Test Impact

- `go build ./...` and `go test -race ./...` in CodeValdAgency must pass
- Existing registrar heartbeat tests are unaffected

---

### Branch Naming (in CodeValdAgency repo)

```
feature/AGENCY-006_service_driven_route_registration
```
