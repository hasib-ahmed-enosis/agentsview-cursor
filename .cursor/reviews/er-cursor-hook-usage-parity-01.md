## Verdict
**Status**: needs_refinement
**Iteration**: 1 of 5
**Summary**: Root-cause and Usage-dashboard fix are sound, but session ID shape, filter-gate SQL, phasing, and dedup/backfill need explicit design before implementation.

## Rubric

| Criterion | Result | Evidence |
|-----------|--------|----------|
| C1 Correctness | partial | Two-pipeline diagnosis matches code; `SessionID()` in `row.go` returns raw UUID, not canonical `cursor:{uuid}` used in archive |
| C2 Completeness | partial | Missing write-lock integration for serve ingest, `cursorUsageRowsSQLForBounds` gate change, backfill for existing hook rows |
| C3 Alignment | partial | Matches `storage.md` (ALTER only) but scope mixes MVP (Usage) with large SQL union work without phases; §4 contradicts §3 on message `model` |
| C4 Quality | pass | Clear diagram, surfaces table, verification steps |
| C8 Conventions | partial | Should cite `docs/agents/background-work.md` + `docs/agents/storage.md` as pre-read gates |

## Required changes

1. [critical] Specify canonical AgentsView session id as `cursor:` + conversation UUID (fix or replace `UsageRow.SessionID()`); do not store bare UUID in `cursor_usage_events.session_id`.
2. [important] Phase work: **Phase 1** = JSONL watch + `cursorhook.Ingest` on serve (write lock) + `TestGetDailyUsageIncludesCursorHookEvents`; **Phase 2** = `session_id` column + SQL unions + filter-gate updates.
3. [important] Document change to `cursorUsageRowsSQLForBounds` / `usageCursorIncluded`: project (and related) filters must include hook rows when `session_id` joins `sessions`, not drop all cursor rows.
4. [important] Remove or defer §4 (message `model` + dedup guard); session cost/breakdown via session-linked `cursor_usage_events` union only to avoid double-count indecision.
5. [important] Backfill: one-time pass over JSONL (or ingest helper) to set `session_id` on existing `kind = hook` rows by `dedup_key` / generation fingerprint without re-inserting duplicates.
6. [minor] Note Windows append-only file watch behavior and reuse serve shutdown/lifecycle patterns from `cmd/agentsview/main.go`.
7. [minor] Avoid `cursorhook` → `parser` import cycle; share prefix via small helper or duplicate `cursor:` constant with test.

## Preserve

- Dual-pipeline diagram and “admin API parity” goal (`kind = hook`, estimated cost).
- Post-sync ingest as safety net.
- Postgres/DuckDB schema parity requirement.
- Usage page caveat for account-level admin rows with empty `session_id`.
- No frontend changes unless API shape changes.
- Out of scope: real Cursor-reported dollars without Admin API.
