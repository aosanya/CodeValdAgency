# Problem Definition

## What is an Agency?

An **Agency** is an entity with a specific Mission. It is the primary organizing unit in the CodeVald platform — a purpose-driven container that coordinates AI agents toward a defined objective through structured Goals and Workflows.

An Agency is not a generic tenant or namespace. It is an **accountable unit**: it declares what it exists to do (Mission), where it intends to go (Vision), and what it is working toward right now (Goals).

---

## Mission

> **To coordinate AI agents toward a defined objective through structured goals and work items.**

An Agency's mission is always active and present-tense. It describes what the Agency *does*, not what it aspires to become.

---

## Vision

> **Become the unit of accountability for autonomous AI-driven work across the enterprise.**

Each Agency aspires to be fully accountable — meaning all work, outcomes, and responsibilities within it are traceable to clearly defined Goals, Workflow steps, and assigned Roles.

---

## The Problem Before CodeValdAgency

Before CodeValdAgency, the CodeVald platform had no structured entity to represent a purpose-driven unit of work. Specifically, there was no way to:

- **Declare a Mission** — work had no stated purpose or ownership boundary
- **Define Goals** — there was no goal structure to orient agent work toward outcomes
- **Own Workflows** — no entity owned the sequences of Work Items that agents executed
- **Assign accountability** — no RACI-based role structure to govern who was responsible for what
- **Track progress** — no lifecycle to reflect the overall state of a unit of work

Work existed as loosely connected tasks with no higher organizing principle. Agents could execute work items without a shared understanding of what they were collectively trying to achieve.

---

## The Solution

CodeValdAgency introduces the **Agency** as the top-level unit of purpose in the CodeVald platform.

When an Agency is created:

1. It is persisted to ArangoDB with a unique name
2. A `cross.agency.created` event is published — triggering **CodeValdGit** to initialize a repository and **CodeValdWork** to set up task management

Each Agency progresses through a lifecycle:

```
Draft ──► Active ──► Achieved
```

| Status | Meaning | Transition Trigger | Guard |
|---|---|---|---|
| **Draft** | Being configured; Goals and Workflows are being defined | — *(initial state)* | — |
| **Active** | Work is in progress; agents are executing Work Items | `UpdateAgency(status: active)` | Must have at least one Goal and one Workflow with at least one Work Item |
| **Achieved** | The Mission has been fulfilled; all Goals reached | `UpdateAgency(status: achieved)` | Caller must hold Super Admin or Admin role |

> Lifecycle transitions are **forward-only**. An Agency can never move back to a previous state. An `achieved` Agency is read-only.
