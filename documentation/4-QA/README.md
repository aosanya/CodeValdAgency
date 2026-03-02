# CodeValdAgency — QA & Testing

## Overview

This directory contains quality-assurance documentation, test strategies, and test data for the **CodeValdAgency** service.

---

## 📁 Directory Structure

```
4-QA/
├── README.md              # This file — QA overview & test index
└── (future: postman/, integration-tests/, test-data/)
```

---

## 🧪 Testing Strategy

### Unit Tests
- Interface-level tests with mock `Backend`
- Run with: `go test ./...` from the `CodeValdAgency/` root

### Integration Tests
- Require running ArangoDB instance and CodeValdCross server
- Environment variables: see `config.yaml` / `.env`

### gRPC Contract Tests
- Verify `AgencyService` proto contract compliance
- Client stub generated from `proto/codevaldagency/`

---

## ✅ Acceptance Criteria

| Scenario | Expected |
|----------|----------|
| `CreateAgency` — valid input | Returns `AgencyId`, publishes `cross.agency.created` |
| `CreateAgency` — duplicate name | Returns typed `ErrAlreadyExists` |
| `GetAgency` — unknown id | Returns typed `ErrNotFound` |
| `ListAgencies` — empty collection | Returns empty list, no error |
| Service registration | Registers with CodeValdCross within 30s of startup |

---

## 🗺️ Related Documentation

| Section | Link |
|---------|------|
| Requirements | [../1-SoftwareRequirements/README.md](../1-SoftwareRequirements/README.md) |
| Architecture | [../2-SoftwareDesignAndArchitecture/README.md](../2-SoftwareDesignAndArchitecture/README.md) |
| Development | [../3-SofwareDevelopment/README.md](../3-SofwareDevelopment/README.md) |
