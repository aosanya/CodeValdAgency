---
description: Generate a Mermaid event-flow diagram from an agency's per-workflow flow files and write flowchart.md alongside the agency.json.
---

# Agency Flowchart Generator

Reads per-workflow `flows_<workflow.code>.json` siblings of `agency.json` and writes a colour-coded Mermaid flowchart as `flowchart.md` in the same directory.

> **Format note (FEAT-20260609-002):** event_flows now live in per-workflow files named `flows_<workflow.code>.json`, not inline in `agency.json`. Each file has shape `{"flows": [{"name": "...", "steps": [...]}]}` where each step is one of:
> - `type: "start"` — entry point. No `trigger`; has `emits_topics: [...]`.
> - regular step — has `trigger`, `trigger_publisher`, `consumer`, optional `action.emits_topics: [...]`, optional `on-error.emits_topics: [...]`.

---

## Step 1 — Resolve the agency path

If `$ARGUMENTS` is set, treat it as either:
- A full path to an `agency.json` file, or
- A short agency code (e.g. `utility-app-builder`), in which case search under `/workspaces/CodeVald-AIProject/CodeValdImplementations/Agencies/` for a matching directory.

If no argument is given, list all `agency.json` files under `/workspaces/CodeVald-AIProject/CodeValdImplementations/Agencies/` and ask the user to pick one.

Resolve the final absolute path to the `agency.json` before proceeding.

---

## Step 2 — Discover per-workflow flow files

Read `agency.json` and extract:
- `agency.code` — used for the heading
- `agency.name` — used for the heading
- `workflows[].code` and `workflows[].name` — used to locate per-workflow files and label subgraphs

For each workflow, look for a sibling `flows_<workflow.code>.json` in the same directory as `agency.json`. Skip workflows that have no matching file (they are legitimately empty).

If no flow files are found at all, print "no per-workflow flow files found beside agency.json" and exit — do not overwrite a possibly-correct `flowchart.md` with an empty diagram.

---

## Step 3 — Parse each flow file

Each per-workflow file is `{"flows": [{"name": "...", "steps": [...]}]}`.

For each `flow` in `flows`, walk `flow.steps[]` and categorise each step:

- **Start step** (`step.type == "start"`): produces one or more entry edges. Render an entry node `entry_<workflow_code>_<step_id>(("[<step.step>] <step.name>"))` and one edge per topic in `step.emits_topics[]` pointing into that topic node.
- **Regular step** (no `type`, has `trigger`): produces edges from the trigger node to each topic in `step.action.emits_topics` (solid) and to each topic in `step["on-error"].emits_topics` (dashed `-.->`).

`step.step` is the dotted step number (e.g. `1.1.1.1`). Use it verbatim in edge labels.

---

## Step 4 — Build the Mermaid diagram

### 4a — Collect nodes

Gather every unique topic name from:
- `trigger` on every regular step
- `action.emits_topics[]` and `on-error.emits_topics[]` on every regular step
- `emits_topics[]` on every start step

Node ID rule: replace every `.` and `-` with `_`.
Example: `task.needs-direction` → `task_needs_direction`

Node label: the raw topic name, e.g. `["task.assigned"]`.

### 4b — Emit nodes (grouped by workflow as subgraphs)

For each workflow that has a matching flow file, open a Mermaid subgraph:

```
subgraph <workflow_code_id>["<workflow.name>"]
    <node_id>["<topic_name>"]
    ...
end
```

A node belongs to a workflow when at least one step in that workflow's flow file references it as a `trigger`, an `action.emits_topics` entry, an `on-error.emits_topics` entry, or a start step `emits_topics` entry. A topic referenced by multiple workflows lands in the FIRST workflow that references it (order = `agency.json` workflow ordering).

Entry nodes (the `entry_<workflow_code>_<step_id>` synthetic nodes for start steps) go inside their workflow's subgraph.

### 4c — Emit edges

For each flow file (in workflow order) and each step within it (in array order):

- **Start step**: for each topic in `emits_topics`, emit
  ```
  entry_<workflow_code>_<step_id> -->|"[<step.step>] <step.name>"| <topic_id>
  ```
- **Regular step action**: for each topic in `action.emits_topics`, emit
  ```
  <trigger_id> -->|"[<step.step>] <step.name>"| <topic_id>
  ```
- **Regular step on-error**: for each topic in `on-error.emits_topics`, emit
  ```
  <trigger_id> -.->|"[<step.step>] on-error"| <topic_id>
  ```

Emit edges in step order so the diagram reads top-to-bottom.

### 4d — Colour nodes by publisher

A topic is published by either:
- The `consumer` of the regular step that emits it via `action.emits_topics` (the consumer performs the action then emits), OR
- The `trigger_publisher` of the regular step that first uses it as `trigger` (used as fallback when no step emits it explicitly), OR
- For start-only topics: `external` (white/grey).

Map each topic to its first publisher according to the rule above, then assign one Mermaid class per publisher service:

```
classDef work      fill:#dbeafe,stroke:#2563eb,color:#1e3a8a
classDef ai        fill:#dcfce7,stroke:#16a34a,color:#14532d
classDef functions fill:#fef3c7,stroke:#d97706,color:#78350f
classDef git       fill:#f3e8ff,stroke:#9333ea,color:#581c87
classDef comm      fill:#ffe4e6,stroke:#e11d48,color:#881337
classDef external  fill:#f3f4f6,stroke:#6b7280,color:#374151
```

Publisher → class:
- `codevaldwork` → `work`
- `codevaldai` → `ai`
- `codevaldfunctions` → `functions`
- `codevaldgit` → `git`
- `codevaldcomm` → `comm`
- unknown / start-only → `external`

Emit one `class <ids> <classname>` line per publisher grouping all matching node IDs.

Entry nodes (the synthetic start-step nodes) are always classed `external`.

---

## Step 5 — Write flowchart.md

Write to `flowchart.md` in the **same directory** as the agency.json. Always overwrite — this file is fully auto-generated.

```markdown
# <agency.name> — Event Flow

```mermaid
flowchart TD
    <subgraph blocks with their nodes>

    <edges>

    <classDef blocks>

    <class assignments>
```

_Auto-generated from `agency.json` + per-workflow `flows_<code>.json` files on <today's date YYYY-MM-DD>_
```

---

## Step 6 — Report

Tell the user:
- Full path of the written `flowchart.md`
- Number of workflows rendered (those with a flow file)
- Number of nodes and edges in the diagram
- Any workflows that were skipped because no `flows_<code>.json` sibling exists
