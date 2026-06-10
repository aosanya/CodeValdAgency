# FEAT-20260610-004 — Upstream `dev-agency-flowchart` skill into `CodeValdAgency/.claude/commands/`

**Status:** 📋 Not Started
**Severity:** Low — the skill currently works correctly from `/home/vscode/.claude/commands/`; this is a reproducibility / discoverability fix, not a functional gap.
**Owner:** CodeValdAgency
**Estimated effort:** ~30 minutes (file move + .gitignore carve-out + verification across services)
**Source finding:** 2026-06-10 session — while completing FEAT-20260610-003 (Phase 3 flowchart regen), `dev-agency-flowchart` had to be rewritten for the FEAT-20260609-002 per-workflow file layout. The rewrite lives only in `/home/vscode/.claude/commands/dev-agency-flowchart.md` — tied to one workspace, invisible to anyone cloning the repo fresh.

---

## Problem

Two patterns coexist for slash commands today:

| Location | Tracked in git? | Examples |
|---|---|---|
| `CodeValdAgency/.claude/commands/` | **No** — `.gitignore` line 4 excludes `.claude/*` and line 5 excludes `.claude` | `dev-reimport-agency.md`, `dev-start-prioritized.md` |
| `/home/vscode/.claude/commands/` (user home) | n/a — outside any repo | `dev-agency-flowchart.md`, `dev-agency-research.md`, `dev-check-arch.md`, `dev-document-feature.md`, `dev-research.md`, `dev-rollback-workflow.md` |

Neither pattern actually puts the skill source under version control. Past `mvp_done.md` entries (e.g. FEAT-20260610-001) cite branch names like `feature/Dev-AGENCY-FEAT20260610001_dev-reimport-agency-bundling` for slash-command work, but no such branch exists in the agency repo history — the work was done, the file was edited locally, and nothing was ever committed.

Consequences:

1. **No reproducibility** — a teammate cloning `CodeValdAgency` does not get the skills. `git log` cannot show who changed what, when, or why.
2. **No review** — slash-command edits never go through a feature branch, so no code review happens.
3. **No rollback** — a regression in `dev-agency-flowchart` is not revertable via `git revert`; the only fallback is rewriting from memory.
4. **Drift** — two workspaces editing the same skill silently diverge.

The 2026-06-10 rewrite of `dev-agency-flowchart` for the per-workflow file layout makes the cost concrete: a non-trivial logic change (subgraphs per workflow, dashed `-.->` edges for `on-error.emits_topics`, publisher-based node colouring) lives in one developer's home directory with no audit trail.

## Evidence

```text
$ grep -n claude /workspaces/CodeVald-AIProject/CodeValdAgency/.gitignore
4:.claude/*
5:.claude

$ ls /workspaces/CodeVald-AIProject/CodeValdAgency/.claude/commands/
dev-reimport-agency.md
dev-start-prioritized.md

$ git -C /workspaces/CodeVald-AIProject/CodeValdAgency ls-files .claude/
(empty)

$ ls /home/vscode/.claude/commands/dev-agency-*
/home/vscode/.claude/commands/dev-agency-flowchart.md
/home/vscode/.claude/commands/dev-agency-research.md
```

## Root cause

`.gitignore` was set to a blanket `.claude/*` + `.claude` when slash-command authoring was new and treated as throw-away. The pattern stuck even as commands grew non-trivial and codified important workflows.

## Fix plan

### Phase 1 — Allow tracked slash commands under `.claude/commands/`

In `CodeValdAgency/.gitignore`, replace the blanket exclude with a carve-out that keeps user-specific settings local while allowing slash commands to be tracked:

```diff
- .claude/*
- .claude
+ # Keep slash commands under version control, but never commit user settings.
+ .claude/*
+ !.claude/commands/
+ !.claude/commands/*.md
```

Apply the same pattern to `CodeValdCross/.gitignore` (lines 15–16) and any other service repo that has the same blanket rule.

`settings.json` / `settings.local.json` remain ignored (they contain per-user permissions, sandbox state, and env vars that must not be shared).

### Phase 2 — Move `dev-agency-flowchart` into the repo

```bash
cp /home/vscode/.claude/commands/dev-agency-flowchart.md \
   /workspaces/CodeVald-AIProject/CodeValdAgency/.claude/commands/dev-agency-flowchart.md
rm /home/vscode/.claude/commands/dev-agency-flowchart.md
```

The Claude Code skill loader picks up `.claude/commands/*.md` files from the **project** before falling back to `~/.claude/commands/`, so the repo copy takes precedence automatically. After Phase 1 the file shows up under `git status` and can be committed.

Verify the skill still resolves and runs by invoking it: `/dev-agency-flowchart utility-app-builder` — the regenerated `flowchart.md` should be identical to the one committed in FEAT-20260610-003 (`CodeValdImplementations` main `58fac0b`).

### Phase 3 — Backfill `dev-reimport-agency`

`CodeValdAgency/.claude/commands/dev-reimport-agency.md` already exists on disk but is gitignored. After Phase 1 it will appear as untracked. `git add` it and commit so the FEAT-20260610-001 bundling logic finally has a real audit trail.

## Verification

- [ ] `.gitignore` carve-out lands in CodeValdAgency (and CodeValdCross if symmetry is preferred).
- [ ] `dev-agency-flowchart.md` is removed from `/home/vscode/.claude/commands/` and present + tracked under `CodeValdAgency/.claude/commands/`.
- [ ] `dev-reimport-agency.md` is committed under `CodeValdAgency/.claude/commands/`.
- [ ] `/dev-agency-flowchart utility-app-builder` regenerates `flowchart.md` byte-identical to the version on main.
- [ ] `/dev-reimport-agency` continues to bundle + post + verify utility-app-builder.

## Out of scope

- **Upstreaming `dev-agency-research`, `dev-check-arch`, `dev-document-feature`, `dev-research`, `dev-rollback-workflow`** — these may belong in different repos (CodeValdCross? CodeValdSharedLib?) and the ownership question deserves its own FEAT. Filed here only as a known follow-up.
- **A shared skills repo** (e.g. `CodeValdSkills`) for cross-service commands — bigger architectural question, not blocking this fix.
- **Linting / testing slash commands** (markdown structure, frontmatter validation) — out of scope; current skills are human-authored and reviewed in PR.

## Dependencies

- [FEAT-20260610-003](FEAT-20260610-003_utility_app_builder_flow_file_rename.md) — defines the skill rewrite that triggered this FEAT. ✅ Done.
