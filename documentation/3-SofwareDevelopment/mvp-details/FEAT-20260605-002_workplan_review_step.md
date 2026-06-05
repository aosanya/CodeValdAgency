# FEAT-20260605-002 — WorkPlan review step type

**Status:** 📋 Not Started
**Severity:** Medium — required to gate task progression on AcceptanceCriteria results
**Owner:** CodeValdAgency
**Estimated effort:** ~2 days (schema + WorkPlan fields + agency.json wiring + tests)
**Source finding:** Research session 2026-06-05 — task decomposition improvement discussion
**Depends on:** [FEAT-20260605-001 (CodeValdWork schema v4)](../../../CodeValdWork/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260605-001_deliverable_acceptance_criteria_schema.md)

---

## Problem

Once `Deliverable` and `AcceptanceCriteria` entities exist in CodeValdWork, there is no mechanism to enforce that all criteria are satisfied before the WorkPlan advances to the next step. The gate must live in the WorkPlan — not in CodeValdWork (which is a passive data store) and not in a hard-coded rule inside any single service.

## Goal

Extend the `WorkPlan` entity in CodeValdAgency to support a **review step**: a named step defined in `agency.json` whose trigger fires after a task completes and whose success event unlocks the next WorkPlan step. The review step type determines who or what evaluates the acceptance criteria.

Initial review types:
- `"ai_review"` — CodeValdAI reads Deliverables + AcceptanceCriteria and writes results (FEAT-20260605-003)
- `"human_review"` — operator manually sets `result` fields via the API; WorkPlan waits for a success event
- `"functional_review"` — a Functions job evaluates the criteria and emits the result event

## Non-goals

- The reviewer logic itself — that is FEAT-20260605-003.
- Hard-coding review logic inside CodeValdWork.
- A UI for human review (deferred).

---

## Design

### WorkPlan schema additions (`schema.go` in CodeValdAgency)

Add to the `WorkPlan` TypeDefinition:

| Field | Type | Notes |
|---|---|---|
| `review_step_type` | string | `"ai_review"`, `"human_review"`, `"functional_review"`. Empty = no review step |
| `review_trigger_topic` | string | Cross topic that fires the review agent, e.g. `"work.task.completed"` |
| `review_success_topic` | string | Cross topic the reviewer emits on pass, e.g. `"work.review.passed"` |
| `review_failure_topic` | string | Cross topic the reviewer emits on fail, e.g. `"work.review.failed"` |

### agency.json pattern

```json
{
  "work_plans": [
    {
      "code": "implement-task",
      "trigger_conditions": [{ "topic": "work.task.assigned" }],
      "review_step_type": "ai_review",
      "review_trigger_topic": "work.task.completed",
      "review_success_topic": "work.review.passed",
      "review_failure_topic": "work.review.failed"
    }
  ]
}
```

The next WorkPlan step's `TriggerConditions` must reference `review_success_topic`, not `work.task.completed` directly, so it only fires after the review gate clears.

### Event flow

```
work.task.assigned
  → WorkPlan step fires → AI executes task
    → work.task.completed
      → review_step fires (ai_review | human_review | functional_review)
        → work.review.passed  → next WorkPlan step unlocks
        → work.review.failed  → recovery / direction flow
```

---

## Files to create / modify

| File | Change |
|---|---|
| `schema.go` | Add four new properties to `WorkPlan` TypeDefinition |
| `models.go` | Add `ReviewStepType`, `ReviewTriggerTopic`, `ReviewSuccessTopic`, `ReviewFailureTopic` fields to `WorkPlan` Go value type |
| `proto/codevaldagency/v1/agency.proto` | Add fields to `WorkPlan` message; regenerate with `make proto` |
| `internal/server/` | Update gRPC handlers to read/write the new WorkPlan fields |

---

## Acceptance tests

- A `WorkPlan` with `review_step_type = "ai_review"` round-trips through gRPC correctly.
- A `WorkPlan` with `review_step_type = ""` (no review) is unchanged from current behaviour.
- `agency.json` import correctly populates all four new fields.
- `go build ./...` succeeds.
- `go test -race ./...` passes.

---

## Dependencies

- Depends on FEAT-20260605-001 (Deliverable + AcceptanceCriteria must exist in Work schema before review can be useful)
- FEAT-20260605-003 (AI reviewer) depends on this providing the trigger/success/failure topics
