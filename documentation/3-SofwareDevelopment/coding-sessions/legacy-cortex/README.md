# Legacy CodeValdCortex Session Logs

The files in this folder were authored against **CodeValdCortex** in 2025 —
before the CodeValdAgency service existed as a separate repository. They were
copy-pasted into this repo (originally under `coding-sessions.md` and
`updates/`) and are preserved here verbatim so the historical record is
intact, but **none of the content describes the current CodeValdAgency code**:

- branches use the `feature/MVP-001` / `feature/MVP-005` / `feature/MVP-015` series, not `feature/AGENCY-*`
- env vars are prefixed `CVXC_*` (Cortex), not `CODEVALDAGENCY_*` / `AGENCY_*`
- file paths reference `/workspaces/CodeValdCortex/...` and `documents/...` (this repo uses `documentation/`)
- struct fields, collection names, and feature descriptions don't match this service
- features described (HTMX dashboard, agent-to-agent message bus, RACI matrix UI, "Agency Designer") live elsewhere

Treat these as archive material. New sessions go in the parent
[`coding-sessions/`](../) folder, dated and tagged with the relevant
`MVP-AGENCY-*` task.

| Date | Topic | File |
|---|---|---|
| 2025-10-20 | MVP-001 — infrastructure setup | [2025-10-20-mvp-001-infrastructure.md](2025-10-20-mvp-001-infrastructure.md) |
| 2025-10-20 | MVP-005 — agent communication design | [2025-10-20-mvp-005-communication-design.md](2025-10-20-mvp-005-communication-design.md) |
| 2025-10-22 | MVP-015 — management dashboard | [2025-10-22-mvp-015-management-dashboard.md](2025-10-22-mvp-015-management-dashboard.md) |
| 2025 | Agency Operations Framework summary (RACI / Goals / WorkItems UI design) | [2025-update-agency-operations-framework.md](2025-update-agency-operations-framework.md) |
