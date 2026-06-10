# CodeValdAgency — Interfaces & Service Surface

> Part of the split architecture. Index: [architecture.md](architecture.md)
>
> Companion files:
> - [architecture-models.md](architecture-models.md) — value-type definitions (Agency, Workflow, AgencyDraft, …)
> - [architecture-flows.md](architecture-flows.md) — lifecycle flows and errors

---

## 1. AgencyManager Interface

`AgencyManager` is the sole business-logic entry point. gRPC handlers hold
the interface, never the concrete type. The implementation wraps
`entitygraph.DataManager` to expose agency-specific convenience methods.

```go
// AgencyManager is the top-level interface for managing a single Agency.
// All gRPC handlers delegate to AgencyManager — never to storage directly.
type AgencyManager interface {
    // ── Live agency (read-only once published) ────────────────────────────────

    // GetAgency returns the current live Agency.
    // Returns ErrAgencyNotPublished if no draft has been promoted yet.
    GetAgency(ctx context.Context) (Agency, error)

    // GetGoals returns all Goal entities linked to the live Agency.
    GetGoals(ctx context.Context) ([]Goal, error)

    // GetWorkflows returns all Workflow entities linked to the live Agency.
    // Each Workflow includes its per-workflow event_flows JSON blob
    // (FEAT-20260609-002) when one was bundled at import time.
    GetWorkflows(ctx context.Context) ([]Workflow, error)

    // GetConfiguredRoles returns all ConfiguredRole entities linked to the live Agency.
    GetConfiguredRoles(ctx context.Context) ([]ConfiguredRole, error)

    // ── Bootstrap (first-time setup only) ────────────────────────────────────

    // SetAgencyDetails is the bootstrap path for first-time database setup.
    // It creates the initial Agency entity and the first open draft.
    // Returns ErrAgencyReadOnly if a live agency already exists — use
    // CreateDraft + PromoteDraft for subsequent changes.
    // Publishes agency.created after a successful create.
    SetAgencyDetails(ctx context.Context, jsonStr string) (Agency, error)

    // ── Drafts ────────────────────────────────────────────────────────────────

    // CreateDraft forks the agency graph into a new open AgencyDraft.
    // forkedFromType must be "live" or "draft".
    // Returns ErrAgencyNotPublished if forkedFromType="live" and no live agency exists.
    // Returns ErrDraftNotFound if forkedFromType="draft" and the source does not exist.
    // Returns ErrDraftNotOpen if forkedFromType="draft" and the source is not open.
    // description must be non-empty.
    CreateDraft(ctx context.Context, description, forkedFromID, forkedFromType string) (AgencyDraft, error)

    // GetDraft retrieves a single draft by its ID.
    // Returns ErrDraftNotFound if no draft with that ID exists.
    GetDraft(ctx context.Context, draftID string) (AgencyDraft, error)

    // ListDrafts returns all drafts in descending creation order.
    ListDrafts(ctx context.Context) ([]AgencyDraft, error)

    // UpdateDraftDescription updates the human-readable description of an open draft.
    // Returns ErrDraftNotFound if the draft does not exist.
    // Returns ErrDraftNotOpen if the draft is not open.
    UpdateDraftDescription(ctx context.Context, draftID, description string) (AgencyDraft, error)

    // PromoteDraft replaces the live agency with the full sub-graph of the given draft.
    // The draft transitions from "open" to "promoted". Other open drafts are unaffected.
    // Publishes agency.promoted on success.
    // Returns ErrDraftNotFound if the draft does not exist.
    // Returns ErrDraftNotOpen if the draft is not open.
    PromoteDraft(ctx context.Context, draftID string) (Agency, error)

    // ArchiveDraft soft-discards an open draft.
    // The draft transitions from "open" to "archived".
    // Returns ErrDraftNotFound if the draft does not exist.
    // Returns ErrDraftNotOpen if the draft is not open.
    ArchiveDraft(ctx context.Context, draftID string) (AgencyDraft, error)

    // ── Publications ──────────────────────────────────────────────────────────

    // PublishAgency creates an immutable AgencyPublication snapshot of the
    // current live Agency state. Version numbers are auto-incremented.
    // Returns ErrAgencyNotPublished if no live agency exists.
    PublishAgency(ctx context.Context) (AgencyPublication, error)

    // GetPublication retrieves a publication by its version number.
    GetPublication(ctx context.Context, version int) (AgencyPublication, error)

    // ListPublications returns all publications in ascending version order.
    ListPublications(ctx context.Context) ([]AgencyPublication, error)

    // UpdatePublicationStatus transitions a publication to a new status.
    // Allowed: draft → active, active → archived (archived is terminal).
    // Returns ErrPublicationNotFound if no publication with that version exists.
    // Returns ErrInvalidPublicationStatus if the transition is not permitted.
    UpdatePublicationStatus(ctx context.Context, version int, status string) (AgencyPublication, error)

    // ── WorkPlan dispatch methods ─────────────────────────────────────────────

    // CreateWorkPlan stores a new WorkPlan and links it to the Agency.
    // Returns ErrInvalidRegex if trigger_topic or payload_condition is not a valid Go regexp.
    CreateWorkPlan(ctx context.Context, req CreateWorkPlanRequest) (WorkPlan, error)

    // GetWorkPlan retrieves a single WorkPlan by ID.
    // Returns ErrWorkPlanNotFound if no WorkPlan with that ID exists.
    GetWorkPlan(ctx context.Context, workPlanID string) (WorkPlan, error)

    // ListWorkPlans returns all WorkPlan entities linked to this Agency.
    ListWorkPlans(ctx context.Context) ([]WorkPlan, error)

    // UpdateWorkPlan applies updates to the WorkPlan identified by workPlanID.
    // Returns ErrWorkPlanNotFound if no WorkPlan with that ID exists.
    // Returns ErrInvalidRegex if trigger_topic or payload_condition is not a valid Go regexp.
    UpdateWorkPlan(ctx context.Context, workPlanID string, req UpdateWorkPlanRequest) (WorkPlan, error)

    // DeleteWorkPlan removes the WorkPlan identified by workPlanID.
    // Returns ErrWorkPlanNotFound if no WorkPlan with that ID exists.
    DeleteWorkPlan(ctx context.Context, workPlanID string) error

    // AddContextSource creates a typed ContextSource and links it to the WorkPlan.
    // Returns ErrWorkPlanNotFound if the WorkPlan does not exist.
    AddContextSource(ctx context.Context, workPlanID string, req AddContextSourceRequest) (ContextSource, error)

    // ListContextSources returns all ContextSource entities linked to the WorkPlan.
    // Returns ErrWorkPlanNotFound if the WorkPlan does not exist.
    ListContextSources(ctx context.Context, workPlanID string) ([]ContextSource, error)

    // RemoveContextSource deletes the ContextSource identified by sourceID.
    // Returns ErrContextSourceNotFound if no such entity exists.
    RemoveContextSource(ctx context.Context, workPlanID, sourceID string) error

    // MatchWorkPlans evaluates topic and payload against all enabled WorkPlans.
    // TriggerTopic is compiled as a Go regex and matched against topic;
    // if PayloadCondition is non-empty it is also matched against payload.
    // Returns all matching work plans with their ContextSource entities,
    // ordered by WorkPlan.Ordinality ascending.
    MatchWorkPlans(ctx context.Context, topic, payload string) ([]WorkPlanMatch, error)
}
```

> ℹ️ NOTE — `ImportDraft` is **not** on `AgencyManager`. It is a gRPC handler that
> bypasses the manager and writes draft entities directly through
> `entitygraph.DataManager`. See §5 below and
> [architecture-flows.md § ImportDraft Flow](architecture-flows.md).

---

## 2. agencyManager Struct

The concrete implementation is unexported. `cmd/main.go` constructs it via
`NewAgencyManager` and passes it to the gRPC server.

```go
// agencyManager is the concrete implementation of AgencyManager.
// It wraps entitygraph.DataManager to provide agency-specific convenience methods.
type agencyManager struct {
    dataManager   entitygraph.DataManager   // graph CRUD — injected by cmd/main.go
    schemaManager AgencySchemaManager       // schema versioning — injected by cmd/main.go
    publisher     CrossPublisher            // publishes agency.* events
    agencyID      string                    // loaded from the stored Agency entity at startup
}

// NewAgencyManager constructs an AgencyManager. The agencyID is read from the
// single Agency entity already present in the database, or left empty if no
// Agency has been created yet.
func NewAgencyManager(
    dm entitygraph.DataManager,
    sm AgencySchemaManager,
    pub CrossPublisher,
    agencyID string,
) AgencyManager {
    return &agencyManager{dataManager: dm, schemaManager: sm, publisher: pub, agencyID: agencyID}
}
```

---

## 3. AgencySchemaManager

`AgencySchemaManager` is a type alias for `entitygraph.SchemaManager` from
`CodeValdSharedLib`. No wrapping is needed — the Agency-specific logic is in
`DefaultAgencySchema()` (see [architecture-graph.md](architecture-graph.md)).

```go
// AgencySchemaManager manages schema versions for the Agency entity graph.
// It is a type alias for entitygraph.SchemaManager.
type AgencySchemaManager = entitygraph.SchemaManager
```

---

## 4. CrossPublisher Interface

```go
// CrossPublisher publishes events to CodeValdCross over gRPC.
// It is injected into agencyManager and implemented by internal/registrar.
type CrossPublisher interface {
    // Publish sends a named event with a string payload to CodeValdCross.
    // Errors are logged but never returned to the caller — the agency record
    // has already been persisted at the point of publication.
    Publish(ctx context.Context, topic, payload string) error
}
```

---

## 5. gRPC Service Definitions

### AgencyService

Manages the live agency, drafts, publications, the bulk import path, and
convenience accessors for Goals, Workflows, and ConfiguredRoles.

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

  // Bulk import (declarative agency.json/yaml → DraftWorkflow et al.)
  rpc ImportDraft              (ImportDraftRequest)              returns (ImportDraftResponse);

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

`ImportDraft` bypasses the `AgencyManager` interface and writes Draft\*
entities directly through `entitygraph.DataManager`. The request body is the
raw YAML or JSON of an `agency.yaml`/`agency.json` file; per-workflow
`event_flows` are accepted inline on each workflow entry (see
[architecture-flows.md § ImportDraft Flow](architecture-flows.md) for the
full per-workflow `event_flows` contract and the caller-side
`flows_<workflow.code>.json` bundling expectation).

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

## 6. CodeValdCross Registration

`cmd/main.go` starts a registration heartbeat on startup. The loop calls
`OrchestratorService.Register` on CodeValdCross every **20 seconds**.

Routes are assembled in `internal/registrar/registrar.go:agencyRoutes()` and
fall into two groups:

**Static routes** — fixed AgencyService methods:

| Method | Pattern | gRPC method |
|---|---|---|
| `POST` | `/agency/{agencyId}` | `AgencyService/SetAgencyDetails` |
| `GET`  | `/agency/{agencyId}` | `AgencyService/GetAgency` |
| `GET`  | `/agency/{agencyId}/workflows` | `AgencyService/GetWorkflows` |
| `POST` | `/agency/{agencyId}/import` | `AgencyService/ImportDraft` |
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
