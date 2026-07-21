---
id: O6g
title: No branch protection exists, so no CI check can block anything
epic: EOPS
status: blocked   # deferred by founder ruling 2026-07-21 — see Ruling
phase: V1
priority: high
sprint: 1
estimate: 0.25ew
deps: [O6f]
issue: 0
labels: [DevOps, Security]
module: Engineering OS
contexts: []
files:
  - AGENTS.md
  - .github/workflows/ci.yml
verify:
  - "gh api repos/steloit/cloud/branches/main/protection returns 200 with required_status_checks including validate"
  - "a PR with a failing authority-paths check cannot be merged"
owner: founder
---

## Goal

Make at least one control actually binding. Today none is.

## Why — verified, not assumed

```
$ gh api repos/steloit/cloud/branches/main/protection
403 "Upgrade to GitHub Pro or make this repository public to enable this feature."
$ gh api repos/steloit/cloud/rulesets
403 (same)
$ gh api repos/steloit/cloud --jq '{visibility, private}'
{"visibility":"private","private":true}
```

Private repo on a free org plan. Consequences, all of them real today:

- **CI cannot block a merge.** `authority-paths`, `validate`, `go`, `infra` all *report*.
  A red PR merges with `gh pr merge` exactly as a green one does.
- **`.github/CODEOWNERS` is decorative** — required review is also a protection feature.
- **The `FOUNDER-RATIFIED` marker is agent-writable.** The same agent that opens the PR
  writes its title and body, so the marker constrains nobody. There is no identity binding.

This was found closing O6f, which had just moved the load-bearing claim from the hook to
CI — and CI binds nothing. That would have been the fourth restatement of the same defect
in this family: **a document asserting a guarantee no mechanism provides.** AGENTS.md now
says what is true instead.

## Options — founder decision, not an engineering one

| | Cost | What it buys |
|---|---|---|
| **GitHub Team** (~$4/user/mo) | money, per-seat | branch protection + required checks + CODEOWNERS on a private repo. Smallest change, keeps the repo private. |
| **Make the repo public** | discloses the spec package, pricing, and roadmap | protection free. Almost certainly wrong before launch. |
| **Accept it, and say so** | none | AGENTS.md keeps its current honest wording; controls stay advisory. Defensible while the team is two founders, but it must not be forgotten once anyone else can push. |

Recommended: **GitHub Team**, on the reasoning that five committed clients and a
founder-owned authority boundary are already worth more than the seat cost, and every
control the O6 family built is advisory until this changes.

## Identity binding (only meaningful after the above)

Even with protection, `FOUNDER-RATIFIED`-in-the-body is a string an agent can type. If
ratification is to mean anything mechanically, bind it to something the agent cannot
produce: a label only the founder can apply, or a `Founder-Ratified:` trailer on a
commit signed with the founder's key and verified with `git verify-commit`. Decide this
together with the plan question — a marker that constrains nobody is worse than no
marker, because it reads as a control.

## Ruling (founder, 2026-07-21) — do not re-ask

**Option 3: accept advisory-only, and keep the documentation honest.** Verbatim:

> Keep repository documentation honest. Do not claim enforcement that GitHub
> cannot provide on the current plan. Leave O6g open until we reach external
> collaboration or pre-production hardening, at which point we'll enable GitHub
> Team (or an equivalent solution) and enforce branch protection properly.

So the current wording in AGENTS.md — a hook stops common accidental writes, CI
flags the diff, **nothing blocks a merge** — is the ratified state, not a
placeholder. Do not "fix" it by asserting enforcement.

**Revisit trigger (concrete, per the review's objection to "must not be
forgotten"):** the *first* of — (a) anyone outside the two founders gains push
access, or (b) pre-production hardening begins. At that point enable GitHub Team,
require the `validate` check, and pair it with CODEOWNERS on `.github/`, then
close this task and re-word AGENTS.md in the same PR.

## Status

`blocked` by **founder ruling**, not by missing information — deliberately
deferred with a named trigger. Owner remains the founder: an agent cannot buy a
plan or make a private repo public, and should not.

## Related

O6f (built the controls this would make binding) · ADR-0002 · AGENTS.md hard rules
