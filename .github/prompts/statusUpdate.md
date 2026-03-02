````prompt
---
agent: agent
---

# CodeValdAgency — Status Update Prompt

## Purpose
Record status updates, findings, and progress notes for **CodeValdAgency** into
topic files under:

```
CodeValdAgency/documentation/3-SofwareDevelopment/status/
```

---

## 📊 CodeValdAgency — Current Capabilities

> For full architecture details see
> `documentation/2-SoftwareDesignAndArchitecture/architecture.md`

### Role in the Platform
CodeValdAgency is the **agency management service** — it owns the authoritative
record of every Agency. Every other service scopes its data by `agencyID`.

### gRPC Endpoints (Inbound)

| Service | Method | Description |
|---|---|---|
| `AgencyService` | `CreateAgency` | Creates a new agency; publishes `cross.agency.created` |
| `AgencyService` | `GetAgency` | Returns an agency by ID |
| `AgencyService` | `UpdateAgency` | Updates agency metadata |
| `AgencyService` | `DeleteAgency` | Soft-deletes an agency |
| `AgencyService` | `ListAgencies` | Paginated agency list |

### HTTP Routes (proxied via CodeValdCross)

| Method | Pattern |
|---|---|
| `POST`   | `/agencies` |
| `GET`    | `/agencies` |
| `GET`    | `/agencies/{agencyID}` |
| `PUT`    | `/agencies/{agencyID}` |
| `DELETE` | `/agencies/{agencyID}` |

### Pub/Sub

| Topic | Direction | Description |
|---|---|---|
| `cross.agency.created` | **produces** | Published after every successful `CreateAgency` |

### Key Design Properties
- **Single interface** — `AgencyManager` is the only business-logic entry point
- **Backend-agnostic** — `Backend` interface injected; ArangoDB is the production impl
- **Heartbeat** — `Register` called every 20 s; Cross treats repeat calls as liveness
- **No AI/LLM logic** — this service manages agencies only

---

## 🗂️ Status File Rules

### Target directory
```
CodeValdAgency/documentation/3-SofwareDevelopment/status/
```

### File size limit
- **≤ 400 lines** → write/append to a single topic file: `status/{topic}.md`
- **> 400 lines** → escalate to a subfolder with a `README.md` index

### Workflow (enforce every session)

```bash
# Step 1 — Check existing file size
wc -l documentation/3-SofwareDevelopment/status/{topic}.md

# Step 2 — Choose write target
# If file doesn't exist → create status/{topic}.md
# If file ≤ 400 lines  → append to status/{topic}.md
# If file > 400 lines  → create status/{topic}/ subfolder

# Step 3 — Write the status entry
```

---

### Status Entry Format

```markdown
## {YYYY-MM-DD} — {Short title}

**Status**: {In Progress | Blocked | Done | Investigating}
**Topic**: {agency-lifecycle | storage | grpc-service | cross-registration | general}

### What changed / was found
- ...

### Gaps / open questions
- ...

### Next actions
- [ ] ...
```

---

### Topic → File Mapping

| Topic | File |
|---|---|
| Agency lifecycle (create, get, update, delete) | `status/agency-lifecycle.md` |
| Storage / ArangoDB backend | `status/storage.md` |
| gRPC service and proto | `status/grpc-service.md` |
| Cross registration & heartbeat | `status/cross-registration.md` |
| General / cross-cutting | `status/general.md` |
| Recommendations | `status/recommendations/codevaldagency.md` |
````
