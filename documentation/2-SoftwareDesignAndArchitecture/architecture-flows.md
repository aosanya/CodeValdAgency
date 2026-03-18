# CodeValdAgency — Lifecycle, Flows & Errors

> Part of the split architecture. Index: [architecture.md](architecture.md)

---

## 1. Agency Lifecycle

Lifecycle progresses **forward only**. No backward transitions are permitted.

```
draft ──► active ──► achieved
```

| State | Meaning | Mutability |
|---|---|---|
| `draft` | Configured, not yet running | Fully mutable |
| `active` | Work in progress | Mutable (Name, Mission, Vision; not Status backward) |
| `achieved` | All Goals met; terminal | **Read-only** — no further updates |

### Transition Guards

| From | To | Guard | Side-effect |
|---|---|---|---|
| `draft` | `active` | Agency must have ≥1 Goal and ≥1 Workflow with ≥1 WorkItem | Write `AgencySnapshot` entity (immutable) |
| `active` | `achieved` | None beyond valid status value | — |
| any | backward | **Always rejected** | — |
| `achieved` | any | **Always rejected** (terminal) | — |

---

## 2. SetAgencyDetails Flow

Upserts the root `Agency` entity and publishes the creation event.

```
AgencyManager.SetAgencyDetails(ctx, jsonStr)
    │
    ├─ parse JSON → Agency{ID, Name, Mission, Vision, Status}
    │       → ErrInvalidJSON if malformed or ID empty
    │
    ├─ dataManager.ListEntities(ctx, EntityFilter{AgencyID: id, TypeID: "Agency"})
    │       exists?
    │         ├─ No  → dataManager.CreateEntity(...)
    │         └─ Yes → dataManager.UpdateEntity(...)
    │
    └─ publisher.Publish(ctx, "cross.agency.created", agencyID)
            publish errors are logged; never returned to caller
```

---

## 3. UpdateAgency Flow

```
AgencyManager.UpdateAgency(ctx, req)
    │
    ├─ dataManager.GetEntity(ctx, agencyID, agencyEntityID)
    │       → ErrAgencyNotFound if missing
    │
    ├─ CanTransitionTo guard (if req.Status != "")
    │       → ErrInvalidLifecycleTransition if backward or from achieved
    │
    ├─ draft → active guard: check ≥1 Goal + ≥1 Workflow with ≥1 WorkItem
    │       → ErrInvalidAgency (FailedPrecondition) if violated
    │
    ├─ dataManager.UpdateEntity(ctx, agencyID, entityID, updateReq)
    │       → updated Agency
    │
    └─ (if draft → active) dataManager.CreateEntity — AgencySnapshot (immutable)
            snapshot errors are logged; never returned to caller
```

---

## 4. PublishAgency Flow

```
AgencyManager.PublishAgency(ctx)
    │
    ├─ GetAgency(ctx) → Agency (must be active or achieved)
    │
    ├─ auto-increment version:
    │       ListPublications → len(pubs) + 1
    │
    ├─ dataManager.CreateEntity — AgencyPublication{
    │       TypeID:    "AgencyPublication",
    │       AgencyID:  agencyID,
    │       Properties: {version, tag, published_at},
    │   }  → stored in agency_publications (Immutable: true)
    │
    └─ publisher.Publish(ctx, "cross.agency.published", agencyID)
```

---

## 5. Error Types

Defined in `errors.go` at the module root.

```go
var (
    ErrAgencyNotFound              = errors.New("agency not found")
    ErrInvalidLifecycleTransition  = errors.New("invalid agency lifecycle transition")
    ErrInvalidAgency               = errors.New("invalid agency: missing required fields")
    ErrInvalidJSON                 = errors.New("invalid agency: malformed JSON payload")
    ErrPublicationNotFound         = errors.New("agency publication not found")
)
```

### gRPC Code Mapping

Mapping lives exclusively in `internal/server/server.go` — never in the manager.

| Error | gRPC code |
|---|---|
| `ErrAgencyNotFound` | `codes.NotFound` |
| `ErrPublicationNotFound` | `codes.NotFound` |
| `ErrInvalidJSON` | `codes.InvalidArgument` |
| `ErrInvalidAgency` | `codes.InvalidArgument` |
| `ErrInvalidLifecycleTransition` | `codes.FailedPrecondition` |
| all others | `codes.Internal` |

---

## 6. gRPC Service Definition

```protobuf
syntax = "proto3";
package codevaldagency.v1;

// AgencyService manages the lifecycle of a single Agency.
service AgencyService {
    // SetAgencyDetails creates or upserts the root Agency from a JSON payload.
    rpc SetAgencyDetails (SetAgencyDetailsRequest)  returns (AgencyResponse);

    // GetAgency returns the single Agency in this database.
    rpc GetAgency        (GetAgencyRequest)          returns (AgencyResponse);

    // UpdateAgency applies partial updates and enforces lifecycle transitions.
    rpc UpdateAgency     (UpdateAgencyRequest)       returns (AgencyResponse);

    // GetGoals returns all Goal entities linked to the Agency.
    rpc GetGoals         (GetGoalsRequest)           returns (GetGoalsResponse);

    // GetWorkflows returns all Workflow entities, each with its WorkItems.
    rpc GetWorkflows     (GetWorkflowsRequest)       returns (GetWorkflowsResponse);

    // GetConfiguredRoles returns all ConfiguredRole entities.
    rpc GetConfiguredRoles (GetConfiguredRolesRequest) returns (GetConfiguredRolesResponse);

    // PublishAgency writes an immutable AgencyPublication snapshot.
    rpc PublishAgency    (PublishAgencyRequest)      returns (AgencyPublicationResponse);

    // GetPublication retrieves a publication by version number.
    rpc GetPublication   (GetPublicationRequest)     returns (AgencyPublicationResponse);

    // ListPublications returns all publications in ascending version order.
    rpc ListPublications (ListPublicationsRequest)   returns (ListPublicationsResponse);
}
```

Generated Go stubs live in `gen/go/`. **Do not hand-edit generated files.**

---

## 7. CodeValdCross Registration

`cmd/main.go` starts a registration heartbeat on startup. The loop calls
`OrchestratorService.Register` on CodeValdCross every **20 seconds**.

```go
RegisterRequest{
    ServiceName: "codevaldagency",
    Addr:        ":50053",
    Produces:    []string{"cross.agency.created", "cross.agency.published"},
    Consumes:    []string{},
    Routes: []Route{
        {Method: "POST", Pattern: "/{agencyId}/agency"},
        {Method: "GET",  Pattern: "/{agencyId}/agency"},
        {Method: "PUT",  Pattern: "/{agencyId}/agency"},
        {Method: "GET",  Pattern: "/{agencyId}/agency/goals"},
        {Method: "GET",  Pattern: "/{agencyId}/agency/workflows"},
        {Method: "GET",  Pattern: "/{agencyId}/agency/configured-roles"},
        {Method: "POST", Pattern: "/{agencyId}/agency/publish"},
        {Method: "GET",  Pattern: "/{agencyId}/agency/publications"},
        {Method: "GET",  Pattern: "/{agencyId}/agency/publications/{version}"},
    },
}
```

Repeat calls are the **liveness signal**. Cross expires registrations that
stop heartbeating. If Cross is unavailable, the loop retries silently.
