# Steloit Cloud

**AI-native infrastructure.** Steloit reads an application, works out the infrastructure it needs,
prices it before anything exists, and then provisions, deploys, configures, monitors, and scales
it — so the people who build software don't have to become infrastructure experts. (Why now: AI
made writing software fast; running it did not get any easier.) Positioning is
owned by [`docs/product/17-brand/messaging.md`](docs/product/17-brand/messaging.md) (`docs/plan/positioning-v2.md`).

This repository holds the whole thing — console, control plane, data plane, and CLI — and is the
**single source of truth**: design authority in `docs/product/`, execution contract in `tasks/`,
domain knowledge in `contexts/`. GitHub Issues and the Project board are generated views of it.

**Agents (and humans): start with [`AGENTS.md`](AGENTS.md)** — the map, commands, authority
order, task protocol, and hard rules live there.

| Path | What |
|---|---|
| `apps/console/` | The console SPA (all 152 frames, canon mode) |
| `services/` | Backend deployables — modular monolith `api` + `cell-agent` (born with their first tasks) |
| `docs/product/` | The design & spec authority (GOV-002, INF-001, frame gallery, derived docs, canon, ADRs) |
| `docs/plan/` | Implementation plan · AI-native workflow · migration record |
| `tasks/` · `contexts/` | Work items (one file each) · reusable Context Packs |
| `scripts/spec-sync/` | Repo → GitHub projection (issues, board) + validation |

```sh
pnpm install
pnpm --filter console dev     # console in canon mode
pnpm build && pnpm test
```
