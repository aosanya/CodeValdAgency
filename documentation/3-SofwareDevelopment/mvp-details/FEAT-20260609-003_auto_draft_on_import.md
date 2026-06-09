# FEAT-20260609-003 — Auto-draft-and-promote on import of a published agency

**Status:** ✅ Done (2026-06-09) — `auto_promote` flag added to `ImportDraftRequest` / `promoted` to `ImportDraftResponse`; `importSetDetails` swallows `ErrAgencyReadOnly` (after refreshing `event_flows`) when `auto_promote=true`; `ImportDraft` calls `PromoteDraft` on the auto-created/reused draft and maps the no-flag published case to `FAILED_PRECONDITION` (was `INTERNAL`). 4 new unit tests cover the matrix. Landed on main via `feature/Dev-AGENCY-FEAT20260609003_auto-draft-and-promote-on-import`.
**Severity:** Medium — every `POST /agency/{id}/import` against a published agency fails with `agency is read-only`, breaking operator scripts (scenario-12 QA Step 1, CI re-import flows, the `dev-reimport-agency` skill); operators presently work around it by manually invoking the draft → promote endpoints
**Owner:** CodeValdAgency
**Estimated effort:** ~0.5 day (importer flow change + opt-in flag + error path)
**Source finding:** `/document_issues` run during scenario-12 setup on 2026-06-09 — `curl -X POST .../import @agency.json` returned `INTERNAL: ImportDraft utility-app-builder: set details: agency is read-only: create and promote a draft to modify the live state`

---

## Problem

[`ImportDraft`](../../../internal/server/import_server.go) — the public import entry point — calls `SetAgencyDetails` directly on the live agency entity. If the agency was previously published, `agency.go` returns [`ErrAgencyReadOnly`](../../../errors.go) and the entire import rolls back. The HTTP response is:

```json
{
  "code": "INTERNAL",
  "message": "ImportDraft utility-app-builder: set details: agency is read-only: create and promote a draft to modify the live state"
}
```

The error message is technically correct — the draft-promote dance is the documented mutation path. But there are three legitimate operator flows that hit this every time:

1. **QA re-import** — scenario-12 Step 1 (and every QA series before it) POSTs agency.json to refresh fields. Each time, the operator has to manually:
   - `POST /agency/{id}/drafts` (create)
   - apply changes to the draft
   - `POST /agency/{id}/drafts/{draftId}/promote`

2. **CI re-import** — the [`dev-reimport-agency`](/home/vscode/.claude/skills/dev-reimport-agency/) skill is supposed to be one command; today it needs the same 3-step wrapper.

3. **`/document_issues`-style fixes** — when a per-field correction lands (e.g. add the missing `task-split-handler` work plan from [FEAT-20260609-001](FEAT-20260609-001_task_split_handler_workplan.md)), `POST /import` should "just work" against the live agency.

The current behavior pushes orchestration onto every caller and silently incentivises operators to skip the draft lifecycle entirely (delete-and-reimport from scratch, losing the audit trail).

## Evidence

```text
$ curl -s -X POST "http://codevaldcross:8081/agency/utility-app-builder/import" \
    -u "codevald:..." \
    --data-binary @agency.json
{
  "code": "INTERNAL",
  "message": "ImportDraft utility-app-builder: set details: agency is read-only: create and promote a draft to modify the live state"
}

$ grep -n 'ErrAgencyReadOnly' CodeValdAgency/internal/server/import_server.go
474:    // Published agencies block structural field updates. event_flows is display-only
476:    if errors.Is(err, codevaldagency.ErrAgencyReadOnly) && eventFlowsJSON != "" {
477:        _, efErr := s.mgr.SetAgencyEventFlows(ctx, eventFlowsJSON)
```

The importer already has special-case logic for `event_flows` (treating it as display-only, allowed on read-only agencies). That carve-out is the right shape but it does not cover the full payload.

## Root cause

`ImportDraft` was designed when the assumption was: imports always run against unpublished agencies. The promotion-required model emerged later (read-only enforcement, snapshots, publications). The importer was never re-aligned to the new model — it still calls `SetAgencyDetails` directly instead of going through the draft pipeline.

## Fix plan

### Option A (recommended) — Auto-draft when read-only is hit

Modify `ImportDraft` to, when the target agency is published:

1. Open a new internal `AgencyDraft` (label: `auto-import-YYYYMMDD-HHMMSS`).
2. Run the existing import logic against that draft scope (the importer already supports draft-scoped upserts — see the `DraftWorkflow`, `DraftWorkItem`, `DraftDeliverable` paths).
3. Call `PromoteDraft` immediately on the new draft.
4. Return the new draft ID + a `Promoted bool` field so the caller knows the import landed.

Concretely in [`internal/server/import_server.go`](../../../internal/server/import_server.go):

```go
// Pseudocode
agency, err := s.mgr.GetAgency(ctx, agencyID)
if err != nil { return nil, ... }

if agency.Published {
    if !req.AutoPromote {
        return nil, status.Error(codes.FailedPrecondition,
            "agency is published; pass auto_promote=true or create + promote a draft manually")
    }
    draft, derr := s.mgr.CreateDraft(ctx, CreateDraftRequest{
        AgencyID: agencyID,
        Label:    fmt.Sprintf("auto-import-%s", time.Now().UTC().Format("20060102-150405")),
    })
    if derr != nil { return nil, ... }
    draftID = draft.ID
    defer func() {
        if err == nil {
            _, perr := s.mgr.PromoteDraft(ctx, draftID)
            if perr != nil { /* surface */ }
        }
    }()
}

// existing import logic continues, scoped to draftID
```

### Phase 2 — Proto

Add to [`ImportDraftRequest`](../../../proto/codevaldagency/v1/agency.proto):

```proto
// auto_promote, when true, lets the importer auto-create a draft, apply the
// import, and promote it in one shot when the target agency is published.
// Default: false (caller must explicitly opt in).
bool auto_promote = N;
```

Add to [`ImportDraftResponse`](../../../proto/codevaldagency/v1/agency.proto):

```proto
// promoted is true when the import auto-promoted a draft on a published agency.
bool promoted = N;
// draft_id is the ID of the draft used (whether auto-created or pre-existing).
string draft_id = N;
```

Regenerate via `make proto`.

### Phase 3 — HTTP / Cross proxy

The Cross proxy already routes `POST /agency/{id}/import`. Accept an optional `?auto_promote=true` query param (or a JSON envelope `{"auto_promote": true, "agency": {...}}`) — pick whichever fits the current proxy convention. The QA Step 1 examples should be updated to pass `auto_promote=true`.

### Phase 4 — Docs

- [README.md](../../README.md): document the auto-promote behavior in the import section.
- Scenario 12 Step 1: switch from raw `--data-binary @agency.json` to the auto-promote-enabled form.
- `dev-reimport-agency` skill: pass `auto_promote=true`.

## Verification

- [ ] Import an unchanged agency.json against a published agency with `auto_promote=true` → 200, response shows `promoted=true, draft_id=...`. Verify a new AgencySnapshot was written.
- [ ] Import a modified agency.json (e.g. add a new work plan) with `auto_promote=true` → the change is live without a manual draft-promote step.
- [ ] Import with `auto_promote=false` against a published agency → `FAILED_PRECONDITION` (clean error, not `INTERNAL`).
- [ ] Import against a never-published agency → no auto-draft path, original direct-write behavior preserved.
- [ ] Audit trail: `GET /agency/{id}/drafts` shows the auto-import drafts with their `auto-import-...` labels and `promoted` status.

## Dependencies

- None blocking.
- Companion: [FEAT-20260609-002](FEAT-20260609-002_per_workflow_event_flows.md) (multi-flow event_flows) — easier to verify once auto-draft works, since each test iteration won't need a manual draft cycle.

## Open questions

- Should auto-promote skip if the diff is empty (no-op import)? Probably yes — avoid creating empty snapshots.
- Should the auto-created draft be visible in `GET /drafts` or hidden behind a `system: true` flag? Recommend visible with a clear label so operators can audit.
- Concurrency: if two callers race on auto-import, do we serialize on agency ID? Recommend a brief lock at the manager layer to avoid two simultaneous PromoteDraft calls competing for the same head pointer.
