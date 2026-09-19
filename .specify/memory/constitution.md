<!--
Sync Impact Report
- Version change: [TEMPLATE] → 1.0.0 (initial ratification)
- Modified principles: n/a (first concrete adoption; all five slots filled from
  scratch, derived from AGENTS.md, README.md, and SECURITY.md)
- Added sections: Core Principles (I-V), Content & Publishing Standards,
  Development Workflow, Governance
- Removed sections: none (template placeholders replaced, none retained)
- Templates requiring updates:
  - ✅ .specify/templates/plan-template.md (Constitution Check gate is
    generic and reads from this file; no hardcoded references to fix)
  - ✅ .specify/templates/spec-template.md (no constitution-specific
    references found)
  - ✅ .specify/templates/tasks-template.md (no constitution-specific
    references found)
  - ✅ .specify/templates/checklist-template.md (no constitution-specific
    references found)
- Follow-up TODOs: none
-->

# AgentsView Constitution

## Core Principles

### I. Local-First Data Privacy

Session data stays on the user's machine unless the user explicitly chooses a
feature that shares it. The SQLite archive is the persistent source of truth
and MUST NOT be deleted, dropped, truncated, or recreated to handle
data-version or parser changes; schema migrations must evolve the existing
archive in place. Optional mirrors (PostgreSQL, DuckDB) and any remote sync
are opt-in only and never a substitute for the local archive.

**Rationale**: AgentsView's value proposition is that coding-agent session
history stays private by default. Silent data loss or unintended egress would
break that trust irreversibly.

### II. Non-Destructive Operations

Agents and contributors MUST NOT install over a live binary, run migrations
against production, or write to live data directories without explicit
permission. Branch builds, profiling, and experiments use isolated scratch
data. Destructive git operations (force-push, hard reset, branch deletion,
history rewrite) and destructive shell operations require explicit user
request; do not run them to route around a failing check.

**Rationale**: This project manages irreplaceable local session history and
runs as a background daemon; an accidental overwrite or destructive command
can silently corrupt a user's only copy of their data.

### III. Documentation Written for the Reader

Documentation (README, docs/, changelogs, PR descriptions) MUST be written for
the person trying to use or maintain AgentsView, not for the author. Lead with
the outcome, name who does what, use short sentences, and explain unfamiliar
terms. State rules directly (exact commands, field names, authorization
checks, limits, and failure behavior) rather than paraphrasing them away.
Distinguish current, shipped behavior from proposed or unbuilt work.

**Rationale**: AgentsView is distributed to end users and operators who are
not reading the source; unclear or stale docs directly cause support burden
and mistrust in a tool that already asks for trust with local data.

### IV. Disciplined Git and Delivery

Every turn that changes tracked files MUST be committed with a focused,
conventional commit message; do not create empty commits. Do not amend,
squash, rebase, push, or pull unless the user asks. Branch creation and
switching require the user's permission. Changes are delivered through pull
requests from feature branches; merging a pull request is always the user's
decision, never the agent's.

**Rationale**: Predictable, reviewable history lets the maintainer audit
agent-driven changes and keeps automated and human contributions
distinguishable and reversible.

### V. Verified Build Quality

After any Go code change, run `go fmt ./...` and `go vet ./...` before
committing. Prefer the standard library over adding new dependencies. Observable
behavior MUST be preserved across refactors unless a change is the explicit
goal, and documentation MUST be updated whenever behavior changes. Read the
focused guide matching the task (testing, storage, background-work, build,
S3 providers, frontend) before editing the files it covers.

**Rationale**: AgentsView ships a CLI, daemon, and web UI relied on for cost
and usage accuracy; unformatted or unvetted code and undocumented behavior
changes erode both correctness and trust.

## Content & Publishing Standards

Private project names, hostnames, personal identities, infrastructure
details, and absolute user paths MUST be kept out of code, tests, fixtures,
documentation, commit messages, and pull request text; run the private-data
scrub before publishing anything externally. Changelog entries MUST be
written in plain language, leading with the outcome a user or operator will
notice before implementation detail. Release notes MUST be verified against
release tags and source, and contributor credit MUST come from merged pull
requests or commit history, not inferred from names or issue participation.

## Development Workflow

Reviews, analyses, or explanations are read-only unless the user also asks
for changes. Contributors MUST read every focused guide whose task route
matches the work (see `AGENTS.md`'s Task Routes table) before editing the
matching files, and more specific instructions override broader ones. Roborev
commands and skills that trigger a review or fix MUST NOT run unless the user
asks for them. Pull requests carry summary-only descriptions — no test
plans, checklists, or command transcripts — explaining what changed, why, the
tradeoffs, limits, and where reviewers should look.

## Governance

This constitution captures the durable, project-wide rules that govern how
work is done in this repository; `AGENTS.md` (kept in sync with `CLAUDE.md`
as a symlink) and the focused guides under `docs/agents/` and
`frontend/AGENTS.md` provide the operational detail that implements these
principles day to day. Where the two conflict, this constitution takes
precedence; a discovered conflict should be resolved by amending
`AGENTS.md`/the focused guides to match, or by amending this constitution if
the principle itself needs to change.

Amendments are made by editing this file, incrementing `CONSTITUTION_VERSION`
per semantic versioning (MAJOR for backward-incompatible principle removals
or redefinitions, MINOR for new or materially expanded principles/sections,
PATCH for clarifications and wording fixes), updating `Last Amended`, and
recording the change in a Sync Impact Report comment at the top of this file.
Every pull request and review MUST verify compliance with these principles;
unavoidable complexity or deviation must be justified in the change itself
(e.g., in the plan's Complexity Tracking section) rather than left implicit.

**Version**: 1.0.0 | **Ratified**: 2026-09-19 | **Last Amended**: 2026-09-19
