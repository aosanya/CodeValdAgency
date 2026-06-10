# FEAT-20260610-003 — utility-app-builder: align `flows_planning.json` with a workflow code

> **Architecture:** see [architecture-flows.md § 8.2 Caller-Side Bundling Convention](../../2-SoftwareDesignAndArchitecture/architecture-flows.md).

**Status:** ✅ Done (2026-06-10) — `flows_planning.json` renamed to `flows_feature-development.json` in `CodeValdImplementations` (main `58fac0b`); `/dev-reimport-agency` now reports `Bundled 1 flow file(s)` with 0 orphans; `GET /agency/utility-app-builder/workflows` reports `{flows: 1, steps: 7}` on `feature-development`. `dev-agency-flowchart` skill rewritten for the per-workflow file layout (FEAT-20260609-002); `flowchart.md` regenerated.
**Severity:** Medium — until this lands, no per-workflow `event_flows` reach the live utility-app-builder agency; the bundler reports 0 flows bundled and 1 orphan on every `/dev-reimport-agency` run.
**Owner:** CodeValdAgency (convention owner) — data lives in `CodeValdImplementations/Agencies/utility-app-builder/`
**Estimated effort:** ~30 minutes (rename + reimport + verify)
**Source finding:** 2026-06-10 session — `/dev-reimport-agency` bundler dry-run output:

```
skip:    no flow file for workflow code 'app-ideation-and-scoping'
skip:    no flow file for workflow code 'feature-development'
skip:    no flow file for workflow code 'testing-and-quality-assurance'
skip:    no flow file for workflow code 'release-and-deployment'
orphan:  flows_planning.json (no workflow with code 'planning' — rename file or add the workflow)
Bundled 0 flow file(s)
```

---

## Problem

[`CodeValdImplementations/Agencies/utility-app-builder/flows_planning.json`](../../../../CodeValdImplementations/Agencies/utility-app-builder/flows_planning.json) sits beside [`agency.json`](../../../../CodeValdImplementations/Agencies/utility-app-builder/agency.json), but the four workflow codes declared in `agency.json` (`app-ideation-and-scoping`, `feature-development`, `testing-and-quality-assurance`, `release-and-deployment`) do not include `planning`. The bundler in `/dev-reimport-agency` matches files to workflows by `flows_<workflow.code>.json`, so the orphan file is silently dropped on every import.

The 7 steps in `flows_planning.json` cover the planner→split/decompose handler chain and the post-decomposition todo persist/dispatch chain — content that semantically belongs to the **feature-development** workflow (the workflow where developer tasks are planned and split).

## Fix plan

### Phase 1 — Rename

```bash
cd CodeValdImplementations/Agencies/utility-app-builder
mv flows_planning.json flows_feature-development.json
```

This is the agreed approach from the 2026-06-10 session: rename to the nearest matching workflow code. Splitting the 7-step planning flow across all 4 workflows (the more granular alternative) is out of scope for this FEAT — it is a separate content-design task and would be a follow-up.

### Phase 2 — Reimport and verify

Run `/dev-reimport-agency`. Expected output:

```
bundled: flows_feature-development.json -> workflows[code=feature-development].event_flows
skip:    no flow file for workflow code 'app-ideation-and-scoping'
skip:    no flow file for workflow code 'testing-and-quality-assurance'
skip:    no flow file for workflow code 'release-and-deployment'
Bundled 1 flow file(s)
```

Step-5 verification should report `{flows: 1, steps: 7}` on the `feature-development` workflow and `{flows: 0, steps: 0}` on the other three.

### Phase 3 — Update flowchart

Regenerate [`flowchart.md`](../../../../CodeValdImplementations/Agencies/utility-app-builder/flowchart.md) via `/dev-agency-flowchart` so the rendered diagram reflects the new per-workflow grouping.

## Verification

- [ ] `flows_planning.json` renamed to `flows_feature-development.json`.
- [ ] `/dev-reimport-agency` reports `Bundled 1 flow file(s)` with no orphans.
- [ ] `GET /agency/utility-app-builder/workflows` returns `eventFlows` JSON on the `feature-development` workflow that, when parsed, has `flows.length === 1` and `flows[0].steps.length === 7`.
- [ ] `flowchart.md` regenerated and the planning chain shows under feature-development.

## Out of scope

- **Splitting the planning flow** across multiple workflows — the 7 steps cover one continuous planner→subtask chain that does not naturally subdivide; if a future workflow design wants finer granularity, file a separate FEAT.
- **Migrating `flows copy.json` content** into other per-workflow files — `flows copy.json` is the source-of-truth pool for the remaining ~20 unmigrated flows and must NOT be deleted as part of this FEAT.
- **Adding a `planning` workflow to agency.json** — rejected in favour of renaming the file, since the existing 4-workflow structure is canonical.

## Dependencies

- [FEAT-20260609-002](FEAT-20260609-002_per_workflow_event_flows.md) — provides the `Workflow.event_flows` schema property. ✅ Done.
- [FEAT-20260610-001](FEAT-20260610-001_dev_reimport_agency_bundling.md) — provides the bundler that consumes `flows_<workflow.code>.json` files. ✅ Done.
