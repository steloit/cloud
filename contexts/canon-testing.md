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

## More techniques (E3, 2026-07)

- **Ordering is unprovable from final output.** A substring check on completed stdout
  cannot establish that A happened before B — the output is the same either way. Make the
  wrong order impossible instead: CK-M3 proves the estimate reaches the operator BEFORE
  anything provisions by declining the prompt and asserting the price is on screen while
  zero services exist. Ordering by construction, not by inspection.
- **Anti-vacuity guards.** A test whose subject may legitimately be empty must fail loudly
  on empty rather than passing: `if len(needles) == 0 { t.Fatal("this case proves
  nothing") }`, `if ctLen <= 16 { t.Fatal("no payload stored — this scan proves
  nothing") }`. Costs one line; the alternative is discovering the vacuity months later.
- **Assert the shipped artifact.** CK-M3 BUILDS the CLI binary and execs it rather than
  importing its packages — importing proves the library works, not that the binary a
  customer runs does. Where a seam matters, cross it the way the customer crosses it.
- **Derive the selector, do not list it.** A registry-driven test must iterate the SOURCE
  (the schema, the spec, the fixtures) so a member added later is covered without anyone
  remembering. A hand-maintained list covers exactly what it lists.

## Mistake bank

- Retyping a canon number in a test (import from packages/canon; drift must fail, not fork).
- **A shared map has exactly ONE owner; two mutexes each guarding half the accesses exclude
  nothing.** `reconcile_test.go`'s `fakeTrans` wrote `fakeQ.services` under its own mutex while
  `fakeQ` read it under another — every access "locked", none excluded. The double, not the
  production code, corrupted the very property it exists to check. Found TWICE (Q10 and O14
  independently; `platform/idempotency`'s `fakeStore` carries the same fix). CI was flaking at
  ~1%, not passing: a plain run hits `fatal error: concurrent map read and map write` ~1/100,
  killing the whole binary. `-race` catches it in one run; CI has none (O13).
- **A mutation result is only evidence about the INVOCATION it was measured at.** Q10's
  one-owner fix also serialised the calls the test existed to interleave. Killing-mutation
  detection at CI's plain `go test ./...`: **18/100 before, 3/100 after, 100/100 once a
  barrier forced all N callers past the pre-check.** Implementer and architecture review both
  measured at `-count=50` — which CI never runs. Quote the invocation beside the rate.
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
- A conformance/coverage test that checks a HANDFUL of cases and reads as "covered" — Q3 first shipped
  checking 5 of 18 canon response sections, so drift in the other 13 passed green. Drive coverage off a
  registry with a completeness tripwire (every fixtures section must have a check, or fail), so the
  gap is visible, not silent.
- Detecting a MISSING required field by zero-ness of the decoded value — `used: 0` is a legitimate
  required value, not a missing one. Check key PRESENCE in the raw JSON instead; strict decode
  (`DisallowUnknownFields`) catches extra fields, presence catches dropped/renamed required ones.
- `git diff --exit-code` as a drift gate misses newly-generated UNTRACKED files (returns clean). Stage
  first: `git add -A -- <paths> && git diff --cached --exit-code -- <paths>` (Q3).

### The proxy-assertion class (E3 — seven classes, drawn from five tasks)

Every one of these passed its suite, was reported as verified, and was caught only by an
adversarial reviewer. They share one shape: **the assertion is a proxy for the property,
so it passes for a reason unrelated to what it claims.**

- **An assertion satisfiable by output from a DIFFERENT part of the program.** CK-M3's
  `Contains(out, "orders")` named the per-service estimate line, but the post-create
  confirmation line also prints the service name — so deleting the entire estimate loop
  passed. Tie the label and the value on the SAME line (split on newlines, require both),
  or assert on a structured value instead of rendered text.
- **An assertion true BY CONSTRUCTION.** `w.(http.Flusher)` succeeds because the wrapper
  declares the method; it says nothing about whether the call forwards. Assert the EFFECT
  — a spy that counts calls reaching the underlying writer — never interface satisfaction.
  (Related: a wrapper must not advertise a capability the wrapped value lacks. Declaring
  `Hijack` unconditionally made `w.(http.Hijacker)` succeed over a non-hijackable writer,
  so a handler took the hijack branch instead of its fallback: worse than stripping it.)
- **A scan for a value that has more than one on-the-wire REPRESENTATION.** `[]byte` in
  JSON is base64, so a literal scan for a secret cannot see the body. Check every encoding
  the value can take, and fail loudly when the needle is too short for the check to be
  meaningful — an empty comparison target matches everything, which is the same bug
  wearing the opposite mask.
- **An assertion of DIFFERENCE where the property is INVARIANCE (or vice versa).** "The
  two identities differ" is satisfied by corruption, dropping and mangling alike — a
  field silently dropped from a resolved map still made the identities differ, so a
  registry test that iterated every declared field still missed one. Assert what must be
  true (every declared field survives the round trip carrying the value it was given).
- **A test that SELECTS its cases by a property of today's values.** Choosing "fields whose
  default looks like a zero value" skipped exactly the field whose default had been
  wrongly changed. Derive the selector from BEHAVIOUR instead ("fields where changing the
  value does not move the price"), so the mutated case cannot exclude itself.
- **A `0 == 0` comparison over a collection that may be empty.** `len(a.live)` unchanged
  after a retry is satisfied by an apply that did nothing. Assert the collection is
  non-empty first, then that it did not grow.
- **A fixture the API could never produce.** `intent: "transactional"` is not in the
  catalog enum; an override with no `expires_at` cannot be created by the handler that
  always stamps one. Such a test asserts on an impossible state and its green means
  nothing — both surfaced only when unrelated validation was tightened, revealing that
  three tests had been exercising a non-existent API state. Drive fixtures through the
  real construction path; when hand-building, name the handler that produces this shape.

**The discipline these imply:** when an assertion covers something a customer perceives —
a price on screen, a credential at rest, an ordering — one mutation is not verification.
Mutate once per way the property can break, and if a check can be satisfied by output from
elsewhere in the program, it is not checking what its failure message claims.
