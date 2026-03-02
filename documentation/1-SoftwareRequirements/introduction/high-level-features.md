# High-Level Features

## Feature Overview

CodeValdAgency provides the following capabilities to the CodeVald platform:

---

### 1. Agency Entity Management

- **Create** an Agency with a unique name, Mission statement, and Vision
- **Read** an Agency by ID
- **Update** an Agency's attributes (name, description, lifecycle status)
- **Delete** an Agency
- **List** Agencies with optional filtering

Each Agency has a lifecycle: `Draft → Active → Achieved`.

---

### 2. Goals

An Agency pursues its Mission through one or more **Goals**.

| Field | Description |
|---|---|
| `id` | Unique identifier within the Agency |
| `title` | Short name for the Goal |
| `description` | Full description of what the Goal achieves |
| `ordinality` | Ordering / priority among Goals |

**Rules:**
- An Agency can have **multiple concurrent Goals**
- Goals do not have their own lifecycle — the Agency lifecycle reflects overall Goal progress
- Work Items reference one or more Goals to indicate which Goals they contribute to

---

### 3. Workflows and Work Items

An Agency executes its Goals through **Workflows** — named groupings of **Work Items**.

**Workflow:**
- A named container (no own lifecycle or status)
- Groups related Work Items under a common purpose
- Can span multiple Goals (Goal mapping lives at the Work Item level)

**Work Item:**
- Represents a discrete step in a Workflow
- **Ordered** within the Workflow (explicit sequence)
- Can optionally **run in parallel** with adjacent Work Items
- Maps to **one or more Goals** — contributing to Goal achievement
- Has a **Role** assigned with a **RACI label** (Responsible / Accountable / Consulted / Informed)

**Example structure:**

```
Agency: "Platform Security Hardening"
│
├── Goal-1: Reduce attack surface (ordinality: 1)
├── Goal-2: Achieve audit compliance (ordinality: 2)
│
└── Workflow: "Vulnerability Remediation"
    ├── Work Item 1: Scan all services       → Goal-1 [R: Security Officer]
    ├── Work Item 2: Triage findings         → Goal-1, Goal-2 [R: Technical Lead]
    └── Work Item 3: Patch critical issues   → Goal-1 [R: Agent Coordinator]
```

---

### 4. Roles

Every Agency has two **default roles** that are always present and cannot be removed:

| Role | Purpose |
|---|---|
| **Super Admin** | Platform-level administrator; full access to create, configure, and delete any Agency |
| **Admin** | Agency-level administrator; manages members, Workflows, and configuration |

Beyond these two, Agencies define their own role names as free-form strings stored in `ConfiguredRoles[]`. Only roles listed there may be assigned to Work Items.

See [stakeholders.md](stakeholders.md) for the RACI model and role assignment patterns.

---

### 5. Platform Event Integration

On successful `CreateAgency`, CodeValdAgency publishes:

```
cross.agency.created
```

This event is consumed by:
- **CodeValdGit** — initializes a Git repository for the Agency
- **CodeValdWork** — sets up task management for the Agency

---

## What CodeValdAgency Does NOT Do

| Out of Scope | Reason |
|---|---|
| Executing Work Items | Work execution is handled by CodeValdWork and AI agents |
| Managing tasks / tickets | Delegated to CodeValdWork |
| Git artifact storage | Delegated to CodeValdGit |
| Agent assignment logic | Handled by CodeValdCross orchestration |
| Authentication / authorization | Handled by the platform layer |
