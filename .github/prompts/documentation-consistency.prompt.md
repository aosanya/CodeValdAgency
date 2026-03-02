````prompt
---
agent: agent
---

# Documentation Consistency & Organization Checker

## Purpose
Perform systematic documentation consistency checks for **CodeValdAgency**
through **one question at a time**, identifying outdated references,
consolidating related files, and ensuring the documentation structure matches
the actual implementation.

---

## Instructions for AI Assistant

Conduct a comprehensive documentation consistency analysis through **iterative
single-question exploration**. Ask ONE question at a time, wait for the
response, then decide whether to:
- **🔍 DEEPER**: Go deeper into the same topic
- **📝 NOTE**: Record an issue/gap for later action
- **➡️ NEXT**: Move to the next consistency check area
- **📊 REVIEW**: Summarise findings and determine next steps

---

## Current Technology Stack (Reference)

```yaml
Service:
  Language: Go 1.21+
  Module: github.com/aosanya/CodeValdAgency
  gRPC: google.golang.org/grpc
  Storage: ArangoDB (arangodb/go-driver)
  Registration: CodeValdCross OrchestratorService.Register RPC

Key interfaces:
  - AgencyManager: CreateAgency, GetAgency, UpdateAgency, DeleteAgency, ListAgencies
  - Backend: Insert, Get, Update, Delete, List

Cross-service events:
  Produces: cross.agency.created
  Consumes: (none in Layer 1)

Documentation structure:
  1-SoftwareRequirements:
    requirements: documentation/1-SoftwareRequirements/requirements.md
    introduction: documentation/1-SoftwareRequirements/introduction/
  2-SoftwareDesignAndArchitecture:
    architecture: documentation/2-SoftwareDesignAndArchitecture/architecture.md
  3-SofwareDevelopment:
    mvp: documentation/3-SofwareDevelopment/mvp.md
    mvp-details: documentation/3-SofwareDevelopment/mvp-details/
  4-QA:
    qa: documentation/4-QA/README.md
```

---

## Consistency Check Areas (in priority order)

1. **Interface contract** — does `architecture.md` match actual Go interfaces in source?
2. **Data models** — do `models.go` field names match what's documented?
3. **Error types** — does `errors.go` match the error table in `architecture.md`?
4. **Registration payload** — does the `RegisterRequest` in code match Section 6 of `architecture.md`?
5. **HTTP routes** — do the declared routes match what's actually implemented?
6. **ArangoDB schema** — does the collection name and index in code match Section 7?
7. **mvp.md task status** — are completed tasks marked ✅?
8. **File size limits** — are any files over 500 lines? (run `wc -l` on suspects)

---

## Stop Conditions

- ❌ Any file in `documentation/` over 400 lines without a subfolder → **must refactor first**
- ❌ Architecture doc references interfaces that don't exist in code → **must update**
- ❌ `mvp.md` tasks marked 🔲 that are already implemented → **must update**
````
