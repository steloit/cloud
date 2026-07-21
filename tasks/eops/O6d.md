---
id: O6d
title: ADR-0008 asserts reviewers are read-only; nothing enforces it
epic: EOPS
status: ready
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
verify:
  - "ADR-0008 states read-only as a behavioral constraint, naming the Bash caveat"
  - "AGENTS.md step 5a carries the same qualification — grep -c 'read-only and independent' AGENTS.md returns 0"
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

- [ ] ADR-0008's reviewer bullet states read-only as a behavioral constraint and
  names the `Bash` caveat in one clause — not a section, and not a hedge that
  invites reviewers to start writing.
- [ ] The ADR says what a reader should therefore *not* do: cite an absent
  `Write` tool as evidence of a sandbox.
- [ ] **AGENTS.md step 5a is amended in the same PR.** It still reads "The two
  reviewers are read-only and independent" — the identical unqualified claim.
  O6b deliberately left it rather than qualifying an ADR-owned policy from a
  steering file; amending the ADR is exactly what licenses the steering-file
  edit, so the two belong in one change. Editing the ADR alone would leave the
  repo contradicting itself in the more-read file.
- [ ] No change to review POLICY. Reviewers must still not write; the amendment
  changes what the repo *claims about enforcement*, not what is required.
- [ ] `.claude/agents/README.md` and `tasks/eops/O6c.md` are checked for
  consistency with the new wording (they already carry it; confirm no drift).

## Open question — do not decide silently

Whether to *make* the constraint technical (deny `Bash` to `reviewer`/`qa`, or
constrain it) is **out of scope and genuinely undecided**. It trades the
reviewers' ability to run `git diff`, `go test`, and `grep` — which is most of
how they gather evidence — against a guarantee. Both reviewers used `Bash`
substantively in every O6/O6b round. If pursued it needs its own task and its
own ADR, with the DX cost measured rather than assumed (ADR-040 review order:
developer experience first). This task only makes the written claim honest.

## Related

ADR-0008 · O6 (evidence) · O6c (pins declared grants, cannot close this)
