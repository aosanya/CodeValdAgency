# CodeValdAgency — ArangoDB Storage

> Part of the split architecture. Index: [architecture.md](architecture.md)

---

## 1. Collections

| Collection | Type | Contents |
|---|---|---|
| `agency_entities` | Document | All mutable entity types: Agency, Goal, Workflow, WorkItem, ConfiguredRole |
| `agency_relationships` | **Edge** | All relationship edges (has_goal, has_workflow, has_work_item, advances_goal, assigned_role) |
| `agency_schemas` | Document | Schema version documents (managed by AgencySchemaManager) |
| `agency_snapshots` | Document | Immutable AgencySnapshot entities (routed here by TypeDefinition.StorageCollection) |
| `agency_publications` | Document | Immutable AgencyPublication entities (routed here by TypeDefinition.StorageCollection) |

`agency_relationships` **must** be created as an edge collection — not a
regular document collection. ArangoDB graph traversal requires edge collections.

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
    "mission": "To ...",
    "vision":  "By 2030 ...",
    "status":  "draft"
  },
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

### `agency_entities/{key}` (Goal)

```json
{
  "_key":      "<uuid>",
  "type_id":   "Goal",
  "agency_id": "<agencyID>",
  "properties": {
    "title":       "Reduce onboarding time",
    "description": "...",
    "ordinality":  1
  },
  "created_at": "...",
  "updated_at": "..."
}
```

### `agency_entities/{key}` (Workflow)

```json
{
  "_key":      "<uuid>",
  "type_id":   "Workflow",
  "agency_id": "<agencyID>",
  "properties": {
    "name": "Onboarding Workflow"
  },
  "created_at": "...",
  "updated_at": "..."
}
```

### `agency_entities/{key}` (WorkItem)

```json
{
  "_key":      "<uuid>",
  "type_id":   "WorkItem",
  "agency_id": "<agencyID>",
  "properties": {
    "title":       "Collect requirements",
    "description": "...",
    "order":       1,
    "parallel":    false
  },
  "created_at": "...",
  "updated_at": "..."
}
```

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

`assigned_role` edges carry an extra `raci` property inside `properties`:

```json
{
  "_from":     "agency_entities/<workItemKey>",
  "_to":       "agency_entities/<configuredRoleKey>",
  "name":      "assigned_role",
  "agency_id": "<agencyID>",
  "properties": { "raci": "R" }
}
```

### `agency_snapshots/{key}`

```json
{
  "_key":       "<uuid>",
  "type_id":    "AgencySnapshot",
  "agency_id":  "<agencyID>",
  "properties": {
    "snapshot_at": "2024-06-01T12:00:00Z"
  },
  "created_at": "..."
}
```

### `agency_publications/{key}`

```json
{
  "_key":       "<uuid>",
  "type_id":    "AgencyPublication",
  "agency_id":  "<agencyID>",
  "properties": {
    "version":      1,
    "tag":          "v1.0",
    "published_at": "2024-06-15T09:00:00Z"
  },
  "created_at": "..."
}
```

---

## 3. Named Graph

ArangoDB named graph for `TraverseGraph` queries:

```
Graph name:        agency_graph
Edge collections:  agency_relationships
Vertex collections: agency_entities, agency_snapshots, agency_publications
```

AQL traversal template used by `TraverseGraph`:

```aql
FOR v, e, p IN 1..@depth @direction @startVertex
  GRAPH 'agency_graph'
  FILTER e.name IN @edgeNames    /* optional edge-label filter */
  RETURN v
```

---

## 4. Indexes

| Collection | Index type | Fields | Purpose |
|---|---|---|---|
| `agency_entities` | Persistent | `agency_id`, `type_id` | Efficient `ListEntities` by type |
| `agency_entities` | Persistent | `agency_id`, `properties.status` | Lifecycle-filtered Agency lookup |
| `agency_relationships` | Persistent | `agency_id`, `name` | Filter edges by label |
| `agency_relationships` | Persistent | `_from` | Outbound traversal without full graph scan |
| `agency_relationships` | Persistent | `_to` | Inbound traversal |
| `agency_snapshots` | Persistent | `agency_id`, `properties.snapshot_at` | Chronological snapshot listing |
| `agency_publications` | Persistent | `agency_id`, `properties.version` | Version-keyed publication lookup |

---

## 5. Storage Package Layout

```
storage/
└── arangodb/
    ├── storage.go         # ensureCollections, ensureGraph; New(db) constructor
    ├── entities.go        # CreateEntity, GetEntity, UpdateEntity, DeleteEntity, ListEntities
    ├── relationships.go   # CreateRelationship, GetRelationship, DeleteRelationship,
    │                      # ListRelationships, TraverseGraph
    └── schemaops.go       # SetSchema, GetSchema, ListSchemaVersions
```

Each file targets one collection group. No file may exceed 500 lines.
The `New(db driver.Database)` constructor in `storage.go` calls `ensureCollections`
and `ensureGraph`, then returns a struct implementing both
`entitygraph.DataManager` and `entitygraph.SchemaManager`.
