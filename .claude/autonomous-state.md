# Autonomous session state

A **small current-state handoff**, not a diary. **Not authoritative** — the repo,
git, CI and `tasks/` are. Verify before trusting; correct this file when wrong.
Durable lessons live in `contexts/provisioning.md` (mistake bank) and `AGENTS.md`,
never here.

**Updated:** 2026-08-25 · `main` @ `8352675` · **US-3.16 in flight (#348)**

---

## Current objective

Work the `tasks/` ready queue under the AGENTS.md protocol: claim → implement →
`verify:` → reviewer + QA (ADR-0008) → CI → merge. The queue is the plan.

## In flight

**US-3.16 — PR #348**, branch `task/US-3.16-ha-provisions`. `ha: true` was charged
$19/mo and rendered `instances: 1`. Fixed; reviewer running. Next: QA, then
`node scripts/ci/await-checks.mjs 348 --allow-skipped build-sign`, then merge
pinned with `--match-head-commit`.

## Next action

Finish #348. Then the ready queue by priority — `US-3.15` (a shape PATCH reprices
and commits with no estimate), `US-3.3k` (agent has no RBAC), `US-10.7`, `O9`.

## Blocked, and why — do not attempt these

| task | needs |
|---|---|
| **O30** | founder: which side of the cent-seconds/cents seam moves, and the proration divisor. The repo settles proration for *subscription* only; infrastructure metering is unruled. Divisor arithmetic is tabulated in the task. |
| **US-3.3d** | founder: **`performance`'s vCPU/RAM only**. `dev` (1 vCPU / 2 GB) and `standard` (2 / 4) are already ruled by 00-sources create frames — the task's own *proposal* contradicts the spec on `dev`; the frame wins. |
| **O2** | founder: six cost-guardrail values. Module is inert (`billing_account` defaults `""`), so no technical work remains. |

## Do not redo

- **US-3.3c** live-cell verification (GKE cell built, verified, destroyed — 0 clusters).
- **T1.7/T1.8/T1.9** — API list, ordering, and founder-config live state, all merged and enforced in CI.
- **T3.4c** storage ratchet — invariant suite merged.
- **O34** — the local flake is a colima/lima host-port collision (`rapportd` held the port). Diagnosed, guard merged. **Q11 is NOT explained by it** and stays open.
- **O37** — CI check polling. `await-checks.mjs` is now required by AGENTS.md step 5b.

## Active risks

- **`gcloud` credentials expire mid-session.** `scripts/infra/verify-live.sh`
  exits 2 rather than reporting an empty project. Re-auth is interactive.
- **Local integration tests need** `DOCKER_HOST=unix://$HOME/.colima/default/docker.sock`
  and `TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock`, or
  container-backed subtests skip and parent tests fail with "every subtest skipped".
- **No branch protection** (O6g): discipline is the only thing stopping a bad merge.

## Standing decisions

- Merges go through `scripts/ci/await-checks.mjs` and are pinned with
  `gh pr merge --match-head-commit`. Never read a rollup by eye (O37).
- Fault injection happens in a `cp -R` copy, never the working tree.
- GCP work is `steloit-dev` only, scoped per-command with `CLOUDSDK_CORE_PROJECT`.
