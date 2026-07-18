# Steloit Cloud — Agent Guide

The Steloit developer cloud: the console (built), the control plane, data plane, and CLI (being built).
**This repository is the single source of truth.** GitHub Issues and the Project board are *generated
views* — never edit them; edit the files here and run sync.

## Map

| Path | What |
|---|---|
| `apps/console/` | The console SPA (complete vs canon mocks) — its own AGENTS.md has the frontend rules |
| `services/` | Backend deployables (`api` = modular monolith, `cell-agent`) — born with their first tasks |
| `packages/` | `contracts` (generated from openapi.yaml) · `canon` (fixtures + invariants as test utils) |
| `docs/product/` | **The design authority** (the spec package; internal numbering 00–24 is load-bearing) |
| `docs/architecture.md` | **Architecture v1 (FROZEN, ADR-0001)** — stack, boundaries, toolchain; deltas need an ADR |
| `docs/adr/` | Engineering ADRs · `docs/plan/` — roadmap, workflow, migration record |
| `contexts/` | Context Packs — cross-cutting domain knowledge; load the ones your task lists |
| `tasks/` | Execution layer: one file per work item (YAML frontmatter + implementation closure) |
| `scripts/spec-sync/` | tasks → issues/board projection + validation |

## Commands

```sh
pnpm install
pnpm --filter console dev | build | test | typecheck | lint
node scripts/spec-sync/validate.mjs     # task frontmatter, deps, caps
node scripts/spec-sync/sync.mjs         # one-way repo → GitHub (idempotent; never syncs back)
```

## Authority order (conflicts are findings, never judgment calls)

Canonical ranking lives in `docs/product/22-agents/agent-guide.md` §1 — read it once. In short:
`00-sources/` (GOV-002 · INF-001 · 152-frame gallery · design spec) → derived docs (01–24, one
owner per concern) → everything else. Check `docs/product/18-philosophy/decisions.md` (ADR log)
before proposing anything structural. `docs/product/99-history/` is **never** authority.
Microcopy verbatim from frames · API types generated from `docs/product/08-api/openapi.yaml`,
never hand-written · demo data from `19-canon` only.

## Task protocol

1. **Ready work** = a `tasks/**.md` file with `status: ready` and all `deps:` done. Never start a stub.
2. **Claim** = push branch `task/<id>` — the branch *is* the lock. Set `status: in-progress` on it.
3. **Load** this file (automatic) → the task file → its `contexts:` packs → its Read-first list. Stop there.
4. Implement on the branch (worktree recommended). Touch only paths in the task's `files:` globs.
5. **Done** = every command in the task's `verify:` block passes. Put evidence (output) in the PR — assertions don't count.
6. PR title `<id>: <title>`. Flip `status: done` and append the task's `## Outcome` (5–10 lines) in the same PR.
7. Spec conflicts you discover are **findings**: record them in the PR and file a follow-up task — never resolve silently.
8. Lessons that should outlive the task go to a *living file* — the domain pack's mistake bank, the nearest
   AGENTS.md, or an ADR. A lesson that didn't change a living file didn't happen.

## Hard rules

- Never invent a component, endpoint, or term — propose the owner-level change first (agent-guide §3 binds here).
- **Architecture v1.2 is frozen** (`docs/architecture.md`, ADR-0001/0003/0004): Go/stdlib-http/sqlc/River/
  REST+SSE, CNPG+ZFS substrate, and the **product surface `[postgres, valkey, web, worker]`** are decided.
  Storage & AI are external **Bindings**; queue is a **Postgres capability** (pgmq); GPU is out. Don't
  introduce alternative frameworks/ORMs/protocols or new managed products — deltas require a new ADR with
  a measured trigger. The Engineering OS (this file, tasks/, contexts/, spec-sync) is likewise frozen.
- Money is integer cents end-to-end (ADR-025). Service status vocabulary is `ready`/`deleting` (ADR-024).
  Errors are problem+json and always carry `remediation`.
- The AI four laws bind generated code: **no auto-apply path exists in the API, ever.**
- Estimate-before-provision is enforced at the API layer, never only in a client.
- `docs/product/00-sources/**` and `docs/product/18-philosophy/decisions.md` change by **human decision
  only** (hook-enforced). Engineering decisions get an ADR in `docs/adr/`.
- Never mix restructuring and feature work in one commit.

## Don't read (token discipline)

- `docs/product/99-history/` — superseded; never authority.
- The frame gallery HTML end-to-end — search your frame id instead.
- `docs/product/19-canon/fixtures.json` raw — import via `packages/canon`.
- `tasks/_archive/` — only when investigating a past decision in that exact area.
