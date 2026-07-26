# Steloit — Full-Platform Interactive Design Prototype

A living design prototype of the approved Steloit specifications — the complete platform,
every phase, one clickable system. Not an implementation — a faithful rendering of the whole
product for stakeholders, designers, and engineers. Shell (PDS-SHELL v1.0), service depth,
observability, IAM, billing, secrets, templates, the assistant, and the v2 deployments preview
all share one design system, one mock world, and one navigation.

## Run it

Open **`index.html`** in any modern browser. No build, no dependencies.
(Fonts load from Google Fonts when online; the system falls back gracefully offline.)

Best experience — serve it (enables the smooth environment-switch crossfade; plain
`file://` falls back to full page reloads on env switch, which browsers require):

```
cd steloit-proto && python3 -m http.server 4173
# → http://localhost:4173
```

Built with semantic HTML5, modern CSS (custom properties, Grid, Flexbox), and minimal
vanilla JavaScript. No frameworks, no libraries, no backend. All data is mock data in
`js/data.js`; nothing persists — reload to reset.


## Coverage map — all phases

| Phase | Surface | Where |
|---|---|---|
| **P0 Shell** | Context bar, nav, ⌘K (search/`>`/`?`), notifications, wizard + estimate gate, connect, danger tiers, settings ×4, topology, environments | `index` `overview` `services` `topology` `environments` `wizard` `connect` `settings-*` `error` |
| **P0 PostgreSQL** | Service page: overview, **branches** (copy-on-write, $/day estimate before create), **backups** (snapshots + PITR → restore-to-new-branch), connection (`env pull` first, reveal is audited), logs (live tail + filters), settings (resize shows cost delta; destroy = T2, 72h) | `service.html?id=db-main` |
| **P1 Observability** | Metrics grid, log explorer (level/service/text filters, CLI parity line), alert rules (3 firing, mute 1h, new rule), event stream | `observe.html` (4 tabs) |
| **P1 IAM** | Members (role changes, remove + undo, 2FA), roles matrix, API keys (reveal-once, revoke) | `members.html` (3 tabs) |
| **P2 Valkey** | Keyspaces, hit ratio, memory-pressure degraded state (staging cache), flush = T2 typed | `service.html?id=cache&env=staging` |
| **P2 Storage** | Buckets, top objects, usage | `service.html?id=media` |
| **P2 Queue** | Queues, depth, **DLQ peek + redrive**, purge = T2 | `service.html?id=jobs` |
| **P2 Secrets** | Env-scoped table, reveal-audited, add / rotate / delete | `secrets.html` |
| **P3 Billing** | Plan card, daily spend, per-project splits, invoices, budget alert, compare plans (equal-weight actions) | `billing.html` |
| **P3 Templates** | Four starting points → wizard preselected (a template is a saved answer to the wizard — same estimate gate) | `templates.html` |
| **P3 Assistant** | Ask surface: grounded answers, citation chips → docs anchors, deep links into permissioned surfaces, never executes; E-AI-503 + policy-off demos | `assistant.html` |
| **P4 Deployments (v2 preview)** | Deploy history + rollback, **preview environments with copy-on-write DB branches** — the flagship story | `deployments.html` |
| — Docs | Anchor hub every citation lands on | `docs.html` |

## Where to start (suggested review path — J1, the first five minutes)

1. `index.html?firstrun=1` — the first-run state (SH-03)
2. **Create your first project** → wizard: name → services (try **Describe your app instead** and
   type e.g. *"A SaaS tool with user accounts, file uploads, and a weekly email digest"*) →
   configure (try **View as CLI**) → review the estimate → create → provisioning → **connect**
   (wait ~6s for the first-connection celebration)
3. Then the daily loop (J2): `overview.html` → topology → services → notifications (🔔)

## Keyboard map (§6.11)

| Keys | Action |
|---|---|
| `⌘K` / `Ctrl K` | Search · `>` commands · `?` ask (v0.5 stub) |
| `g` `o` / `g` `s` / `g` `t` / `g` `e` | Overview / Services / Topology / Environments |
| `g` `,` | Project settings |
| `[` `]` | Cycle environments (filter, never navigation) |
| `n` `p` | New project |
| `⌘\` | Collapse navigation |
| `?` | Shortcut sheet · `Esc` closes any layer |
| Topology | `Tab` into map → arrows move → `Enter` opens → `b` lists bindings |

## Details worth noticing

- **The browser tab is a status surface** (EXP-002 J2): the title reads `● ecommerce — Steloit`
  and flips to `◐` when any service in the current environment degrades — try `[` to switch
  to staging, where the cache is degraded.
- **Cost renders neutral** — never green/red (money is fact, not judgment).
- **Gold appears exactly once** in the system: the production marker. Violet exactly once: AI attribution.
- **Healthy topology nodes stay quiet** (faint ring); only non-ready states color the full ring.
- Every danger dialog's typed confirmation **blocks paste** — deliberateness is the point.

## Prototype review controls

The **Prototype** button (bottom-right) is meta-chrome for design review — not product UI.
Global toggles: light theme, reduced motion, offline pill. Per-screen: loading skeletons,
empty states, partial failures, page errors, estimate-unavailable, grace banners, error-page cases.

## Screen coverage (PDS-SHELL §6.1 → files)

| PDS ID | Screen | Where |
|---|---|---|
| SH-01 | App frame, context bar, nav | every page (`js/shell.js`) |
| SH-02 / SH-03 | Projects home / first-run | `index.html` (+ `?firstrun=1`) |
| SH-04 | Project overview | `overview.html` |
| SH-05 | Services list (Data filter ruling §4.4) | `services.html` |
| SH-06 | Topology full canvas | `topology.html` |
| SH-07 / SH-26 | Environments / create dialog | `environments.html` (+ `?create=1`) |
| SH-08–13, SH-19 | Wizard: name → select → describe (AI) → configure → estimate → provisioning → connect | `wizard.html` (single-service mode: `?mode=service`) |
| SH-14–17 | Org / project / env / user settings frames | `settings-*.html` |
| SH-20 | ⌘K palette (search / `>` / `?`) | global overlay |
| SH-21 | Notification inbox | global panel (🔔) |
| SH-22/23/24 | Archive T1 · destroy project T3 + grace · destroy env T2 (production blocked) | project/env settings danger zones |
| SH-25 | Context errors E-CTX-404 / 410 / E-PERM-403 | `error.html?case=…` |
| Service boundary | PDS-001 stub honoring the Shell's edge; exercises the C-12 missing-route redirect | `service.html` |
| SH-19 standalone | Connect reachable outside the wizard (overview button, topology hint) | `connect.html` |
| Observe slot | v0.5 nav destination: Shell-owned event stream + firing alerts; contents PDS-005 | `observe.html` |
| Docs hub | Every docs link lands here (EXP-007 obligations listed) | `docs.html` |

## Spec fidelity notes

- **Environment switch = filter** (G1/§6.4): switching re-renders the same page in place with a
  150ms crossfade and updates the URL; missing routes redirect to Services with toast C-12.
- **No estimate → no create** (G4/E-EST-503): demo via Prototype panel on the wizard review step.
- **Danger tiers** (G3): T1 consequence summary, T2 typed name, T3 typed name + 7-day grace banner;
  production environment destroy is blocked (C-38). Cancel precedes Confirm in tab order.
- **AI** (§12): only AIC-SHELL-1 (proposal with reasons, verbatim evidence, toggles, attribution,
  low-confidence default-off) and the AIC-SHELL-2 ask stub. Nothing is created by the assistant.
- **Celebrations** happen exactly twice (C-21, C-22); reduced-motion renders them static.
- **A11y**: skip link, focus-visible everywhere, combobox switchers, aria-live context
  announcements, alertdialogs for danger, status = icon + word, topology list view.

## Structure

```
steloit-prototype/
  index.html …
  css/tokens.css        design tokens (semantic-first, dark default + light)
  css/base.css          reset + typography
  css/components.css    shared component library
  css/shell.css         shell chrome, topology, palette, wizard
  js/data.js            mock data + helpers
  js/shell.js           chrome engine: context, switchers, ⌘K, inbox, keyboard, dialogs
  js/topology.js        deterministic topology renderer (P-4)
```
