# CodeValdAgency — Software Requirements

## Overview

This directory captures the requirements for the **CodeValdAgency** microservice — the agency lifecycle management service in the CodeValdCortex platform.

---

## Scope

CodeValdAgency is responsible for:
- Creating, reading, updating, deleting, and listing **Agency** entities
- Persisting agencies to **ArangoDB** (`agencies` collection)
- Publishing `cross.agency.created` events on successful creation
- Registering with **CodeValdCross** `OrchestratorService.Register` on startup

---

## Key Requirements

| ID | Requirement | Priority |
|----|-------------|----------|
| REQ-001 | `CreateAgency` validates name uniqueness before insert | P0 |
| REQ-002 | `CreateAgency` publishes `cross.agency.created` after successful insert | P0 |
| REQ-003 | `GetAgency` returns typed `ErrNotFound` for unknown IDs | P0 |
| REQ-004 | All operations are context-cancellable (deadline propagated to ArangoDB) | P0 |
| REQ-005 | Service registers with CodeValdCross within 30s of startup; heartbeat every 20s | P0 |
| REQ-006 | gRPC server listens on `CODEVALDAGENCY_GRPC_PORT` (default `:50053`) | P1 |

---

## Introduction

| Document | Description |
|---|---|
| [Introduction / Problem Definition](introduction/problem-definition.md) | What an Agency is; Mission, Vision, and the problem CodeValdAgency solves |
| [High-Level Features](introduction/high-level-features.md) | Goals, Workflows, Work Items, lifecycle, and platform events |
| [Stakeholders & Roles](introduction/stakeholders.md) | Fixed role set, RACI model, and platform consumers |

---

## 🗺️ Related Documentation

| Section | Link |
|---------|------|
| Architecture | [../2-SoftwareDesignAndArchitecture/README.md](../2-SoftwareDesignAndArchitecture/README.md) |
| Development | [../3-SofwareDevelopment/README.md](../3-SofwareDevelopment/README.md) |
| QA | [../4-QA/README.md](../4-QA/README.md) |

This requirements documentation provides the foundation for developing CodeValdCortex as an enterprise-grade multi-agent AI orchestration platform that meets the demanding needs of modern enterprise environments.