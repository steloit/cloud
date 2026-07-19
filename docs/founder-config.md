# Founder Configuration — source of truth for founder-owned decisions

**Purpose.** One place for every decision only the founder can make: provider
choices, infrastructure identifiers, org identity, pricing/quota knobs, and the
operational secrets an agent must never invent. Agents **consume this directly**
instead of asking; interrupt the founder **only** when a decision appears that is
*not yet recorded here* (a genuinely new founder decision or external dependency).

**Authority.** Where a row cites an **ADR** (`docs/adr/`) or a product decision
(`docs/product/18-philosophy/decisions.md`), that document is authoritative and
this file only *summarizes* it — on any conflict the ADR wins and the conflict is
a finding, never resolved here. Rows without an ADR are operational config owned
by this file. This file is **not** hook-protected `00-sources/`; it is
maintained by agents as values arrive.

**Status legend.** ✅ set · ⏳ `PENDING` (founder will provide — do not invent) ·
❓ `NEEDS FOUNDER INPUT` (no decision on record; interrupt before assuming).

---

## 1 · Organization & identity

| Key | Value | Status | Source |
|---|---|---|---|
| Organization name | **Steloit** (product: *Steloit Cloud*) | ✅ | product spec |
| Primary domain (platform) | `steloit.app` (email `noreply@steloit.app`) | ✅ | T10.4 / mailer config |
| Customer-content domain | a **separate eTLD+1** (never a platform subdomain) | ⏳ P2 | INF-001 A1.4 |
| Branding (logo, palette, marks) | console design system (frames) | ✅ derived | design spec |
| Support email | — | ❓ | not on record |
| License (repo + distributed code) | — | ❓ | no `LICENSE` file exists |

## 2 · Cloud & infrastructure (P1)

GCP-first is ratified (ADR-029 / INF-001 — *hyperscaler primitives, never
resold-managed-services; GCP first*). The concrete identifiers below are the P1
dependency the founder is providing; **fill each as it arrives, then proceed with
T1.0 → T1.2 → T3.4 and remaining P1-gated work** (no re-approval needed).

| Key | Value | Status |
|---|---|---|
| Cloud provider | **GCP** | ✅ (ADR-029) |
| GCP Project ID | — | ⏳ P1 |
| Billing account | — | ⏳ P1 |
| Primary region | **Mumbai / `asia-south1`** (partner-touchable envs born in Mumbai) | ✅ INF-001 A1.7 *(confirm exact region string on P1)* |
| Artifact Registry name | — | ⏳ P1 |
| Required GCP APIs to enable | — | ⏳ P1 |
| Service account / Workload Identity | — | ⏳ P1 |
| GitHub repo variables & secrets | — | ⏳ P1 |
| DNS provider | — | ❓ |
| Substrate | CNPG + ZFS storage pool; object storage **proxied, never self-hosted** | ✅ | architecture.md / INF-001 |

## 3 · Providers (all behind a provider interface — never a platform dependency)

| Concern | Provider | Interface | Status | Source |
|---|---|---|---|---|
| Email | **Resend** (SES / Postmark / SMTP addable) | `mailer.Provider` (Noop when unset) | ✅ | ADR-0009 |
| Payments | **Stripe** (an implementation, not a platform dependency) | `BillingProvider` (to build) | ✅ decided, ⏳ keys | ADR-0010 |
| Container registry | GCP Artifact Registry | — | ⏳ P1 | INF-001 |
| Secrets (customer app) | in-cluster; **object storage proxied** | `secrets.Vault` | ✅ | architecture.md |

**Provider credentials (env, never committed):** `RESEND_API_KEY` (empty ⇒ Noop),
`STRIPE_*` (TBD when payments land). Absent keys degrade to a safe no-op, never a
crash — but a prod boot missing a required key should fail-fast (open finding).

## 4 · Secrets strategy

| Key | Value | Source |
|---|---|---|
| At-rest encryption | envelope: KEK wraps per-secret DEKs; AES-256-GCM; AAD-bound | `secrets.Seal/Open` |
| KEK | `SECRETS_KEK` (base64 32-byte, **required**), `SECRETS_KEK_ID` (`env-v1`) | config.go |
| Cookies | `COOKIE_SECURE=true` (prod) | config.go |
| What is sealed | TOTP secrets, password-reset tokens, webhook signing secrets | MFA / T7.2 / T10.3 |
| Rotation | KEK id versioned; rotation procedure | ❓ (procedure not yet written) |

## 5 · Pricing, quotas & budgets

Pricing is **ratified — do not change without an ADR** (ADR-041). The one pricing
table is `services/api/internal/billing/plans.json`; a second pricing constant
anywhere is a bug.

| Key | Value | Source |
|---|---|---|
| Plans | Free **$0** / Pro **$29** / Business **$99** / Enterprise **custom** | ADR-041 |
| Overage (soft, keep working + bill) | egress **$0.09/GB** · seats **$7** · builds **$0.01/min** · events **$1.20/M** · AI **$2/1k** | plans.json / feature-specs |
| Included seats | Free 3 · Pro 5 · Business 20 | plans.json (F9) |
| Project limit | Free 1 · Pro 3 · Business ∞ · Enterprise ∞ | plans.json |
| Never plan-gated (safety) | TLS · backups · MFA · policies · alerts · dunning · self-deletion | plans.json / F9 |
| Hard spend cap (per-org budget) | a product **feature** — an org sets its own bound (T11.6) | feature-specs §31 |
| Founder org default budget | — | ❓ (if the founder's own org wants a cap) |
| Money representation | integer cents end-to-end | ADR-025 |

## 6 · Engineering governance (founder-ratified operating rules)

| Rule | Value | Source |
|---|---|---|
| Review pipeline | **mandatory 3-agent** (Impl → Architecture → Security/QA → CI → merge) for every significant PR | ADR-0008 |
| Review order | DX first, implementation-simplicity second, architectural-elegance third | ADR-040 |
| Account-level events | keep **org-scoped** for MVP; principal-scoped spine deferred | ADR-0011 |
| Product classifier | the **State Test** (Service / Capability / Binding) | ADR-038 |
| Architecture | **FROZEN** v1.2 (Go/stdlib-http/sqlc/River/REST+SSE; deltas need an ADR) | ADR-0001/0003/0004/0005 |
| AI | governed as an **AI Binding**; no hard in-line spend caps; no LLM proxying; the four-laws assistant has **no auto-apply path, ever** | ADR-0004/005 |

## 7 · Feature flags

No runtime feature-flag system is defined yet. Staged API conformance is by
`oapi-server.cfg.yaml` `include-operation-ids` (an op is served only when its
handler exists). A product-facing flag system is ❓ **NEEDS FOUNDER INPUT** if/when
gated rollouts are wanted.

---

## How agents use this file

1. Before asking the founder anything, **look here first.** If the answer is a
   ✅ row, use it. If it's an ADR-cited row, follow the ADR.
2. A ⏳ `PENDING` value means *the founder will provide it* — do the work that
   doesn't need it, and switch to the gated work the moment the value lands here.
   Never invent a project id, key, domain, or region.
3. A ❓ `NEEDS FOUNDER INPUT` row is the **only** reason to interrupt in this
   space — surface it as a founder decision, then record the answer here.
4. When the founder provides a value, **update the row** (status ✅ + the value),
   in the same PR as the work it unblocks where possible.
5. This file summarizes; it never overrides an ADR or `00-sources/`. A conflict
   is a finding.
