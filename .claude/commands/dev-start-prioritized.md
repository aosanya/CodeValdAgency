---
description: Pick the next task from prioritization.md, create a Dev- prefixed feature branch in the target repo, walk through the standard implementation workflow, and run /dev-document-feature before merging back.
---

# Start Prioritized Task

> ⚠️ **Before starting**, after picking a task, confirm no feature branch is currently open in **the target repo** for that task.
> If one exists, finish and merge it first in that repo only (see the **Finish** section at the bottom). Branches in other repos do not block you.

The **authoritative task list** for the whole platform is:
`/workspaces/CodeVald-AIProject/CodeValdCross/documentation/3-SofwareDevelopment/prioritization.md`

All task selection, status updates, and done-tracking flow through that file.

---

## Step 1 — Sweep `prioritization.md` Down to Active Work

**Do this first, on every invocation, before selecting anything.** Do not skip, defer, or batch this with later edits.

`prioritization.md` is the *active to-do list* — nothing else. Anything that is not a currently-actionable row is bloat and must be removed. The sweep has **five passes**; all are mandatory. The most common failure modes are stale rows and accumulated history — Passes C–E exist because they regularly slip past anyone trusting the active-table status alone.

> **File-size sanity check.** A healthy `prioritization.md` is typically **under ~80 lines** when only active rows remain. If it's significantly longer after your sweep, Passes C–E almost certainly have more to remove — go back and re-check.

### Pass A — drop rows already marked done in the table

1. Read `prioritization.md` (path above).
2. Remove **every row** whose Status is `✅ Done` / `✅ Fixed` (or any other "complete" marker — e.g. ✔, "Done", "Complete", "Merged").

### Pass B — drop rows that are *actually* done but still mislabelled in the table

Tasks frequently get merged and recorded in a service's `mvp_done.md` / `bugs_done.md` without anyone clearing the row from `prioritization.md`. These rows are **stale** and must be removed regardless of what their in-table status says.

For **every remaining row** in the active table:

1. Identify the target service from the Task ID prefix (use the table in Step 3).
2. Open that service's `documentation/3-SofwareDevelopment/mvp_done.md` (features/MVP) or `bugs_done.md` (bugs).
3. `grep` for the row's **exact Task ID** (e.g. `MVP-DT-006`, `FEAT-20260602-001`, `BUG-20260603-004`).
   - For per-service IDs that collide across services (e.g. `FEAT-20260602-001` exists in multiple services), only the **target service's** done-file counts.
4. If the Task ID appears in that done-file:
   - The task is done. Remove the row from `prioritization.md`.
   - Also strike through any references to it in other rows' `Depends On` columns (`~~TASK-ID~~ ✅`).
   - Briefly note the removal to the user (Task ID + the done-file date you found).
5. If the Task ID does **not** appear, leave the row alone.

### Pass C — delete "Landed:" / historical prose blocks

Completed work tends to leave behind "Landed:", "Landed (all X merged YYYY-MM-DD):", or similar bullet lists of `~~strikethrough~~` items inside `prioritization.md`. **These are historical records and they do not belong in the master file** — they belong in each service's `mvp_done.md` / `bugs_done.md`, which already capture the same information.

For every such block:

1. Confirm each `~~ITEM~~` it contains is a *completed* item (struck-through, "merged YYYY-MM-DD", "landed", or otherwise terminal).
2. Confirm each item is recorded in its service's `mvp_done.md` / `bugs_done.md`. If it isn't, **stop and report** — the done-file is missing an entry; that gets fixed before the row is deleted from `prioritization.md`.
3. Delete the entire `Landed:` paragraph (heading line + bullet list) once everything in it is accounted for in the proper done-file.

Do **not** leave a "Landed:" block "for context" — context lives in the done-files and in `git log`.

### Pass D — collapse orphan section headers + source narrative

`prioritization.md` accrues section scaffolding when items are added:

```
## QA 03 bugs (incoming 2026-06-03)

Source: QA scenario 03 run 2026-06-03 — ...

| Bug ID | Title | ... |
|--------|-------|-----|
| ... one row ... |
```

When Passes A–C empty out or shrink a section:

1. If the section's table is **empty** *and* its `Landed:` block (if any) was removed in Pass C: delete the entire section — heading, source narrative, table skeleton, separator. Nothing remains worth keeping.
2. If the section still holds at least one active row: keep the heading + table, but **delete the "Source:" / "Incoming:" narrative paragraph** unless the row itself genuinely depends on that context. A one-line "Source: ..." for a single still-active row is fine; multi-paragraph design notes are not.

The active table at the top of the file (`## Active Task Table`) is the canonical home for active rows. Per-incident section tables exist only to group *currently active* items by their introducing event; once empty, they go.

### Pass E — drop per-task detail tables that duplicate the active row

Sections like `## CodeValdOrg — Task Details` followed by per-task subsections (`#### ORG-020 — Integration tests`) restate information already in the active table. They bloat the file with no new actionable content. For every such per-task detail block:

1. Confirm the actionable scope (and any link to a per-service `mvp-details/` doc) is already captured in the active-table row — typically the `Reason` column plus the row's link to the per-service detail file.
2. If yes: delete the per-task detail block (heading, scope paragraphs, link line, separator).
3. If a useful detail genuinely isn't anywhere else: move it to the per-service `mvp-details/` doc, then delete the block here.

Per-service workflow boilerplate ("Completion Process", "Branch Management", "Status Legend") that duplicates this skill or the per-service README must also be removed — the skill itself is the workflow source of truth.

### Save and report

Save `prioritization.md` once all five passes are complete. If you removed rows, "Landed:" blocks, orphan sections, or detail tables, list each removal to the user before moving to Step 2 (one short line each — e.g. "Removed Landed: block from 2026-06-09 section (6 items, all present in respective done-files)"). The sweep runs whether or not you find a task to start.

After saving, **run `wc -l`** on the file and report the new line count. If it's still well over ~80 lines without a clear reason (e.g. many genuinely-active rows), call that out — it usually means a pass missed something.

> Rationale: the active table is the *to-do list*, not the history, not a design doc, not a per-task spec. History lives in `mvp_done.md` / `bugs_done.md`; design lives in `architecture.md`; per-task specs live in `mvp-details/`. Anything else in `prioritization.md` accumulates as noise that hides the few rows that actually matter. Sweeping aggressively every run keeps the file scannable and keeps Step 2's "first row" selection honest.

---

## Step 2 — Select the Next Task

1. Re-read `prioritization.md` after the sweep.
2. **Build the busy-repo set first.** Scan every row whose Status is `🚀 In Progress` and collect the set of target repos those tasks belong to (use the Task ID prefix → repo lookup in Step 3). These repos are **busy** — no new work should pile on top of them.
3. Pick the **first row** (top of the file = highest priority) that satisfies **all** of:
   - Status is `📋 Not Started` **or** `📋 Open` — both mean "ready to pick up." (Features and MVP tasks use `📋 Not Started`; bug rows use `📋 Open`. Never `🚀 In Progress`, never `⏸️ Blocked`, never `✅ Done`, never `✅ Fixed`.)
   - Its target repo is **not in the busy-repo set** from step 2.
   - All `Depends On` items are ✅ complete.
4. If no eligible row exists (every pickable task targets a busy repo, or there are no `📋 Not Started`/`📋 Open` rows at all), **stop and report this to the user** — list the busy repos and the first few skipped tasks so they can decide whether to finish an in-progress task first.
5. Note the **Task ID**, **Title**, **Service**, **Depends On**, and **starting Status sentinel** (`📋 Not Started` or `📋 Open`) of the selected row. Step 5 uses the sentinel as the literal `old_string` for the Edit — they're not interchangeable.
6. ⚠️ **Always** flip the selected task to `🚀 In Progress` in Step 4 before doing anything else — this is what frees the user to start a new task each invocation. Never leave a freshly-picked task at `📋 Not Started` or `📋 Open`.
7. ⚠️ Do not overthink!!!
   - **Update status on `main` in both repos** (prioritization.md + service mvp.md/bugs.md) and **push** — Step 4
   - **Then** create the feature branch — Step 5
   - Dive into the task and do it! WE have never deployed the applications, do not be afraid of breaking things, especially tests

> Rationale: one in-progress task per repo at a time. Stacking a second feature branch on a repo whose first branch isn't merged yet creates merge conflicts and confuses the Finish step. Spreading work across repos keeps each branch independent.
---

## Step 3 — Identify the Target Repository

Use the Task ID prefix to look up the correct repo, branch prefix, and validation stack:

| Task ID prefix | Service | Repository path | Branch prefix | Validation |
|---|---|---|---|---|
| `MVP-AI-` | CodeValdAI | `/workspaces/CodeVald-AIProject/CodeValdAI` | `feature/Dev-AI-NNN_` | Go |
| `SHAREDLIB-` | CodeValdSharedLib | `/workspaces/CodeVald-AIProject/CodeValdSharedLib` | `feature/Dev-SHAREDLIB-NNN_` | Go |
| `CROSS-` | CodeValdCross | `/workspaces/CodeVald-AIProject/CodeValdCross` | `feature/Dev-CROSS-NNN_` | Go |
| `MVP-GIT-` | CodeValdGit | `/workspaces/CodeVald-AIProject/CodeValdGit` | `feature/Dev-GIT-NNN_` | Go |
| `MVP-WORK-` | CodeValdWork | `/workspaces/CodeVald-AIProject/CodeValdWork` | `feature/Dev-WORK-NNN_` | Go |
| `MVP-AGENCY-` | CodeValdAgency | `/workspaces/CodeVald-AIProject/CodeValdAgency` | `feature/Dev-AGENCY-NNN_` | Go |
| `MVP-DT-` | CodeValdDT | `/workspaces/CodeVald-AIProject/CodeValdDT` | `feature/Dev-DT-NNN_` | Go |
| `MVP-COMM-` | CodeValdComm | `/workspaces/CodeVald-AIProject/CodeValdComm` | `feature/Dev-COMM-NNN_` | Go |
| `MVP-HI-` | CodeValdHi | `/workspaces/CodeVald-AIProject/CodeValdHi` | `feature/Dev-HI-NNN_` | Flutter |

> `NNN` is the numeric portion of the Task ID.
> Examples:
> - `MVP-AI-001` → `feature/Dev-AI-001_module-scaffolding`
> - `MVP-DT-002` → `feature/Dev-DT-002_arangodb-backend`
> - `MVP-AGENCY-007` → `feature/Dev-AGENCY-007_agency-publishing`
> - `SHAREDLIB-010` → `feature/Dev-SHAREDLIB-010_entitygraph-package`

---

## Step 4 — Update Status on `main` (MANDATORY, with verification) — DO THIS BEFORE THE BRANCH

**This step is mandatory on every invocation** and it must happen **before the feature branch is created**. The status flip is a *shared coordination signal* — it has to land on `main` in both repos so the next run of `/dev-start-prioritized` (in any clone) sees it. Do not skip it, defer it, batch it with later commits, or stage it on the feature branch.

⚠️ **Both flips go on `main`** — not on the feature branch. The feature branch doesn't exist yet at this point, and that is intentional.

### 4a — Flip the status in both files (on each repo's `main`)

**CodeValdCross `prioritization.md`** (always lives in CodeValdCross regardless of which service you are working in):

```bash
cd /workspaces/CodeVald-AIProject/CodeValdCross
git checkout main
git pull --rebase origin main
```

- Use the `Edit` tool to change the selected task's Status to `🚀 In Progress`. **The literal you pass as `old_string` must match the row exactly:**
  - Feature/MVP rows use `📋 Not Started` → flip to `🚀 In Progress`.
  - Bug rows use `📋 Open` → flip to `🚀 In Progress`.
  - Use whichever sentinel you noted in Step 2.5 — they are not interchangeable, and if the literal doesn't match the row the Edit silently does nothing.
- Edit the row in the Active Task Table **and** any per-section detail table that lists the same task.

**Target service's `mvp.md` (features) or `bugs.md` (bugs)** — also on `main`, **not** on a feature branch:

```bash
cd {REPO_PATH}
git checkout main
git pull --rebase origin main
```

- Change the same task's Status to `🚀 In Progress` in every place it appears, using the same literal-match rule.

### 4b — Verify the flip landed (do not skip)

After the edits, **re-read both files** and `grep` for the Task ID. The status next to it must now read `🚀 In Progress` everywhere — not `📋 Not Started`, not `📋 Open`, not missing.

```bash
TASK_ID="{TASK-ID}"
PRIORITY=/workspaces/CodeVald-AIProject/CodeValdCross/documentation/3-SofwareDevelopment/prioritization.md
SERVICE_TRACKER={REPO_PATH}/documentation/3-SofwareDevelopment/mvp.md   # or bugs.md for bug rows

# Each command MUST return zero matches:
grep -n "$TASK_ID.*📋 Not Started" "$PRIORITY"   "$SERVICE_TRACKER"
grep -n "$TASK_ID.*📋 Open"        "$PRIORITY"   "$SERVICE_TRACKER"

# And this MUST return at least one match in each file:
grep -n "$TASK_ID.*🚀 In Progress" "$PRIORITY"   "$SERVICE_TRACKER"
```

If any `📋 Not Started` or `📋 Open` line for this Task ID remains, you forgot to edit that line — fix it now. If the `grep` returns no matches in the service tracker, double-check whether the row lives in `mvp.md` (features) or `bugs.md` (bugs), and that the Task ID format matches what that service uses (some use `MVP-XXX-NNN`, some use `XXX-NNN`).

### 4c — Commit AND PUSH both flips on `main` immediately (MANDATORY)

A local commit is invisible to teammates, to other Claude sessions running in other clones, and to CI. **Both pushes are non-negotiable** — do them now, before creating the feature branch.

**CodeValdCross `main`:**

```bash
cd /workspaces/CodeVald-AIProject/CodeValdCross
git add documentation/3-SofwareDevelopment/prioritization.md
git commit -m "status: mark {TASK-ID} in progress"
git push origin main                                          # MANDATORY
```

**Target service `main`** (only if its `mvp.md` / `bugs.md` was edited):

```bash
cd {REPO_PATH}
git add documentation/3-SofwareDevelopment/mvp.md             # or bugs.md
git commit -m "status: mark {TASK-ID} in progress"
git push origin main                                          # MANDATORY
```

If either push is rejected, rebase on top of `origin/main` and push again — do not skip, do not stash to a feature branch.

### 4d — Tell the user what you flipped AND pushed

Briefly report: "Marked `{TASK-ID}` In Progress on `main` in both repos: CodeValdCross prioritization.md (line N, `<sha1>`) and {service}/{mvp,bugs}.md (line M, `<sha2>`). Both pushed to `origin/main`." Including the shas **and** the push confirmation makes the change verifiable by anyone else looking at the remote.

> Why this is enforced so strictly: silently leaving a task at `📋 Not Started` / `📋 Open` while working on it — or flipping it locally without committing, or committing without pushing, or stashing the flip on a feature branch where it won't be visible until merge — causes the next `/dev-start-prioritized` invocation (in this clone OR a teammate's clone) to either pick the same task again or pile a second branch onto the same repo. The status table is a *shared coordination signal*; if it only lives in a local commit or on an unmerged feature branch, it isn't a signal at all.

---

## Step 5 — Create the Feature Branch (AFTER status is on `main`)

Only now, with both status flips committed and pushed on `main`, create the feature branch in the **target repo**:

```bash
cd {REPO_PATH}
git checkout main
git pull origin main                                          # picks up the status flip you just pushed
git checkout -b feature/Dev-{PREFIX}-{NNN}_{short-description}
```

- Description: lowercase, words separated by hyphens.
- **Never commit directly to `main` from this point on** — `main` was only touched for the status flip in Step 4.
- The branch must exist before any code file is touched.

---

## Step 6 — Read Service Context

Inside the **target repo**, read these files before writing any code:

1. `.github/instructions/rules.instructions.md` — interface-first design, error types, file size limits, naming, concurrency rules.
2. `documentation/2-SoftwareDesignAndArchitecture/architecture.md` — interface contracts, data models, storage schema, gRPC definitions.
3. `documentation/3-SofwareDevelopment/mvp.md` — confirm every task listed in **Depends On** is ✅ complete.

**Always treat CodeValdAgency as the canonical reference implementation.**
Read `/workspaces/CodeVald-AIProject/CodeValdAgency/.github/instructions/rules.instructions.md`
and mirror its patterns (file layout, injection style, heartbeat registrar, error mapping, etc.)
when scaffolding or implementing any Go gRPC service.

**SharedLib extraction rule (applies throughout the entire task):**
> Whenever you encounter infrastructure code that is — or could soon be — used by more than one
> service (e.g. registration helpers, ArangoDB bootstrap, gRPC server utilities, shared types),
> **stop and flag it explicitly**: describe what the candidate is, which services would benefit,
> and ask the user how to proceed before continuing.
> Never silently copy code across services; instead surface the opportunity for SharedLib extraction.

**For CodeValdAI tasks specifically**, also read:
- `/workspaces/CodeVald-AIProject/CodeValdAI/documentation/` — full architecture split files
- `/workspaces/CodeVald-AIProject/CodeValdAI/documentation/3-SofwareDevelopment/mvp.md` — 16-task breakdown with acceptance tests and implementation walkthroughs
- `/workspaces/CodeVald-AIProject/CodeValdAI/documentation/3-SofwareDevelopment/mvp-details/` — per-task specs (scaffolding, llm-client, agent-management, run-intake, run-execution)

---

## Step 7 — Pre-Implementation Checklist

- [ ] `prioritization.md` swept — Pass A: all `✅ Done` / `✅ Fixed` rows removed
- [ ] `prioritization.md` swept — Pass B: every remaining row cross-checked against its service's `mvp_done.md` (or `bugs_done.md` for bug rows); stale rows removed
- [ ] Task selected from `prioritization.md` is `📋 Not Started` (feature) or `📋 Open` (bug); never an already `🚀 In Progress` row; all `Depends On` items are ✅ complete
- [ ] Target repo has **no other `🚀 In Progress` task** in `prioritization.md` (one in-progress task per repo, max)
- [ ] Target repo identified from the Task ID prefix lookup table
- [ ] `prioritization.md` updated to `🚀 In Progress` **on CodeValdCross `main`** (matching `📋 Not Started` OR `📋 Open` per row type)
- [ ] Service `mvp.md` (feature) or `bugs.md` (bug) updated to `🚀 In Progress` **on the target service's `main`** (NOT on a feature branch)
- [ ] **Step 4b verification done** — `grep`'d both files and confirmed the status now reads `🚀 In Progress` (no remaining `📋 Not Started` *or* `📋 Open` for this Task ID)
- [ ] **Step 4c commit AND push done in BOTH repos** — both flips are committed to their respective `main` branches AND pushed to `origin/main` (verify with `git log -1 origin/main` in each repo showing the `status: mark {TASK-ID} in progress` commit; a local-only commit or a feature-branch commit is insufficient)
- [ ] Feature branch created in the **target repo** from the now-updated `main`: `feature/Dev-{PREFIX}-{NNN}_{description}` (Step 5 — AFTER the status flips are pushed)
- [ ] `rules.instructions.md` and `architecture.md` read for the target service
- [ ] Checked `models.go` / `errors.go` / `types.go` — no duplicate types
- [ ] Todo list created with actionable implementation steps

---

## Step 8 — Implement

- Use the `manage_todo_list` tool to track each step.
- Mark items in-progress and completed as you go.
- Commit regularly: `git add . && git commit -m "{TASK-ID}: Descriptive message"`
- Keep files under 500 lines; functions under 50 lines.
- Every exported symbol gets a godoc comment; every exported method takes `context.Context` first.

---

## Finish — Validate, Merge, and Close the Branch

Run all checks **in the target repo** before merging.

### Go services (all except CodeValdHi)

```bash
cd {REPO_PATH}

go build ./...           # must succeed
go vet ./...             # must show 0 issues
go test -v -race ./...   # must pass
golangci-lint run ./...  # must pass

# If proto files changed:
buf lint
buf generate && git diff --exit-code gen/
```

### Flutter — CodeValdHi only

```bash
cd /workspaces/CodeVald-AIProject/CodeValdHi

flutter analyze                                        # must show 0 issues
flutter test                                           # must pass
dart format --set-exit-if-changed lib/ test/           # must show 0 changes
```

### Document the feature — `/dev-document-feature` (MANDATORY, before merge)

Once validation has passed and **before** merging the feature branch back into `main`, run the `/dev-document-feature` skill from the **target repo** to capture the architectural/design footprint of the work just completed.

```bash
cd {REPO_PATH}
# still on feature/Dev-{PREFIX}-{NNN}_{description}
```

Then invoke `/dev-document-feature` (no arguments → document every distinct design-level feature surfaced this session; or pass a short slug if only a subset of the conversation's features belongs to this task).

Why this lands here:
- The feature branch still holds the in-progress design context (new types, flows, interfaces) — easiest to document while it's fresh.
- The skill's contradiction sweep against existing `architecture*.md` files should run **before** the merge so any "doc says X, code does Y" findings get resolved on the feature branch alongside the code, not as a follow-up PR.
- Any doc edits the skill writes must be committed to the **feature branch** (`git add documentation/2-SoftwareDesignAndArchitecture/ && git commit -m "{TASK-ID}: document <feature>"`) so they merge into `main` together with the implementation.

Only after `/dev-document-feature` has completed (including the contradiction resolutions it asks about) and its doc changes are committed to the feature branch may you proceed to the Merge step below. If the skill reports zero new feature material to document (rare — usually a pure refactor / status-only change), state that explicitly to the user and continue.

### Merge and delete — ALWAYS, in the target repo (MANDATORY)

**This step is non-negotiable.** Once validation passes, you MUST merge the
feature branch into `main` and delete it before the run is considered
complete. Do not stop at "committed to the feature branch" and ask whether
to proceed — finish the cycle.

```bash
cd {REPO_PATH}
git checkout main
git pull origin main                                          # rebase-safe sync
git merge feature/Dev-{PREFIX}-{NNN}_{description} --no-ff
git branch -d feature/Dev-{PREFIX}-{NNN}_{description}
```

If `git branch -d` rejects the delete because the branch is "not fully
merged," investigate **before** force-deleting — that error usually
means the merge above did not actually happen (e.g. fast-forward refused,
working tree dirty). Re-run the merge, then re-try the delete; only fall
back to `git branch -D` once you've confirmed the commits made it to
`main`.

After the merge + delete, briefly report:
"Merged `feature/Dev-{PREFIX}-{NNN}_{description}` into `main` in `{REPO_PATH}` and deleted the branch."

> ⚠️ Do **not** merge in CodeValdCross unless the task itself belongs to CodeValdCross.
> Each service owns its own `main` branch and its own feature branches.

> Why this is enforced: leaving a feature branch open after the task is
> "done" defeats the one-in-progress-per-repo rule (Step 2 builds the
> busy-repo set from any `🚀 In Progress` row — but it can't see an open
> feature branch). The next `/dev-start-prioritized` invocation will
> happily pile a second branch on top, producing merge conflicts and a
> tangled history. Always close the loop.

### Update documentation after merge

1. **`prioritization.md`** (CodeValdCross) — **remove the completed task's row entirely**.
   - Do **not** strike it through and leave it in place.
   - Do **not** move it to a "Landed:" / "Recently merged" list inside `prioritization.md`. That history belongs in the per-service `mvp_done.md` / `bugs_done.md`, not in the master file. (Step 1 Pass C exists to clean these up — don't reintroduce them.)
   - If the row was the last active item under a section heading (`## QA 03 bugs (incoming ...)`, `## Pipeline failure handling (incoming ...)`, etc.) **also** delete that section's heading + source narrative + empty table skeleton. Do not leave an orphan header behind.
2. **Service `mvp.md` (features) / `bugs.md` (bugs)** — change the task status to `✅ Done` / `✅ Fixed`.
3. **Service `mvp_done.md` (features) / `bugs_done.md` (bugs)** — add a row for the completed task with today's date, commit SHA, and a one-line summary. This is the authoritative historical record — write it here, not in `prioritization.md`.
4. If new architecture artefacts were introduced (topics, gRPC methods, flows, ArangoDB collections),
   update `documentation/2-SoftwareDesignAndArchitecture/architecture.md` in the target service.

### Commit AND PUSH the documentation updates (MANDATORY)

Same rule as Step 5c: a local commit is invisible to teammates. Push both repos so the shared status table reflects reality on the remote:

```bash
# CodeValdCross — the row removal
cd /workspaces/CodeVald-AIProject/CodeValdCross
git checkout main
git pull --rebase origin main
git add documentation/3-SofwareDevelopment/prioritization.md
git commit -m "status: remove completed {TASK-ID} from prioritization.md"
git push origin main                                          # MANDATORY

# Target service — the mvp.md / mvp_done.md updates (on its own main, post-merge)
cd {REPO_PATH}
git push origin main                                          # MANDATORY
```

Briefly confirm to the user that both pushes succeeded ("Pushed `<sha>` to CodeValdCross origin/main and `<sha>` to {service} origin/main").

### Post-Finish Sweep (MANDATORY)

After step 3 above (the task's own row + `mvp_done.md` / `bugs_done.md` entry are in place), **re-run the full Step 1 sweep** against `prioritization.md`. All five passes:

- **Pass A** — drop any row whose Status is `✅ Done` / `✅ Fixed`.
- **Pass B** — for every remaining row, `grep` its Task ID in the target service's `mvp_done.md` / `bugs_done.md`; if found, drop the row and strike through any `Depends On` references to it.
- **Pass C** — delete any `Landed:` / historical prose blocks (every item must be present in the proper done-file first).
- **Pass D** — collapse orphan section headers + source narrative once their tables are empty (or trim multi-paragraph design narrative when only a single still-active row remains).
- **Pass E** — drop per-task detail tables that duplicate the active row, plus per-service workflow boilerplate.

Then **verify**: re-read `prioritization.md` and `wc -l` it. Confirm
- the just-completed Task ID no longer appears,
- no row in the active table has a Task ID that exists in its service's done-file,
- no `Landed:` blocks remain, and
- the line count is in the healthy range (typically under ~80 lines).

Briefly report to the user: "Post-finish sweep removed N stale rows + M historical blocks; file now {L} lines." (or "no additional stale items").

> Why this matters at *both* ends: the start-of-run sweep catches stale items from past sessions; the post-finish sweep catches the row you just completed plus any siblings that happened to be merged in the meantime, and prevents the file from re-growing between runs. Sweeping only at the start means the table is always one cycle behind reality.
