---
id: canon-testing
owns: [packages/canon/**, docs/product/19-canon/**, docs/product/16-qa/**]
see: [api-conventions, billing]
---

# Canon & testing

`docs/product/19-canon/` is the ONE demo world (ADR-026): API-response-shaped fixtures,
arithmetic-verified. **No demo data exists outside canon.** Import fixtures and invariants via
`packages/canon` — never retype a number, never read fixtures.json raw into context.

## The arithmetic invariants (imported at every layer)

- `61+22+58+24+22+21 = 208` (ecommerce services → project; assets = external Storage Binding, outside totals — P4 ruling 2026-07-18)
- `199.10+6.70+2.20 = 208` (environments → project)
- `208+96+41+38+0 = 383` (projects → org resources) · `383+99 = 482` (org total, everywhere)
- Estimate line grammar == invoice line grammar; org total == Σ projects + plan fee on every surface.

Same assertions run in three places: estimate engine, invoice generator, console. Drift fails CI.

## The 10 canonical QA scenarios (16-qa/qa.md — the E2E backbone)

1 incident replay (#142→#143, p95 431ms) · 2 quota warning (87/100GB, ~$1.62, no upsell) ·
3 dunning timeline · 4 downgrade block (12 members & 2 cells → 409 both reasons) · 5 template
safety (zero secrets captured) · 6 token reveal-once · 7 AI disable (platform byte-identical) ·
8 invite lifecycle (7-day) · 9 env-as-filter · 10 typed-confirm delete (dependents + final backup).

Each lands as an automated scenario when its epic ships (mapping in docs/plan/implementation-plan.md §10).

## Techniques

- **Fault injection:** the console audit's fetch-wrapper (problem+json 4xx/5xx + latency via a
  flag) — every query-backed surface proves loading/empty/error/data.
- **Clock warping** for billing time (see `billing`).
- Canon mode (MSW) ships forever as the demo world — keep it green alongside real-API paths.

## Mistake bank

- Retyping a canon number in a test (import from packages/canon; drift must fail, not fork).
- Inventing demo/seed data outside canon (ADR-026 violation).
- Testing the happy path of a four-state surface only (fault-inject the other three).
- Weakening an invariant to make a feature fit (that's a canon decision for founders, e.g. the X1 $208 case).
- A Docker-gated integration test as the ONLY assertion of a security invariant — it `t.Skip`s
  where no container runtime exists and the suite still prints `PASS ok`, masking the guard green.
  Every critical invariant needs a store-free unit assertion that always runs (Q4: the org-key
  delegation ceiling; the user-minter half can stay integration-gated, but not the whole property).
- A property test whose assertions live inside an `if allowed {…}` branch — if the positive branch
  never runs (a regressed matrix, a mistyped grant list), it passes vacuously. Count the positive
  cases and `t.Fatal` on zero (Q4: tighten-only logs "N allowed", org-key sweep asserts len(granted)).
- A "drift guard" that checks a hand-maintained list instead of DERIVING from source — it can't catch
  the drift it's named for (a new call site the list forgot). Scan the actual call sites and assert
  set-equality both directions (Q4: enforced-permission set regex'd from the handler source).
- Claiming "drift fails CI" via a `make gen` + `git diff` gate WITHOUT checking the gate actually runs
  that gen step and diffs that path. The go job runs `gen-go` (not `gen-ts`); the diff was scoped
  `-- services packages` (excluding `apps/`). A synced COPY is only protected if some CI step
  regenerates it AND the diff covers its path. Prefer an in-process test that reads the canonical
  source (fs, via import.meta.url / runtime.Caller) and asserts the copy equals it — it fails wherever
  it runs, independent of CI plumbing (Q2).
- A copy-vs-copy identity test (`consoleCopy.toEqual(packagesCopy)`) proves nothing when both copies
  can be equally stale — compare each copy to the CANONICAL source, not to a sibling (Q2).
- A guard that passes on a zeroed world: `proj == null` is false for `0`, so `0 === 0` sails through.
  Reject empty/zero explicitly, and keep Go and TS guards symmetric or one language passes what the
  other rejects (Q2).
- A test placed outside the runner's include glob (console vitest is `tests/**`, not `src/**`) never
  runs and never fails — a green suite that silently skips your new test. Confirm the file is actually
  collected (the test count goes up) before trusting it (Q2).
