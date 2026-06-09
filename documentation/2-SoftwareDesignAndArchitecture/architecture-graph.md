# CodeValdAgency — Graph Topology & Schema

> Part of the split architecture. Index: [architecture.md](architecture.md)

---

## 1. Graph Topology

### Authoring sub-graph

```
Agency ──has_goal──────────────► Goal
       ──has_workflow──────────► Workflow ──has_work_item──────► WorkItem
       ──has_configured_role──► ConfiguredRole                        │
       ──has_snapshot─────────► AgencySnapshot   (Immutable)          ├──has_instruction──► Instruction ──has_content_ref──► ContentRef
       ──has_publication──────► AgencyPublication (Immutable)         ├──has_deliverable──► Deliverable
       ──has_draft────────────► AgencyDraft (Mutable)                 ├──has_content_ref──► ContentRef
                                    │                                 └──assigned_role────► ConfiguredRole
                                    ├──has_goal──────────────► Goal   (draft copy)
                                    ├──has_workflow──────────► Workflow (draft copy)
                                    └──has_configured_role──► ConfiguredRole (draft copy)

                               AgencyPublication ──has_status──► AgencyPublicationStatus (mutable)

Deliverable ──has_result──────► DeliverableResult ──has_content_ref──► ContentRef
            ──reviewer_role───► ConfiguredRole  (waiver authority)
```

### Dispatch + AI configuration sub-graph

```
Agency ──has_work_plan─────────► WorkPlan ──has_context_source──► GitContextSource
                                          ──has_context_source──► CommContextSource
                                          ──has_context_source──► WorkContextSource
                                          ──assigned_role───────► ConfiguredRole
                                          ──has_work_item───────► WorkItem
       ──has_ai_provider───────► AIProvider
       ──has_ai_agent──────────► AIAgent
```

`MatchWorkPlans` (called by CodeValdAI when a dispatched topic arrives) returns
every `WorkPlan` whose `trigger_topic` regex matches the incoming Cross topic
and whose `payload_condition` regex matches the raw JSON payload. The matched
plan's `has_context_source` edges drive how Git/Comm/Work context is assembled
before the plan's handler (`codevaldai`, `codevaldfunction`, or `codevaldcomm`)
is invoked.

All authoring nodes live in two ArangoDB collections:
- **`agency_entities`** — document collection (most mutable entity types)
- **`agency_relationships`** — **edge** collection (all relationship types)
- Dispatch/AI types (`WorkPlan`, `*ContextSource`, `AIProvider`, `AIAgent`) and
  immutable types use dedicated collections (see §4).

---

## 2. Entity Types

| Type | Mutable | Properties | Notes |
|---|---|---|---|
| `Agency` | ✅ | `name`(req), `mission`, `vision`, `enabled`(req) | Root entity; one per database; read-only after first publish |
| `Goal` | ✅ | `title`(req), `description`, `ordinality`(req) | Strategic objective |
| `Workflow` | ✅ | `name`(req), `description`, `ordinality`(req) | Ordered container of WorkItems |
| `WorkItem` | ✅ | `title`(req), `description`, `ordinality`(req), `prompt` | Unit of work; same `ordinality` = parallel execution |
| `Instruction` | ✅ | `content`(req), `ordinality`(req) | Rule or constraint; attaches to Workflow or WorkItem |
| `Deliverable` | ✅ | `title`(req), `description`, `ordinality`(req), `blocking`(req) | Spec: expected output from a WorkItem |
| `DeliverableResult` | ❌ immutable | `status`(req), `produced_at` | Instance: actual output submitted against a Deliverable |
| `ContentRef` | ❌ immutable | `path`(req) | CodeValdGit artifact path; attaches to DeliverableResult, Instruction, or WorkItem |
| `ConfiguredRole` | ✅ | `name`(req), `description`, `actor_type`(req), `ordinality`(req) | `actor_type`: `"human"` / `"ai_agent"` / `"compute_agent"` |
| `AgencyDraft` | ✅ | `description`(req), `status`(req), `forked_from_id`, `forked_from_type`, `created_at`, `updated_at` | Mutable full-copy of the agency graph; `status`: `"open"` / `"promoted"` / `"archived"` |
| `AgencySnapshot` | ❌ immutable | `snapshot_at`(req) | Written on `PromoteDraft`; promotion audit record |
| `AgencyPublication` | ❌ immutable | `version`(req), `tag`(req), `published_at`(req) | Content record; status is stored in the linked `AgencyPublicationStatus` entity |
| `AgencyPublicationStatus` | ✅ | `status`(req) | Mutable status node (`"draft"` / `"active"` / `"archived"`) linked via `has_status` |
| `WorkPlan` | ✅ | `ref_code`(req), `code`, `name`(req), `description`, `trigger_topic`(req), `payload_condition`, `instructions`, `agent_id`, `agent_code`, `handler_service`, `function_code`, `function_params`, `enabled`(req), `ordinality`(req), `success_event`, `failure_event`, `on_failure_pipeline`, `step_timeout`, `review_step_type`, `review_trigger_topic`, `review_success_topic`, `review_failure_topic` | Dispatch rule: binds a Cross trigger topic to a handler service + ConfiguredRole + WorkItem. `handler_service` ∈ {`codevaldai`, `codevaldfunction`, `codevaldcomm`}; `function_params` is a JSON-encoded options object |
| `GitContextSource` | ✅ | `ref_code`(req), `code`, `signals`, `max_results`, `match_mode`, `cascade`, `file_types` | Configures what CodeValdGit files to fetch for the AgentRun bundle; `signals` is a comma list (e.g. `"authority,contributor"`); `match_mode` ∈ {`"AND"`, `"OR"`} |
| `CommContextSource` | ✅ | `ref_code`(req), `code`, `lookback_days`, `max_results` | Configures what CodeValdComm threads to fetch for the AgentRun bundle |
| `WorkContextSource` | ✅ | `ref_code`(req), `code`, `include_description`, `include_history` | Configures what CodeValdWork task fields to fetch for the AgentRun bundle |
| `AIProvider` | ✅ | `ref_code`(req), `code`(req), `name`(req), `provider_type`, `api_key_env`, `base_url`, `provider_route` | LLM provider config declared in `ai_config`; `code` is unique within the agency |
| `AIAgent` | ✅ | `ref_code`(req), `code`(req), `name`(req), `provider_code`, `model`, `system_prompt`, `temperature`, `max_tokens`, `session_max_seconds`, `session_max_tokens`, `session_max_sessions` | LLM agent config declared in `ai_config`; `provider_code` resolves to an `AIProvider`; `code` is unique within the agency |

---

## 3. Relationship Types

Edges are stored in `agency_relationships`. Each edge has `_from`, `_to`, `name` (the label), and an optional `properties` map.

**`ToMany=false`** (upsert) — at most one edge of that label from the source.  
**`ToMany=true`** (insert) — collection of edges.  
**`Inverse`** — `CreateRelationship` writes both forward + inverse edges atomically.  
**`Required=true`** — `CreateEntity` must supply that relationship inline; omitting it returns `ErrRequiredRelationshipViolation`.

### Forward relationships

| Label | From | To | ToMany | Inverse |
|---|---|---|---|---|
| `has_goal` | `Agency` | `Goal` | ✅ | `belongs_to_agency` |
| `has_workflow` | `Agency` | `Workflow` | ✅ | `belongs_to_agency` |
| `has_configured_role` | `Agency` | `ConfiguredRole` | ✅ | `belongs_to_agency` |
| `has_snapshot` | `Agency` | `AgencySnapshot` | ✅ | `belongs_to_agency` |
| `has_publication` | `Agency` | `AgencyPublication` | ✅ | `belongs_to_agency` |
| `has_draft` | `Agency` | `AgencyDraft` | ✅ | `belongs_to_agency` |
| `has_work_item` | `Workflow` | `WorkItem` | ✅ | `belongs_to_workflow` |
| `has_instruction` | `Workflow` | `Instruction` | ✅ | `belongs_to_workflow` |
| `has_instruction` | `WorkItem` | `Instruction` | ✅ | `belongs_to_work_item` |
| `has_deliverable` | `WorkItem` | `Deliverable` | ✅ | `belongs_to_work_item` |
| `has_content_ref` | `WorkItem` | `ContentRef` | ✅ | `belongs_to_work_item` |
| `assigned_role` | `WorkItem` | `ConfiguredRole` | ✅ | `assigned_work_item` |
| `has_content_ref` | `Instruction` | `ContentRef` | ✅ | `belongs_to_instruction` |
| `has_result` | `Deliverable` | `DeliverableResult` | ✅ | `belongs_to_deliverable` |
| `reviewer_role` | `Deliverable` | `ConfiguredRole` | ❌ | `reviews_deliverable` |
| `has_content_ref` | `DeliverableResult` | `ContentRef` | ✅ | `belongs_to_result` |
| `has_status` | `AgencyPublication` | `AgencyPublicationStatus` | ❌ | `belongs_to_publication` |
| `has_work_plan` | `Agency` | `WorkPlan` | ✅ | `belongs_to_agency` |
| `has_ai_provider` | `Agency` | `AIProvider` | ✅ | `belongs_to_agency` |
| `has_ai_agent` | `Agency` | `AIAgent` | ✅ | `belongs_to_agency` |
| `has_context_source` | `WorkPlan` | `GitContextSource` \| `CommContextSource` \| `WorkContextSource` † | ✅ | `belongs_to_work_plan` |
| `assigned_role` | `WorkPlan` | `ConfiguredRole` | ❌ | `assigned_work_plan` |
| `has_work_item` | `WorkPlan` | `WorkItem` | ❌ | — |

† The `has_context_source` `RelationshipDefinition` in `schema.go` declares
`ToType: "GitContextSource"`. In practice each `*ContextSource` type carries
its own `belongs_to_work_plan` inverse, so a single `WorkPlan` may link any
mix of the three variants via the same `has_context_source` label.

### Inverse relationships (auto-written by `CreateRelationship`)

| Label | On Type | Points To | Required |
|---|---|---|---|
| `belongs_to_agency` | `Goal`, `Workflow`, `ConfiguredRole`, `AgencySnapshot`, `AgencyPublication` | `Agency` | ✅ |
| `belongs_to_workflow` | `WorkItem` | `Workflow` | ✅ |
| `belongs_to_workflow` | `Instruction` | `Workflow` | — |
| `belongs_to_work_item` | `Instruction` | `WorkItem` | — |
| `belongs_to_work_item` | `Deliverable` | `WorkItem` | ✅ |
| `belongs_to_work_item` | `ContentRef` | `WorkItem` | — |
| `assigned_work_item` | `ConfiguredRole` | `WorkItem` | — |
| `belongs_to_instruction` | `ContentRef` | `Instruction` | — |
| `belongs_to_deliverable` | `DeliverableResult` | `Deliverable` | ✅ |
| `reviews_deliverable` | `ConfiguredRole` | `Deliverable` | — |
| `belongs_to_result` | `ContentRef` | `DeliverableResult` | — |
| `belongs_to_publication` | `AgencyPublicationStatus` | `AgencyPublication` | — |
| `belongs_to_agency` | `WorkPlan`, `AIProvider`, `AIAgent` | `Agency` | ✅ |
| `belongs_to_work_plan` | `GitContextSource`, `CommContextSource`, `WorkContextSource` | `WorkPlan` | ✅ |
| `assigned_work_plan` | `ConfiguredRole` | `WorkPlan` | — |
| `belongs_to_draft` | `Goal`, `Workflow`, `ConfiguredRole` | `AgencyDraft` | — |
| `belongs_to_goal` | `WorkItem` | `Goal` | — |

---

## 4. Pre-Delivered Schema

`schema.go` exposes `DefaultAgencySchema()`. `cmd/main.go` seeds this idempotently on startup via `AgencySchemaManager.SetSchema`.

### TypeDefinitions

| Type | Immutable | StorageCollection |
|---|---|---|
| `Agency` | — | `agency_entities` (default) |
| `Goal` | — | `agency_entities` (default) |
| `Workflow` | — | `agency_entities` (default) |
| `WorkItem` | — | `agency_entities` (default) |
| `Instruction` | — | `agency_entities` (default) |
| `Deliverable` | — | `agency_entities` (default) |
| `DeliverableResult` | **true** | `agency_entities` (default) |
| `ContentRef` | **true** | `agency_entities` (default) |
| `ConfiguredRole` | — | `agency_entities` (default) |
| `AgencyDraft` | — | `agency_drafts` (explicit) |
| `DraftGoal` | — | `agency_draft_entities` (explicit) |
| `DraftWorkflow` | — | `agency_draft_entities` (explicit) |
| `DraftWorkItem` | — | `agency_draft_entities` (explicit) |
| `DraftConfiguredRole` | — | `agency_draft_entities` (explicit) |
| `DraftInstruction` | — | `agency_draft_entities` (explicit) |
| `DraftDeliverable` | — | `agency_draft_entities` (explicit) |
| `DraftDeliverableResult` | — | `agency_draft_entities` (explicit) |
| `AgencySnapshot` | **true** | `agency_entities` (default) |
| `AgencyPublication` | **true** | `agency_entities` (default) |
| `AgencyPublicationStatus` | — | `agency_entities` (default) |
| `WorkPlan` | — | `agency_work_plans` (explicit) |
| `GitContextSource` | — | `agency_git_context_sources` (explicit) |
| `CommContextSource` | — | `agency_comm_context_sources` (explicit) |
| `WorkContextSource` | — | `agency_work_context_sources` (explicit) |
| `AIProvider` | — | `agency_ai_providers` (explicit) |
| `AIAgent` | — | `agency_ai_agents` (explicit) |

> Drafts use **dedicated `Draft*` types**, not the live types scoped by
> `draft_id`. Each draft sub-entity carries a `draft_ref_code` property that
> ties it to its `AgencyDraft` root. The single `agency_relationships` edge
> collection spans every vertex collection via full ArangoDB document
> handles.

**`Immutable: true`** — `UpdateEntity` returns `ErrImmutableType` for these types. Each submission or review decision creates a new record, giving a full audit trail.

> **Publication status exception**: `AgencyPublication` is immutable (version, tag, published\_at never change), but its lifecycle status (`draft`/`active`/`archived`) must be mutable. This is handled by the separate `AgencyPublicationStatus` entity, linked via `has_status`. `UpdatePublicationStatus` updates the `AgencyPublicationStatus` entity, leaving the immutable publication record untouched.

### `DeliverableResult` status lifecycle

```
(actor submits)
     ▼
  pending ──► completed   (reviewer accepts, or auto-accepted when blocking=false)
          ──► rejected    (reviewer rejects)
                 │
                 └──► waived   (reviewer_role actor waives; unblocks Workflow)
```

When `Deliverable.blocking=true` and the latest `DeliverableResult.status` is `"rejected"`, the Workflow engine must not advance past the owning `WorkItem` until a new result reaches `"completed"` or `"waived"`.

### `ContentRef` multi-parent pattern

`ContentRef` follows the same optional multi-parent pattern as `Instruction`. A single `ContentRef` attaches to whichever parent is relevant — none of its `belongs_to_*` relationships are `Required`:

| Relationship | Parent | Meaning |
|---|---|---|
| `belongs_to_result` | `DeliverableResult` | Output artifact committed to CodeValdGit |
| `belongs_to_instruction` | `Instruction` | Supporting material for a rule or constraint |
| `belongs_to_work_item` | `WorkItem` | Input reference material given to the actor |

### `ordinality` convention

| Type | Meaning |
|---|---|
| `Goal` | Display order of strategic objectives |
| `Workflow` | Execution order of workflows within the agency |
| `WorkItem` | Execution order within a workflow; **same value = parallel**, **higher value = sequential after lower** |
| `Instruction` | Application order of rules/constraints |
| `Deliverable` | Evaluation order of expected outputs |
| `ConfiguredRole` | Display order of roles |

---

## 5. Single-Agency-Per-Database

There is exactly **one `Agency` entity per ArangoDB database**. The `agencyID` is injected into `agencyManager` at startup — read from the stored `Agency` entity or from the `AGENCY_ID` environment variable on first boot.

All `entitygraph.DataManager` calls pass this `agencyID` as the scope key. No multi-tenancy in v1.

---

## 6. Graph Traversal

`TraverseGraph` is used for queries that walk edges — e.g. all WorkItems for all Workflows in a single AQL hop.

For flat type-filtered lists, `ListEntities` with a `TypeID` filter is preferred — it avoids traversal overhead.
