## Session — 2026-05-06

**Branch**: `main`
**Focus**: Rename `Task`/`Role` → `WorkPlan`; consolidate `event_topic`+`pubsub_topic` → `trigger_topic`

---

### Context

An ArchiMate enterprise architecture review (Henrik von Scheel™ framework —
Strategy / Purpose & Goal / Competency / Service / Process layers) showed that
the `Task` entity name was confusing: it conflated the *process trigger* concept
with the *work item* concept. The chosen replacement is `WorkPlan` — an entity
that **binds a trigger topic regex, optional context sources, and an agent** into
a single dispatchable plan.

The implementation had diverged across layers:
- **Domain layer** (`raci.go`, `agency.go`, `models.go`, `errors.go`) still used `Task`
- **Proto + test files** were partially renamed to `Role` in an earlier pass
- The rename to `WorkPlan` needed to unify both layers simultaneously

Additionally, the two separate fields `event_topic` and `pubsub_topic` were
collapsed into a single `trigger_topic` field — one Go regex that matches the
incoming Cross topic.

---

### Objectives

- [x] Rename `Task` → `WorkPlan` throughout all layers
- [x] Consolidate `event_topic` + `pubsub_topic` → `trigger_topic`
- [x] Regenerate proto-generated code via `buf generate`
- [x] All tests pass with zero compilation errors

---

### Changes Made

#### schema.go
- `TypeDefinition` name `"Task"` → `"WorkPlan"`
- `StorageCollection` `"agency_entities"` → `"agency_work_plans"` (WorkPlan gets its own collection)
- Agency relationship edge `has_task` → `has_work_plan`; ToType `"Task"` → `"WorkPlan"`
- WorkPlan property `event_topic` + `pubsub_topic` → single `trigger_topic`
- ConfiguredRole inverse edge `assigned_task` → `assigned_work_plan`
- All three ContextSource types: `belongs_to_task` → `belongs_to_work_plan`

#### models.go
- `Task` struct → `WorkPlan`; field `EventTopic` → `TriggerTopic`
- `CreateTaskRequest` → `CreateWorkPlanRequest`; field `EventTopic` → `TriggerTopic`
- `UpdateTaskRequest` → `UpdateWorkPlanRequest`; field `EventTopic` → `TriggerTopic`
- `TaskMatch` → `WorkPlanMatch`; field `Task Task` → `WorkPlan WorkPlan`
- `ContextSource.TaskID` → `ContextSource.WorkPlanID`

#### errors.go
- `ErrTaskNotFound` → `ErrWorkPlanNotFound = errors.New("work plan not found")`
- `ErrInvalidRegex` message updated: `"trigger_topic or payload_condition"`

#### raci.go
- All methods renamed: `CreateTask→CreateWorkPlan`, `GetTask→GetWorkPlan`, `ListTasks→ListWorkPlans`, `UpdateTask→UpdateWorkPlan`, `DeleteTask→DeleteWorkPlan`, `MatchTasks→MatchWorkPlans`
- TypeID `"Task"` → `"WorkPlan"`; relationship `"has_task"` → `"has_work_plan"`
- Property key `"event_topic"` → `"trigger_topic"`; `"pubsub_topic"` removed
- `entityToTask` → `entityToWorkPlan`; `validateTaskRegexes` → `validateWorkPlanRegex`
- `ContextSource.TaskID` → `ContextSource.WorkPlanID` in helper

#### agency.go
- Interface comment updated; all 9 method signatures renamed/updated

#### raci_test.go
- `mustSetupRole` → `mustSetupWorkPlan`; return type `Role` → `WorkPlan`
- All request/response types updated: `CreateRoleRequest→CreateWorkPlanRequest`, etc.
- `EventTopic:` → `TriggerTopic:` throughout
- `ErrRoleNotFound` → `ErrWorkPlanNotFound`

#### proto/codevaldagency/v1/agency.proto
- `message Role` → `message WorkPlan`; field `event_topic` at position 5 → `trigger_topic`
- `ContextSource.role_id` → `ContextSource.work_plan_id`
- `message RoleMatch` → `message WorkPlanMatch`; field `role` → `work_plan`
- All request/response messages and RPC names updated

#### gen/go/codevaldagency/v1/ (regenerated)
- `buf generate` re-run after installing `protoc-gen-go` and `protoc-gen-go-grpc`

#### internal/server/server.go
- All Task/Role RPC handlers replaced with WorkPlan equivalents
- `taskToProto` → `workPlanToProto` returning `*pb.WorkPlan` with `TriggerTopic`
- `contextSourceToProto` fixed (pre-existing truncation bug) and updated to use `WorkPlanId`
- `MatchWorkPlans` handler returns `*pb.MatchWorkPlansResponse` with `*pb.WorkPlanMatch`

#### internal/server/errors.go
- Added `ErrWorkPlanNotFound` → `codes.NotFound` mapping

#### internal/server/server_test.go
- Mock struct fields renamed: `createRoleResult→createWorkPlanResult`, etc.
- Mock methods renamed to match updated `AgencyManager` interface

#### internal/server/raci_server_test.go
- All tests rewritten for `WorkPlan` naming; `EventTopic` → `TriggerTopic`; `ErrRoleNotFound` → `ErrWorkPlanNotFound`

---

### Testing

```
ok  github.com/aosanya/CodeValdAgency              0.002s
ok  github.com/aosanya/CodeValdAgency/internal/server  0.005s
ok  github.com/aosanya/CodeValdAgency/storage/arangodb  0.002s
```

---

### Issues Resolved

- **proto plugins missing**: `protoc-gen-go` and `protoc-gen-go-grpc` were not on `PATH`.
  Fixed by `go install` + prepending `$(go env GOPATH)/bin` when running `buf generate`.
- **`contextSourceToProto` truncation**: Pre-existing bug where the function body was
  cut off mid-implementation. Fixed as part of the WorkPlan rename pass.

---

### Files Modified

```
schema.go
models.go
errors.go
raci.go
agency.go
raci_test.go
proto/codevaldagency/v1/agency.proto
gen/go/codevaldagency/v1/agency.pb.go
gen/go/codevaldagency/v1/agency_grpc.pb.go
internal/server/server.go
internal/server/errors.go
internal/server/server_test.go
internal/server/raci_server_test.go
documentation/3-SofwareDevelopment/mvp-details/raci-matrix.md
documentation/2-SoftwareDesignAndArchitecture/architecture-interfaces.md
```

---

### Next Steps

- [ ] MVP-AGENCY-013: Register WorkPlan HTTP routes with CodeValdCross
- [ ] Update CodeValdAI to call `MatchWorkPlans` instead of `MatchRoles`
- [ ] Update CodeValdAgencyFrontend route/component names if they reference `Role`
