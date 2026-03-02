````prompt
---
agent: agent
---

# Research & Documentation Gap Analysis Prompt

## Purpose
This prompt guides a structured Q&A session to explore and complete documentation
for any feature or architectural component in **CodeValdAgency** through
**one question at a time**, allowing for deep dives into specific topics.

---

## 🔄 MANDATORY REFACTOR WORKFLOW (ENFORCE BEFORE ANY RESEARCH SESSION)

**BEFORE starting any research or writing new task documentation:**

### Step 1: CHECK File Size
```bash
wc -l documentation/3-SofwareDevelopment/mvp-details/{topic-file}.md
```

### Step 2: IF >500 lines OR individual AGENCY-XXX.md files exist:

**a. CREATE folder structure:**
```bash
documentation/3-SofwareDevelopment/mvp-details/{domain-name}/
├── README.md              # Domain overview, architecture, task index (MAX 300 lines)
├── {topic-1}.md           # Topic-based grouping of related tasks (MAX 500 lines)
└── {topic-2}.md
```

**b. CREATE README.md** with:
- Domain overview
- Architecture summary
- Task index with links

**c. SPLIT content by TOPIC (NOT by task ID)**

**d. MOVE architecture diagrams** → `architecture/` subfolder

**e. MOVE examples** → `examples/` subfolder

### Step 3: ONLY THEN add new task content to appropriate topic file

---

## 🛑 STOP CONDITIONS (Do NOT proceed until fixed)

- ❌ **Domain file exceeds 500 lines** → **MUST refactor first**
- ❌ **README.md exceeds 300 lines** → **MUST split content**
- ❌ **Individual `AGENCY-XXX.md` files exist** → **MUST consolidate by topic**

---

## Instructions for AI Assistant

Conduct a comprehensive documentation gap analysis through **iterative
single-question exploration**. Ask ONE question at a time, wait for the
response, then decide whether to:

- **Go Deeper**: Ask follow-up questions on the same topic
- **Take Note**: Record a gap for later exploration
- **Move On**: Proceed to the next topic area
- **Review**: Summarise what we've learned and identify remaining gaps

---

## Research Framework

### Current Technology Stack (Reference)

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

### Research Areas (in priority order)

1. **Agency data model** — what fields does an `Agency` need?
2. **`CreateAgency` flow** — validation, storage, publish sequence
3. **Cross registration** — what routes does Agency declare to Cross?
4. **ArangoDB schema** — collection name, document structure, indexes
5. **Error handling** — what error cases need typed errors?
6. **gRPC proto definition** — `AgencyService` method signatures
7. **Configuration** — what env vars / YAML keys does the service need?
8. **Testing strategy** — unit tests with mock Backend; integration tests?
````
