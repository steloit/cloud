# Context Packs

Reusable cross-cutting domain knowledge. A task lists the packs it needs in frontmatter
(`contexts: [...]`); load those before its Read-first list. Rules (AGENTS.md + workflow Part II A2):

- Knowledge that maps to **one directory** → that directory's `AGENTS.md`, not a pack.
- A **procedure** → a skill, not a pack.
- Packs are ≤160 lines each, ≤12 total (CI-enforced — `scripts/spec-sync/validate.mjs` names the
  number once, so this line and the error message cannot drift apart again; O12). Each carries the domain's **mistake bank** —
  when an agent repeats a mistake in a domain, it gets one line here, not in every task.

| Pack | Domain |
|---|---|
| `api-conventions` | REST contract rules: problem+json, pagination, ids, money, contract-first codegen |
| `provisioning` | Reconciler, cells, estimates, drivers, service lifecycle |
| `rbac` | Two-layer authorization: matrix ceiling + policy evaluation |
| `events-spine` | The one event pipeline: audit, SSE, notifications, deploy markers |
| `billing` | Metering → usage → quotas → plans → invoices; the one-arithmetic law |
| `canon-testing` | Canon world, arithmetic invariants, 10 QA scenarios, fault injection |
| `frontend-console` | Console integration: slices, four-state grammar, canon mode |
| `ai-plane` | The four laws as architecture: tools, resolver, proposals, disable policy |
