# CLAUDE.md

**Canonical agent instructions live in `22-agents/agent-guide.md` — read it first.** This file is the Claude Code-specific pointer (one room, many doors); it never duplicates the guide, only routes into it.

## The three sentences that prevent the worst mistakes

1. This repo is a **design & spec handoff package**, not source code — "build X" means generating application code *from* these specs; there is no build/lint/test here.
2. **Authority order:** `00-sources/` (GOV-002 + INF-001 → 152-frame gallery → design spec) > derived docs (01–22, one owner per concern) ; `18-philosophy/decisions.md` before proposing structural changes ; `99-history/` never.
3. Resolve any screen question by **searching its frame id** (`W3`, `U6`, `AI3`…) in `00-sources/`; copy microcopy **verbatim**; generate API types from `08-api/openapi.yaml`; demo data from `19-canon/` only.

## Claude Code specifics

- Reading order for a task: agent-guide §2 (orient → ADR check → build from owners → verify against `16-qa` → report with citations).
- The hard rules (agent-guide §3) and stop-and-ask triggers (§4) are binding; conflicts between authorities are findings to surface, never to resolve silently.
- Commit only when explicitly asked.


## Knowledge base

Project context lives in my vault: `~/atlas/projects/steloit/`

- Before architectural work or when missing context:
  read `~/atlas/hot.md`, then `~/atlas/projects/steloit/INDEX.md`,
  then drill into linked notes as needed.
- Coding conventions: `~/atlas/standards/`
- When I correct the same kind of output a second time — across sessions or
  repos — offer to record it as a `~/atlas/standards/` entry, citing both
  incidents. Offer-based.
- Do NOT read the vault for routine tasks that don't need it.
- If we make an architectural decision here, offer to record an ADR in
  `~/atlas/projects/steloit/decisions/`.
- At session end, if material project context changed, offer to update the
  vault's INDEX.md and log.md for this project (or run `/atlas-sync`).
