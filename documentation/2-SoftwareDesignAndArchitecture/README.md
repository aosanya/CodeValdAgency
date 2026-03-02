# CodeValdAgency — Software Design & Architecture

## Overview

This directory contains the design and architecture documentation for the **CodeValdAgency** Go microservice — responsible for agency lifecycle management within the CodeValdCortex platform.

**Module**: `github.com/aosanya/CodeValdAgency`  
**Language**: Go 1.21+  
**Transport**: gRPC (`AgencyService`)  
**Storage**: ArangoDB  
**Registration**: CodeValdCross `OrchestratorService.Register`

---

## 📄 Documents

| File | Description |
|------|-------------|
| [architecture.md](architecture.md) | Core design decisions, interfaces, data models, ArangoDB schema, error types, configuration |

---

## 🗺️ Related Documentation

| Section | Link |
|---------|------|
| Requirements | [../1-SoftwareRequirements/README.md](../1-SoftwareRequirements/README.md) |
| Development | [../3-SofwareDevelopment/README.md](../3-SofwareDevelopment/README.md) |
| QA | [../4-QA/README.md](../4-QA/README.md) |

## Key Design Decisions at a Glance

| Decision | Choice | Rationale |
|---|---|---|
| Business-logic entry point | `AgencyManager` interface | gRPC handlers delegate to it; never put logic in handlers |
| Domain model | Embedded Goals + Workflows | Agency is the root aggregate; all sub-entities live inside it |
| Goal ordinality | Explicit `int` field | Deterministic ordering without relying on array position |
| Work Item execution | `Order` + `Parallel` fields | Supports sequential-by-default with optional concurrent execution |
| Goal mapping | `GoalIDs []string` per Work Item | A Work Item may advance multiple Goals simultaneously |
| Role model | Fixed 10-role enum | Reduces misconfiguration risk; roles are universal across all Agencies |
| RACI assignment | Per Work Item per Role | One step may have multiple RACI assignments (e.g. R + A on different roles) |
| Storage injection | `Backend` interface injected by `cmd/main.go` | Backend-agnostic core; easy to test with mocks |
| Downstream communication | gRPC only — no direct Go imports | Stable, versioned contracts; independent deployment |
| Cross registration | `OrchestratorService.Register` RPC on startup + heartbeat | Standard CodeVald onboarding pattern; liveness via repeat calls |
| Pub/sub event | `cross.agency.created` published on every `CreateAgency` | Cross listens to trigger `GitInitRepo` + Work setup |
| Error types | `errors.go` at module root | All exported errors in one place; no scattered sentinels |

---

## Component Architecture

```
github.com/aosanya/CodeValdAgency    ← module root
├── cmd/
│   └── main.go                      # Wires dependencies only — no business logic
├── go.mod
├── errors.go                        # ErrAgencyNotFound, ErrAgencyAlreadyExists
├── models.go                        # Agency, Goal, Workflow, WorkItem, RoleAssignment,
│                                    # AgencyRole, RACILabel, AgencyLifecycle, request types
├── internal/
│   ├── config/
│   │   └── config.go                # Configuration struct + loader (env / YAML)
│   ├── manager/
│   │   └── manager.go               # Concrete AgencyManager — holds Backend + CrossClient
│   └── server/
│       └── server.go                # Inbound gRPC server — AgencyService handlers
├── storage/
│   └── arangodb/
│       └── storage.go               # ArangoDB Backend implementation
├── proto/
│   └── codevaldagency/
│       └── agency.proto             # AgencyService gRPC definition
├── gen/
│   └── go/                          # Generated protobuf code (buf generate — do not hand-edit)
└── bin/
    └── codevaldagency-server        # Compiled binary
```

