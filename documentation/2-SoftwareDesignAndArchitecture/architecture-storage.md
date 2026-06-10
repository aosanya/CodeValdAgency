# CodeValdAgency — ArangoDB Storage

> Part of the split architecture. Index: [architecture.md](architecture.md)

---

## 1. Collections

The ArangoDB adapter in [`storage/arangodb/storage.go`](../../storage/arangodb/storage.go)
fixes the agency-specific collection names. Routing of individual entity types
into the right collection is driven by `TypeDefinition.StorageCollection` in
[`schema.go`](../../schema.go) — types without an explicit `StorageCollection`
fall through to the default `agency_entities`.

| Collection | Type | Holds |
|---|---|---|
| `agency_entities` | Document | Default vertex collection. All non-draft entity types: Agency, Goal, Workflow, WorkItem, Instruction, Deliverable, DeliverableResult, ContentRef, ConfiguredRole, AgencySnapshot, AgencyPublication, AgencyPublicationStatus |
| `agency_drafts` | Document | `AgencyDraft` root entities (routed via `StorageCollection: "agency_drafts"`) |
| `agency_draft_entities` | Document | Draft sub-entity types — `DraftGoal`, `DraftWorkflow`, `DraftWorkItem`, `DraftConfiguredRole`, `DraftInstruction`, `DraftDeliverable`, `DraftDeliverableResult`. Each carries a `draft_ref_code` property pointing to its `AgencyDraft`. |
| `agency_relationships` | **Edge** | All edges across all vertex collections. `_from`/`_to` use full ArangoDB document handles (e.g. `agency_draft_entities/<uuid>`), so a single edge collection covers live, draft, and root edges. |
| `agency_schemas_draft` | Document | In-progress schema versions managed by the `SchemaManager` |
| `agency_schemas_published` | Document | Published, versioned schema documents — seeded on startup by `SeedSchema` and read on every boot |

`agency_relationships` **must** be created as an edge collection (not a
document collection); ArangoDB graph traversal requires it.

`AgencySnapshot` and `AgencyPublication` are stored in `agency_entities` and
marked `Immutable: true` — `UpdateEntity` against those `TypeID`s returns
`ErrImmutableType`. Their mutable companion (`AgencyPublicationStatus`) is a
separate non-immutable type linked via the `has_status` edge.

---

## 2. Document Shapes

### `agency_entities/{key}` (Agency)

```json
{
  "_key":      "<uuid>",
  "type_id":   "Agency",
  "agency_id": "<agencyID>",
  "properties": {
    "name":    "Acme Corp",
    "mission": "...",
    "vision":  "...",
    "enabled": true
  },
  "created_at": "2026-04-27T00:00:00Z",
  "updated_at": "2026-04-27T00:00:00Z"
}
```

### `agency_drafts/{key}` (AgencyDraft)

```json
{
  "_key":      "<uuid>",
  "type_id":   "AgencyDraft",
  "agency_id": "<agencyID>",
  "properties": {
    "ref_code":             "<uuid>",
    "code":                 "q3-restructure",
    "description":          "Q3 restructure",
    "status":               "open",
    "forked_from_ref_code": "<sourceRefCode>",
    "forked_from_type":     "live"
  },
  "created_at": "...",
  "updated_at": "..."
}
```

### `agency_draft_entities/{key}` (DraftGoal — same shape for all Draft* types)

Draft sub-entities are scoped by `draft_ref_code`, not by `agency_id` directly,
so a single AQL filter on `draft_ref_code` returns the entire draft sub-graph.

```json
{
  "_key":      "<uuid>",
  "type_id":   "DraftGoal",
  "agency_id": "<agencyID>",
  "properties": {
    "ref_code":       "<uuid>",
    "code":           "reduce-onboarding",
    "draft_ref_code": "<draftRefCode>",
    "title":          "Reduce onboarding time",
    "description":    "...",
    "ordinality":     1
  },
  "created_at": "...",
  "updated_at": "..."
}
```

`DraftWorkItem` adds `draft_workflow_ref_code`; `DraftInstruction` adds
`draft_workflow_ref_code` *or* `draft_work_item_ref_code` depending on its
parent; `DraftDeliverable` adds `draft_work_item_ref_code` and an optional
`reviewer_role_code` (string) — the `ConfiguredRole.code` of the reviewer
declared in `agency.json`, resolved to a `reviewer_role` edge on the live
`Deliverable` during `PromoteDraft`. All Draft* types enforce uniqueness on
`(draft_ref_code, code)`.

### `agency_entities/{key}` (AgencySnapshot — immutable)

```json
{
  "_key":      "<uuid>",
  "type_id":   "AgencySnapshot",
  "agency_id": "<agencyID>",
  "properties": { "snapshot_at": "2026-04-27T12:00:00Z" },
  "created_at": "..."
}
```

### `agency_entities/{key}` (AgencyPublication — immutable)

```json
{
  "_key":      "<uuid>",
  "type_id":   "AgencyPublication",
  "agency_id": "<agencyID>",
  "properties": {
    "version":      1,
    "tag":          "v1",
    "draft_id":     "<sourceDraftID>",
    "content_hash": "<sha256>",
    "published_at": "2026-04-27T09:00:00Z"
  },
  "created_at": "..."
}
```

### `agency_entities/{key}` (AgencyPublicationStatus — mutable companion)

```json
{
  "_key":      "<uuid>",
  "type_id":   "AgencyPublicationStatus",
  "agency_id": "<agencyID>",
  "properties": { "status": "draft" }
}
```

Linked to its `AgencyPublication` via a `has_status` edge in
`agency_relationships`. `UpdatePublicationStatus` mutates this entity only —
the immutable publication record is never touched.

### `agency_relationships/{key}` (any edge)

```json
{
  "_key":      "<uuid>",
  "_from":     "agency_entities/<fromKey>",
  "_to":       "agency_entities/<toKey>",
  "name":      "has_goal",
  "agency_id": "<agencyID>",
  "properties": {}
}
```

`assigned_role` edges carry an additional `raci` value inside `properties`:

```json
{
  "_from":     "agency_entities/<workItemKey>",
  "_to":       "agency_entities/<configuredRoleKey>",
  "name":      "assigned_role",
  "agency_id": "<agencyID>",
  "properties": { "raci": "R" }
}
```

Edges that point to draft sub-entities use the draft collection's handle:

```json
{
  "_from": "agency_drafts/<draftKey>",
  "_to":   "agency_draft_entities/<draftGoalKey>",
  "name":  "has_goal",
  "agency_id": "<agencyID>"
}
```

---

## 3. Named Graph

```
Graph name:         agency_graph
Edge collections:   agency_relationships
Vertex collections: agency_entities, agency_drafts, agency_draft_entities
```

Because `_from`/`_to` use full ArangoDB document handles, a single edge
collection covers all three vertex collections without duplication.

AQL traversal template used by `TraverseGraph`:

```aql
FOR v, e, p IN 1..@depth @direction @startVertex
  GRAPH 'agency_graph'
  FILTER e.name IN @edgeNames    /* optional edge-label filter */
  RETURN v
```

---

## 4. Indexes

Indexes are created by the SharedLib backend (`entitygraph/arangodb`) based on
the schema and are listed here for reference rather than re-declared by this
service.

| Collection | Index type | Fields | Purpose |
|---|---|---|---|
| `agency_entities` | Persistent | `agency_id`, `type_id` | Efficient `ListEntities` by type |
| `agency_drafts` | Persistent | `agency_id`, `properties.status` | List open / promoted / archived drafts |
| `agency_drafts` | Persistent | `agency_id`, `properties.created_at` | Order `ListDrafts` by creation time |
| `agency_draft_entities` | Persistent | `agency_id`, `type_id`, `properties.draft_ref_code` | Efficient draft-scoped lookups |
| `agency_draft_entities` | Persistent (unique) | `properties.draft_ref_code`, `properties.code` | Enforce per-draft `code` uniqueness |
| `agency_relationships` | Persistent | `agency_id`, `name` | Filter edges by label |
| `agency_relationships` | Persistent | `_from`, `_to` | Outbound + inbound traversal without full graph scan |

---

## 5. Storage Package Layout

```
storage/
└── arangodb/
    └── storage.go    # Thin adapter — fixes agency_entities, agency_relationships,
                      # agency_schemas_draft, agency_schemas_published, agency_graph
                      # via toSharedConfig and delegates to SharedLib.
```

The CRUD logic itself lives in
`github.com/aosanya/CodeValdSharedLib/entitygraph/arangodb`. This package is a
~90-line wrapper. `New(db, schema)` returns a `(DataManager, SchemaManager)`
pair; `NewBackend(cfg)` opens its own connection. The adapter does not make
its own ArangoDB calls — it just configures collection names.
