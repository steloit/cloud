# Design note — notification copy for a non-human actor

**Status:** product decision required. **No wording has been implemented.** The
options below are drafted for selection, not applied.

## The problem in one line

N1's deploy row reads `Deploy #142 to production by priya`. When the actor is not
a person, there is no frame that says what goes after "by" — so the projection
withholds the notification entirely.

## 1. Scenarios that require non-human actor text

Verified against the spine's actual actor values, not assumed:

| Actor | Reaches the bell today? | Detail |
|---|---|---|
| **Org key / API token** (`kind='org'`) | **Yes — this is live** | ADR-0007 org keys carry `user_id NULL` (`tokens.up.sql:7`), so `CreateDeployment` records an empty actor. Every CI/automation deploy hits this. |
| **`system`** | Not yet | Platform-initiated work (lifecycle sweeps, scheduled tasks, invoice close, dunning, cert renewal). None is classified as notification-worthy yet, but lifecycle and billing are two of the seven ruled kinds, so this arrives with E1/E11. |
| **`github`** | Not yet | Git integration emits `git.push` / `git.pr_*`. Unclassified today; a deploy triggered by a push is the obvious future case. |
| **Personal token** (`kind='personal'`) | Yes, and it renders fine | Carries a `user_id`, so `users.name` resolves and it reads `by priya`. **Worth a decision anyway:** the frame makes no distinction between "priya clicked deploy" and "priya's CI token deployed". If that distinction matters, this is in scope too. |
| **User with an empty name** | Edge | `name` is required by `SignupRequest` but the column defaults to `''`, and only one code path creates users. Rare, not impossible. |

**Scale note:** for a team using CI, automation is likely the *majority* of
deploys — so this is not an edge case in practice; it is the common path being
silently dropped.

## 2. Constraints

1. **No invented copy.** Standing founder rule; microcopy comes verbatim from
   frames (`AGENTS.md` hard rule).
2. **`00-sources/**` is human-only, hook-enforced.** Every option below requires a
   *frame amendment by you*. No agent can land the copy.
3. **A notification is a historical fact** (ADR-043 proposed / Option A): the text
   is frozen at projection. Whatever is chosen is what that row says forever.
4. **Deterministic, local reads only** — no external or async lookup at render time.
5. **The taxonomy is closed at seven kinds.** This is a *copy* decision, not a new
   kind.
6. **Grammar must survive into email** (US-10.4): subject `[org/project] <fact>`,
   one event → one email → one action. Whatever is chosen is read aloud in a
   subject line as well as a bell row.
7. **Existing rows are recoverable.** `bell_scanned.unrenderable` flags withheld
   events, so a ruling applies retroactively via a targeted DELETE. Nothing is lost
   while this is open — but nothing rings either.

## 3. Wording options

All three need a frame amendment; they differ in what the frame must show.

### Option A — name the credential
```
Deploy #142 to production by ci-deploy-key
```
`tokens.name` is `NOT NULL`, so every org key already has a human-chosen name.

- **For:** keeps N1's grammar *exactly* — still "by ‹someone›", so one frame row
  covers both cases and the email subject grammar is untouched. Precise and
  auditable: it says which credential acted, which is what an on-call reader
  actually wants at 3 a.m. Requires no new vocabulary.
- **Against:** token names are user-chosen and unvalidated, so quality varies
  (`test`, `key2`). A name can be edited or the token revoked, after which the
  frozen title references something that no longer exists — acceptable under
  "historical fact", but worth stating. Mild information disclosure: the bell now
  names a credential to every org member.

### Option B — a generic automation phrase
```
Deploy #142 to production by automation
```
- **For:** simplest; one fixed string, no dependency on user-supplied data, no
  disclosure question. Reads cleanly in an email subject.
- **Against:** loses the "which one" that makes the notification actionable — with
  several CI pipelines, every deploy notification looks identical. Introduces a
  new noun ("automation") that appears in no frame today and would need defining
  in the glossary to stay consistent with `system`/`github`.

### Option C — drop the clause when there is no person
```
Deploy #142 to production
```
- **For:** invents no vocabulary at all; the absence of a "by" clause honestly
  reflects the absence of a person. Smallest semantic commitment.
- **Against:** needs a *second* frame variant (N1 currently shows only the
  with-actor form), so the frame amendment is structural rather than additive, and
  every future title with an actor clause inherits the same fork. Two shapes per
  notification type is a lasting tax on US-10.4's templates.

## 4. Recommendation

**Option A**, with a fallback to B where no name is resolvable.

The reasoning is DX-first (the standing review order, ADR-040): the question a
developer asks when a deploy notification arrives is *"which pipeline did this?"*
Option A answers it, B does not, and C removes the question from the surface
entirely. A also preserves N1's grammar exactly, so the frame amendment is one
additional example row rather than a structural variant — and US-10.4's email
subject grammar needs no special case.

The disclosure concern is real but small: org keys are already listed with their
names to any member who can read the integrations surface.

If you find token names too unreliable to put in front of users, **B** is the
honest second choice; I would avoid **C** purely on the two-shapes tax it imposes
on every downstream template.

**`system` and `github` are a separate sub-decision.** They have no credential
name, so they need their own noun whichever option is chosen. Suggest deferring
until a `system`/`github` action is actually classified as notification-worthy —
nothing is blocked by leaving it open.

## 5. Impact

**Notifications (US-10.3, shipped).** One `renderer` change plus a frame amendment
for the copy. The withheld rows are already flagged, so re-projection is a targeted
`DELETE FROM bell_scanned WHERE unrenderable` — note the flag is **coarse** (it
also covers empty env names and unresolvable subjects), so a blanket re-queue
retries those too. Harmless, they simply re-flag.

**Email templates (US-10.4, not started).** This is why deciding *before* US-10.4
matters. The 12 templates encode the same actor grammar in both subject and body.
If the ruling lands first, they are authored once; if it lands after, every
template carrying an actor clause is revisited. Option C is the costliest here —
it forks each affected template into two shapes.

**Console (N1/N2, not started).** No structural change under A or B; C adds a
second row layout to build and test.

**Audit and the spine.** None. The spine already records the true actor
(`tok_…`/`system`/`github`); this decision only governs how that is *presented*.
No denormalization, no schema change.
