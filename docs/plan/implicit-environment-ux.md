# Implicit Environment — Complete UX Specification

**Status:** **RATIFIED & FROZEN** · founder-approved 2026-07-18 (ADR-037; ADR-013 extended; glossary updated; ruling S10 filed; `PATCH /envs` in the S-process ledger). The Environment model and its UX are frozen — further changes only from real customer feedback during implementation and design-partner validation. **No architectural change** — the resource model, API contract, and Architecture v1.2 are untouched; this is a presentation-layer specification.
**Governs:** console UX, CLI behavior, API/Terraform documentation patterns, and the E8 integration slices. New n=1 frame states are design-owner additions (ruling S10).

---

## The six laws (everything below derives from these)

1. **Born, not created.** Every project is born with exactly one environment, named `production`, with a real id, at project creation. No project ever exists without an environment; no user ever creates their first environment.
2. **Implicit ≠ anonymous.** The default environment is *named* — `production` — everywhere truth surfaces: API responses, CLI context echoes, audit events, billing internals. The UI hides the *chrome*, never falsifies the *record*. (This is the "no hidden magic that becomes surprising" guarantee: when a user first peeks behind the curtain — API, CLI, an audit row — the name was always there, always stable.)
3. **Implicitness is presentation-layer only.** The model, API, CLI grammar, and Terraform stay fully explicit and truthful. Hiding happens in the console's chrome rules and in *not requiring* env input — never by removing env from the contract.
4. **One reveal rule, applied everywhere at once.** Environment chrome (pill, columns, pickers, pages) is visible iff **`envs ≥ 2` OR previews are enabled/have history** on the project. One boolean, uniform across every page — no page-by-page cleverness. The *previews* clause prevents flicker: a team whose only second environment is an ephemeral PR preview would otherwise see chrome appear/disappear with every PR cycle.
5. **URLs never change shape.** `?env=` is optional (ADR-013 / IA convention: query param, never a path segment); omitted means the default environment. The day staging arrives, no URL breaks, no bookmark dies, nothing renames.
6. **The reveal is lossless and additive.** Because the environment always existed in the model, all history (billing, audit, metrics, deploys) was env-labeled from day one. When chrome appears, nothing is recomputed, renamed, or moved — the user's world gains a dimension without any existing thing changing.

---

## 1 · Project creation

**What happens:** `POST /projects` (or W2/C1, or `steloit project create`, or Terraform) creates the project **and atomically auto-creates one environment named `production`** carrying the project's home region (ADR-004). The create response includes it (additive response field, S-process note). The user sees none of this — they named a project and picked a region; the environment is a birth fact, not a step.

**Why `production` (tradeoffs weighed):**

| Name | Verdict | Why |
|---|---|---|
| **`production`** | ✅ **Chosen** | Honest (it's where the app actually runs); universally understood; matches canon and Vercel's implicit Production; **no rename migration ever** — when staging arrives, production is already correctly named; signals the wedge's seriousness (you deploy real things) |
| `default` | ❌ | Vague; the moment a second env exists, "default vs staging" reads as nonsense; forces a rename (a breaking, confusing migration) exactly at the moment of least user confidence |
| `development` | ❌ | Wrong on day one for anyone who ships; going live then forces adding `production` *and* re-pointing everything — the worst migration |
| Hidden/unnamed | ❌ | Violates law 2. The env must appear in API responses, audit rows, and billing internals; an anonymous env resurfaces later as *surprising* magic ("where did this come from?") |
| Role-flips-on-first-deploy | ❌ | The hidden magic law 2 exists to ban: either the flip is behavioral (governance changed *as a side effect of deploying* — surprising) or non-behavioral (ceremony). Deploys never mutate governance. |
| `main` (Neon's pattern) | ❌ narrowly | Right for Neon (branches of *code/data*), wrong for deploy targets: `main` answers "which branch?" not "which instance is live?"; collides with git's `main` and Postgres branch names; and the promotion/masking/policy language is built on the production word |

**Challenged and re-ratified (2026-07-18):** the founder attacked `production`-from-birth (experimenters, internal tools, months-from-launch projects; signal-dilution risk). It survives on three grounds: **(a) production's strictness is relational and therefore vacuous at n=1** — every guardrail associated with the word is a layer-2 policy that only means something relative to another environment, so there is no premature strictness and no alarm fatigue; the name is a stable *anchor* for future strictness, not strictness itself; **(b) the experimenter never sees the word until reveal** (laws 2+4) — and Vercel's implicit Production across millions of toy projects is the empirical proof this doesn't confuse or scare anyone; **(c) the semantic, now explicit:** *production = the primary instance — the one others branch from and the one policies protect — not "publicly launched."* An internal tool's live instance is its production; an experiment's only instance is trivially its primary. The challenge produced two refinements: the definition above enters the glossary (owner: 18-philosophy — founder stamp), and **environment rename** becomes a supported capability (the honest escape hatch for teams that want a different word — no role machinery): `PATCH /envs/{env}` with rename consequences stated, filed in the S-process backlog alongside the existing env-endpoint gaps.

## 2 · First-time experience

A brand-new user never encounters the word "environment."

**What they see:** org → first project (A8/A9 onboarding unchanged) → project home: the topology (W3), the rail with their services, Create (C1), Deploy, Observe. **What is absent (per law 4, n=1):** the env crumb pill · the Environments page from any nav · env columns in every table · env pickers in every drawer (alert routes, policy scopes, create flows) · `?env=` in any URL · env breakdown rows in Billing (project cost *is* the one env's cost) · `--env` in any CLI example shown in empty states.

**What is quietly present (law 2):** API responses carry `environment`; audit events carry the env; the CLI context echo on state-changing commands reads `acme/sourcegrid · production` — a gentle, truthful teaching surface that costs the user nothing.

**Where "Add environment" lives at n=1:** Project settings → an "Environments" row reading `1 environment (production) · Add environment` — discoverable when sought, invisible when not. Deploy · Previews shows its empty state ("Open a pull request to get a preview of your app") *without* using the word environment.

## 3 · Visibility triggers (exhaustive)

Environment chrome appears when the reveal rule flips true. Every trigger:

1. **Explicit env creation** — console (Project settings → Add environment), CLI (`steloit env create staging`), API (`POST /projects/{project}/envs`), Terraform (`steloit_environment` resource).
2. **First PR preview** — previews enabled + a PR opens → a preview environment is born → reveal (and *stays* revealed via the previews clause of law 4, even after the PR merges and the preview is torn down — its history persists in Deploy · Previews).
3. **Template/duplicate flows that instantiate additional environments.**

**Not triggers (deliberately):** database branches (W5/D5 — a Postgres *capability*, service-scoped; branches ≠ environments); env-scoped policies/secrets/domains (they cannot exist without a second env already existing — creating the env is always the trigger, the configuration follows).

**Does the selector appear automatically? Yes** — at the moment of reveal, everywhere at once, with a **one-time coachmark** on the env pill (pattern: AI5): *"This project now has two environments. You're viewing production — everything you've built lives here, unchanged."* Plus one bell event ("Environment `staging` created" / "First preview environment created for PR #12"). Announced once, never nagged.

## 4 · Existing pages — the uniform rule, applied

One rule (law 4), not per-page judgment. At **n=1 / no previews**: hidden everywhere. At **reveal**: visible everywhere, simultaneously. The table records what "visible" means per family, and the two places env becomes *required*:

| Page family | n=1 (hidden) | Revealed | Required? |
|---|---|---|---|
| Project overview (W3) | No pill; topology is the app | Pill filters topology presence/scale (ADR-013) | — |
| Create canvas (C1/W2) | Creates into production silently | **Target environment stated** in the estimate line | **Required at reveal** (a service must state where it lands — certainty) |
| Service pages (S/D) | No env context shown | Instance's env shown in context line | — |
| Deploy (DP1–3) | Previews empty-state, no env vocabulary | Fully env-aware; promotion flows between envs | Promotion **requires** source→target |
| Observe (O1–6) | All data = the one env | Env filter law applies (F6) | — |
| Environments page (M3 parity matrix) | Not in nav; settings row only | Reachable via env menu ⚙ (ADR-017: manager behind the filter) | — |
| Billing (B1/B2) | Project line only | Env breakdown rows appear; history already attributed (law 6) | — |
| Policies (G7–11) | Scope chips hidden (scope = project) | Env-scope chips appear | — |
| Alerts (U8), Dashboards | Env pickers/filters hidden | Appear | — |

This *revises the frames' implied default* (canon shows 3 envs everywhere, so every frame renders env chrome): the frames stay authoritative for the revealed state; the n=1 state is a **new set of frame variants** (design-owner addition — ruling S10).

## 5 · URL structure

Existing convention holds and answers this cleanly — **no `/environments/{env}/` path segments exist or will** (ADR-013; console convention: `?env=` is always a query param).

- **n=1:** all URLs clean — `/acme/sourcegrid/observe/metrics`. No `?env=` anywhere.
- **Revealed:** same URLs; `?env=staging` appears only when a non-default env is selected. Omitted `?env=` **always means production** — forever, at any n.
- **Deep links:** `?env=staging` works iff staging exists (sharing it implies n≥2 for every viewer — no paradox). A link to a deleted/expired env → the four-state error page with remediation: *"Environment `pr-142` expired · view its history in Deploy · Previews."*
- **The transition breaks zero URLs** (law 5).

## 6 · CLI

The existing grammar (flags → repo link → profile default; context echoed) gains two precise rules:

- **n=1: `--env` is never required and never asked.** Every command targets the only environment. The context echo on state-changing commands includes it truthfully: `acme/sourcegrid · production` (law 2 — the CLI teaches the name without demanding it).
- **Revealed (n≥2):** resolution order = `--env` flag → repo-link mapping (linked repo's production branch → production; PR branches → their preview) → profile default → **then:** read-only commands default to `production` and print the env in the output header; **state-changing/destructive commands never guess** — interactive TTY prompts with the env list; non-TTY fails with exit code 2 and remediation `pass --env`.
- `steloit env list` at n=1 prints the one `production` row — **the CLI never hides truth; hiding is console chrome only** (law 3). `steloit env create staging` is the explicit trigger.

## 7 · API

**The API always exposes environments — no convenience shadow-API, no `/envs/default` alias.** One grammar, no console-only *or* convenience-only capabilities; a magic alias is exactly the hidden magic that surprises later. Two documentation-level conveniences (zero contract change beyond one additive field):

1. `POST /projects` response includes the auto-created environment (additive field — minor S-process note) so quickstarts need no second call.
2. SDK/docs pattern for the simple case: `project.environments[0]` when n=1; every project-scoped list already accepts `?env=` and omitting it already does the right thing.

## 8 · Terraform

**Explicit, mirroring reality — IaC must never contain invisible resources.** `steloit_project` documents that it auto-creates `production` and **exports** it (`steloit_project.app.production_environment_id`) so configs can reference it without declaring it. Additional environments are explicit `steloit_environment` resources. The implicit env is importable if a team wants to manage its attributes (region override, etc.). No count-magic, no auto-generated blocks — a Terraform plan always shows exactly what exists.

## 9 · The 1→2 transition (adding staging)

The moment n flips (or previews first run):

1. **Env pill appears** in the crumb, selected on `production`, with the one-time coachmark (§3). One bell event. Nothing else interrupts.
2. **Chrome appears everywhere at once** (law 4): env columns, scope chips, pickers, the M3 matrix behind the env menu.
3. **Nothing existing changes:** same URLs (prod needs no param), same service ids, same names, same costs. The coachmark says so explicitly.
4. **History is already correct** (law 6): billing rows, audit events, metrics were env-labeled from birth — the env breakdown in B2 shows *historical* production costs immediately, because they were never unlabeled. (This is the payoff of implicit-in-model vs retrofitted: platforms that added environments later had unlabeled history; Steloit's reveal is lossless.)
5. **Reverting:** if the second env is deleted *and* no preview history exists, chrome hides again (pure function of the reveal rule — predictable, user-caused). With preview history, chrome stays (law 4's previews clause) — previews *are* environments and their cost history remains queryable.

## 10 · Ripple (small; no architecture)

- **ADR-013 amendment** (product log, human-only — drafted for stamp): *"Environment is implicit-by-default: every project is born with `production`; env chrome is presentation-layer, revealed by the one rule (envs≥2 ∨ previews); implicit ≠ anonymous — the name is real everywhere truth surfaces. The Vercel-validated refinement of the filter model."*
- **Ruling S10** (design owner): the n=1 frame variants (env chrome absent) + the coachmark microcopy — additions to the gallery/design spec, per the §4 table.
- **openapi.yaml** (S-process, minor): `POST /projects` response documents the auto-created environment.
- **Tasks:** US-3.1/T3.2 note (project creation auto-creates `production` — same work, clarified contract); E8 slices gain the reveal-rule verification (chrome hidden at n=1 in every slice's four-state checks); CLI tasks (T5.x) gain the n=1/n≥2 resolution rules.
- **Context packs:** `frontend-console.md` gains the reveal rule + laws; `api-conventions.md` one line (omitted `?env=` = production, always).
- **No change:** resource model, nine primitives, Architecture v1.2, the enum, billing arithmetic, INF-001, the frames' authority for revealed states.

**Success criteria check:** solo developer — never sees or types "environment," ever, until the day a PR preview or a staging env makes it real (and then learns it in one coachmark). Enterprise — full first-class environments, policies, parity matrix, promotion flows, untouched. No hidden magic — the name `production` was on every API response and audit row from birth. No architectural change — the model never varied; only the chrome does.
