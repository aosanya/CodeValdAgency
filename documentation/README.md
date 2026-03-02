# CodeValdAgency — Documentation

CodeValdAgency is the **agency lifecycle management** microservice in the CodeVald platform.

An **Agency** is an entity with a specific Mission. It coordinates AI agents toward a defined objective through structured Goals and Workflows.

---

## Documentation Sections

| Section | Description |
|---|---|
| [1 — Software Requirements](1-SoftwareRequirements/README.md) | Requirements, introduction, problem definition, and stakeholders |
| [2 — Software Design & Architecture](2-SoftwareDesignAndArchitecture/README.md) | Architecture, data models, interfaces, and service design |
| [3 — Software Development](3-SofwareDevelopment/README.md) | Development sessions, MVP tracking, and progress updates |
| [4 — QA](4-QA/README.md) | Testing strategy, test cases, and quality criteria |

---

## Quick Reference

| Item | Value |
|---|---|
| **gRPC Port** | `:50053` |
| **Storage** | ArangoDB — `agencies` collection |
| **Registers with** | CodeValdCross `OrchestratorService.Register` |
| **Publishes** | `cross.agency.created` on successful creation |
| **Module** | `github.com/aosanya/CodeValdAgency` |
