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
| `POST` | `/agencies` | `CreateAgency` | Create a new agency |
| `GET` | `/agencies` | `ListAgencies` | List agencies with optional filters |
| `GET` | `/agencies/{agencyId}` | `GetAgency` | Retrieve a single agency |
| `PUT` | `/agencies/{agencyId}` | `UpdateAgency` | Update / lifecycle-advance an agency |
| `DELETE` | `/agencies/{agencyId}` | `DeleteAgency` | Delete an agency |

---

### Acceptance Criteria

#### `internal/registrar/registrar.go` (updated)

- [ ] `RegisterRequest` includes a `DeclaredRoutes` (or equivalent) field listing all
  five routes above, each with `Method`, `Pattern`, `GrpcMethod`, and `PathBindings`
- [ ] `PathBindings` maps `{agencyId}` → gRPC field `agency_id` where applicable

#### CodeValdCross (no changes to CodeValdAgency repo)

- [ ] The dynamic proxy in CodeValdCross forwards the five route patterns to the
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
