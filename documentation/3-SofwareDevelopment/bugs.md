# CodeValdAgency — Active Bugs

_Newest items first. Fixed bugs are moved to [`bugs_done.md`](bugs_done.md)._

| Bug ID | Title | Severity | Status | Detail |
|--------|-------|----------|--------|--------|
| [BUG-20260609-001](bug-details/BUG-20260609-001_architecture_graph_missing_entities.md) | `architecture-graph.md` missing WorkPlan, AI*, and ContextSource entity types | Medium | 📋 Open | [detail](bug-details/BUG-20260609-001_architecture_graph_missing_entities.md) |
| [BUG-20260609-002](bug-details/BUG-20260609-002_storage_doc_missing_reviewer_role_code.md) | `architecture-storage.md` does not document `DraftDeliverable.reviewer_role_code` | Low | 📋 Open | [detail](bug-details/BUG-20260609-002_storage_doc_missing_reviewer_role_code.md) |

---

## BUG-20260609-001 — `architecture-graph.md` missing WorkPlan, AI*, and ContextSource entity types

**Severity:** Medium — design docs diverge from shipped schema
**Status:** 📋 Open
**Detail:** [bug-details/BUG-20260609-001](bug-details/BUG-20260609-001_architecture_graph_missing_entities.md)

The graph architecture doc stops at the Agency → Goal/Workflow/WorkItem tree. It does not document `WorkPlan`, `AIProvider`, `AIAgent`, or the three `ContextSource` variants — all of which exist in `schema.go` with relationships, edges, and TypeDefinitions. Reviewers cannot infer the dispatch sub-graph from the doc alone.

---

## BUG-20260609-002 — `architecture-storage.md` does not document `DraftDeliverable.reviewer_role_code`

**Severity:** Low — minor doc gap
**Status:** 📋 Open
**Detail:** [bug-details/BUG-20260609-002](bug-details/BUG-20260609-002_storage_doc_missing_reviewer_role_code.md)

`DraftDeliverable.reviewer_role_code` is declared in `schema.go` and populated by `import_server.go`, but the storage doc's draft-entity description never mentions it. May be bundled with BUG-20260609-001 into a single doc-sweep pass.

---

## Status legend

- 📋 Open — backlog, not yet started
- 🚀 In Progress — actively being fixed
- ⏸️ Blocked — waiting on a dependency
- ✅ Fixed — moved to [`bugs_done.md`](bugs_done.md)

## Completion workflow

1. Implement and validate the fix (for doc-only bugs: edit the doc, re-run the contradiction sweep).
2. Move the row from this file to [`bugs_done.md`](bugs_done.md) and add the commit reference.
3. Update [`../../../CodeValdCross/documentation/3-SofwareDevelopment/prioritization.md`](../../../CodeValdCross/documentation/3-SofwareDevelopment/prioritization.md) — strike-through the row and mark ✅.
