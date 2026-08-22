# Steloit Cloud — Agent Guide

The Steloit developer cloud — **AI-native infrastructure**: it reads an application, recommends and
prices the infrastructure it needs, then runs it, so developers don't have to become infrastructure
experts (positioning owner: `docs/product/17-brand/messaging.md`;
`docs/plan/positioning-v2.md`). The console (built), the control plane, data plane, and CLI (being built).
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
| `docs/founder-config.md` | **Founder-owned decisions** (providers, P1 identifiers, pricing, secrets) — consume directly; interrupt only for a `NEEDS FOUNDER INPUT` row |
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
5a. **Review pipeline (MANDATORY for every significant PR — ADR-0008):** Implementation Agent →
   Architecture Reviewer → Security/QA Reviewer → CI → Merge. Invoke the two **by name** —
   `subagent_type: "reviewer"` and `subagent_type: "qa"` (`.claude/agents/README.md`); never a
   `kernel:*`/plugin agent, and never a generic runner fed their prompts — that workaround is
   retired, though older task Outcomes still show it. The two are independent and must not write
   (behavioral — they hold `Bash`; ADR-0008), and run on the branch diff *before* merge; blocking
   findings are fixed and re-verified, non-blocking ones recorded. Only pure typo/comment/doc edits
   are exempt.
6. PR title `<id>: <title>`. Flip `status: done` and append the task's `## Outcome` (5–10 lines) in the same PR.
7. Spec conflicts you discover are **findings**: record them in the PR and file a follow-up task — never resolve silently.
8. Lessons that should outlive the task go to a *living file* — the domain pack's mistake bank, the nearest
   AGENTS.md, or an ADR. A lesson that didn't change a living file didn't happen.
   **Examples are normative** (founder, 2026-07-27): a canonical example is held to at least the
   evidentiary standard of the rule it teaches. If the example contradicts the principle, the
   EXAMPLE is wrong even when the rule is right — O11 shipped a mutation-class rule whose own
   example named the same representation on both sides, erasing the distinction it existed to
   introduce. And cite only what the committed history supports: three O11 entries narrated
   incidents that did not happen as described, one of them a counterfactual from the author's
   own code comment. A mistake bank entry without a verifiable incident is a style guide.

## Hard rules

- Never invent a component, endpoint, or term — propose the owner-level change first (agent-guide §3 binds here).
- **Platform architecture is FROZEN** (`docs/architecture.md` v1.2, ADR-0001/0003/0004/0005): Go/stdlib-http/
  sqlc/River/REST+SSE, CNPG+ZFS substrate, the **product surface `[postgres, valkey, web, worker]`**, and
  BYOC (Enterprise/v3, exit-criteria-gated — ADR-0005) are decided. Storage & AI are external **Bindings**;
  queue is a **Postgres capability** (pgmq); GPU is out. The enum names the *architecture* plane only —
  the **catalog is outcome-first** (ADR-039/040): it sells intents the Composer resolves to named
  resolutions with stated semantics; **execution models are replaceable, semantic contracts are not.**
  The Engineering OS (this file, tasks/, contexts/, spec-sync) is frozen too.
- **New-managed-product gate — the Developer-First test, then the State Test (ADR-038/040):** first:
  what developer problem does it solve, which execution model gives the best DX, does it strengthen
  the certainty story? Then classification is decided by *state*,
  because lifecycle, backup, branching, scaling, permissions, billing, and ownership all follow state:
  **Service** iff independent state lifecycle + isolation boundary · **Capability** iff its state is *of*
  a parent (branches/joins/transacts with it) · **Binding** iff state lives outside the platform. A
  proposed managed product must first fail Binding, capability, and integration — only then, via a new
  ADR. Dependencies are satisfied by the composer (catalog sells intents), never surfaced as homework.
  Architecture deltas require evidence from implementation or real customer feedback, never speculation;
  a cleaner abstraction is never a trigger (ADR-040).
- **Review order (ADR-040):** developer experience first, implementation simplicity second, architectural
  elegance third. Reviews default to implementation thinking — every proposal answers: does it improve
  DX? does it reduce customer uncertainty? is there implementation or customer evidence for an
  architecture change? If not, the architecture stands.
- Money is integer cents end-to-end (ADR-025). Service status vocabulary is `ready`/`deleting` (ADR-024).
  Errors are problem+json and always carry `remediation`.
- The AI four laws bind generated code: **no auto-apply path exists in the API, ever.**
- Estimate-before-provision is enforced at the API layer, never only in a client.
- `docs/product/00-sources/**` and `docs/product/18-philosophy/decisions.md` change by **human decision
  only**. A hook stops common accidental writes and CI's `authority-paths` flags any PR that touches
  them — but **nothing blocks a merge** (this repo has no branch protection: O6g), so the real control
  is still you and the diff. Engineering decisions get an ADR in `docs/adr/`.
- **Fault injection happens in a throwaway copy, never the working tree** — this binds the
  IMPLEMENTER as much as the reviewers. A mutate-then-restore leaves no diff, so an
  interrupted run is invisible to review; and a reviewer reading the tree mid-mutation
  gets a false baseline. In US-3.7 this corrupted an independent review sweep twice (a
  mutated `engine.go` and a mid-edit test file were copied into the reviewer's sandbox,
  producing a red baseline it had to chase down before it could trust any result).
  `cp -R` the module, mutate the copy, delete it. If you nonetheless mutate the tree,
  **say so plainly in the PR** — ADR-0008 makes disclosure the operative obligation,
  because a mutate-then-restore is otherwise invisible to everyone downstream.
- **A worktree is not a commit, and `git push` only pushes commits.** O16 (#310) merged
  the money primitive but NONE of its two review rounds: the fixes were made in a
  `git worktree`, never committed, `git push` sent only the pre-existing commit, the PR
  merged that, and `git worktree remove --force` then destroyed the rest. The PR
  description claimed all of it. **Commit after each review round, before pushing, and
  check `git status` before removing any worktree** — `--force` means it. The tell is a
  push that reports an unchanged range, or `--force` on a remove that prints nothing.
- Never mix restructuring and feature work in one commit.

## Don't read (token discipline)

- `docs/product/99-history/` — superseded; never authority.
- The frame gallery HTML end-to-end — search your frame id instead.
- `docs/product/19-canon/fixtures.json` raw — import via `packages/canon`.
- `tasks/_archive/` — only when investigating a past decision in that exact area.
