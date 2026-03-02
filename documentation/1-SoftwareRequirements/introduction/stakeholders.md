# Stakeholders

## Platform Consumers

| Consumer | Role | Integration |
|---|---|---|
| **CodeValdCross** | Orchestrator — routes HTTP requests; registers CodeValdAgency as a service | Receives heartbeat every 20s; brokers `cross.agency.created` events |
| **CodeValdGit** | Consumes `cross.agency.created` | Calls `RepoManager.InitRepo(agencyID)` to initialize the Agency's Git repository |
| **CodeValdWork** | Consumes `cross.agency.created` | Sets up task management context for the Agency |

---

## Roles

Roles are **defined at the Agency level**. The role set is **fixed** across the platform, but Agencies choose which configurable roles to enable.

### Default Roles (always present)

Every Agency automatically has two management roles that cannot be removed:

| Role | Description |
|---|---|
| **Super Admin** | Platform-level administrator; can create, configure, and delete any Agency |
| **Admin** | Agency-level administrator; manages members, Workflows, and configurations |

### Configurable Roles (free-form, defined per Agency)

Beyond the two defaults, an Agency defines its own role names as free-form strings in `ConfiguredRoles[]`. There is no platform-enforced fixed list. Common examples include roles such as Agency Lead, Technical Lead, Domain Expert, or any domain-specific name the Agency requires. Only roles present in `ConfiguredRoles[]` may be assigned to Work Items.
| **Agency Lead** | Accountable owner of the Agency's Mission, Vision, and Goal progress |
| **Technical Lead** | Owns technical decisions, Workflow design, and Work Item specifications |
| **Domain Expert** | Provides domain knowledge; consulted on Goal definitions and Work Item design |
| **Quality Assurance** | Reviews and validates Work Item outputs against acceptance criteria |
| **Stakeholder Representative** | Represents business or external interests; informed on Agency progress |
| **Agent Coordinator** | Coordinates AI agent assignment, scheduling, and orchestration |
| **Data Analyst** | Analyzes outcomes and Goal achievement metrics |
| **Security Officer** | Consulted on security implications of Goals, Workflows, and agent operations |

### RACI Labels

Roles are assigned to Workflow Work Items with a **RACI label** for that step:

| Label | Meaning |
|---|---|
| **R** — Responsible | Does the work for this step |
| **A** — Accountable | Owns the outcome; approves the result |
| **C** — Consulted | Input required before the step proceeds |
| **I** — Informed | Notified when the step is complete |

---

## Role Assignment Pattern

Each Work Item in a Workflow is assigned one or more Role–RACI pairs:

```
Work Item: "Analyze training data"
  ├── Data Analyst           → R (Responsible)
  ├── Technical Lead         → A (Accountable)
  ├── Domain Expert          → C (Consulted)
  └── Agency Lead            → I (Informed)
```

This ensures every step has clear ownership, oversight, and communication paths — whether the role is filled by a human or an AI agent.

---

## Human vs. AI Agent Assignment

| Scenario | Supported |
|---|---|
| Human fills a role | ✅ |
| AI agent fills a role | ✅ |
| Mixed (human + AI on same Agency) | ✅ |

The role structure is actor-agnostic. The RACI label applies equally to human operators and AI agents executing Work Items autonomously.
