# ADR-0014 — Platform invariants live in types, not at call sites

**Status:** Accepted (founder-ratified 2026-08-23) · proposed by agent 2026-07-27

The rule now BINDS: where a platform invariant can be encoded in a type, it must
be. Reviews may cite this as standing authority.

*Pre-ratification framing, kept because it scopes what was decided:* the code was
never contingent on the ruling. `money.Cents` and `problem.FromDenial` each close
specific, reproduced defects (a spend cap permanently bypassable by one
authenticated request; a 403/404 divergence between two transports of one
operation). What was open was only the PRECEDENT — whether the general rule binds
future work. It does.

**Deciders:** Founder
**Relates to:** ADR-025 (money is integer cents), ADR-0008 (review pipeline),
`contexts/api-conventions.md` (404-not-403 for no-standing)

## Context

US-3.8 went through six review rounds. The feature itself was substantially
correct after the first pass; **every subsequent round found a defect in the
previous round's fix**, and every one of those was the same error:

| round | the fix | what it missed |
|---|---|---|
| 1 | a test for the desired-doc guard | the test re-implemented the guard, so deleting the real one stayed green |
| 2 | a guard on the sweep's cast | an earlier OR arm already held the same guard, so the new one was dead |
| 3 | 404-for-no-standing on `listEvents` | the SSE half of the same operation still answered 403 |
| 4 | 404 for authenticated callers | the env lookup ran before the principal check, so anonymous callers still got an oracle |
| 5 | an overflow bound on `override.instances` | two sibling arms of the same pricing switch still wrapped |
| 6 | a bound derived from one multiply | the billing rollup multiplies again, so amounts just under it wrapped on the invoice |

The pattern is not carelessness about any one case. It is that **the design made
correctness a matter of remembering at each site**. `int64` cents admits
`a * b`, so every priced dimension was an independent opportunity to forget. The
denial classification was a prefix match written out in each handler, so every
transport was an independent opportunity to diverge.

Reviews caught all six. That is the pipeline working, and it is also the problem:
a rule whose enforcement depends on a reviewer noticing is a rule that holds at
the rate reviewers notice.

## Decision

**Where a platform invariant can be encoded in a type or a single shared
decision, it must be — and the unsafe form must not compile.**

Two applications, both landed:

### 1. `internal/platform/money` owns all monetary arithmetic

`money.Cents` is a **struct**, not `type Cents int64`. A named integer type
still compiles `a + b` and `a * b`; a struct does not. Every combination goes
through `Add` / `Sub` / `MulInt` / `AddMul` / `Sum`, each returning an error.
Overflow detection uses `math/bits` for the exact 128-bit product.

The invariant belongs to the type: **a `Cents` in hand is in `[0, MaxMonthly]`**,
because no constructor, operation or decoder can produce one outside it.
Callers never range-check.

`MaxMonthly` is DERIVED — `MaxInt64 / secondsInLongestMonth` — because
`metering.Rollup` multiplies an accepted rate by a month of seconds. It is not a
commercial ceiling; picking "at most N instances" remains a pricing decision the
implementation never makes (founder, 2026-07-27). What is merely unaffordable is
still the hard spend cap's job.

**THE CEILING BOUNDS ONE SERVICE-MONTH, NOT THE AGGREGATE (O19, 2026-08-23).**
That derivation makes `rate × one month` fit an int64 *exactly*, so a single
service at the ceiling consumes the entire budget and the **second** service
wraps. `Rollup` accumulates across every span of every service in the org, and
this ADR's own text said the amount survives "the whole money path" — false as
written for the org-wide sum, which is the number the invoice, the month-to-date
spend and the hard cap all read.

The fix is not a bigger constant and not a check at the accumulation site. It is
a **second type**: `money.Accrual`, a 128-bit Σ(rate × seconds). An accrual is
not an amount — bounding it by `MaxMonthly` would refuse arithmetic that is
simply correct (two service-months is a legitimate business fact), and bounding
it by `MaxInt64` is the wrap itself. At 128 bits the accumulation cannot
overflow in practice (~3.7e19 service-months at the ceiling), so the class is
unrepresentable rather than tested-against — the same move as `Cents`, applied
to the quantity `Cents` is multiplied into. The single narrowing point,
`Accrual.Int64()`, is the only place that can fail, and it fails loudly without
writing.

`Accrual.DivSeconds` exists for the same reason `AddMul` does: the 128-by-64
division has a trap — `bits.Div64` **panics** when the high word is at least the
divisor, which is precisely the large-org case. A caller writing that division
by hand crashes the process instead of getting an error.

### 2. `problem.FromDenial` owns the denial→response mapping

One decision (`AccessDeniedError.AccessDeniedNoStanding`, next to the strings it
classifies) and one mapping (`problem.FromDenial`, importable by every transport
including the ones that cannot import `identity`). No standing → 404,
indistinguishable from a missing id. Has standing, lacks the permission → an
honest 403 naming what denied it.

This is the PDP/PEP shape: **one decision point, many mechanical enforcement
points.** It matches how GitHub answers a private repository you cannot see.

## Consequences

- **The compiler becomes the reviewer for this class.** Converting the codebase
  surfaced eight money boundaries across five files, three of which no review
  round had looked at. That exhaustive search is now free and repeats on every
  change.
- **Two live defects fell out of the conversion**: the JSON decoder accepted a
  quoted number (`"5800"`) — this is about `money.Cents`'s OWN decoder, not the
  HTTP wire: no client could ever have sent it, because request bodies decode
  through the generated `*int` types. Corrected after review; the original
  wording implied a live API defect the history does not support; and
  valkey's GB rounding used `float64`, which loses precision above 2^53 on
  caller-supplied input.
- **Two paths now fail closed rather than computing on garbage.** `enforceBudget`
  re-validates the stored run-rate and `UpdateService` re-validates the stored
  price, so a row left out of range by a past wrap refuses the request instead of
  disabling the cap.
- **Net −62 lines** despite adding a package: the type absorbed checks that were
  scattered and partial.
- **Cost:** every money boundary is now explicit at conversion points
  (`.Int64()`). That is the intended trade — each one is a place a raw number
  re-enters the world, and it is greppable.
- **This ADR is a standing test for future work.** A new priced dimension, a new
  transport, or a new denial class inherits the invariant instead of
  re-implementing it. If a future change needs to re-state one of these rules at
  a call site, that is the signal the abstraction is wrong, not the site.

## What this does NOT decide

It does not widen to "encode every invariant in a type". Many invariants are
genuinely local, and a type that models one badly is worse than a check. The
rule is narrower: **when the same invariant must hold at more than one site, it
gets one owner** — and where the language can make the alternative
uncompilable, it should.

## Open for founder revision

The money ceiling is derived from today's billing arithmetic. If `Rollup` ever
weights across a longer window, `MaxMonthly` must be re-derived; it is a single
constant with its derivation written next to it. That coupling is now asserted
from **metering's** side (`TestEveryRealPeriodFitsWhatTheCeilingWasDerivedFrom`),
not only from money's — the earlier test re-implemented `AddDate(0,1,0)` rather
than driving the real `periodBounds`, so it could catch the constant drifting
and never the period window growing, which is the half that breaks the
derivation.

**Not decided here, and not decidable in implementation: what `quota_usage.
rate_cents` MEANS.** `Rollup` writes Σ(seconds × monthly rate) — cent-seconds —
and its column comment says so, while `invoice.Close`, `mtdSpend`,
`billing_export` and `usage_http` all read the same column as cents. Measured
end to end: one service at $24.00/month running for one hour produces an invoice
of **$86,400.00**. Which side is authoritative, and the proration convention if
the writer is the one that moves, are pricing decisions — filed as **O30** with a
`NEEDS FOUNDER INPUT` row rather than settled here.
