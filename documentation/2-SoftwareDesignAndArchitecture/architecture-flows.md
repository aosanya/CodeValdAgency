# CodeValdAgency — Lifecycle, Flows & Errors

> Part of the split architecture. Index: [architecture.md](architecture.md)
>
> gRPC service definitions and Cross registration live in
> [architecture-interfaces.md § 5–6](architecture-interfaces.md).

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
    └─ publisher.Publish(ctx, "agency.created", agencyID)
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
    └─ publisher.Publish(ctx, "agency.draft.created", agencyID)
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
    └─ publisher.Publish(ctx, "agency.promoted", agencyID)

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
    └─ publisher.Publish(ctx, "agency.draft.archived", agencyID)
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
    └─ publisher.Publish(ctx, "agency.published", agencyID)
```

`UpdatePublicationStatus(version, status)` mutates the linked
`AgencyPublicationStatus` entity only — the immutable `AgencyPublication`
record is never updated. Allowed transitions: `draft → active`,
`active → archived`. `archived` is terminal.

---

## 8. ImportDraft Flow

`ImportDraft` is the bulk declarative path: a single agency.json/yaml body
becomes (or refreshes) a complete open draft. Unlike the manager-driven flows
above, the gRPC handler in `internal/server/import_server.go` writes
`Draft*` entities directly via `entitygraph.DataManager` — it does **not**
go through `AgencyManager`.

```
AgencyService.ImportDraft(ctx, body, auto_promote)
    │
    ├─ parse body as YAML (fallback JSON)
    │     → InvalidArgument if both parses fail or agency.code is empty
    │
    ├─ 1. importSetDetails(ctx, agencyID, agencySpec, spec.EventFlows, auto_promote)
    │       creates the live Agency entity on first import,
    │       refreshes scalar fields (name, mission, vision, enabled),
    │       writes the legacy top-level event_flows blob to Agency.event_flows
    │       (deprecated — see § 8.2 below).
    │       Returns ErrAgencyReadOnly if the agency is published AND
    │       auto_promote=false. With auto_promote=true the handler creates
    │       an open draft and promotes it after import.
    │
    ├─ 2. importEnsureDraft(ctx, agencyID, agencySpec) → draftID
    │       finds the most recent open draft or creates a new one.
    │
    ├─ 3. for each role in spec.configured_roles:
    │         upsert DraftConfiguredRole {draft_ref_code: draftID, code, ...}
    │
    ├─ 4. for each goal in spec.goals:
    │         upsert DraftGoal {draft_ref_code: draftID, code, ...}
    │
    ├─ 5. for each workflow in spec.workflows:
    │         marshal wf.event_flows (if non-nil) → JSON string
    │             ─ this is FEAT-20260609-002: per-workflow event_flows
    │             ─ deprecation warning emitted only when zero per-workflow
    │               blocks were seen AND spec.EventFlows (top-level) was used
    │         upsert DraftWorkflow {draft_ref_code: draftID, code, ...,
    │                                event_flows: <json string>}
    │         for each instruction in wf.instructions:
    │             upsert DraftInstruction {draft_workflow_ref_code: wfID, ...}
    │         for each work_item in wf.work_items:
    │             upsert DraftWorkItem {draft_workflow_ref_code: wfID, ...}
    │             for each instruction in work_item.instructions:
    │                 upsert DraftInstruction {draft_work_item_ref_code: wiID, ...}
    │             for each deliverable in work_item.deliverables:
    │                 upsert DraftDeliverable {draft_work_item_ref_code: wiID,
    │                                          reviewer_role_code: <role.code>, ...}
    │
    ├─ 6. for each work_plan in spec.work_plans:
    │         upsert WorkPlan (live, not draft-scoped)
    │
    └─ if auto_promote:
            PromoteDraft(draftID) — moves Draft* sub-graph to live
                                    and writes AgencySnapshot.
```

All entity writes go through `entitygraph.DataManager.UpsertEntity`, the same
idempotency path used by `EntityService.CreateEntity`. Re-running the import
updates existing entities rather than creating duplicates.

### 8.1 Per-Workflow `event_flows` Contract

Each workflow entry in agency.json may carry an inline `event_flows` object:

```json
{
  "workflows": [
    {
      "code": "planning",
      "name": "Planning",
      "event_flows": { "flows": [{ "name": "planning", "steps": [...] }] },
      "work_items": [...]
    }
  ]
}
```

`importWorkflowSpec.EventFlows` decodes this as `interface{}` (yaml.v3
accepts both YAML mappings and JSON objects) and the importer re-marshals to
a JSON string before storing on `DraftWorkflow.event_flows`. The field is
optional — workflows authored before FEAT-20260609-002 have no per-workflow
flows and the property is omitted.

After `PromoteDraft`, `DraftWorkflow.event_flows` is copied verbatim onto the
live `Workflow.event_flows` via `copyPropsExcluding("draft_ref_code")`.
Readers (e.g. the flowchart renderer) consume the JSON string from
`GetWorkflows().eventFlows`; legacy callers may fall back to
`Agency.event_flows` when the workflow-level field is empty.

### 8.2 Caller-Side Bundling Convention

The agency.json author writes per-workflow content into separate
`flows_<workflow.code>.json` sibling files. The importer does **not** touch
the filesystem — it only consumes what arrives in the request body. The
naming convention is enforced at bundling time by the caller (typically the
`/dev-reimport-agency` skill):

```python
# pseudo — bundle helper
for wf in agency["workflows"]:
    flow_file = dir / f"flows_{wf['code']}.json"
    if flow_file.exists():
        wf["event_flows"] = json.loads(flow_file.read_text())
post(f"{BASE}/agency/{agency_id}/import", json=agency)
```

The bundler must surface three states per workflow:

| State | Cause | Implication |
|---|---|---|
| `bundled` | `flows_<code>.json` exists for workflow `<code>` | Will land in `DraftWorkflow.event_flows` |
| `skip` | workflow `<code>` exists but no matching file | Legitimate when that workflow has no flows yet |
| `orphan` | `flows_<x>.json` exists but no workflow has code `<x>` | Misnaming — file is silently ignored by the importer |

> ⚠️ WARNING — `orphan` files are dropped on the floor by the importer. The
> bundler MUST log them; otherwise authors waste cycles wondering why their
> flows never appear on the live agency.

### 8.3 Verification Surface

Verify post-import by calling the per-workflow endpoint, not the legacy
agency-level one:

```
GET /agency/{agencyId}/workflows
→ { workflows: [{ name, eventFlows: "<json string>", ... }] }
```

Parsing `workflow.eventFlows` with `JSON.parse` yields the same
`{ flows: [...] }` shape as the source `flows_<code>.json` file, byte-for-byte
(modulo whitespace). `Agency.eventFlows` is only populated from the legacy
top-level `event_flows` field and **must not** be relied on for per-workflow
content.

### 8.4 Idempotency & Republish Semantics

- All Draft\* upserts key on `(draft_ref_code, code)` — re-running the import
  with the same draft updates entities in place rather than creating
  duplicates.
- `auto_promote=true` against an already-published agency calls
  `CreateDraft` + `PromoteDraft` once per import; structural fields are
  frozen by the readonly guard but `event_flows`, `work_plans`, and similar
  refresh-tolerant fields are always rewritten.
- The deprecation warning for top-level `event_flows` (legacy monolithic
  blob) fires only when **zero** workflows had inline `event_flows` — mixed
  imports silently prefer the per-workflow path.

---

## 9. Error Types

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
