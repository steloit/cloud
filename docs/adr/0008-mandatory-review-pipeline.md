# ADR-0008 — The three-stage review pipeline is mandatory for every significant PR

**Status:** Accepted (founder-ratified 2026-07-19) · **Amended 2026-07-21 (O6d, agent):** the
reviewer bullet described the two reviewers as "read-only" as though enforced; corrected to a
behavioral constraint with the evidence. **Descriptive, not decisional** — no decision in this
ADR changed, and the reviewers must still not write.
**Deciders:** Founder
**Relates to:** ADR-0002 (AI-native engineering OS), the Phase-2 support agents (`.claude/agents/`)

## Context

Autonomous implementation ships PRs faster than a single reviewer can guard.
Across T7.1, Q2, Q3, and Q4 a multi-agent review pass caught defects that
*neither local testing nor CI detected* — a TOTP replay window, a delegation-
ceiling guard that was only Docker-gated (masked green), a "drift fails CI"
gate that never actually ran the generator, and contract coverage that held for
only 5 of 18 response shapes. Each was a correctness or security hole that would
have reached `main` under CI-alone.

## Decision

**Every significant PR passes through a fixed pipeline before merge:**

```
Implementation Agent      (the ONLY agent that writes code)
        │
        ▼
Architecture Reviewer     (architecture, quality, regressions, ADR compliance)
        │
        ▼
Security / QA Reviewer     (attempts to break it: edge cases, adversarial inputs,
        │                   security flaws, coverage gaps)
        ▼
CI Verification           (build, vet, tests, drift gates)
        │
        ▼
Merge
```

- The **Implementation Agent** is the sole writer; the two reviewers are
  independent and **must not write**. That is a behavioral constraint they
  observe, not a technical control: they hold `Bash`, which writes files
  (verified — `echo x > file` executes with no prompt), and the `protect-authority`
  hook inspects only a tool's `file_path`, so a shell redirect never reaches it.
  Their frontmatter withholds `Write`/`Edit`, and `validate.mjs` pins that
  (`agents-readonly`), but **an absent `Write` tool is not a sandbox and must
  never be cited as one.** A review that edited files is a process violation, and
  there is currently **no reliable detector** for it: reading the PR diff catches
  a review that left changes behind, but not a mutate-then-restore, which is the
  shape actually observed (a reviewer reproducing a fault in the working tree).
  Reviewers are therefore instructed to work in a temp copy and to state plainly
  when they have mutated the tree.
- **Reviewer identity is fixed and repo-native.** The two reviewers are exactly
  the Phase-2 support agents in this repo: the **Architecture Reviewer** is
  `.claude/agents/reviewer.md` (`subagent_type: "reviewer"`) and the
  **Security/QA Reviewer** is `.claude/agents/qa.md` (`subagent_type: "qa"`).
  **No external, plugin, or machine-level reviewer may stand in for either.**
  In particular, the Kernel/`kernel:*` agents were evaluated and **explicitly
  rejected for this repo** (`docs/plan/kernel-workflow-review.md`: "Do not use
  Kernel for steloit/cloud… the native Engineering OS is the standard workflow").
  Invoking an external reviewer is the same class of process violation as
  skipping a reviewer, and any review produced by one does not count.
- Both reviewers run **before** the PR merges, on the branch diff. Findings are
  triaged; blocking findings are fixed by the Implementation Agent and
  re-verified. Non-blocking findings are recorded.
- Reviewers follow the standing **review order (ADR-040):** developer experience
  first, implementation simplicity second, architectural elegance third.
- CI is the final gate, never the first line of defense — it catches what the
  reviewers' structural analysis is not designed to (integration, drift).
- Lessons that outlive the PR go to a living file (the domain pack's mistake
  bank, the nearest AGENTS.md, or an ADR) — a lesson that changed no living file
  didn't happen.

**"Significant"** = any PR that changes behavior, adds a feature, touches
security/authorization/money/contracts, or modifies CI. Pure typo/comment/doc
edits are exempt.

## Consequences

- Slower per-PR, materially higher merged quality; the review cost has already
  paid for itself in defects caught pre-`main`.
- The pipeline is the standard engineering process, not a per-task option.
- Skipping a reviewer on a significant PR is a process violation, recorded as a
  finding.
