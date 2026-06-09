# BUG-20260609-001 — `architecture-graph.md` missing WorkPlan, AI*, and ContextSource entity types

**Status:** ✅ Fixed (2026-06-09)
**Severity:** Medium — design docs diverge from shipped schema; new contributors and architecture reviewers cannot understand the full graph from the doc alone
**Owner:** CodeValdAgency (documentation)
**Estimated effort:** ~0.5 day (topology diagram extension + entity table rows + relationship rows + TypeDefinitions rows)
**Source finding:** Contradiction sweep during `/dev-document-feature` on 2026-06-09 for the agency.json import-schema alignment work

---

## Problem

`documentation/2-SoftwareDesignAndArchitecture/architecture-graph.md` documents only the Agency → Goal/Workflow/WorkItem/ConfiguredRole/Instruction/Deliverable/DeliverableResult/ContentRef tree plus the AgencyDraft/AgencySnapshot/AgencyPublication versioning entities. It is silent on every entity in the **dispatch + AI configuration sub-graph**, all of which are first-class entries in [`schema.go`](../../../schema.go):

| Entity type | Defined in schema.go | Documented in architecture-graph.md? |
|---|---|---|
| `WorkPlan` | line ~659 | ❌ |
| `GitContextSource` | line ~754 | ❌ |
| `CommContextSource` | line ~781 | ❌ |
| `WorkContextSource` | line ~801 | ❌ |
| `AIProvider` | line ~819 | ❌ |
| `AIAgent` | line ~839 | ❌ |

Each of these participates in real relationships — e.g. `Agency --has_work_plan--> WorkPlan`, `WorkPlan --has_context_source--> {GitContextSource | CommContextSource | WorkContextSource}`, `Agency --has_ai_provider--> AIProvider`, `Agency --has_ai_agent--> AIAgent` — none of which appear in the topology diagram (§1) or the forward/inverse relationship tables (§3).

## Evidence

```
$ grep -nc 'WorkPlan\|AIProvider\|AIAgent\|ContextSource' \
    documentation/2-SoftwareDesignAndArchitecture/architecture-graph.md
0   # zero mentions

$ grep -c '"WorkPlan"\|"AIProvider"\|"AIAgent"\|"GitContextSource"' schema.go
5   # five TypeDefinitions
```

The Entity Types table (§2), forward-relationships table (§3), and TypeDefinitions table (§4) all stop short of the dispatch sub-graph.

## Root cause

The graph doc was authored when only the core Agency-tree was implemented. Subsequent feature work (RACI matrix → MVP-AGENCY-010..014; WorkPlan dispatch; AI provider/agent config for utility-app-builder) added entity types and relationships to schema.go without back-filling the doc.

## Fix plan

In `documentation/2-SoftwareDesignAndArchitecture/architecture-graph.md`:

1. **§1 Topology** — extend the diagram with the dispatch sub-graph:
   ```
   Agency ──has_work_plan─────► WorkPlan ──has_context_source──► GitContextSource
                                          ──has_context_source──► CommContextSource
                                          ──has_context_source──► WorkContextSource
                                          ──assigned_role───────► ConfiguredRole
          ──has_ai_provider──► AIProvider
          ──has_ai_agent─────► AIAgent
   ```
2. **§2 Entity Types** — add rows for `WorkPlan` (with the new `agent_code`, `function_params` properties), `GitContextSource`, `CommContextSource`, `WorkContextSource`, `AIProvider`, `AIAgent`.
3. **§3 Forward relationships** — add: `has_work_plan` (Agency → WorkPlan, ToMany, inverse `belongs_to_agency`), `has_ai_provider`, `has_ai_agent`, `has_context_source` (WorkPlan → each ContextSource variant), `assigned_role` (WorkPlan → ConfiguredRole).
4. **§3 Inverse relationships** — add `belongs_to_agency` on WorkPlan/AIProvider/AIAgent, `belongs_to_work_plan` on each ContextSource type, `assigned_work_plan` on ConfiguredRole.
5. **§4 TypeDefinitions** — add rows for the six missing types (all `StorageCollection: agency_entities`, none immutable).

## Verification

- `grep -c 'WorkPlan' architecture-graph.md` ≥ 5 (topology, entity table, relationships, TypeDefinitions, prose).
- Every TypeDefinition in `schema.go` has a corresponding row in §4's TypeDefinitions table.
- Every relationship label in `schema.go` (`Name: "<label>"` lines) appears in either §3 forward or §3 inverse table.

## Dependencies

- None. Pure documentation update against shipped code.
- Touching architecture-graph.md will push it from 191 → ~260 lines (still well under the 300-line split threshold).
