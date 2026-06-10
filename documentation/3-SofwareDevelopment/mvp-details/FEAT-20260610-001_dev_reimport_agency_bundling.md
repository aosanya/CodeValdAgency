# FEAT-20260610-001 — `/dev-reimport-agency` per-workflow flow bundling + verification

> **Architecture:** see [architecture-flows.md § 8.2 Caller-Side Bundling Convention](../../2-SoftwareDesignAndArchitecture/architecture-flows.md) and [§ 8.3 Verification Surface](../../2-SoftwareDesignAndArchitecture/architecture-flows.md).

**Status:** ✅ Done
**Severity:** Medium — closes the caller-side gap left by FEAT-20260609-002; without it, `flows_<workflow.code>.json` files on disk never reach the live agency.
**Owner:** CodeValdAgency (skill lives at `.claude/commands/dev-reimport-agency.md`)
**Estimated effort:** ~1 hour (in-session)
**Source finding:** 2026-06-10 session — investigation of whether utility-app-builder's `flows_planning.json` reaches the live agency. Bundler step was missing; verification was querying the legacy agency-level field instead of per-workflow.

---

## Problem

FEAT-20260609-002 shipped the schema and importer for per-workflow `event_flows`, but the server-side importer never touches the filesystem (deliberately, by design). The caller was expected to bundle `flows_<workflow.code>.json` siblings of `agency.json` into each workflow's `event_flows` field before POSTing — but the `/dev-reimport-agency` skill posted `agency.json` raw, with no bundling step. As a result, nothing in `flows_planning.json` (or any other `flows_*.json`) ever reached `Workflow.event_flows` on the live agency. The skill's verification step also queried `Agency.eventFlows` (the legacy monolithic blob), which no longer reflects the per-workflow design.

## Evidence

```text
$ grep -nc 'flows_<\|bundle' CodeValdAgency/.claude/commands/dev-reimport-agency.md
0   # before this FEAT
```

The skill POSTed `agency.json` directly to `${BASE}/agency/{id}/import`. The verification step:

```bash
curl -s "$URL/agency/$AGENCY_ID" | jq '.eventFlows | ...'  # legacy agency-level
```

Per-workflow `event_flows` were invisible from this query.

## Fix

Added Step 3 (Bundle) to [`.claude/commands/dev-reimport-agency.md`](../../../.claude/commands/dev-reimport-agency.md):

- Copies `agency.json` to a temp file.
- For each `workflows[*].code`, looks for sibling `flows_<code>.json` and `jq --slurpfile` injects its content into `workflows[i].event_flows`.
- Reports three states per workflow: `bundled` (file matched code), `skip` (workflow has no matching file), `orphan` (file exists with no matching workflow code).

Updated verification (Step 5) to query `GET /agency/{id}/workflows` and report `{name, flows, steps}` per workflow.

Dry-run verified:
- Negative path (no matching files): `Bundled 0`, `flows_planning.json` correctly logged as `orphan: no workflow with code 'planning'`.
- Positive path (temporarily renamed `flows_planning.json` → `flows_feature-development.json`): bundled correctly, JSON shape `{flows: 1, steps: 7}` round-tripped onto the matched workflow.

## Out of scope (follow-ups)

- Migrating `flows copy.json` content into per-workflow files — tracked in [FEAT-20260610-003](FEAT-20260610-003_utility_app_builder_flow_file_rename.md).
- Generalising the bundler into a reusable `/dev-bundle-agency` skill for other agencies — defer until a second agency exists.

## Verification

- [x] Bundling logic tested against the real `agency.json` (Bundled 0, orphan logged).
- [x] Positive path verified by temporarily renaming `flows_planning.json` to a matching code.
- [x] Verification step queries `GET /agency/{id}/workflows` and reports per-workflow counts.

## Dependencies

- [FEAT-20260609-002](FEAT-20260609-002_per_workflow_event_flows.md) — provides the `Workflow.event_flows` schema property and importer support.
