# Steloit Cloud

The Steloit developer cloud — console, control plane, data plane, and CLI — in one repository
that is the **single source of truth**: design authority in `docs/product/`, execution contract
in `tasks/`, domain knowledge in `contexts/`. GitHub Issues and the Project board are generated
views of this repo.

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
