# FEAT-20260610-002 — Architecture doc refactor (split + relocate + numbering fix)

> **Architecture:** see [architecture.md](../../2-SoftwareDesignAndArchitecture/architecture.md) (index of all split files).

**Status:** ✅ Done
**Severity:** Low — documentation hygiene; unblocks adding per-workflow `event_flows` content without breaching the 400-line file cap.
**Owner:** CodeValdAgency
**Estimated effort:** ~1 hour (in-session)
**Source finding:** 2026-06-10 session — `/dev-document-feature` run for FEAT-20260609-002 backfill. `architecture-interfaces.md` (381 lines) and `architecture-flows.md` (354 lines) were both over the 300-line "split before adding" threshold; `architecture-flows.md` also had duplicate `## 6.` and `## 7.` section numbers.

---

## Problem

The architecture docs had three accumulated hygiene issues that blocked clean addition of FEAT-20260609-002 backfill content:

1. `architecture-interfaces.md` mixed two distinct concerns — Go interface definitions and value-type structs — in one 381-line file.
2. `architecture-flows.md` mixed lifecycle flows with gRPC service definitions and Cross-registration content, producing duplicate `## 6.` (ArchiveDraft Flow vs gRPC Service Definition) and `## 7.` (PublishAgency Flow vs Cross Registration) section numbers.
3. The `architecture.md` Split-docs index block referenced only 5 companion files; the new architecture-models.md needed to be added.

## Fix

1. **Extracted** `architecture-interfaces.md` § 5 Data Models (~180 lines) into a new [`architecture-models.md`](../../2-SoftwareDesignAndArchitecture/architecture-models.md).
2. **Relocated** `architecture-flows.md` §§ 6 gRPC Service Definition + 7 Cross Registration → `architecture-interfaces.md` §§ 5–6. This consolidates the RPC surface in one file and leaves `architecture-flows.md` focused purely on lifecycle.
3. **Renumbered** `architecture-flows.md` to clean sequential headings (§§ 1–9, no duplicates).
4. **Updated** the Split-docs index block in [`architecture.md`](../../2-SoftwareDesignAndArchitecture/architecture.md) to list all 6 companion files including architecture-models.md.

## Verification

- [x] All `architecture*.md` files under the 400-line cap (largest: `architecture-flows.md` 384).
- [x] No duplicate section numbers in any `architecture-*.md` file.
- [x] Index block in `architecture.md` lists all companion files correctly.
- [x] Cross-references (`[architecture-X.md](architecture-X.md)`) resolve.

## Dependencies

- None blocking.
- Enabled by: this refactor unblocked the FEAT-20260609-002 backfill content (added in the same session).
