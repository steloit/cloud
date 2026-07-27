# Engineering OS review — 2026-07-27

**Scope:** `.claude/agents/*`, `.claude/skills/*`, `contexts/*` mistake banks, ADR-0008,
`tasks/_template.md`, `scripts/spec-sync/validate.mjs`, AGENTS.md.
**Evidence base:** US-1.3, US-1.3a, T3.4, US-3.3, US-3.6, US-3.6a, CK-M3, US-3.7 —
17 review rounds across 4 merged PRs (#300–#303).
**Status:** implemented as O11 (2026-07-27).

> **Corrections — read before citing this document.** O11's review round rejected three
> claims below as unsupported by the committed history, and QA found a fourth
> misattribution. They are struck in place; the corrected versions live in the files O11
> shipped. This document is evidence for the *recommendations*, not for those four
> incidents.
>
> 1. **`Price`'s `default:` arm was never deleted** (H1 below). `34774cd` already carried
>    it; US-3.7 repurposed it in place. The 0-cent outcome existed only as a counterfactual
>    in a code comment. Corrected text: `.claude/agents/reviewer.md` dimension 7.
> 2. **The CK-M3 claim is UNVERIFIABLE, not false** (C5 below). PR #302 was squashed, so
>    CK-M3's pre-review state is not in the history at all — "its implementation commit
>    already carried the grep form" is an artifact of squashing, not evidence about the
>    first submission, and `4b9ace6`'s own body says "a skipped checkpoint also read as
>    green". A first correction here asserted the claim was false, which read absence of
>    evidence as evidence of absence — the thing AGENTS.md step 8 forbids, applied to
>    entries but not, at first, to corrections. The entry now cites **US-1.3**
>    (`tasks/e1-substrate/US-1.3.md:196`) because its incident IS directly citable.
>    Corrected text: `.claude/skills/verify/SKILL.md`.
> 3. **"Every survivor was found in an unexplored class, never in a re-run"** (H6 below) is
>    falsified by C1's own evidence — the base64 finding came from a reviewer re-running the
>    builder's mutation against a different representation. Corrected text:
>    `.claude/skills/task-pickup/SKILL.md`.
> 4. **The NUL-separator incident belongs to US-3.6, not US-3.7**, and it was caught **by the
>    gate**, not by a reviewer — so it is weak evidence for dimension 7, whose thesis is that
>    reviewers catch what the gate cannot. `.claude/agents/reviewer.md` cites it without
>    naming a task, so the misattribution did not propagate.

---

## The finding that organises this review

Across eight tasks, **one defect class produced the majority of blocking review
findings**, and it was never caught by the gate:

> A test asserts a **proxy** for the property rather than the property, so it passes
> for a reason unrelated to what it claims. Running one mutation against it confirms
> the proxy, not the property — and the builder then reports the class as closed.

Every instance, with the mutation that survived:

| Task | The assertion | What it actually allowed |
|---|---|---|
| US-3.6 | `len(a.live)` unchanged after retry | `0 == 0` — an apply that did nothing |
| US-3.6a | scan stored row for the secret | body is base64 in JSON → **the whole response in the clear passed** |
| US-3.6a | `w.(http.Flusher)` succeeds | true by construction; the wrapper declared the method and never forwarded |
| CK-M3 | `Contains(out, "$24/mo")` | satisfied by *either* surface; `total $0.00/mo` passed |
| CK-M3 | `Contains(out, "orders")` | matched the **post-create confirmation line**, not the estimate line |
| US-3.7 | two identities `!=` | satisfied by a field being silently **dropped** |
| US-3.7 | unpriced default "looks like a zero value" | skipped exactly the field whose default was wrong |

Seven instances, one shape. The reviewers caught all seven; the gate caught none.
That asymmetry is the argument for most of what follows: **the gate proves code runs;
only an adversary proves a test discriminates.**

A second, quieter class: **silent acceptance**. The estimate gate accepted a
substituted configuration; `resolve` dropped an unknown-kind field; `services.intent`
accepted `"transactional"`, which is not in the catalog enum — so three tests written
in US-3.6/US-3.6a had been exercising an API state that cannot exist. In each case
something accepted input it should have refused, and everything downstream inherited
the falsehood.

---

# CRITICAL — adopt immediately

## C1 · Mutation testing must be specified as class-coverage, not existence

**Why.** The current instruction (implicit, nowhere written) is "mutation-verify your
guards". Seven times a builder ran one mutation, saw red, and reported the class closed.
Nothing in the OS says a mutation is a *hypothesis about a class*.

**Evidence.** US-3.6a: I verified the plaintext scan by bypassing the seal and watching
it fail — correct, but only for the **header** path. QA re-ran it with the production
`createWebhook` header shape and it **passed with the entire response stored in the
clear**, because `[]byte` base64-encodes in JSON and my scan was literal-only. The
motivating case of the whole ADR was uncovered while I reported it verified.

**Change.** `.claude/agents/qa.md` — add a numbered method step:

```md
6. **Mutation classes, not mutation examples.** For each guard, enumerate the DISTINCT
   WAYS the property can be violated, and mutate once per way. A guard over data that
   has more than one representation (raw/encoded, present/absent/dropped, one surface
   or another) needs one mutation per representation. Report survivors as
   `[risk: high]` even when a sibling mutation died — "one mutation failed" is not
   evidence the class is closed.
```

And `.claude/skills/verify/SKILL.md` — add step 6:

```md
6. For any guard you claim is mutation-verified, state IN THE PR which mutations you ran
   and which representation each covers. One mutation is evidence about one mutation.
```

**Benefit.** Converts the single most expensive recurring defect into a checklist item.
**Migration.** None — additive text.

---

## C2 · An assertion that can be satisfied by unrelated output is a defect

**Why.** Three of the seven were *substring or existence checks* that another part of
the program satisfied. `Contains(out, "orders")` matched the post-create confirmation
line while the estimate lines it named were deleted. This has no name in the OS today,
so it is rediscovered each time.

**Evidence.** CK-M3, both display assertions, found by two reviewers independently
from different angles.

**Change.** `contexts/canon-testing.md` mistake bank — three entries:

```md
- An assertion satisfiable by output from a DIFFERENT part of the program. CK-M3's
  `Contains(out,"orders")` named the per-service estimate line but was satisfied by the
  post-create confirmation line, so deleting the estimate loop passed. Tie the label and
  the value on the SAME line (split on newlines and require both), or assert on a
  structured value rather than rendered text.
- An assertion true BY CONSTRUCTION. `w.(http.Flusher)` succeeds because the wrapper
  declares the method — it says nothing about forwarding. Assert the EFFECT (a spy that
  counts calls reaching the underlying writer), never the interface satisfaction.
- A scan for a value that has more than one on-the-wire REPRESENTATION. `[]byte` in JSON
  is base64, so a literal scan for a secret cannot see the body. Check every encoding the
  value can take, and fail loudly if the needle is too short for the check to be
  meaningful (an empty comparison target matches everything).
```

**Benefit.** Names the class where builders and reviewers both look.
**Migration.** None.

---

## C3 · "Self-extending" tests must derive expectations from semantics, not from today's values

**Why.** `TestCanonicalCoversEveryDeclaredField` iterated the schema — genuinely
self-extending in intent — and still missed a field, because it asserted the two
identities *differ*, which a silently-dropped field also satisfies. Separately,
`TestUnsetUnpricedFieldIsADifferentContract` selected fields by whether the default
*looked* like a zero value, so it skipped exactly the field whose default was wrong.

**Evidence.** US-3.7, both found by QA after I'd verified the first one myself with a
different mutation.

**Change.** `contexts/canon-testing.md`:

```md
- A registry-driven test that asserts a DIFFERENCE rather than the PROPERTY. "The two
  identities differ" is satisfied by corruption, dropping, and mangling alike. Assert
  what must be true (every declared field survives the round trip carrying the value it
  was given), not that two outputs are unequal.
- A test that SELECTS its cases by a property of today's values ("defaults that look
  like zero") skips exactly the case whose value was wrongly changed. Derive the
  selector from behaviour instead ("fields where changing the value does not move the
  price"), so the mutated case cannot exclude itself.
```

**Benefit.** The two highest-value tests in the codebase now have a stated shape.
**Migration.** None.

---

## C4 · Fault injection happens in a copy — and the OS must say why, with the incident

**Why.** ADR-0008 and both agent files already say "temp copy, disclose mutations". **I
violated it repeatedly**, mutating the working tree throughout US-3.6a, CK-M3 and
US-3.7. It corrupted an independent reviewer's evidence **twice** (QA rsync'd a mutated
`engine.go`, then a mid-edit test file, and chased a false red baseline before it could
trust any result).

**Evidence.** QA's US-3.7 report, verbatim: *"at one point I read engine.go and got
`if false {` where `if !catalogIntents[intent] {` now sits — a live mutation in a tracked
file… it poisoned my sweep twice."*

**Change.** The rule exists but is addressed only to reviewers. Move it to
`AGENTS.md` **Hard rules**, binding on every agent including the implementer:

```md
- **Fault injection happens in a throwaway copy, never the working tree** — including
  the implementer's own mutation testing. A mutate-then-restore leaves no diff, so an
  interrupted run is invisible to review, and a reviewer reading the tree mid-mutation
  gets a false baseline (this happened twice in US-3.7). Copy, mutate the copy, delete it.
```

**Benefit.** Closes a live process defect that has already damaged review evidence.
**Migration.** None. Note it is a *tightening* — the implementer was previously unbound.

---

## C5 · Definition of Done must require the negative evidence

**Why.** `verify:` blocks prove the suite runs. Nothing requires evidence that a test
*can fail*. Every one of the seven vacuous assertions passed its verify block.

**Evidence.** All seven. Also CK-M3, where `go test -run CKM3` exited **0** with
`[no tests to run]` — the milestone gate was green on any machine without Docker.

**Change.** `.claude/skills/verify/SKILL.md`:

```md
7. **Negative evidence.** For every NEW guard the task introduces, the PR states the
   mutation that kills its test and the observed failure message. A guard with no
   recorded mutation is not verified — it is merely exercised.
8. **A skipped test is not a pass.** If a verify command can print `ok` while its tests
   skip (no container runtime) or match nothing (renamed test), it is not a gate. Make
   the skip fatal under an env var the verify block sets, and grep for `--- PASS: <name>`
   rather than trusting the exit code.
```

**Benefit.** Makes the milestone-green-without-running failure structurally impossible.
**Migration.** Existing tasks' verify blocks stay valid; new ones carry the stricter form.
Retrofitting CK-M3's pattern to older checkpoints is optional.

---

# HIGH VALUE

## H1 · Reviewer dimension 7: "what did this change stop checking?"

**Why.** Three defects were introduced *by a fix for a previous finding*: the unfenced
discard (US-3.6a), the NUL path separator that Postgres cannot store (US-3.7), and
`Price`'s deleted `default:` arm, which made a declared-but-unpriced product cost **zero
cents**. In each case I verified the thing I set out to fix and not the thing I disturbed.

**Evidence.** US-3.7, reviewer: *"resolve() owns the unknown-product check, so without
this arm the two tables can now drift silently in the mis-billing direction."*

**Change.** `.claude/agents/reviewer.md`, add dimension 7:

```md
7. **Displaced guards** — for every check the diff MOVES, RENAMES or DELETES, ask what it
   was catching and whether anything still catches it. A refactor that relocates
   validation between layers is the highest-risk shape: the new check is tested, the
   thing the old check quietly did is not. Name the guard and where its responsibility
   now lives, or report it as a finding.
```

**Benefit.** Targets a class that produced three blockers in two tasks.

## H2 · A fix's blast radius belongs in the review request

**Why.** The founder-facing report and the reviewer prompt both described *what was
fixed*. Neither asked *what else the fix touches*. `resolve()` validating intent turned
out to be a **breaking change to `POST /v1/estimates`**, which I did not notice and QA
had to point out.

**Change.** `.claude/skills/task-pickup/SKILL.md`, step 6:

```md
   When requesting review, state the BLAST RADIUS explicitly: which endpoints, contracts
   or clients change behaviour, including ones outside the task's stated scope. A
   validation added to a shared path applies everywhere that path is called.
```

**Benefit.** Public behaviour changes stop being discovered by reviewers.

## H3 · Test fixtures must be constructible by the API

**Why.** Two tests asserted on data the API cannot produce: `intent: "transactional"`
(not in the catalog enum) and an override with **no `expires_at`** (the handler always
stamps one). Both passed for years-equivalent and proved nothing about real behaviour.
Both surfaced only when validation was tightened.

**Evidence.** US-3.7 (three tests), US-3.8 (one test).

**Change.** `contexts/canon-testing.md`:

```md
- A fixture the API could never produce. `intent:"transactional"` is not in the catalog
  enum; an override with no `expires_at` cannot be created by the handler. Such a test
  asserts on an impossible state, and its green is meaningless. Prefer driving fixtures
  through the real construction path; when hand-building, state which handler produces
  this exact shape.
```

**Benefit.** Catches a class that hides until unrelated tightening exposes it.

## H4 · Promote three durable techniques into `contexts/canon-testing.md` Techniques

These earned their place by catching real defects:

```md
- **Ordering is unprovable from final output.** A substring check on completed stdout
  cannot establish that A happened before B. Make the wrong order impossible instead:
  CK-M3 declines the prompt and asserts the price is on screen while zero services
  exist — ordering by construction.
- **Anti-vacuity guards.** A test whose subject may be empty must fail loudly on empty:
  `if len(needles) == 0 { t.Fatal("this case proves nothing") }`, `if ctLen <= 16
  { t.Fatal("no payload stored") }`. Cheaper than discovering the vacuity later.
- **Assert the shipped artifact.** CK-M3 BUILDS the CLI binary rather than importing its
  packages, because importing proves the library works and not that the binary does.
```

## H5 · Record the "silent acceptance" class in `api-conventions.md`

```md
- **Silent acceptance is the upstream of most downstream lies.** An enum column with no
  server-side validation, a shape field dropped by an unhandled `kind`, a wrong-typed
  value defaulted instead of refused — each accepts input that cannot be true, and every
  consumer inherits it (three US-3.6 tests were exercising a non-existent API state for
  weeks). Validate at the boundary that OWNS the vocabulary, and make the refusal a
  field-level 422 BEFORE any one-shot resource is consumed.
```

## H6 · Reviewers should be told what the builder already verified

Both reviewers re-derived mutations I had already run, and QA spent a full sweep
rediscovering three I'd reported. Adding the builder's mutation list to the review
request lets them spend their budget on *unexplored* classes — which is exactly where
every survivor was found.

**Change.** `task-pickup` step 6: include a "mutations already run" list in the review
request. Reviewers remain free to re-run any of them.

---

# NICE TO HAVE

## N1 · `spec-author` should require an "anti-vacuity" line per acceptance criterion
For each AC, state what a *false* pass would look like. Cheap; forces the question early.

## N2 · A `retro` step that harvests mistake-bank entries at task close
Every lesson in this review was extracted manually. `task-pickup` step 6 could ask:
"which mistake bank does this task's finding belong in?" — CLAUDE.md already says a
lesson that didn't change a living file didn't happen; nothing operationalises it.

## N3 · `validate.mjs` check: every `verify:` command is skip-proof
Statically flag `go test -run X` without a `-v … grep -q '^--- PASS'` companion. Narrow,
but it is the exact CK-M3 hole.

## N4 · Record the review-agent reliability problem
Agents failed **4 times** on infrastructure during US-3.7 (one API drop, two watchdog
stalls, one mid-run drop). No guidance exists on what to do. Recommend: retry twice,
then surface as a blocked merge — never treat a stall as a pass. I followed this by
judgement; it should be written down.

---

## What I deliberately did NOT recommend

- **Rewriting the reviewer/QA dimensions.** They found every defect that mattered. The
  gap was never their coverage — it was that the builder's self-verification was weaker
  than their adversarial one. C1–C3 raise the builder's floor rather than their ceiling.
- **Automating mutation testing.** Tempting, but the value came from *choosing* mutations
  by reasoning about failure classes. A mutation harness would generate many shallow ones
  and few of the seven that mattered.
- **Loosening ADR-0008 after four agent failures.** The stalls were infrastructure. The
  one time I nearly merged with a single reviewer, QA's late report contained the
  base64 finding — the single most serious defect of the session.

---

## Priority summary

| | Recommendation | File(s) |
|---|---|---|
| **C1** | Mutation = class coverage, not existence | `qa.md`, `verify/SKILL.md` |
| **C2** | Assertions satisfiable by unrelated output | `canon-testing.md` |
| **C3** | Self-extending tests derive from semantics | `canon-testing.md` |
| **C4** | Fault injection in a copy — bind the implementer | `AGENTS.md` |
| **C5** | DoD requires negative evidence + skip-proof gates | `verify/SKILL.md` |
| **H1** | Reviewer dimension 7: displaced guards | `reviewer.md` |
| **H2** | Blast radius in the review request | `task-pickup/SKILL.md` |
| **H3** | Fixtures must be API-constructible | `canon-testing.md` |
| **H4** | Three durable techniques | `canon-testing.md` |
| **H5** | Silent acceptance class | `api-conventions.md` |
| **H6** | Tell reviewers what was already verified | `task-pickup/SKILL.md` |
| **N1–N4** | Spec anti-vacuity · retro harvest · validator check · agent-failure policy | various |

Six files. No structural change to the OS, no new agent, no new ceremony —
every recommendation is text in a file an agent already reads.
