# FEAT-20260609-002 — Per-Workflow `event_flows` field + multi-file flow import (`flows_<workflow.code>.json`)

**Status:** ✅ Done
**Severity:** Medium — blocks the per-workflow-file convention already used in [CodeValdImplementations/Agencies/utility-app-builder/](../../../../CodeValdImplementations/Agencies/utility-app-builder/); a new [`flows_planning.json`](../../../../CodeValdImplementations/Agencies/utility-app-builder/flows_planning.json) sits on disk but is silently ignored by every consumer (live agency, flowchart renderer, scenario-12 QA)
**Owner:** CodeValdAgency (schema + importer); coordinated touch in CodeValdImplementations (split agency.json), CodeValdAgencyFrontend (flowchart renderer)
**Estimated effort:** ~1 day (schema + proto + importer; the file-bundling convention is a doc + the importer change to accept inline `event_flows` per workflow)
**Source finding:** `/document_issues` run during scenario-12 setup on 2026-06-09 — `flows_planning.json` exists but the importer only reads a single top-level `event_flows` field

---

## Problem

The current schema models `event_flows` as a **monolithic JSON string property on the `Agency` entity**. The implementation layer documents a different convention: flows split into per-workflow files (`flows_planning.json`, `flows_implementation.json`, …) — see the `[[project_agency_flows_folder_structure]]` memory and [CodeValdImplementations/Agencies/utility-app-builder/flows_planning.json](../../../../CodeValdImplementations/Agencies/utility-app-builder/flows_planning.json) (7-node planning workflow).

Three consumers diverge as a result:

1. **Importer** ([`internal/server/import_server.go`](../../internal/server/import_server.go)): reads only `spec.EventFlows` at the top level of agency.json. Does not scan for `flows_*.json` siblings, does not merge per-workflow blocks. The new `flows_planning.json` is dead weight.
2. **Live agency**: still carries the old monolithic 27-flow blob from when agency.json used to embed `event_flows` inline. No way to express "the planning workflow has these flows, the implementation workflow has those."
3. **Frontend flowchart renderer** (`dev-agency-flowchart` skill): can only generate one chart per agency, not one per workflow.

Until this lands:

- Scenario 12 QA is verifying flows that exist only on disk, never reach Cross.
- Every reviewer has to manually open `flows_planning.json` to know the planning workflow's contract.
- The "per-workflow files" convention is aspirational, not enforced.

## Evidence

```text
$ ls CodeValdImplementations/Agencies/utility-app-builder/
README.md
agency.json
flowchart.md
flows copy.json          ← legacy monolithic; 27 flows; un-prefixed topics
flows_planning.json      ← new per-workflow file; 7 nodes; created 2026-06-09

$ python3 -c "import json; d=json.load(open('CodeValdImplementations/Agencies/utility-app-builder/agency.json')); print(list(d.keys()))"
['agency', 'goals', 'configured_roles', 'workflows', 'work_plans', 'ai_config']
# No 'event_flows' field at top level — the importer has nothing to read.

$ grep -nc 'event_flows\|flows_planning' CodeValdAgency/internal/server/import_server.go
6                # only the top-level spec.EventFlows path
0                # zero references to flows_planning or flows_*.json scanning
```

The live agency was last imported from a version of agency.json that did embed `event_flows`. The current agency.json doesn't, but the live `eventFlows` field on the Agency entity still holds the old blob because no subsequent import has rewritten it.

## Root cause

The schema was authored before the per-workflow-file convention was established. When the convention emerged, only the implementation-layer file layout was updated; the schema, the proto, the importer, and the Workflow entity stayed unchanged.

## Fix plan

### Phase 1 — Schema (CodeValdAgency)

In [`schema.go`](../../../schema.go) add an `event_flows` property to both `Workflow` and `DraftWorkflow` TypeDefinitions:

```go
{Name: "event_flows", Type: types.PropertyTypeString, Required: false},
// JSON-encoded { name, steps: [...] } for this workflow. Imported from
// flows_<workflow.code>.json (per-workflow-file convention). Stored as a
// JSON string for the same reasons as Agency.event_flows: schema is
// document-shaped here, full validation happens at the consumer.
```

Keep `Agency.event_flows` as a deprecated-but-tolerated field for one release (legacy frontend may read it). The importer should populate both for backwards compat: union of all workflow flows → `Agency.event_flows`; per-workflow → each `Workflow.event_flows`.

### Phase 2 — Models (CodeValdAgency)

In [`models.go`](../../../models.go) add `EventFlows string` to:

- `Workflow` struct (line ~85 region — matches Agency.EventFlows)
- `CreateWorkflowRequest`
- `UpdateWorkflowRequest` (as `*string`)

### Phase 3 — Proto (CodeValdAgency)

In [`proto/codevaldagency/v1/agency.proto`](../../../proto/codevaldagency/v1/agency.proto), add `string event_flows = N;` to:

- `Workflow` message
- `CreateWorkflowRequest`
- `UpdateWorkflowRequest`

Regenerate via `make proto`. Bundle the rebuilt `gen/go/codevaldagency/v1/agency.pb.go` with the schema change.

### Phase 4 — Importer (CodeValdAgency)

In [`internal/server/import_server.go`](../../../internal/server/import_server.go):

- Extend `importWorkflowSpec` with `EventFlows interface{} \`yaml:"event_flows" json:"event_flows"\``.
- When upserting the DraftWorkflow, marshal `wf.EventFlows` to a JSON string (same pattern as the top-level `importSetDetails` marshalling at line ~448) and write it into the props map.
- Keep the existing top-level `spec.EventFlows` path as a fallback for legacy agency.json files. Log a deprecation warning when the top-level form is used and per-workflow forms are absent.

### Phase 5 — Convention + caller bundling

The agency.json author writes per-workflow content into separate `flows_<workflow.code>.json` files. To deliver them as one HTTP POST body to `${BASE}/agency/{id}/import`, the caller bundles them inline into the request payload immediately before sending:

```python
# pseudo — bundle helper (initially manual; later a dev-bundle-agency skill)
import json, pathlib
dir = pathlib.Path("CodeValdImplementations/Agencies/utility-app-builder")
agency = json.loads((dir / "agency.json").read_text())
for wf in agency.get("workflows", []):
    flow_file = dir / f"flows_{wf['code']}.json"
    if flow_file.exists():
        wf["event_flows"] = json.loads(flow_file.read_text())
post(f"{BASE}/agency/{agency_id}/import", json=agency)
```

The server-side importer does NOT touch the filesystem — it only consumes what arrives in the request. The file-naming convention is enforced at bundling time.

### Phase 6 — Implementations layer (CodeValdImplementations)

- Delete `flows copy.json` after confirming `flows_planning.json` content reaches the live agency end-to-end.
- Add `flows_implementation.json`, `flows_failure_recovery.json`, etc. once their workflows are designed.
- Update the bundle helper / scenario-12 QA Step 1 to use the bundling pattern.

## Verification

- [ ] `make proto && make build` succeeds.
- [ ] Unit test: importing an agency.json with `workflows[code=planning].event_flows = {...inline...}` stores the JSON string on the live Workflow entity (`GET /agency/{id}/workflows` returns it).
- [ ] Integration test: bundle `flows_planning.json` into utility-app-builder agency.json, reimport, confirm the planning workflow's `event_flows` matches the file content byte-for-byte.
- [ ] Existing top-level `event_flows` path still works (legacy agency.json files unchanged).
- [ ] Frontend flowchart renderer can read `Workflow.event_flows` and produce per-workflow flowcharts (out-of-scope for this FEAT but unblocked).
- [ ] `bugs.md` / `mvp.md` updated; this FEAT moved to `mvp_done.md` after merge.

## Dependencies

- None blocking.
- Companion item: [FEAT-20260609-003](FEAT-20260609-003_auto_draft_on_import.md) (auto-draft on import). Without auto-draft, re-importing a published agency to test the multi-flow path requires manual draft-promote steps.
- Coordinates with: scenario 12 QA updates (CodeValdCross), agency.json structure (CodeValdImplementations).

## Out of scope

- The bundling CLI helper (`dev-bundle-agency` skill) — file as a follow-up if the convention sticks.
- Per-workflow flowchart rendering — separate frontend FEAT.
- Migrating `flows copy.json` content into the new per-workflow files — see scenario-12 doc-drift bug.
