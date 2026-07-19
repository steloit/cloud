# ADR-0008 — The three-stage review pipeline is mandatory for every significant PR

**Status:** Accepted (founder-ratified 2026-07-19)
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
  read-only and independent.
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
