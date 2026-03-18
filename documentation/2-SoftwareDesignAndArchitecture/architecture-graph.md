# CodeValdAgency — Graph Topology & Schema

> Part of the split architecture. Index: [architecture.md](architecture.md)

---

## 1. Graph Topology

```
Agency ──[has_goal]──────────────────► Goal
  │
  └──[has_workflow]─────────────────► Workflow
                                          │
                                          └──[has_work_item]──► WorkItem
                                                                    │
                                                          ┌─────────┴──────────┐
                                                          │                    │
                                                 [advances_goal]        [assigned_role]
                                                          │                    │  (edge property: raci)
                                                          ▼                    ▼
                                                        Goal          ConfiguredRole
```

All nodes and edges are stored in two ArangoDB collections:
- **`agency_entities`** — document collection (all entity types)
- **`agency_relationships`** — **edge** collection (all relationship types)

---

## 2. Entity Types

Each entity has a `TypeID` matching one of the TypeDefinitions in the
pre-delivered schema. The `Properties` map carries the entity's domain fields.

| TypeID | Properties | Notes |
|---|---|---|
| `Agency` | `name`, `mission`, `vision`, `status` | Root entity; one per database |
| `Goal` | `title`, `description`, `ordinality` | Strategic objective |
| `Workflow` | `name` | Container for ordered WorkItems |
| `WorkItem` | `title`, `description`, `order`, `parallel` | Unit of work |
| `ConfiguredRole` | `name` | Custom role beyond super_admin / admin |
| `AgencySnapshot` | `snapshot_at` | Immutable; written on draft → active |
| `AgencyPublication` | `version`, `tag`, `published_at` | Immutable; written by PublishAgency |

---

## 3. Relationship Types (Edge Labels)

Edges are documents in `agency_relationships`. Each edge has `_from`, `_to`,
a `name` field (the label), and an optional `properties` map.

| Edge label | From type | To type | Edge properties |
|---|---|---|---|
| `has_goal` | `Agency` | `Goal` | — |
| `has_workflow` | `Agency` | `Workflow` | — |
| `has_work_item` | `Workflow` | `WorkItem` | — |
| `advances_goal` | `WorkItem` | `Goal` | — |
| `assigned_role` | `WorkItem` | `ConfiguredRole` | `raci` (`"R"` / `"A"` / `"C"` / `"I"`) |

---

## 4. Pre-Delivered Schema

`schema.go` exposes `DefaultAgencySchema()` which returns a `types.Schema`
containing all TypeDefinitions. `cmd/main.go` seeds this schema idempotently
on startup via `AgencySchemaManager.SetSchema`.

### TypeDefinitions

| TypeID | Immutable | StorageCollection |
|---|---|---|
| `Agency` | false | (default: `agency_entities`) |
| `Goal` | false | (default: `agency_entities`) |
| `Workflow` | false | (default: `agency_entities`) |
| `WorkItem` | false | (default: `agency_entities`) |
| `ConfiguredRole` | false | (default: `agency_entities`) |
| `AgencySnapshot` | **true** | `agency_snapshots` |
| `AgencyPublication` | **true** | `agency_publications` |

**`Immutable: true`** means `entitygraph.DataManager.UpdateEntity` is rejected
with `ErrEntityImmutable` for that entity type. Immutable entities are created
once and never modified.

### DefaultAgencySchema sketch

```go
// DefaultAgencySchema returns the pre-delivered schema for CodeValdAgency.
// cmd/main.go seeds this via AgencySchemaManager.SetSchema on startup.
func DefaultAgencySchema() types.Schema {
    return types.Schema{
        ID:      "agency-schema",
        Version: 1,
        Types: []types.TypeDefinition{
            {TypeID: "Agency",             Immutable: false},
            {TypeID: "Goal",               Immutable: false},
            {TypeID: "Workflow",           Immutable: false},
            {TypeID: "WorkItem",           Immutable: false},
            {TypeID: "ConfiguredRole",     Immutable: false},
            {TypeID: "AgencySnapshot",     Immutable: true,  StorageCollection: "agency_snapshots"},
            {TypeID: "AgencyPublication",  Immutable: true,  StorageCollection: "agency_publications"},
        },
    }
}
```

---

## 5. Single-Agency-Per-Database

There is exactly **one `Agency` entity per ArangoDB database**. The `agencyID`
is injected into `agencyManager` at startup — either read from the stored
`Agency` entity or from the `AGENCY_ID` environment variable on first boot.

All `entitygraph.DataManager` calls pass this `agencyID` as the scope key.
No multi-tenancy is supported in v1.

---

## 6. Graph Traversal

`TraverseGraph` is used for queries that need to walk edges — e.g. fetching all
WorkItems for all Workflows in a single AQL hop, or finding all Goals advanced
by a given WorkItem.

For simple type-filtered lists (`GetGoals`, `GetWorkflows`,
`GetConfiguredRoles`), `ListEntities` with a `TypeID` filter is preferred — it
avoids traversal overhead for flat lookups.
