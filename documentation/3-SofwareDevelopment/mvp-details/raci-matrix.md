# RACI Matrix — Implementation Details

Topics: Event Routing · Role-Based Dispatch · Context Assembly · Cross-Service Integration

---

## Overview

The RACI matrix is a composable, queryable event-routing layer stored in
CodeValdAgency. It maps **agency roles** to the events they respond to and
the context they need — so CodeValdAI can determine, for any inbound event,
which agent to invoke and exactly what to fetch before running.

Each `Role` entity carries:
- **event_topic** — Go regex matched against the incoming Cross topic
- **payload_condition** — Go regex matched against the raw JSON payload string
- **instructions** — prompt template injected into the triggered `AgentRun`
- **agent_id** — cross-service string reference to a CodeValdAI `Agent` entity
- **enabled** / **ordinality** — activation flag and ordering when multiple roles match

Context is assembled from typed `ContextSource` entities linked to the `Role`:

| Type | Service Queried | Parameters |
|---|---|---|
| `GitContextSource` | CodeValdGit | signals, max_results, match_mode, cascade, file_types |
| `CommContextSource` | CodeValdComm | lookback_days, max_results |
| `WorkContextSource` | CodeValdWork | include_description, include_history |

**Graph topology:**

```
Agency ──has_role──► Role ──has_context_source──► GitContextSource
                         ──has_context_source──► CommContextSource
                         ──has_context_source──► WorkContextSource
```

**Dispatch flow** (CodeValdAI side — see MVP-AI-018):
1. CodeValdAI receives `work.task.status.changed` with `"To":"in_progress"`
2. Calls `AgencyService/MatchRoles(topic, payload)` → returns matching `Role`s with their sources
3. For each matched role: fetches context from each linked source's target service
4. Assembles a context bundle, triggers `IntakeRun` + `ExecuteRunStreaming` against `role.AgentID`

---

## MVP-AGENCY-010 — RACI Schema: Role + ContextSource Entity Types

**Status**: 🔲 Not Started
**Branch**: `feature/AGENCY-010_raci_schema`

### Goal

Add four new entity types and two new edge labels to `DefaultAgencySchema()`.
Schema seeding is idempotent — no migration needed for existing deployments.

### New Entity Types

#### Role

| Property | Type | Notes |
|---|---|---|
| `ref_code` | string | Stable short ID |
| `code` | string | Machine-readable slug |
| `name` | string | Human-readable label |
| `description` | string | What this role does |
| `event_topic` | string | Go regex matched against the Cross topic |
| `payload_condition` | string | Go regex matched against raw JSON payload; empty = match all |
| `instructions` | string | Prompt template injected into the AgentRun context |
| `agent_id` | string | Cross-service reference to a CodeValdAI `Agent` entity ID |
| `enabled` | bool | When false, role is excluded from `MatchRoles` results |
| `ordinality` | int | Ascending sort order when multiple roles match the same event |

**StorageCollection**: `agency_entities` · **Mutable**: true

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
| Agency | `has_role` | Role |
| Role | `has_context_source` | GitContextSource, CommContextSource, WorkContextSource |

### Acceptance Criteria

- [ ] `DefaultAgencySchema()` includes `Role`, `GitContextSource`, `CommContextSource`, `WorkContextSource`
- [ ] `has_role` and `has_context_source` edge labels declared in schema
- [ ] Schema seeded idempotently in `cmd/main.go` — no panics on repeated startup
- [ ] All four types land in `agency_entities` collection after seed

---

## MVP-AGENCY-011 — RACI AgencyManager Methods

**Status**: 🔲 Not Started
**Branch**: `feature/AGENCY-011_raci_manager`
**Depends On**: MVP-AGENCY-010

### Goal

Add CRUD methods for `Role` and `ContextSource` entities to `AgencyManager`,
plus `MatchRoles` — the query CodeValdAI calls on every inbound event.

### New Methods on `AgencyManager`

```go
// Role CRUD
CreateRole(ctx context.Context, req CreateRoleRequest) (Role, error)
GetRole(ctx context.Context, roleID string) (Role, error)
ListRoles(ctx context.Context) ([]Role, error)
UpdateRole(ctx context.Context, roleID string, req UpdateRoleRequest) (Role, error)
DeleteRole(ctx context.Context, roleID string) error

// ContextSource management — typed by ContextSourceType discriminator
AddContextSource(ctx context.Context, roleID string, req AddContextSourceRequest) (ContextSource, error)
ListContextSources(ctx context.Context, roleID string) ([]ContextSource, error)
RemoveContextSource(ctx context.Context, roleID, sourceID string) error

// Dispatch query — called by CodeValdAI on every inbound event.
// Returns all enabled Roles where event_topic regex matches topic AND
// payload_condition regex matches the raw payload string.
// Results ordered by ordinality ascending.
MatchRoles(ctx context.Context, topic, payload string) ([]RoleMatch, error)
```

### New Model Types

```go
type Role struct {
    ID               string
    AgencyID         string
    Name             string
    Description      string
    EventTopic       string // Go regex
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
    RoleID     string
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

type RoleMatch struct {
    Role           Role
    ContextSources []ContextSource
}
```

### `MatchRoles` Semantics

1. Fetches all `Role` entities where `enabled == true` via `has_role` edge from Agency
2. For each, evaluates `regexp.MatchString(role.EventTopic, topic)`
3. For passing roles, evaluates `regexp.MatchString(role.PayloadCondition, payload)` (skipped when `PayloadCondition == ""`)
4. For each matched role, fetches all linked `ContextSource` entities via `has_context_source` edges
5. Returns `[]RoleMatch` ordered by `role.Ordinality` ascending

### New Error Types

```go
ErrRoleNotFound          // GetRole / UpdateRole / DeleteRole on unknown ID
ErrContextSourceNotFound // RemoveContextSource on unknown ID
ErrInvalidRegex          // CreateRole / UpdateRole when event_topic or payload_condition is not a valid Go regex
```

### Acceptance Criteria

- [ ] `CreateRole` stores entity, creates `has_role` edge from Agency
- [ ] `CreateRole` with invalid `event_topic` returns `ErrInvalidRegex`
- [ ] `MatchRoles("work.task.status.changed", "{...}")` returns roles whose `event_topic` matches
- [ ] Regex `"work\\.task\\..*"` matches `work.task.status.changed` and `work.task.completed`
- [ ] Role with `enabled=false` excluded from `MatchRoles`
- [ ] `PayloadCondition=""` on a role matches any payload
- [ ] `PayloadCondition="\"To\":\"in_progress\""` only matches payloads containing that substring
- [ ] `AddContextSource` stores typed entity, creates `has_context_source` edge from Role
- [ ] `ListContextSources` returns all source types linked to a role
- [ ] `RemoveContextSource` removes entity and edge
- [ ] `MatchRoles` results ordered by `ordinality` ascending

---

## MVP-AGENCY-012 — RACI gRPC RPCs

**Status**: 🔲 Not Started
**Branch**: `feature/AGENCY-012_raci_grpc`
**Depends On**: MVP-AGENCY-011

### Goal

Expose RACI management over gRPC. `MatchRoles` is the critical RPC — CodeValdAI
calls it on every inbound event. All other RPCs are management surface for
operators and the frontend.

### Proto Additions (`proto/codevaldagency/v1/agency.proto`)

```protobuf
// Role management
rpc CreateRole         (CreateRoleRequest)          returns (Role);
rpc GetRole            (GetRoleRequest)              returns (Role);
rpc ListRoles          (ListRolesRequest)            returns (ListRolesResponse);
rpc UpdateRole         (UpdateRoleRequest)           returns (Role);
rpc DeleteRole         (DeleteRoleRequest)           returns (google.protobuf.Empty);

// ContextSource management
rpc AddContextSource   (AddContextSourceRequest)     returns (ContextSource);
rpc ListContextSources (ListContextSourcesRequest)   returns (ListContextSourcesResponse);
rpc RemoveContextSource(RemoveContextSourceRequest)  returns (google.protobuf.Empty);

// Dispatch query — called by CodeValdAI on every inbound event
rpc MatchRoles         (MatchRolesRequest)           returns (MatchRolesResponse);

message MatchRolesRequest {
    string topic   = 1; // e.g. "work.task.status.changed"
    string payload = 2; // raw JSON payload from the event
}
message MatchRolesResponse {
    repeated RoleMatch matches = 1;
}
message RoleMatch {
    Role                   role            = 1;
    repeated ContextSource context_sources = 2;
}
message ContextSource {
    string             id          = 1;
    string             source_type = 2; // "GitContextSource" | "CommContextSource" | "WorkContextSource"
    GitContextSource   git         = 3;
    CommContextSource  comm        = 4;
    WorkContextSource  work        = 5;
}
message GitContextSource  { string signals = 1; int32 max_results = 2; string match_mode = 3; bool cascade = 4; string file_types = 5; }
message CommContextSource { int32 lookback_days = 1; int32 max_results = 2; }
message WorkContextSource { bool include_description = 1; bool include_history = 2; }
```

### Error Mapping

| Domain Error | gRPC Code |
|---|---|
| `ErrRoleNotFound` | `NOT_FOUND` |
| `ErrContextSourceNotFound` | `NOT_FOUND` |
| `ErrInvalidRegex` | `INVALID_ARGUMENT` |

### Cross Route Declarations

| Method | Pattern | gRPC Method |
|---|---|---|
| `POST` | `/agency/{agencyId}/roles` | `AgencyService/CreateRole` |
| `GET` | `/agency/{agencyId}/roles` | `AgencyService/ListRoles` |
| `GET` | `/agency/{agencyId}/roles/{roleId}` | `AgencyService/GetRole` |
| `PUT` | `/agency/{agencyId}/roles/{roleId}` | `AgencyService/UpdateRole` |
| `DELETE` | `/agency/{agencyId}/roles/{roleId}` | `AgencyService/DeleteRole` |
| `POST` | `/agency/{agencyId}/roles/{roleId}/context-sources` | `AgencyService/AddContextSource` |
| `GET` | `/agency/{agencyId}/roles/{roleId}/context-sources` | `AgencyService/ListContextSources` |
| `DELETE` | `/agency/{agencyId}/roles/{roleId}/context-sources/{sourceId}` | `AgencyService/RemoveContextSource` |

### Acceptance Criteria

- [ ] Proto file updated and generated code committed under `gen/go/`
- [ ] `internal/server/server.go` implements all nine RPCs
- [ ] `internal/server/errors.go` maps `ErrRoleNotFound`, `ErrContextSourceNotFound`, `ErrInvalidRegex`
- [ ] `MatchRoles` RPC returns `RoleMatch` with all linked sources in one call

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

**Status**: 🔲 Not Started
**Branch**: `feature/AGENCY-014_raci_tests`
**Depends On**: MVP-AGENCY-011, MVP-AGENCY-012

### Test Matrix

- `CreateRole` with valid regexes → entity stored, `has_role` edge created
- `CreateRole` with invalid `event_topic` regex → `ErrInvalidRegex`
- `CreateRole` with invalid `payload_condition` regex → `ErrInvalidRegex`
- `MatchRoles("work.task.status.changed", payload)` matches role with topic `"work\\.task\\.status\\.changed"`
- `MatchRoles` with wildcard topic `"work\\.task\\..*"` matches `work.task.status.changed` and `work.task.completed`
- `MatchRoles` with non-matching topic → returns empty list
- `MatchRoles` with `payload_condition` `"\"To\":\"in_progress\""` → only matches when payload contains that token
- `MatchRoles` excludes `enabled=false` roles
- `MatchRoles` returns results ordered by `ordinality` ascending
- `AddContextSource(GitContextSource)` → `ListContextSources` returns it with correct typed fields
- `AddContextSource(CommContextSource)` + `AddContextSource(WorkContextSource)` → all three appear in `ListContextSources`
- `RemoveContextSource` → source absent from subsequent `ListContextSources`
- gRPC `MatchRoles` RPC returns `RoleMatch` with all linked sources
- gRPC `CreateRole` with invalid regex → `INVALID_ARGUMENT`
