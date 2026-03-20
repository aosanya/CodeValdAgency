# CodeValdAgency — Graph Topology & Schema

> Part of the split architecture. Index: [architecture.md](architecture.md)

---

## 1. Graph Topology

```
Agency ──has_goal──────────────► Goal
       ──has_workflow──────────► Workflow ──has_work_item──────► WorkItem
       ──has_configured_role──► ConfiguredRole                        │
       ──has_snapshot─────────► AgencySnapshot   (Immutable)          ├──has_instruction──► Instruction ──has_content_ref──► ContentRef
       ──has_publication──────► AgencyPublication (Immutable)         ├──has_deliverable──► Deliverable
                                    │                                 ├──has_content_ref──► ContentRef
                                    └──has_instruction──► Instruction └──assigned_role────► ConfiguredRole

Deliverable ──has_result──────► DeliverableResult ──has_content_ref──► ContentRef
            ──reviewer_role───► ConfiguredRole  (waiver authority)
```

All nodes and edges live in two ArangoDB collections:
- **`agency_entities`** — document collection (all mutable entity types)
- **`agency_relationships`** — **edge** collection (all relationship types)
- Immutable types use dedicated collections (see §4).

---

## 2. Entity Types

| Type | Mutable | Properties | Notes |
|---|---|---|---|
| `Agency` | ✅ | `name`(req), `mission`, `vision` | Root entity; one per database |
| `Goal` | ✅ | `title`(req), `description`, `ordinality`(req) | Strategic objective |
| `Workflow` | ✅ | `name`(req), `description`, `ordinality`(req) | Ordered container of WorkItems |
| `WorkItem` | ✅ | `title`(req), `description`, `ordinality`(req), `prompt` | Unit of work; same `ordinality` = parallel execution |
| `Instruction` | ✅ | `content`(req), `ordinality`(req) | Rule or constraint; attaches to Workflow or WorkItem |
| `Deliverable` | ✅ | `title`(req), `description`, `ordinality`(req), `blocking`(req) | Spec: expected output from a WorkItem |
| `DeliverableResult` | ❌ immutable | `status`(req), `produced_at` | Instance: actual output submitted against a Deliverable |
| `ContentRef` | ❌ immutable | `path`(req) | CodeValdGit artifact path; attaches to DeliverableResult, Instruction, or WorkItem |
| `ConfiguredRole` | ✅ | `name`(req), `description`, `actor_type`(req), `ordinality`(req) | `actor_type`: `"human"` / `"ai_agent"` / `"compute_agent"` |
| `AgencySnapshot` | ❌ immutable | `snapshot_at`(req) | Written on `draft → active` transition |
| `AgencyPublication` | ❌ immutable | `version`(req), `tag`(req), `published_at`(req), `status`(req) | `status`: `"draft"` / `"active"` / `"archived"` |

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
| `DeliverableResult` | **true** | `deliverable_results` |
| `ContentRef` | **true** | `content_refs` |
| `ConfiguredRole` | — | `agency_entities` (default) |
| `AgencySnapshot` | **true** | `agency_snapshots` |
| `AgencyPublication` | **true** | `agency_publications` |

**`Immutable: true`** — `UpdateEntity` returns `ErrEntityImmutable` for these types. Each submission or review decision creates a new record, giving a full audit trail. The latest result by `produced_at` is the current state.

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
