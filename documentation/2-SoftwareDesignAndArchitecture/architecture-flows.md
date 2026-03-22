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

### EntityService Error Mapping

Entity-layer errors are mapped in `internal/server/errors.go` via `toEntityGRPCError()`.

| Error | gRPC code |
|---|---|
| `entitygraph.ErrEntityNotFound` | `codes.NotFound` |
| `entitygraph.ErrEntityAlreadyExists` | `codes.AlreadyExists` |
| `entitygraph.ErrRelationshipNotFound` | `codes.NotFound` |
| `entitygraph.ErrImmutableType` | `codes.FailedPrecondition` |
| `entitygraph.ErrInvalidRelationship` | `codes.InvalidArgument` |
| `entitygraph.ErrRelationshipCardinalityViolation` | `codes.FailedPrecondition` |
| `entitygraph.ErrRequiredRelationshipViolation` | `codes.FailedPrecondition` |
| all others | `codes.Internal` |

---

## 6. gRPC Service Definition

### AgencyService

Manages the lifecycle of the single Agency entity, its publications, and
provides convenience accessors for Goals, Workflows, and ConfiguredRoles.

```protobuf
service AgencyService {
  rpc SetAgencyDetails   (SetAgencyDetailsRequest)      returns (Agency);
  rpc GetAgency          (GetAgencyRequest)              returns (Agency);
  rpc UpdateAgency       (UpdateAgencyRequest)           returns (Agency);
  rpc PublishAgency      (PublishAgencyRequest)          returns (AgencyPublication);
  rpc GetPublication     (GetPublicationRequest)         returns (AgencyPublication);
  rpc ListPublications   (ListPublicationsRequest)       returns (ListPublicationsResponse);

  // Convenience wrappers retained for direct gRPC callers.
  rpc GetGoals           (GetGoalsRequest)               returns (GetGoalsResponse);
  rpc GetWorkflows       (GetWorkflowsRequest)           returns (GetWorkflowsResponse);
  rpc GetConfiguredRoles (GetConfiguredRolesRequest)     returns (GetConfiguredRolesResponse);
}
```

### EntityService

Provides generic CRUD for entities and relationships. HTTP routes for each
entity type are generated by `schemaroutes.RoutesFromSchema` and wired here;
CodeValdCross injects `type_id` (or `name` for relationships) via
`ConstantBinding` at dispatch time, so a single RPC serves all entity types.

Implemented in `internal/server/entity_server.go`.

```protobuf
service EntityService {
  rpc ListEntities        (ListEntitiesRequest)        returns (ListEntitiesResponse);
  rpc CreateEntity        (CreateEntityRequest)        returns (EntityItem);
  rpc GetEntity           (GetEntityRequest)           returns (EntityItem);
  rpc UpdateEntity        (UpdateEntityRequest)        returns (EntityItem);
  rpc DeleteEntity        (DeleteEntityRequest)        returns (DeleteEntityResponse);
  rpc ListRelationships   (ListRelationshipsRequest)   returns (ListRelationshipsResponse);
  rpc CreateRelationship  (CreateRelationshipRequest)  returns (RelationshipItem);
  rpc DeleteRelationship  (DeleteRelationshipRequest)  returns (DeleteRelationshipResponse);
}
```

Generated Go stubs live in `gen/go/`. **Do not hand-edit generated files.**

---

## 7. CodeValdCross Registration

`cmd/main.go` starts a registration heartbeat on startup. The loop calls
`OrchestratorService.Register` on CodeValdCross every **20 seconds**.

Routes are assembled in `internal/registrar/registrar.go:agencyRoutes()` and
fall into two groups:

**Static routes** — fixed AgencyService methods:

| Method | Pattern | gRPC method |
|---|---|---|
| `POST` | `/agency/{agencyId}` | `AgencyService/SetAgencyDetails` |
| `GET`  | `/agency/{agencyId}` | `AgencyService/GetAgency` |
| `PUT`  | `/agency/{agencyId}` | `AgencyService/UpdateAgency` |
| `POST` | `/agency/{agencyId}/publish` | `AgencyService/PublishAgency` |
| `GET`  | `/agency/{agencyId}/publications` | `AgencyService/ListPublications` |
| `GET`  | `/agency/{agencyId}/publications/{version}` | `AgencyService/GetPublication` |
| `PUT`  | `/agency/{agencyId}/publications/{version}/status` | `AgencyService/UpdatePublicationStatus` |

**Dynamic routes** — generated by `schemaroutes.RoutesFromSchema(DefaultAgencySchema(), "/agency/{agencyId}", "agencyId", "/codevaldagency.v1.EntityService")`. Each `TypeDefinition` with a `PathSegment` gets a full CRUD set pointing to the generic `EntityService` RPCs. CodeValdCross injects `type_id` via `ConstantBinding` so the HTTP caller never needs to set it:

```json
[
  {
    "method": "GET",
    "pattern": "/agency/{agencyId}/goals",
    "capability": "list_goal",
    "grpc_method": "/codevaldagency.v1.EntityService/ListEntities",
    "path_bindings": [{"url_param": "agencyId", "field": "agency_id"}],
    "constant_bindings": [{"field": "type_id", "value": "Goal"}]
  },
  {
    "method": "POST",
    "pattern": "/agency/{agencyId}/goals",
    "capability": "create_goal",
    "grpc_method": "/codevaldagency.v1.EntityService/CreateEntity",
    "path_bindings": [{"url_param": "agencyId", "field": "agency_id"}],
    "constant_bindings": [{"field": "type_id", "value": "Goal"}]
  },
  {
    "method": "GET",
    "pattern": "/agency/{agencyId}/goals/{entityId}/agency",
    "capability": "list_goal_agency",
    "grpc_method": "/codevaldagency.v1.EntityService/ListRelationships",
    "path_bindings": [{"url_param": "agencyId", "field": "agency_id"}, {"url_param": "entityId", "field": "entity_id"}],
    "constant_bindings": [{"field": "name", "value": "agency"}]
  }
]
```

Repeat calls are the **liveness signal**. Cross expires registrations that
stop heartbeating. If Cross is unavailable, the loop retries silently.
