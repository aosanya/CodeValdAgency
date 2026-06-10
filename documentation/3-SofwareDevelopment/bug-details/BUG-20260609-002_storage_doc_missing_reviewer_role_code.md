# BUG-20260609-002 — `architecture-storage.md` does not document `DraftDeliverable.reviewer_role_code`

**Status:** 🚀 In Progress
**Severity:** Low — minor doc gap; field works correctly in shipped code
**Owner:** CodeValdAgency (documentation)
**Estimated effort:** < 0.25 day (one-line table edit + brief draft-entity note)
**Source finding:** Contradiction sweep during `/dev-document-feature` on 2026-06-09 — pre-existing reviewer_role mapping in `import_server.go` writes a property that the storage doc never mentions
**Related:** [BUG-20260609-001](BUG-20260609-001_architecture_graph_missing_entities.md) — similar drift; could be bundled if both are fixed in one pass

---

## Problem

`DraftDeliverable` carries an optional `reviewer_role_code` string property (declared in [`schema.go`](../../../schema.go) around line 556 and written by [`import_server.go`](../../../internal/server/import_server.go) line 344). The storage architecture doc's description of `agency_draft_entities` does not list it:

`documentation/2-SoftwareDesignAndArchitecture/architecture-storage.md:19`

```
| agency_draft_entities | Document | Draft sub-entity types — DraftGoal, DraftWorkflow,
                                    DraftWorkItem, DraftConfiguredRole, DraftInstruction,
                                    DraftDeliverable, DraftDeliverableResult. Each carries
                                    a draft_ref_code property pointing to its AgencyDraft. |
```

The doc names `draft_ref_code` (common to all draft sub-entities) but never calls out the per-type discriminator properties — most notably `draft_work_item_ref_code` (mentioned at line 99) and `reviewer_role_code` (not mentioned anywhere).

## Evidence

```
$ grep -n reviewer_role_code documentation/2-SoftwareDesignAndArchitecture/*.md
(no matches)

$ grep -n reviewer_role_code schema.go import_server.go
schema.go:556:                  {Name: "reviewer_role_code", Type: types.PropertyTypeString, ...}
internal/server/import_server.go:344:    deliverableProps["reviewer_role_code"] = d.ReviewerRole
```

## Root cause

`reviewer_role_code` was added as part of the agency.json import-schema alignment work (2026-06-09 session). The schema and import-handler changes landed; the storage doc was not updated alongside.

## Fix plan

In `documentation/2-SoftwareDesignAndArchitecture/architecture-storage.md`:

1. Extend the existing prose section around line 99 (the "draft_ref_code / draft_work_item_ref_code" note) to also mention `reviewer_role_code`:

   > `DraftDeliverable` additionally carries an optional `reviewer_role_code` (string) — the `ConfiguredRole.code` of the reviewer declared in `agency.json`. It is resolved to a `reviewer_role` edge on the live `Deliverable` during `PromoteDraft`.

2. Optionally cross-reference [architecture-graph.md §4](architecture-graph.md) once that doc lists `DraftDeliverable` properties (currently it does — the entry exists at row "DraftDeliverable" in §4's TypeDefinitions table).

## Verification

- `grep -n reviewer_role_code documentation/2-SoftwareDesignAndArchitecture/architecture-storage.md` returns at least one match.
- A reader can trace the property end-to-end: `agency.json reviewer_role` → `importDeliverableSpec.ReviewerRole` → `DraftDeliverable.reviewer_role_code` → (during PromoteDraft) `Deliverable --reviewer_role--> ConfiguredRole` edge.

## Dependencies

- None. Pure documentation update.
- Can be combined with [BUG-20260609-001](BUG-20260609-001_architecture_graph_missing_entities.md) into a single doc-sweep commit.
