# CLAUDE.md

**Canonical agent instructions live in `22-agents/agent-guide.md` — read it first.** This file is the Claude Code-specific pointer (one room, many doors); it never duplicates the guide, only routes into it.

## The three sentences that prevent the worst mistakes

1. This repo is a **design & spec handoff package**, not source code — "build X" means generating application code *from* these specs; there is no build/lint/test here.
2. **Authority order:** `00-sources/` (GOV-002 → 152-frame gallery → design spec) > derived docs (01–22, one owner per concern) ; `18-philosophy/decisions.md` before proposing structural changes ; `99-history/` never.
3. Resolve any screen question by **searching its frame id** (`W3`, `U6`, `AI3`…) in `00-sources/`; copy microcopy **verbatim**; generate API types from `08-api/openapi.yaml`; demo data from `19-canon/` only.

## Claude Code specifics

- Reading order for a task: agent-guide §2 (orient → ADR check → build from owners → verify against `16-qa` → report with citations).
- The hard rules (agent-guide §3) and stop-and-ask triggers (§4) are binding; conflicts between authorities are findings to surface, never to resolve silently.
- Commit only when explicitly asked.
