# FEAT-20260609-001 — Add `task-split-handler` work plan to utility-app-builder agency

**Status:** 🚀 In Progress
**Severity:** Medium — blocks the split path of `flows_planning.json` (step 1.1.1); QA Work-3 cannot run end-to-end without it
**Owner:** CodeValdAgency (agency.json + handler agent definition)
**Estimated effort:** ~0.5 day (agent_code + work_plan entry + payload contract; reuses existing dispatchActions plumbing)
**Source finding:** Series-12 QA design pass on 2026-06-09 — `flows_planning.json` step 1.1.1 references `task-split-handler` but `Agencies/utility-app-builder/agency.json` does not declare it
**Related QA:** [12/work-03-task-complete.md](../../../../CodeValdCross/documentation/4-QA/agencies/utility-app-builder/12/work-03-task-complete.md) (guard at Step 0 skips the whole test when this handler is absent)

---

## Problem

`flows_planning.json` declares the split-path consumer:

```json
{
  "step": "1.1.1",
  "name": "Split task into parallel child tasks",
  "trigger": "task.request-split",
  "trigger_publisher": "codevaldai",
  "consumer": "codevaldai",
  "description": "task-split-handler emits task.subtask-created for each child in the payload.",
  "action": {
    "emits_topics": ["task.subtask-created"],
    "handler": "task-split-handler"
  },
  "on-error": { "emits_topics": ["task.split-failed"] }
}
```

But no `task-split-handler` work plan exists in `Agencies/utility-app-builder/agency.json`. As a result:

- When `planner-assigned-handler` emits `task.request-split` (with a `children` array), nothing consumes it.
- CodeValdWork never receives any `task.subtask-created` events, so no subtasks are persisted.
- Parent task stalls in IN_PROGRESS with no progress signal.
- The Work-3 split-path QA test self-skips at its Step 0 guard.

## Evidence

```
$ grep code Agencies/utility-app-builder/agency.json | grep split
(no match)

$ curl -s "${BASE}/agency/utility-app-builder/work-plans" -u "$CV_AUTH" \
   | python3 -c "import sys,json; ps=json.load(sys.stdin)['workPlans']; \
       print([p.get('properties',p).get('code') for p in ps if 'split' in p.get('properties',p).get('code','')])"
[]
```

## Goal

Declare a `task-split-handler` work plan in `agency.json` so the split path becomes fully exercisable:

- `code: task-split-handler`
- `trigger_topic: task.request-split`
- `handler_service: codevaldai`
- `agent_code:` an AIAgent (new or existing) that iterates the `children` array on the inbound payload and emits one `task.subtask-created` per child.

## Design

### Work plan entry (agency.json)

```json
{
  "code": "task-split-handler",
  "trigger_topic": "task.request-split",
  "handler_service": "codevaldai",
  "agent_code": "deepseek-v4-splitter",
  "function_params": {
    "emit_topic": "task.subtask-created",
    "iterate_field": "children"
  },
  "enabled": true,
  "ordinality": 25
}
```

### Agent definition (ai_config)

Either reuse an existing developer agent with a focused split instruction, or declare a small dedicated agent:

```json
{
  "agents": [
    {
      "code": "deepseek-v4-splitter",
      "provider_code": "deepseek-v4",
      "instructions": "For each child in payload.children, emit one task.subtask-created action with: title, description, role_name, depends_on (mapped from temp_ids to subtask IDs by CodeValdWork). Do not invent children. Do not modify ordering. One action per child."
    }
  ]
}
```

### Payload contract (mirrors the children block in flows_planning.json step 1.1.1)

```json
{
  "task_id": "string — injected by CodeValdAI dispatchActions",
  "workflow_run_id": "string — injected by CodeValdAI dispatchActions",
  "children": [
    {
      "temp_id": "string",
      "title": "string",
      "description": "string",
      "role_name": "string",
      "task_name": "string (optional)",
      "depends_on": ["temp_ids of prerequisite children"]
    }
  ]
}
```

### Error path

If the handler cannot iterate or the children array is empty/malformed, emit `task.split-failed` (matches the on-error topic in `flows_planning.json` step 1.1.1). See [12/work-99-failure-paths.md F-3](../../../../CodeValdCross/documentation/4-QA/agencies/utility-app-builder/12/work-99-failure-paths.md).

## Non-goals

- Changing CodeValdAI's dispatch plumbing — `task.subtask-created` consumption by CodeValdWork already exists (flow step 1.1.1.1).
- Changing the planner's split-vs-decompose decision logic.
- A new SDK or Functions integration — this is purely an agency.json + agent prompt change.

## Fix plan

1. Add a `deepseek-v4-splitter` (or rename to a suitable existing agent) to `ai_config.agents` in `Agencies/utility-app-builder/agency.json` with the split-only instruction above.
2. Add the `task-split-handler` work_plan entry to `work_plans` in the same file.
3. Reimport the agency (`POST /agency/utility-app-builder/import`).
4. Re-run [12/work-03-task-complete.md](../../../../CodeValdCross/documentation/4-QA/agencies/utility-app-builder/12/work-03-task-complete.md) end-to-end — guard at Step 0 should pass; subtasks should be created and dependency-free ones re-assigned.

## Verification

- `GET /agency/utility-app-builder/work-plans` returns a work plan with `code=task-split-handler`, `trigger_topic=task.request-split`, `enabled=true`.
- Assigning a complex task (≥ 3 independent deliverables) results in:
  - `task.request-split` published by planner (already works).
  - **NEW:** `task.subtask-created` events published by `task-split-handler` (one per child).
  - `task.subtasks-persisted` published by CodeValdWork.
  - `task.assigned` re-fires for dependency-free subtasks (already works).
- Work-99 F-3 (empty children payload) emits `task.split-failed` rather than silently dropping the event.

## Dependencies

- None on other services. The downstream consumer (CodeValdWork's handler for `task.subtask-created` → persist + dependency-resolve → re-emit `task.assigned`) already exists.
- Optional: if a dedicated split-only agent is preferred over reusing an existing developer agent, the new `AIAgent` and `AIProvider` (if not already declared) must be added first — both already supported by the import schema.
