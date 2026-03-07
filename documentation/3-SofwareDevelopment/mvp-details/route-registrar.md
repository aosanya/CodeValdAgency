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

| Method | Pattern | gRPC Method | Description |
|--------|---------|-------------|-------------|
| `POST` | `/{agencyId}/agency` | `SetAgencyDetails` | Replace (or create) the full agency document from a JSON body |
| `GET`  | `/{agencyId}/agency` | `GetAgency` | Retrieve the single agency for this database |
| `PUT`  | `/{agencyId}/agency` | `UpdateAgency` | Apply incremental field edits with lifecycle validation |
| `POST` | `/{agencyId}/agency/publish` | `PublishAgency` | Create an immutable versioned publication of the current agency |
| `GET`  | `/{agencyId}/agency/publications` | `ListPublications` | List all publications in ascending version order |
| `GET`  | `/{agencyId}/agency/publications/{version}` | `GetPublication` | Retrieve a specific publication by version number |

---

### Acceptance Criteria

#### `internal/registrar/registrar.go` (updated)

- [x] `RegisterRequest` includes a `DeclaredRoutes` field listing all six routes
  above, each with `Method`, `Pattern`, `GrpcMethod`, and `PathBindings`
- [x] `PathBindings` maps `{version}` → gRPC field `version` for the
  `GetPublication` route

#### CodeValdCross (no changes to CodeValdAgency repo)

- [ ] The dynamic proxy in CodeValdCross forwards the six route patterns to the
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
