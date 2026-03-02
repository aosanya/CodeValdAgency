# CodeValdAgency — Software Development

## Overview

This directory contains development tracking, implementation notes, and coding session logs for the **CodeValdAgency** service — the agency-management microservice within the CodeValdCortex platform.

**Module**: `github.com/aosanya/CodeValdAgency`  
**Language**: Go 1.21+  
**Storage**: ArangoDB  
**Registration**: CodeValdCross `OrchestratorService.Register`

---

## 📁 Directory Structure

```
3-SofwareDevelopment/
├── README.md                         # This file — service overview & task index
├── coding-sessions.md                # Chronological coding session log
├── mvp-progress.md                   # MVP task tracker (active tasks)
├── updates/
│   ├── UPDATES_MVP005_COMMUNICATION_DESIGN.md  # MVP-005 design change log
│   └── DOCUMENTATION_UPDATE_AGENCY_OPERATIONS.md  # Agency ops docs update log
└── (future: core-systems/, deployment/, testing/)
```

---

## 🔑 Key Interfaces

| Interface | Methods |
|-----------|---------|
| `AgencyManager` | `CreateAgency`, `GetAgency`, `UpdateAgency`, `DeleteAgency`, `ListAgencies` |
| `Backend` | `Insert`, `Get`, `Update`, `Delete`, `List` |

## 🔄 Cross-Service Events

| Direction | Topic |
|-----------|-------|
| **Produces** | `cross.agency.created` |
| **Consumes** | *(none in Layer 1)* |

---

## 📄 Document Index

| Document | Description |
|----------|-------------|
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
