````prompt
---
agent: agent
---

# Complete and Merge Current Task

Follow the **mandatory completion process** for CodeValdAgency tasks:

## Completion Process (MANDATORY)

1. **Validate code quality**
   ```bash
   go build ./...           # Must succeed — no compilation errors
   go test -v -race ./...   # Must pass — all tests green, no races
   go vet ./...             # Must show 0 issues
   golangci-lint run ./...  # Must pass
   ```

2. **Remove all debug logs before merge (MANDATORY)**
   - Remove all `fmt.Printf`, `fmt.Println` debug statements
   - Remove all `log.Printf` / `log.Println` debug statements (keep production error logging only)
   - After removal: `go vet ./...` catches unused variables/imports
   - After removal: verify `go build ./...` still succeeds

   ```bash
   # Check for leftover debug output
   grep -r "fmt.Printf\|fmt.Println" . --include="*.go"
   grep -r "log.Printf.*AGENCY-\|log.Println.*AGENCY-" . --include="*.go"
   ```

3. **Verify service contract compliance**
   - [ ] All new exported symbols have godoc comments
   - [ ] All new exported methods accept `context.Context` as first argument
   - [ ] `Backend` is injected — no hardcoded ArangoDB in manager
   - [ ] `cross.agency.created` is published on every successful `CreateAgency`
   - [ ] No AI/LLM logic, no frontend serving added
   - [ ] Errors are typed (`ErrAgencyNotFound`, not raw strings)
   - [ ] No file exceeds 500 lines
   - [ ] gRPC handlers delegate to `AgencyManager` — no business logic in handlers

4. **Update documentation if architecture changed**
   - If the implementation deviated from `documentation/2-SoftwareDesignAndArchitecture/architecture.md`, update it
   - If a new design decision was made, add it to the decision table
   - If an open question in `documentation/1-SoftwareRequirements/requirements.md` was resolved, update it
   - Update task status in `documentation/3-SofwareDevelopment/mvp.md` (🔲 → ✅)
   - Update task status in `documentation/3-SofwareDevelopment/mvp-details/README.md`

5. **Merge to main**
   ```bash
   # Final validation
   go build ./...
   go test -v -race ./...
   go vet ./...

   # Commit implementation
   git add .
   git commit -m "AGENCY-XXX: Implement [description]

   - Key implementation detail 1
   - Key implementation detail 2
   - Removed all debug logs
   - All tests pass with -race
   "

   # Merge to main
   git checkout main
   git merge feature/AGENCY-XXX_description --no-ff -m "Merge AGENCY-XXX: [description]"
   git branch -d feature/AGENCY-XXX_description
   ```

## Success Criteria

- ✅ `go build ./...` succeeds
- ✅ `go test -race ./...` passes — all tests green, no data races
- ✅ `go vet ./...` shows 0 issues
- ✅ All debug logs removed
- ✅ Service contract compliance verified (godoc, context, injected backend, event published)
- ✅ Documentation updated if architecture changed
- ✅ Merged to `main` and feature branch deleted
````
