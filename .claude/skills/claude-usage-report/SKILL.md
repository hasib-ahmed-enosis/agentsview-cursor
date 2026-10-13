---
name: claude-usage-report
description: Use when the user asks how much Claude (Claude Code) they have used, in tokens, messages or estimated cost, between two date-times — e.g. "usage from 9/28 4pm to 10/5 4pm". Reports flat per-message usage straight from the local agentsview DB, with no parent/child rollup like the Session Explorer.
---

# Claude usage report

Queries `~/.agentsview/sessions.db` (read-only) by each message's own timestamp.
Claude agent only. Each API message is counted once (dedup by `claude_message_id`).
The Session Explorer can't answer this: its parents already include their
children's totals and its date filter is by session start day.

## Steps

1. Get the window(s) from the user: one or more `FROM → TO` pairs. Times are the
   user's **local time, UTC+6 by default** (DB stores UTC; the script converts).
   Resolve missing year to the current year; confirm ambiguous dates only if
   truly unclear (e.g. a typo like "120/12" → ask or state your reading).
2. Run from the repo root (pass pairs back to back; format `YYYY-MM-DD HH:MM`):

   ```bash
   python -I .claude/skills/claude-usage-report/usage_range.py --report \
     "2026-09-28 16:00" "2026-10-05 16:00" \
     "2026-10-05 16:00" "2026-10-12 16:00"
   ```

   - Add `--tz <hours>` only if the user says they're in another offset.
   - Omit `--report` to get a per-model table instead (handy for a single window).
3. Paste the script's markdown output **verbatim** as the answer — it is already in
   the user's required format (summary header, one "Week N" block per window,
   cost breakdown by model, footnote). Don't reword, re-order, or add a table.
   Add at most one line if something is notable (e.g. the window extends past the
   latest data).

## Notes

- Cost is an estimate from the DB's `model_pricing` table: 1-hour cache writes
  use the 1-hour rate; tiered bands are ignored (none exist for current Claude
  models). Token counts are the reliable numbers.
- Model names are prettified (`claude-sonnet-5-5` → Sonnet 5.5). Zero-token
  `<synthetic>` placeholder rows are excluded.
- Cursor / cursor-ide sessions are intentionally excluded.
- If the DB is locked or missing, say so; don't copy or modify it.
