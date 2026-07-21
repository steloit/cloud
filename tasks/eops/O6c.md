---
id: O6c
title: Pin the agent directory's load-bearing properties in validate.mjs
epic: EOPS
status: done
phase: V1
priority: medium
sprint: 1
estimate: 0.25ew
deps: [O6]
issue: 0
labels: [DevOps, Tooling]
module: Engineering OS
contexts: []
files:
  - scripts/spec-sync/validate.mjs
  - .claude/agents/README.md
verify:
  - "node scripts/spec-sync/validate.mjs"
  - "adding Edit to reviewer.md frontmatter makes validate.mjs fail, citing ADR-0008"
  - "adding an agent file without a README table row makes validate.mjs fail"
  - "replacing the CLAUDE.md symlink with a regular file makes validate.mjs fail (rm CLAUDE.md && echo forked > CLAUDE.md), and restoring the symlink makes it pass"
  - "moving .claude/agents/README.md aside makes validate.mjs fail with a remediation, not an ENOENT stack trace"
  - "a .claude/agents/*.md with a typo'd frontmatter fence makes validate.mjs fail rather than being skipped"
owner: agent
---

## Goal

`validate.mjs` covers `tasks/`, epics, and packs, but **nothing under
`.claude/**`**. Add rules pinning the two properties ADR-0008 and O6 depend on.

## Why

ADR-0008 makes "the two reviewers are read-only" load-bearing — it is what makes
an independent review independent. Today that property lives only in
`reviewer.md`/`qa.md` frontmatter and in README prose. **Adding `Edit` to that
frontmatter would silently convert a reviewer into a writer**, and no hook, CI
job, or validator would notice. A guarantee nothing checks is a guarantee that
degrades on the first careless edit.

Second, O6's README carries a hand-maintained agent table. The README instructs
authors to add a row, but instruction is not enforcement, and a stale table is
the re-derivation cost O6 exists to remove.

## Acceptance criteria

`validate.mjs` is 72 flat lines with a single `err(file, msg)` helper, no rule
registry, and **no `child_process`** — it imports only `node:fs` and `./lib.mjs`
("No dependencies", line 3). Add four checks in that style, with failure messages
prefixed `agents-readonly:`, `agents-table-sync:`, `entrypoint-symlink:`, and
`agents-readme-exists:`. Do not refactor to introduce a rule abstraction, and do
not shell out to `git` — a subprocess has no precedent here and would break
validation wherever there is no git binary or work tree.

- [x] `agents-readonly`: parse each `.claude/agents/*.md` frontmatter; assert
  `reviewer` and `qa` declare `tools ⊆ {Read, Grep, Glob, Bash}`. Failure cites
  ADR-0008's reviewer-identity clause.
- [x] `agents-table-sync`: the set of frontmatter `name`s equals the README
  table's `subagent_type` column, and each `name` matches its filename stem.
  **Exclude `README.md` by filename**, not by "has no frontmatter" — `lib.mjs`
  `parseFrontmatter` throws the same `missing frontmatter` error for absent and
  malformed blocks, so those two cases are indistinguishable at the parser and a
  content-based discriminator would silently skip a typo'd agent file.
- [x] A file with a malformed frontmatter block fails loudly rather than being
  skipped — test with a `notes.md` carrying a typo'd `---` fence.
- [x] `entrypoint-symlink`: assert **on the working tree**, not the index —
  `lstatSync("CLAUDE.md").isSymbolicLink()` is true *and*
  `readlinkSync("CLAUDE.md") === "AGENTS.md"`. Remediation: "CLAUDE.md must stay
  a symlink to AGENTS.md; edit AGENTS.md instead." Both calls are verified
  working (`isSymbolicLink: true`, `readlink: "AGENTS.md"`).
  **Do not use `git ls-files -s`** — it reads the index, which stays `120000`
  under a `core.symlinks=false` checkout while the working tree holds the 9-byte
  regular file, so it misses the exact outage this check exists for. This is the
  highest-severity check of the four. **Added from O6b's review.**
- [x] `agents-readme-exists`: assert `.claude/agents/README.md` is present, since
  AGENTS.md step 5a now points at it and a rename would dangle silently. Keep it
  separate from `agents-table-sync` even though that check must also read the
  file — the value is a clean `err()` with remediation instead of an unhandled
  `ENOENT`, so run it first and skip the table check if it fails.
- [x] All checks run in the existing `validate.mjs` invocation — no new command.

## Why `entrypoint-symlink` is the urgent one

`CLAUDE.md` is a symlink to `AGENTS.md` (mode `120000`), which is why the two
entry points cannot fork — but **nothing asserts the mode**, so the guarantee is
one careless commit from evaporating. O6b's QA review reproduced it:
`rm CLAUDE.md && echo forked > CLAUDE.md && git add` flips the mode to `100644`
and forks the content, and `validate.mjs` still prints `OK`.

The degraded form is not a fork but silent content loss. Under a checkout with
`core.symlinks=false`, `CLAUDE.md` materializes as a **9-byte regular file whose
entire content is the string `AGENTS.md`**. A session auto-loading it receives
nine bytes in place of the authority order, hard rules, and task protocol — with
no error anywhere. That is a whole-Engineering-OS outage that presents as a
working repo, which makes it worth more than the two agent-directory checks.

## What this cannot check — read before assuming it closes ADR-0008

**`agents-readonly` does not make a reviewer read-only, and must not be described
as if it does.** O6 verified that `reviewer`'s `Bash` grant executes
`echo x > file` with no prompt, and `protect-authority.sh` inspects only a tool's
`file_path`, so a shell redirect bypasses it. Withholding `Write`/`Edit` while
granting `Bash` leaves writing fully available. This rule pins the *declared*
grant against a careless edit — worth having, but ADR-0008's "the two reviewers
are read-only" stays a behavioral constraint until something constrains `Bash`.
If that gap is worth closing, it needs its own task and probably its own ADR.

Frontmatter is also only the *requested* grant: O6 observed the effective runtime
grant to be narrower (`reviewer` → `Read, Bash` from a four-tool list). The
harness may narrow a grant; it will not grant one that was never requested.

Detecting a **stale** served prompt (file edited, old version still in effect)
remains unsolved. A `registration-token` fingerprint each agent echoes was
proposed and deferred — ceremony not yet justified by an observed failure.

## Related

ADR-0008 · O6 · scripts/spec-sync/validate.mjs

## Outcome

Four checks added to `validate.mjs` (+66 lines, still fs-only, no
`child_process`, no rule registry). Every one was verified by *causing* its
failure and restoring, not by asserting it would fire:

| Check | Injected fault | Result |
|---|---|---|
| `entrypoint-symlink` | `rm CLAUDE.md && echo forked > CLAUDE.md` | fails; passes again on restore |
| `agents-readme-exists` | README moved aside | fails with remediation, not an `ENOENT` trace |
| `agents-readonly` | `Edit` added to `reviewer.md` `tools:` | fails, citing ADR-0008 and the allowed set |
| `agents-table-sync` | new `rogue.md` with no table row | fails |
| (malformed fence) | `--` instead of `---` | fails loudly, not skipped |
| (stem mismatch) | `notmatching.md` declaring `name: mismatched` | fails, 2 problems |

This is what O6 and O6b could only document. The chain is now: O6 wrote the
contract, O6b made it discoverable, O6c makes it enforced — the first two are
prose a session can miss, this one fails CI.

The `entrypoint-symlink` check is the highest-value of the four and nearly
shipped broken. O6b originally specified `git ls-files -s`, which reads the
**index** — under a `core.symlinks=false` checkout the index still reads
`120000` while the working tree holds a 9-byte regular file containing the
string `AGENTS.md`. A session auto-loading `CLAUDE.md` would get those nine
bytes instead of the authority order, hard rules, and task protocol, silently,
and the check would have printed `OK`. QA caught it; the implementation asserts
the working tree via `lstatSync`/`readlinkSync`.

Deviation: `agents-readonly` pins the **declared** grant only, and must not be
read as making the reviewers read-only. O6 verified `reviewer` holds `Bash` and
`echo x > file` executes with no prompt. This check stops a careless frontmatter
edit; it does not close the hole. **O6d** carries the wording fix, and whether
to constrain `Bash` at all remains explicitly undecided — it would cost the
reviewers `git diff`, `go test`, and `grep`, which is most of how they gather
evidence.
