# CodeValdAgency — Software Development

## Overview

This directory contains development tracking, implementation notes, and coding session logs for the **CodeValdAgency** service — the agency-management microservice within the CodeValdCortex platform.

**Module**: `github.com/aosanya/CodeValdAgency`  
**Language**: Go 1.21+  
**Storage**: ArangoDB  
**Registration**: CodeValdCross `OrchestratorService.Register`

---

## Index

| Document | Description |
|---|---|
| [mvp.md](mvp.md) | Full MVP scope, task list, and completion status |
| [mvp-details/](mvp-details/README.md) | Per-topic task specifications |

---

## MVP Status

| Task ID | Title | Status |
|---|---|---|
| MVP-AGENCY-001 | Library Scaffolding & Agency Model | 🔲 Not Started |
| MVP-AGENCY-002 | ArangoDB Backend | 🔲 Not Started |
| MVP-AGENCY-003 | gRPC Service (AgencyService) | 🔲 Not Started |
| MVP-AGENCY-004 | CodeValdCross Registration | 🔲 Not Started |
| MVP-AGENCY-005 | Unit & Integration Tests | 🔲 Not Started |
| MVP-AGENCY-006 | Service-Driven Route Registration | 🔲 Not Started |

---

## Execution Order

```
MVP-AGENCY-001 → MVP-AGENCY-002 → MVP-AGENCY-003 → MVP-AGENCY-004 → MVP-AGENCY-005 → MVP-AGENCY-006
```

---

## Task Detail Files

| File | Tasks |
|---|---|
| [mvp-details/agency-management.md](mvp-details/agency-management.md) | MVP-AGENCY-001 through MVP-AGENCY-005 |
| [mvp-details/route-registrar.md](mvp-details/route-registrar.md) | MVP-AGENCY-006 |

---

## 🔑 Key Interfaces

| Interface | Methods |
|-----------|---------|
| `AgencyManager` | `CreateAgency`, `GetAgency`, `UpdateAgency`, `DeleteAgency`, `ListAgencies` |
| `Backend` | `Insert`, `Get`, `Update`, `Delete`, `List`, `InsertSnapshot` |

## 🔄 Cross-Service Events

| Direction | Topic |
|-----------|-------|
| **Produces** | `cross.agency.created` |
| **Consumes** | *(none in Layer 1)* |

---

## 📄 Document Index

| Document | Description |
|----------|-------------|
| [mvp.md](mvp.md) | Full MVP scope and task list |
| [mvp_done.md](mvp_done.md) | Completed tasks archive |
| [mvp-details/agency-management.md](mvp-details/agency-management.md) | Implementation specs for MVP-AGENCY-001 to 005 |
| [mvp-details/route-registrar.md](mvp-details/route-registrar.md) | Implementation spec for MVP-AGENCY-006 |
| [coding-sessions.md](coding-sessions.md) | Session-by-session development log |
| [updates/UPDATES_MVP005_COMMUNICATION_DESIGN.md](updates/UPDATES_MVP005_COMMUNICATION_DESIGN.md) | MVP-005 agent communication design changes |
| [updates/DOCUMENTATION_UPDATE_AGENCY_OPERATIONS.md](updates/DOCUMENTATION_UPDATE_AGENCY_OPERATIONS.md) | Agency operations framework doc additions |

---

## 🗺️ Related Documentation

| Section | Link |
|---------|------|
| Requirements | [../1-SoftwareRequirements/README.md](../1-SoftwareRequirements/README.md) |
| Architecture | [../2-SoftwareDesignAndArchitecture/README.md](../2-SoftwareDesignAndArchitecture/README.md) |
| QA | [../4-QA/README.md](../4-QA/README.md) |
