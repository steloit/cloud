---
id: US-5.1
title: steloit init / create with the estimate-first safety grammar
epic: E5
status: done
phase: MVP
priority: high
sprint: 4
estimate: 0.5ew
deps: [T5.1, T5.2]
issue: 81
labels: [CLI]
module: M10 Clients
contexts: [api-conventions]
files: []
verify:
  - cd apps/cli && go test ./...
owner: agent
---

## Goal

steloit init / create with the estimate-first safety grammar

## Summary

20-clients safety grammar: `--yes` accepts a *shown* estimate (no skip-seeing flag); context org/project/env echoed on every state-changing command. **Implicit-env rules (ADR-037):** at n=1, `--env` is never required or asked (echo still truthfully includes `· production`); at n≥2, resolution = flag → repo-link → profile default, then read-only commands default to production (env printed in header) and **state-changing commands never guess** — TTY prompts with the env list, non-TTY exits 2 with remediation `pass --env`.

## Acceptance criteria

- [ ] create without seeing an estimate is impossible.

## Acceptance criteria

- [x] Estimate-first safety grammar: `db create` always shows the lines + total before
      the prompt; `--yes` accepts a SHOWN estimate (no skip-seeing flag exists); the
      worn context is echoed on state-changing commands (T5.3 tests).
- [x] Implicit-env rules (ADR-037): n=1 never asks; n≥2 reads default to production;
      n≥2 MUTATIONS never guess — TTY prompts with the env list, non-TTY exits 2 with
      `pass --env` (implemented in this closeout; `env_rules_test.go` covers all four
      paths: read-default, non-TTY refusal, TTY prompt+choice, explicit flag).

## Outcome

The verification pass found the never-guess rule unimplemented (resolveEnvID silently
defaulted to production for mutations at n≥2) — closed here with a `mutating` parameter
and TTY detection behind the test seam.
