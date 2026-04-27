# CodeValdAgency — Lifecycle, Flows & Errors

> Part of the split architecture. Index: [architecture.md](architecture.md)

---

## 1. Agency State

The live `Agency` entity has no lifecycle progression. It carries a single
`enabled` boolean that can be toggled.

| State | Meaning |
|---|---|
| `enabled = true` | Agency is active; work may be dispatched |
| `enabled = false` | Agency is disabled; work dispatch is suppressed |

There is no terminal state. `GetAgency` returns `ErrAgencyNotPublished`
until the first `AgencyDraft` has been promoted.

---

## 2. Draft Lifecycle

Draft status progresses forward-only. Both `promoted` and `archived` are
terminal.

```
open ──► promoted
     └─► archived
```

| Status | Meaning |
|---|---|
| `open` | Being edited; may be promoted or archived |
| `promoted` | Became the live agency; read-only |
| `archived` | Soft-discarded; read-only |

---

## 3. SetAgencyDetails Flow (Bootstrap Only)

`SetAgencyDetails` is the **first-time setup path**. Once a live agency exists
it returns `ErrAgencyReadOnly` — subsequent changes go through drafts.

```
AgencyManager.SetAgencyDetails(ctx, jsonStr)
    │
    ├─ parse JSON → Agency{ID, Name, Mission, Vision, Enabled}
    │       → ErrInvalidJSON if malformed or ID empty
    │
    ├─ check if live agency already exists
    │       exists? → ErrAgencyReadOnly
    │
    ├─ dataManager.CreateEntity — Agency entity
    │
    └─ publisher.Publish(ctx, "cross.agency.created", agencyID)
            publish errors are logged; never returned to caller
```

---

## 4. CreateDraft Flow

Forks the entire agency sub-graph (Goals, Workflows, WorkItems,
ConfiguredRoles, Instructions, Deliverables) into a new open draft.

```
AgencyManager.CreateDraft(ctx, description, forkedFromID, forkedFromType)
    │
    ├─ validate description non-empty
    │
    ├─ resolve source:
    │     forkedFromType="live"  → GetAgency() → ErrAgencyNotPublished if absent
    │     forkedFromType="draft" → GetDraft(forkedFromID)
    │                              → ErrDraftNotFound if absent
    │                              → ErrDraftNotOpen  if not open
    │
    ├─ dataManager.CreateEntity — AgencyDraft root entity
    │       Properties: {description, status="open", forked_from_id,
    │                     forked_from_type, created_at, updated_at}
    │
    ├─ deep-copy sub-graph from source into the new draft scope:
    │     for each entity type [Goal, Workflow, WorkItem, ConfiguredRole,
    │                           Instruction, Deliverable]:
    │         CreateEntity (new ID, same properties)
    │         CreateRelationship (draft → entity, preserving internal edges)
    │
    └─ publisher.Publish(ctx, "cross.agency.draft.created", agencyID)
```

---

## 5. PromoteDraft Flow

Replaces the live agency with the draft's sub-graph. Previous live agency
entities are discarded (replaced in-place). The promoted draft becomes a
**terminal record** — it is the immutable link between the authoring history
and the publication chain.

To make further changes after promotion, create a **new draft** from the
live agency.

```
AgencyManager.PromoteDraft(ctx, draftID)
    │
    ├─ GetDraft(draftID)
    │     → ErrDraftNotFound if absent
    │     → ErrDraftNotOpen  if not open
    │
    ├─ replace live agency sub-graph:
    │     delete existing live Goal / Workflow / WorkItem / ConfiguredRole /
    │     Instruction / Deliverable entities and their relationships
    │     upsert Agency scalar fields from draft
    │     deep-copy draft's sub-graph into live agency scope (new IDs)
    │
    ├─ dataManager.UpdateEntity — AgencyDraft {status: "promoted"}
    │       draft is now terminal; cannot be edited or re-promoted
    │
    ├─ dataManager.CreateEntity — AgencySnapshot (immutable, promotion record)
    │
    └─ publisher.Publish(ctx, "cross.agency.promoted", agencyID)

Post-promotion workflow:
    ↓
    CreateDraft(forkedFromType="live")   ← start fresh from the new live state
    [edit] → PromoteDraft → repeat
```

Other open drafts are **not modified** by promotion. They may still be
promoted, but their base will be stale relative to the new live agency.

---

## 6. ArchiveDraft Flow

```
AgencyManager.ArchiveDraft(ctx, draftID)
    │
    ├─ GetDraft(draftID)
    │     → ErrDraftNotFound if absent
    │     → ErrDraftNotOpen  if not open
    │
    ├─ dataManager.UpdateEntity — AgencyDraft {status: "archived"}
    │
    └─ publisher.Publish(ctx, "cross.agency.draft.archived", agencyID)
```

---

## 7. PublishAgency Flow

```
AgencyManager.PublishAgency(ctx, draftID)
    │
    ├─ GetAgency(ctx) → Agency
    │     → ErrAgencyNotFound if no agency entity exists
    │
    ├─ nextPublicationVersion → MAX(version) + 1 (or 1 if none)
    │
    ├─ if draftID != "":
    │       contentHash = draftContentHash(ctx, draftID)
    │       for each existing publication p:
    │           p.ContentHash == contentHash?  → ErrNoChangesDetected
    │
    ├─ dataManager.CreateEntity — AgencyPublication{
    │       TypeID:    "AgencyPublication",
    │       AgencyID:  agencyID,
    │       Properties: {version, tag, published_at, draft_id, content_hash},
    │   }  → immutable
    │
    ├─ dataManager.CreateEntity — AgencyPublicationStatus{status: "draft"}
    ├─ dataManager.CreateRelationship — has_status: publication → status
    │
    └─ publisher.Publish(ctx, "cross.agency.published", agencyID)
```

`UpdatePublicationStatus(version, status)` mutates the linked
`AgencyPublicationStatus` entity only — the immutable `AgencyPublication`
record is never updated. Allowed transitions: `draft → active`,
`active → archived`. `archived` is terminal.

---

## 8. Error Types

Defined in `errors.go` at the module root.

```go
var (
    ErrAgencyNotFound             = errors.New("agency not found")
    ErrAgencyNotPublished         = errors.New("agency not published: promote a draft first")
    ErrAgencyReadOnly             = errors.New("live agency is read-only: use a draft to make changes")
    ErrDraftNotFound              = errors.New("agency draft not found")
    ErrDraftNotOpen               = errors.New("agency draft is not open")
    ErrInvalidAgency              = errors.New("invalid agency: missing required fields")
    ErrInvalidJSON                = errors.New("invalid agency: malformed JSON payload")
    ErrPublicationNotFound        = errors.New("agency publication not found")
    ErrInvalidPublicationStatus   = errors.New("invalid publication status transition")
)
```

### gRPC Code Mapping

Mapping lives exclusively in `internal/server/server.go` — never in the manager.

| Error | gRPC code |
|---|---|
| `ErrAgencyNotFound` | `codes.NotFound` |
| `ErrAgencyNotPublished` | `codes.FailedPrecondition` |
| `ErrDraftNotFound` | `codes.NotFound` |
| `ErrPublicationNotFound` | `codes.NotFound` |
| `ErrInvalidJSON` | `codes.InvalidArgument` |
| `ErrInvalidAgency` | `codes.InvalidArgument` |
| `ErrAgencyReadOnly` | `codes.FailedPrecondition` |
| `ErrDraftNotOpen` | `codes.FailedPrecondition` |
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

Manages the live agency, drafts, publications, and convenience accessors
for Goals, Workflows, and ConfiguredRoles.

```protobuf
service AgencyService {
  // Bootstrap
  rpc SetAgencyDetails   (SetAgencyDetailsRequest)      returns (Agency);

  // Live agency (read-only once published)
  rpc GetAgency          (GetAgencyRequest)              returns (Agency);

  // Drafts
  rpc CreateDraft              (CreateDraftRequest)              returns (AgencyDraft);
  rpc GetDraft                 (GetDraftRequest)                 returns (AgencyDraft);
  rpc ListDrafts               (ListDraftsRequest)               returns (ListDraftsResponse);
  rpc UpdateDraftDescription   (UpdateDraftDescriptionRequest)   returns (AgencyDraft);
  rpc PromoteDraft             (PromoteDraftRequest)             returns (Agency);
  rpc ArchiveDraft             (ArchiveDraftRequest)             returns (AgencyDraft);

  // Publications
  rpc PublishAgency      (PublishAgencyRequest)          returns (AgencyPublication);
  rpc GetPublication     (GetPublicationRequest)         returns (AgencyPublication);
  rpc ListPublications   (ListPublicationsRequest)       returns (ListPublicationsResponse);
  rpc UpdatePublicationStatus (UpdatePublicationStatusRequest) returns (AgencyPublication);

  // Convenience wrappers (live agency sub-resources)
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
| `POST` | `/agency/{agencyId}/drafts` | `AgencyService/CreateDraft` |
| `GET`  | `/agency/{agencyId}/drafts` | `AgencyService/ListDrafts` |
| `GET`  | `/agency/{agencyId}/drafts/{draftId}` | `AgencyService/GetDraft` |
| `PUT`  | `/agency/{agencyId}/drafts/{draftId}` | `AgencyService/UpdateDraftDescription` |
| `POST` | `/agency/{agencyId}/drafts/{draftId}/promote` | `AgencyService/PromoteDraft` |
| `POST` | `/agency/{agencyId}/drafts/{draftId}/archive` | `AgencyService/ArchiveDraft` |
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
