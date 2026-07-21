---
id: O6d
title: ADR-0008 asserts reviewers are read-only; nothing enforces it
epic: EOPS
status: done
phase: V1
priority: medium
sprint: 1
estimate: 0.25ew
deps: [O6]
issue: 0
labels: [DevOps, Docs]
module: Engineering OS
contexts: []
files:
  - docs/adr/0008-mandatory-review-pipeline.md
  - AGENTS.md
  - .claude/agents/README.md
  - .claude/agents/reviewer.md
  - .claude/agents/qa.md
verify:
  - "ADR-0008 states read-only as a behavioral constraint, naming the Bash caveat"
  - "AGENTS.md step 5a carries the same qualification: 'read-only and independent' no longer appears unqualified — grep -q 'read-only and independent' AGENTS.md exits non-zero"
  - "no living file restates read-only as an enforced property"
owner: agent
---

## Goal

ADR-0008 states "the two reviewers are **read-only** and independent" as
unqualified fact. O6 verified it is not enforced at any layer. Amend the ADR so
the claim is true as written, since every downstream restatement inherits it.

## Why

Evidence from O6, verified live rather than reasoned about:

- `reviewer` holds `Bash`. `echo o6-probe > /tmp/o6-bash-write-probe.txt`
  executed with no permission prompt. Withholding `Write`/`Edit` does not make an
  agent read-only.
- `protect-authority.sh` inspects only a tool's `file_path`/`notebook_path`, so a
  shell redirect never reaches the one hook that could stop it.
- The effective grant is also *narrower* than the frontmatter requests
  (`Read, Bash` from a four-tool list), so the frontmatter is not the authority
  on what an agent can do either.

Read-only is a **behavioral constraint the reviewers observe**, not a technical
control. That is not a defect in itself — it is a defect that the ADR says
otherwise, because `.claude/agents/README.md` and O6c already carry the
qualification and a reader comparing them finds the ADR contradicting both. The
ADR is the source; fixing it there is the single edit that makes every
restatement true, including AGENTS.md step 5a, which O6b deliberately left alone
rather than qualifying a policy claim from a steering file.

## Acceptance criteria

- [x] ADR-0008's reviewer bullet states read-only as a behavioral constraint and
  names the `Bash` caveat in one clause — not a section, and not a hedge that
  invites reviewers to start writing.
- [x] The ADR says what a reader should therefore *not* do: cite an absent
  `Write` tool as evidence of a sandbox.
- [x] **AGENTS.md step 5a is amended in the same PR.** It still reads "The two
  reviewers are read-only and independent" — the identical unqualified claim.
  O6b deliberately left it rather than qualifying an ADR-owned policy from a
  steering file; amending the ADR is exactly what licenses the steering-file
  edit, so the two belong in one change. Editing the ADR alone would leave the
  repo contradicting itself in the more-read file.
- [x] No change to review POLICY. Reviewers must still not write; the amendment
  changes what the repo *claims about enforcement*, not what is required.
- [x] `.claude/agents/README.md` and `tasks/eops/O6c.md` are checked for
  consistency with the new wording (they already carry it; confirm no drift).

## Open question — do not decide silently

Whether to *make* the constraint technical is **out of scope and genuinely
undecided**. It is a choice among three options, not a binary — see the Outcome,
which corrects this section's original framing and its evidence. Denying `Bash`
costs the reviewers `git diff`/`go test`/`grep`; a `PreToolUse` hook inspecting
`.tool_input.command` constrains writes without that cost; a git pre-commit hook
catches the outcome instead of the call. **O6f** carries all three, alongside a
strictly more serious instance of the same root cause. If pursued it needs its
own ADR, with the DX cost measured rather than assumed (ADR-040: developer
experience first). This task only makes the written claim honest.

## Related

ADR-0008 · O6 (evidence) · O6c (pins declared grants, cannot close this)

## Outcome

Amended ADR-0008's reviewer bullet from "the two reviewers are read-only and
independent" to "independent and **must not write**", stating plainly that this
is a behavioral constraint rather than a technical control, with the evidence
inline: they hold `Bash`, `echo x > file` executes with no prompt, and
`protect-authority` inspects only a tool's `file_path` so a shell redirect never
reaches it. It names what `validate.mjs` does pin (`agents-readonly`, the
declared grant) and states the rule that motivated the whole amendment — **an
absent `Write` tool is not a sandbox and must never be cited as one.**

AGENTS.md step 5a carries the same qualification in one clause, which is what
O6b deliberately deferred rather than qualifying an ADR-owned claim from a
steering file. Amending the source is what licensed the steering-file edit.

A third restatement surfaced during verification and would have outlived the
fix: `.claude/agents/README.md` *quoted* the old ADR sentence in order to
correct it. With the ADR amended the quotation became a misquote, so the repo
would have contradicted itself in the file most likely to be read first. The
repo-wide scan is now clean — no living file restates read-only as enforced.

**The first draft of this Outcome overstated its own evidence, and QA's review
corrected it. The corrected version is recorded here because a future task will
otherwise cite the deferral as settled.** Three claims did not survive:

- *"Both reviewers used `Bash` substantively in every round"* — over-stated.
  O6b records exactly one clear instance (QA's `rm`/`echo >`/`git add`
  reproduction); O6c's round 2–3 findings were code-*reading* findings.
- *"O6c's bypass was found by a reviewer running injections"* — false. O6c
  attributes the injections to the implementer; round 3's bypass was a
  consistency finding against `lib.mjs`'s recursive walk.
- *"a guarantee against a failure never observed"* — **contradicted.** The one
  clearly-recorded reviewer `Bash` use *was a reviewer writing to the primary
  checkout*. The failure has been observed, once, inside this task family.

Counter-evidence the draft ignored: `docs/plan/kernel-workflow-review.md:10-12`
records a **shell-free** reviewer (`Read`/`Grep`/`Glob` only) catching real
blockers in production — a datapoint directly against "denying `Bash` costs most
of how they gather evidence."

Deliberately NOT decided, on corrected evidence: whether to make the constraint
technical. The draft framed this as a binary (deny `Bash` or accept the gap),
which was the rationalization. The ADR text this task added names the actual
defect — the hook inspects only `file_path` — which points at a **cheaper third
option the draft never surfaced: a `PreToolUse` hook that also inspects
`.tool_input.command`**, constraining writes without denying `Bash`. That is
filed as **O6f**, together with a strictly more serious instance of the same
root cause. Deferral is now a choice among three named options rather than a
foregone conclusion; the DX cost of denying `Bash` is real but thinner than
claimed, and the observed-failure count is one, not zero.
