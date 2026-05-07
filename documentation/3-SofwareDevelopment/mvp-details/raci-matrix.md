# RACI Matrix — Implementation Details

Topics: Event Routing · WorkPlan-Based Dispatch · Context Assembly · Cross-Service Integration

---

## Overview

The RACI matrix is a composable, queryable event-routing layer stored in
CodeValdAgency. It maps **work plans** to the events they respond to and
the context they need — so CodeValdAI can determine, for any inbound event,
which agent to invoke and exactly what to fetch before running.

Each `WorkPlan` entity carries:
- **trigger_topic** — Go regex matched against the incoming Cross topic
- **payload_condition** — Go regex matched against the raw JSON payload string
- **instructions** — prompt template injected into the triggered `AgentRun`
- **agent_id** — cross-service string reference to a CodeValdAI `Agent` entity
- **enabled** / **ordinality** — activation flag and ordering when multiple work plans match

Context is assembled from typed `ContextSource` entities linked to the `WorkPlan`:

| Type | Service Queried | Parameters |
|---|---|---|
| `GitContextSource` | CodeValdGit | signals, max_results, match_mode, cascade, file_types |
| `CommContextSource` | CodeValdComm | lookback_days, max_results |
| `WorkContextSource` | CodeValdWork | include_description, include_history |

**Graph topology:**

```
Agency ──has_work_plan──► WorkPlan ──has_context_source──► GitContextSource
                                   ──has_context_source──► CommContextSource
                                   ──has_context_source──► WorkContextSource
```

**Dispatch flow** (CodeValdAI side — see MVP-AI-018):
1. CodeValdAI receives `work.task.status.changed` with `"To":"in_progress"`
2. Calls `AgencyService/MatchWorkPlans(topic, payload)` → returns matching `WorkPlan`s with their sources
3. For each matched work plan: fetches context from each linked source's target service
4. Assembles a context bundle, triggers `IntakeRun` + `ExecuteRunStreaming` against `workPlan.AgentID`

---

## MVP-AGENCY-010 — RACI Schema: WorkPlan + ContextSource Entity Types

**Status**: ✅ Done
**Branch**: `feature/AGENCY-010_raci_schema`

### Goal

Add four new entity types and two new edge labels to `DefaultAgencySchema()`.
Schema seeding is idempotent — no migration needed for existing deployments.

### New Entity Types

#### WorkPlan

| Property | Type | Notes |
|---|---|---|
| `ref_code` | string | Stable short ID |
| `code` | string | Machine-readable slug |
| `name` | string | Human-readable label |
| `description` | string | What this work plan does |
| `trigger_topic` | string | Go regex matched against the Cross topic |
| `payload_condition` | string | Go regex matched against raw JSON payload; empty = match all |
| `instructions` | string | Prompt template injected into the AgentRun context |
| `agent_id` | string | Cross-service reference to a CodeValdAI `Agent` entity ID |
| `enabled` | bool | When false, work plan is excluded from `MatchWorkPlans` results |
| `ordinality` | int | Ascending sort order when multiple work plans match the same event |

**StorageCollection**: `agency_work_plans` · **Mutable**: true

#### GitContextSource

| Property | Type | Notes |
|---|---|---|
| `ref_code` | string | Stable short ID |
| `code` | string | Machine-readable slug |
| `signals` | string | Comma-separated signal layers, e.g. `"authority,contributor"` |
| `max_results` | int | Cap on files returned (default 20) |
| `match_mode` | string | `"AND"` or `"OR"` (default `"OR"`) |
| `cascade` | bool | Expand keywords to taxonomy descendants when true |
| `file_types` | string | Comma-separated extensions filter, e.g. `".go,.ts"` |

**StorageCollection**: `agency_entities` · **Mutable**: true

#### CommContextSource

| Property | Type | Notes |
|---|---|---|
| `ref_code` | string | Stable short ID |
| `code` | string | Machine-readable slug |
| `lookback_days` | int | How far back to search for threads |
| `max_results` | int | Cap on threads returned |

**StorageCollection**: `agency_entities` · **Mutable**: true

#### WorkContextSource

| Property | Type | Notes |
|---|---|---|
| `ref_code` | string | Stable short ID |
| `code` | string | Machine-readable slug |
| `include_description` | bool | Include full task description in context |
| `include_history` | bool | Include status transition history in context |

**StorageCollection**: `agency_entities` · **Mutable**: true

### New Edge Labels

| From | Edge Label | To |
|---|---|---|
| Agency | `has_work_plan` | WorkPlan |
| WorkPlan | `has_context_source` | GitContextSource, CommContextSource, WorkContextSource |

### Acceptance Criteria

- [x] `DefaultAgencySchema()` includes `WorkPlan`, `GitContextSource`, `CommContextSource`, `WorkContextSource`
- [x] `has_work_plan` and `has_context_source` edge labels declared in schema
- [x] Schema seeded idempotently in `cmd/main.go` — no panics on repeated startup
- [x] `WorkPlan` entities land in `agency_work_plans` collection after seed

---

## MVP-AGENCY-011 — RACI AgencyManager Methods

**Status**: ✅ Done
**Branch**: `feature/AGENCY-011_raci_manager`
**Depends On**: MVP-AGENCY-010

### Goal

Add CRUD methods for `WorkPlan` and `ContextSource` entities to `AgencyManager`,
plus `MatchWorkPlans` — the query CodeValdAI calls on every inbound event.

### New Methods on `AgencyManager`

```go
// WorkPlan CRUD
CreateWorkPlan(ctx context.Context, req CreateWorkPlanRequest) (WorkPlan, error)
GetWorkPlan(ctx context.Context, workPlanID string) (WorkPlan, error)
ListWorkPlans(ctx context.Context) ([]WorkPlan, error)
UpdateWorkPlan(ctx context.Context, workPlanID string, req UpdateWorkPlanRequest) (WorkPlan, error)
DeleteWorkPlan(ctx context.Context, workPlanID string) error

// ContextSource management — typed by ContextSourceType discriminator
AddContextSource(ctx context.Context, workPlanID string, req AddContextSourceRequest) (ContextSource, error)
ListContextSources(ctx context.Context, workPlanID string) ([]ContextSource, error)
RemoveContextSource(ctx context.Context, workPlanID, sourceID string) error

// Dispatch query — called by CodeValdAI on every inbound event.
// Returns all enabled WorkPlans where trigger_topic regex matches topic AND
// payload_condition regex matches the raw payload string.
// Results ordered by ordinality ascending.
MatchWorkPlans(ctx context.Context, topic, payload string) ([]WorkPlanMatch, error)
```

### New Model Types

```go
type WorkPlan struct {
    ID               string
    AgencyID         string
    Name             string
    Description      string
    TriggerTopic     string // Go regex matched against the Cross topic
    PayloadCondition string // Go regex on raw JSON payload; "" = match all
    Instructions     string // prompt template
    AgentID          string // cross-service reference to CodeValdAI Agent
    Enabled          bool
    Ordinality       int
}

type ContextSourceType string

const (
    ContextSourceGit  ContextSourceType = "GitContextSource"
    ContextSourceComm ContextSourceType = "CommContextSource"
    ContextSourceWork ContextSourceType = "WorkContextSource"
)

// ContextSource is a polymorphic wrapper; callers switch on SourceType.
type ContextSource struct {
    ID         string
    WorkPlanID string
    SourceType ContextSourceType
    Git        *GitContextSourceConfig  // non-nil when SourceType == ContextSourceGit
    Comm       *CommContextSourceConfig // non-nil when SourceType == ContextSourceComm
    Work       *WorkContextSourceConfig // non-nil when SourceType == ContextSourceWork
}

type GitContextSourceConfig struct {
    Signals    string
    MaxResults int
    MatchMode  string
    Cascade    bool
    FileTypes  string
}

type CommContextSourceConfig struct {
    LookbackDays int
    MaxResults   int
}

type WorkContextSourceConfig struct {
    IncludeDescription bool
    IncludeHistory     bool
}

type WorkPlanMatch struct {
    WorkPlan       WorkPlan
    ContextSources []ContextSource
}
```

### `MatchWorkPlans` Semantics

1. Fetches all `WorkPlan` entities where `enabled == true` via `has_work_plan` edge from Agency
2. For each, evaluates `regexp.MatchString(workPlan.TriggerTopic, topic)`
3. For passing work plans, evaluates `regexp.MatchString(workPlan.PayloadCondition, payload)` (skipped when `PayloadCondition == ""`)
4. For each matched work plan, fetches all linked `ContextSource` entities via `has_context_source` edges
5. Returns `[]WorkPlanMatch` ordered by `workPlan.Ordinality` ascending

### New Error Types

```go
ErrWorkPlanNotFound      // GetWorkPlan / UpdateWorkPlan / DeleteWorkPlan on unknown ID
ErrContextSourceNotFound // RemoveContextSource on unknown ID
ErrInvalidRegex          // CreateWorkPlan / UpdateWorkPlan when trigger_topic or payload_condition is not a valid Go regex
```

### Acceptance Criteria

- [x] `CreateWorkPlan` stores entity, creates `has_work_plan` edge from Agency
- [x] `CreateWorkPlan` with invalid `trigger_topic` returns `ErrInvalidRegex`
- [x] `MatchWorkPlans("work.task.status.changed", "{...}")` returns work plans whose `trigger_topic` matches
- [x] Regex `"work\\.task\\..*"` matches `work.task.status.changed` and `work.task.completed`
- [x] WorkPlan with `enabled=false` excluded from `MatchWorkPlans`
- [x] `PayloadCondition=""` on a work plan matches any payload
- [x] `PayloadCondition="\"To\":\"in_progress\""` only matches payloads containing that substring
- [x] `AddContextSource` stores typed entity, creates `has_context_source` edge from WorkPlan
- [x] `ListContextSources` returns all source types linked to a work plan
- [x] `RemoveContextSource` removes entity and edge
- [x] `MatchWorkPlans` results ordered by `ordinality` ascending

---

## MVP-AGENCY-012 — RACI gRPC RPCs

**Status**: ✅ Done
**Branch**: `feature/AGENCY-012_raci_grpc`
**Depends On**: MVP-AGENCY-011

### Goal

Expose RACI management over gRPC. `MatchWorkPlans` is the critical RPC — CodeValdAI
calls it on every inbound event. All other RPCs are management surface for
operators and the frontend.

### Proto Additions (`proto/codevaldagency/v1/agency.proto`)

```protobuf
// WorkPlan management
rpc CreateWorkPlan         (CreateWorkPlanRequest)          returns (WorkPlan);
rpc GetWorkPlan            (GetWorkPlanRequest)              returns (WorkPlan);
rpc ListWorkPlans          (ListWorkPlansRequest)            returns (ListWorkPlansResponse);
rpc UpdateWorkPlan         (UpdateWorkPlanRequest)           returns (WorkPlan);
rpc DeleteWorkPlan         (DeleteWorkPlanRequest)           returns (google.protobuf.Empty);

// ContextSource management
rpc AddContextSource   (AddContextSourceRequest)     returns (ContextSource);
rpc ListContextSources (ListContextSourcesRequest)   returns (ListContextSourcesResponse);
rpc RemoveContextSource(RemoveContextSourceRequest)  returns (google.protobuf.Empty);

// Dispatch query — called by CodeValdAI on every inbound event
rpc MatchWorkPlans         (MatchWorkPlansRequest)           returns (MatchWorkPlansResponse);

message MatchWorkPlansRequest {
    string topic   = 1; // e.g. "work.task.status.changed"
    string payload = 2; // raw JSON payload from the event
}
message MatchWorkPlansResponse {
    repeated WorkPlanMatch matches = 1;
}
message WorkPlanMatch {
    WorkPlan               work_plan       = 1;
    repeated ContextSource context_sources = 2;
}
message ContextSource {
    string             id           = 1;
    string             work_plan_id = 2;
    string             source_type  = 3; // "GitContextSource" | "CommContextSource" | "WorkContextSource"
    GitContextSource   git          = 4;
    CommContextSource  comm         = 5;
    WorkContextSource  work         = 6;
}
message GitContextSource  { string signals = 1; int32 max_results = 2; string match_mode = 3; bool cascade = 4; string file_types = 5; }
message CommContextSource { int32 lookback_days = 1; int32 max_results = 2; }
message WorkContextSource { bool include_description = 1; bool include_history = 2; }
```

### Error Mapping

| Domain Error | gRPC Code |
|---|---|
| `ErrWorkPlanNotFound` | `NOT_FOUND` |
| `ErrContextSourceNotFound` | `NOT_FOUND` |
| `ErrInvalidRegex` | `INVALID_ARGUMENT` |

### Cross Route Declarations

| Method | Pattern | gRPC Method |
|---|---|---|
| `POST` | `/agency/{agencyId}/work-plans` | `AgencyService/CreateWorkPlan` |
| `GET` | `/agency/{agencyId}/work-plans` | `AgencyService/ListWorkPlans` |
| `GET` | `/agency/{agencyId}/work-plans/{workPlanId}` | `AgencyService/GetWorkPlan` |
| `PUT` | `/agency/{agencyId}/work-plans/{workPlanId}` | `AgencyService/UpdateWorkPlan` |
| `DELETE` | `/agency/{agencyId}/work-plans/{workPlanId}` | `AgencyService/DeleteWorkPlan` |
| `POST` | `/agency/{agencyId}/work-plans/{workPlanId}/context-sources` | `AgencyService/AddContextSource` |
| `GET` | `/agency/{agencyId}/work-plans/{workPlanId}/context-sources` | `AgencyService/ListContextSources` |
| `DELETE` | `/agency/{agencyId}/work-plans/{workPlanId}/context-sources/{sourceId}` | `AgencyService/RemoveContextSource` |

### Acceptance Criteria

- [x] Proto file updated and generated code committed under `gen/go/`
- [x] `internal/server/server.go` implements all nine RPCs
- [x] `internal/server/errors.go` maps `ErrWorkPlanNotFound`, `ErrContextSourceNotFound`, `ErrInvalidRegex`
- [x] `MatchWorkPlans` RPC returns `WorkPlanMatch` with all linked sources in one call

---

## MVP-AGENCY-013 — Cross Registration & Route Declarations

**Status**: 🔲 Not Started
**Branch**: `feature/AGENCY-013_raci_cross_registration`
**Depends On**: MVP-AGENCY-012

### Goal

Register all eight RACI HTTP routes with CodeValdCross on every heartbeat.

### Topics

No new topics produced or consumed by CodeValdAgency for RACI.
`work.task.status.changed` is consumed by **CodeValdAI** (see MVP-AI-018), not Agency.

### Acceptance Criteria

- [ ] All eight RACI routes added to `DeclaredRoutes` in `internal/registrar/registrar.go`
- [ ] Routes accessible via CodeValdCross HTTP proxy after first successful heartbeat

---

## MVP-AGENCY-014 — Unit & Integration Tests

**Status**: ✅ Done
**Branch**: `feature/AGENCY-014_raci_tests`
**Depends On**: MVP-AGENCY-011, MVP-AGENCY-012

### Test Matrix

- `CreateWorkPlan` with valid regexes → entity stored, `has_work_plan` edge created
- `CreateWorkPlan` with invalid `trigger_topic` regex → `ErrInvalidRegex`
- `CreateWorkPlan` with invalid `payload_condition` regex → `ErrInvalidRegex`
- `MatchWorkPlans("work.task.status.changed", payload)` matches work plan with topic `"work\\.task\\.status\\.changed"`
- `MatchWorkPlans` with wildcard topic `"work\\.task\\..*"` matches `work.task.status.changed` and `work.task.completed`
- `MatchWorkPlans` with non-matching topic → returns empty list
- `MatchWorkPlans` with `payload_condition` `"\"To\":\"in_progress\""` → only matches when payload contains that token
- `MatchWorkPlans` excludes `enabled=false` work plans
- `MatchWorkPlans` returns results ordered by `ordinality` ascending
- `AddContextSource(GitContextSource)` → `ListContextSources` returns it with correct typed fields
- `AddContextSource(CommContextSource)` + `AddContextSource(WorkContextSource)` → all three appear in `ListContextSources`
- `RemoveContextSource` → source absent from subsequent `ListContextSources`
- gRPC `MatchWorkPlans` RPC returns `WorkPlanMatch` with all linked sources
- gRPC `CreateWorkPlan` with invalid regex → `INVALID_ARGUMENT`
