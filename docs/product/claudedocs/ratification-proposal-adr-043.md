# Ratification proposal — ADR-043 (notification semantics)

**Status:** proposed text, awaiting founder action. **Nothing here has been applied.**

`docs/product/18-philosophy/decisions.md` is human-only and hook-enforced, so this
file carries the exact wording for you to paste. No agent has modified, and no
agent may modify, the decision log.

## Why this is needed

Two rulings were made on 2026-07-20 and both are load-bearing in shipped code:

| Ruling | Where it is recorded today | Where the code cites it |
|---|---|---|
| Seven-kind notification taxonomy (Option B) | `spec-change-proposals.md`, on the **unmerged** docs branch (PR #285) | `internal/notify/classify.go` |
| Titles render once at projection (Option A) | `spec-change-proposals.md`, on `task/US-10.3-impl` (PR #286) | `internal/notify/classify.go`, `bell.go` |

Both currently live only in the **proposals** ledger, whose own header states that
nothing in it is resolved — so neither has an authority home. A reader of the ADR
log will not learn either decision exists. That gap is what a review flagged as
indistinguishable from an invented decision.

## Placement

Insert as the newest entry in **Era 4 — The handoff package (Jul 2026)**, i.e.
directly **above `**ADR-042 · …`** (currently line 81). Numbering follows ADR-042.

## Proposed text — paste verbatim

```markdown
**ADR-043 · Notification semantics: a closed seven-kind taxonomy, and titles rendered once at projection.** Two rulings taken together while building the bell projection (US-10.3), both with shipped consequences. **(1) The kind vocabulary is closed at seven** — `alert · proposal · approval · deploy · lifecycle · billing · security`. The P6 routing matrix and N2 type chips carry five; `billing` and `security` are promoted to first-class rather than collapsed into `alert` or left unconstrained. *Alternatives rejected:* folding both into `alert` (cheapest, frames untouched — rejected because N2's "Alerts" chip would then mean *alerts + billing + security*, a user-facing filter that lies, and the collapse is irreversible once backfilled); leaving `kind` an unconstrained string (defers the decision, but any writer can invent a value and the type chips cannot be trusted); pinning five with a tolerant read that maps unknowns to a fallback (hides drift behind a fallback that is itself invented copy). *Evidence:* `billing` is already written by `EmitQuotaWarning` and `flows.md:19` mandates the 80% billing bell; `security` is presupposed by `notify.NotifyInput.Urgent` ("security/paging-class") and the notifications migration. Neither is a stray — **the classes are real and specified; their producers are simply unbuilt.** *Consequence:* an eighth kind is a new founder decision, never an engineering one; P6's matrix and N2's chips must gain `Billing` and `Security` rows before the contract enum can be pinned (US-10.6 stays blocked until then). **(2) A notification is a historical fact, so its title is rendered ONCE at projection and persisted** — not resolved at read time. *Alternatives rejected:* resolving names at read (always-current, but every bell read joins users+environments, a deleted user breaks rendering, and the stored row is not self-describing); a hybrid storing both rendered text and ids (two sources of truth on one row that can disagree). *Why:* rendering once preserves the event exactly as the user experienced it — a later rename must not rewrite history, on the same principle as a sent email — and it keeps the bell read path, which the badge polls, free of joins. *Constraints attached:* persist only presentation fields belonging to the notification itself (no duplicated domain state); rendering is deterministic and uses only local data available during projection (no external or asynchronous lookups); and **no invented fallback copy** — where a frame cannot be rendered exactly, the row is withheld and the gap surfaced as a product question, never papered over with a stand-in. *Consequence:* an unrenderable event is flagged (`bell_scanned.unrenderable`) so a later copy ruling can be applied retroactively; automation deploys currently ring no bell, because ADR-0007 org keys carry no human actor and no frame supplies copy for one — an open product decision. Founder-ratified. (2026-07-20) → US-10.3 · PR #285 · PR #286 · ADR-0007 *(org keys)* · P6/N2 frames
```

## If you would rather split it

The two rulings are independent enough to stand alone. To split, use the same
text with these leads, inserting ADR-044 above ADR-043:

- `**ADR-043 · Notification kind vocabulary closed at seven; billing and security are first-class.**` — ruling (1) only.
- `**ADR-044 · A notification is a historical fact: titles render once at projection.**` — ruling (2) only.

I recommend **one entry**. Both were decided in the same session, and (2)'s
"no invented fallback copy" constraint is what forces (1)'s frame amendment to be
a prerequisite rather than a nicety — the log reads better with that link intact.

## After it lands

1. The `[RULED — founder]` rows in `spec-change-proposals.md` should be updated to
   point at ADR-043 rather than claiming to be the record themselves.
2. `internal/notify/classify.go`'s citation ("recorded on the docs branch, PR #285;
   not yet on main") can be simplified to cite ADR-043.
3. US-10.6 remains blocked — ratification is necessary but not sufficient; the
   **P6/N2 frame amendment** is the actual unblocker.
