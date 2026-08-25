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
❓ `NEEDS FOUNDER INPUT` (no decision on record; interrupt before assuming) ·
⏳ **absent** (T1.9 — *decided and declared in code, but not applied to the live
project*; waits on an apply, **not** on the founder, so it is not an interrupt) ·
a split cell such as ✅ *in code* · ⏳ *not applied* means the two halves of one row
have different answers, and both are stated rather than averaged.

---

## 1 · Organization & identity

| Key | Value | Status | Source |
|---|---|---|---|
| Organization name | **Steloit** (product: *Steloit Cloud*) | ✅ | product spec |
| Primary domain (platform) | **`steloit.com`** (DNS at GoDaddy) | ✅ | founder 2026-07-19 |
| Customer-content domain | a **separate eTLD+1** (never a platform subdomain) | ⏳ P2 | INF-001 A1.4 |
| Branding (logo, palette, marks) | console design system (frames) | ✅ derived | design spec |
| Support email | **`support@steloit.com`** | ✅ | founder 2026-07-19 |
| License (repo + distributed code) | **Apache License 2.0** (`/LICENSE`) | ✅ | founder 2026-07-19 |

## 2 · Cloud & infrastructure (P1 — landed 2026-07-19)

GCP-first is ratified (ADR-029 / INF-001). The founder authorized autonomous
gcloud discovery + provisioning (least-privilege IAM, stay in the free trial).
**Live state last verified 2026-08-25** (T1.9), read-only against `steloit-dev`.
Rows carry ✅ **created** only where that check found the resource; a row the check
found ABSENT says so and keeps its history rather than being quietly deleted.
Re-run the checks in `tasks/e1-substrate/T1.9.md` before trusting a ✅ older than
this line — the previous revision asserted **eight resources** as created that do
not exist (artifacts bucket, `images` AR repo, two WAL buckets, the `github-ci`
pool, the `github-oidc` provider, and both CI service accounts), across four rows,
and nothing detected it for weeks.

| Key | Value | Status |
|---|---|---|
| Cloud provider | **GCP** | ✅ (ADR-029) |
| GCP account | `hashir@humanetechnologies.in` | ✅ |
| GCP organization | `humanetechnologies.in` / `665960555130` | ✅ discovered |
| Billing account | `016006-61AFB9-0DD7E7` ("My Billing Account", **Free Trial $300**) | ✅ discovered |
| **Dev project** | **`steloit-dev`** (# `850447601666`, us-central1) — billing linked | ✅ **created** |
| Cell-0 project (partner) | `steloit-cell0` (asia-south1) — not yet created (cost: create at Cell-1) | ⏳ later |
| Primary region (dev) | `us-central1` (dev default); partner cell `asia-south1` (A1.7) | ✅ |
| Terraform state | `gs://steloit-dev-tfstate` (prefix `dev`) — versioned, migrated | ✅ **created** · verified 2026-08-25 |
| Artifact Registry | `steloit-dev-artifacts` bucket + AR `images` repo (keyless cosign, T1.3) — **DECLARED IN TERRAFORM, NOT PRESENT.** `gcloud storage buckets list` returns only `steloit-dev-tfstate`; `gcloud artifacts repositories list` returns nothing. *Was ✅ **created** here until 2026-08-25; that claim was never true of the live project, not a resource later destroyed — Terraform state holds only the state bucket.* | ⏳ **absent** · verified 2026-08-25 · T1.9 |
| WAL/backup buckets | `steloit-dev-wal-customer`, `steloit-dev-wal-control` (separate, INF-001 #10) — **DECLARED, NOT PRESENT.** *Was ✅ **created** until 2026-08-25. The separation requirement is unchanged and still binding; only the claim that the buckets exist was wrong.* | ⏳ **absent** · verified 2026-08-25 · T1.9 |
| KMS | key ring **`cell-dev-core`** + crypto key **`secrets-envelope`** (envelope KEK substrate). *This row previously said `core` / `secrets`; both are the RENDERED names — `project-base` declares `"${var.cell_id}-core"` and `"secrets-envelope"`.* **Both exist and are NOT in Terraform's state** — see the bootstrap note below before re-applying. | ✅ **created** · verified 2026-08-25 (ring **and** key) |
| Workload Identity Federation | pool `github-ci` + provider `github-oidc`; SAs `ci-terraform-plan`, `ci-image-push` (least-priv) — **zero static keys (D5)** — **DECLARED, NOT PRESENT.** The only pool is GKE's managed `steloit-dev.svc.id.goog`; `gcloud iam service-accounts list` shows only the default compute SA. *Was ✅ **created** until 2026-08-25. **D5 remains the standing rule** — this row records that the mechanism is not yet built, never that keyless CI was abandoned. It is also why T1.7's missing API went unnoticed: `ci_plan_viewer`, its first consumer, has never been applied.* | ⏳ **absent** · verified 2026-08-25 · T1.9 |
| APIs enabled | **cloudresourcemanager**, compute, container, storage, secretmanager, cloudkms, iam(+creds), sts, dns, artifactregistry, cloudscheduler, billingbudgets, monitoring, logging — *15 with `iam` and `iamcredentials` counted separately*. This enumeration is kept per T1.7 option (a) ("fifteen in `local.services`, fifteen in the founder-config row"); it must equal `infra/modules/project-base/main.tf`'s `local.services`, and **the two agreeing is not proof** — they agreed at 14 and were both short one. `scripts/infra/required-apis.mjs` proves the MODULE's list covers every API `infra/` uses, in CI; nothing yet proves this row equals the module (see T1.10). *Live `steloit-dev` reported **42** APIs enabled on 2026-08-25 (`gcloud services list --enabled | wc -l`) — a superset, since GCP enables many by default.* | ✅ **enabled** · verified 2026-08-25 |
| — **15th API: `cloudresourcemanager.googleapis.com`** | **RESOLVED BY THE REPOSITORY, not by a preference** — see `tasks/e1-substrate/T1.7.md`. This was raised as `❓ NEEDS FOUNDER INPUT` on the reading that the service list is an owner-level choice. Investigating the intended OWNER settled it instead: `infra/README.md`'s bootstrap contract is **Terraform-first** — on a clean project step 2 is `apply -target=module.project_base`, with **no** manual `gcloud services enable` — and `project-base` holds the only `google_project_service` in the entire tree. So its list must be sufficient for its own resources to apply, and it was not: `google_project_iam_member.ci_plan_viewer` lives in that module and is served by Cloud Resource Manager (measured on the wire). **The module could not complete its own documented bootstrap step.** That is an internal inconsistency, not a policy question, so it was fixed rather than escalated. Enforced by `scripts/infra/required-apis.mjs` in CI's infra job, which refuses an unclassified `google_*` resource rather than skipping it. *Note: the procedure recorded below inserted a manual `gcloud services enable` of "the 14 APIs", which is what masked this. **T1.9 reconciled both** — the row above now lists 15 and the procedure paragraph is marked historical.* | **evidence 2026-08-25** · T1.7 |
| GKE cluster / compute | **none yet** (~$0 running) — stood up by the T1.0 spike / T1.2 | ⏳ next |
| DNS provider | **GoDaddy** (`steloit.com`) — records generated for founder to apply | ✅ |
| GitHub repo WIF binding | repo attribute condition **is set in `project-base`** (code, verified); the pool/provider it binds are **not applied** — see the row above. *Was plain ✅ / "pool/provider created; repo attribute condition set in `project-base`" until 2026-08-25.* | ✅ *in code* · ⏳ *not applied* · verified 2026-08-25 |
| Substrate | CNPG on **GKE PD-CSI** at dev/alpha (**ADR-0007 ratified 2026-07-19**; Arch v1.3 + INF-001 A6); ZFS-LocalPV re-scoped to a Cell-1 density option; object storage **proxied** | ✅ | ADR-0007 |

**Bootstrap procedure — `infra/README.md` §Bootstrap is the contract; follow that,
not this paragraph.** It is Terraform-first: `init -backend=false` → `apply
-target=module.project_base` → re-init onto the backend → full `apply`. Terraform
auth uses the founder's gcloud user token (`GOOGLE_OAUTH_ACCESS_TOKEN`) — no
ADC/service-account key needed.

*Historical, and kept because it explains a defect (T1.7): the procedure actually
**used** for `steloit-dev` inserted a manual `gcloud services enable` of "the 14
APIs" before the Terraform step. That step is not in the contract, and it masked
`project-base` being unable to complete its own bootstrap — the module declares a
project IAM binding whose API its own list omitted. Do not reintroduce it: a manual
enable makes the contract untestable and hides exactly this class of gap.*

⚠️ **THE STATE IS INCOMPLETE, AND A RE-APPLY WILL FAIL UNTIL IT IS IMPORTED.**
Established 2026-08-25, live:

- `gs://steloit-dev-tfstate` contains **exactly one** state file, `dev/default.tfstate`
  (serial 47, lineage `fe16788f-…`), holding **one** resource:
  `module.project_base.google_storage_bucket.state`.
- The KMS ring and key **exist**, created `2026-07-19T14:13:58Z` and `…14:14:00Z` —
  **two seconds apart**, which is an apply (the key depends on the ring), not console
  clicks. Their names match what `project-base` renders.
- So a Terraform apply on 2026-07-19 created them under the **local** state the
  bootstrap uses, and the migration to GCS carried only the state bucket.

*This refutes the earlier guess that the apply stopped after the state bucket — it
reached KMS. What is missing is state, not resources.*

**Consequence for anyone following the bootstrap above:** step 2
(`apply -target=module.project_base`) will fail with `already exists` on
`google_kms_key_ring.core`. Import first:

    terraform -chdir=infra/envs/dev import module.project_base.google_kms_key_ring.core \
      projects/steloit-dev/locations/us-central1/keyRings/cell-dev-core
    terraform -chdir=infra/envs/dev import module.project_base.google_kms_crypto_key.secrets \
      projects/steloit-dev/locations/us-central1/keyRings/cell-dev-core/cryptoKeys/secrets-envelope

Nothing else needs importing: the ⏳ **absent** rows were re-verified live on
2026-08-25 and genuinely do not exist.

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
| **Span proration: what a running service accrues** — *resolved by ADR-0018* | **The divisor question is DELETED, not answered.** Under allocated-resource pricing an invoice line is `unit_seconds × rate_per_unit_HOUR ÷ 3600`. An hour is exactly 3600 seconds and carries no convention, so no month enters any invoice arithmetic — there is nothing to choose between 730, 744 and the actual period, and February legitimately costs less than January. Decision (a) resolves as recommended: **the writer moves** — `rate_cents` becomes a unit price rather than a monthly-rate snapshot, so the cent-seconds/cents ambiguity has no way to exist. *The repo has no authoritative monthly-hour convention to adopt: `usage_http.go`'s 30-day constant is the artifact O30 flagged, and `money.secondsInLongestMonth` is a deliberately-unexported overflow bound, not a price.* The unit conversion lands with the rate table and is blocked on the unit-rate decision below. | **ADR-0018** · founder 2026-08-25 |
| Money representation | integer cents end-to-end | ADR-025 |
| **Postgres `included_gb` semantics** — *ruled* | storage the customer **GETS**, not merely "not charged extra". An **unset** `storage_gb` means the size's `included_gb` (**50 GB** for `standard`, 0 for `dev`). Verbatim: *"the customer-facing pricing definition takes precedence… Do not interpret unset as 0."* | **founder 2026-08-23** |
| — *derived from it* | a value **below** `included_gb` resolves up to it, so 0/30/50 on a standard are one contract. Follows from "included is what you get" and keeps "the same configuration spelled with its defaults written out must not be refused", which the estimate gate depends on. The price is unchanged: only `storage_gb − included_gb` above zero is billed. | T3.4c (derived, not ruled) |
| — *`performance`'s 50* | analogy with `standard`, **not** derived: `pricing.json`'s `$note` names included_gb on standard/performance as one S9 finding, but the derivation it states covers standard only (db-main $58 @ 50 GB) and canon prices no performance service. | T3.4c · S9 finding |
| **HA is priced as a flat uplift, and every comparable is a multiplier** | ❓ **NEEDS FOUNDER INPUT** — see `tasks/e3-provisioning/US-3.19.md`. `ha_cents: 1900` is flat, so the uplift is **100% on `dev`, 32.8% on `standard`, 17.0% on `performance`**. Every published comparable is 2x-3x: Cloud SQL "an HA-configured instance costs twice as much… includes CPU, RAM, and storage", Crunchy "doubles the cluster price", PlanetScale's 3-node HA is ~3x. **This became a live margin leak on 2026-08-25**, when US-3.16 made `ha: true` actually provision a second instance — before that it rendered one instance and cost us nothing, so the mis-pricing was invisible. A `performance` + `ha` customer now costs a full second cluster and pays $19 for it. **ADR-0018 is accepted, so this row is now waiting on the unit rates only.** The engineering analysis is done (US-3.19): HA provisions **2 instances**, so vCPU **×2**, memory **×2**, and storage **×2** — CNPG creates **one PVC per instance**, so `instances: 2` + `50Gi` provisions **100Gi**. But **backups do NOT double**: WAL archives as a single stream to the object store, so retained bytes are ×1. HA is therefore *not* a uniform multiplier — it doubles the ALLOCATED meters and leaves the CONSUMED one alone, which falls straight out of ADR-0018's split and lands independently on the same semantics Cloud SQL publishes. `ha_cents` is **deleted, not re-rated**: price multiplies the resource meters by the instance count, so HA needs no branch and a future third replica prices correctly without touching pricing code. *Today's $19 is not used as an anchor — it is already established as wrong (17% uplift on `performance`), and reverse-engineering a rate from it would carry that error forward.* | ❓ **needs the unit rates** |
| **Storage is a RATCHET** (technical, not a pricing rule) | `storage_gb` never decreases. A PersistentVolumeClaim cannot shrink — Kubernetes supports expansion only and rejects a request below `.status.capacity` — so a PATCH that lowers it is **refused with a 422 + remediation**. *(This row used to add "as Cloud SQL does". Qualified 2026-08-25: Google's own instance-settings page says both "you cannot decrease it" AND "you can manually shrink your storage capacity" — it contradicts itself, so it is a weak citation. The PVC constraint is the sound one and is sufficient on its own.)* Nobody's bill moves; without the refusal the bill drops for storage the cluster still carries AND the driver renders a PVC the CSI driver rejects, stranding the row. | T3.4c · [k8s.io/…/persistent-volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) |
| **What a SIZE downgrade costs while storage is retained** — *the arithmetic is ruled* | **The effective configuration is what is priced.** Storage is a ratchet: a PostgreSQL volume may grow but must not physically shrink, so a downgrade request must not attempt to shrink the existing volume, and the recorded `storage_gb` stays at the already-provisioned figure. Pricing reflects that resulting configuration — it is **not** re-derived from the smaller requested size, because doing so would price a shrink that cannot happen. So `standard`(50 GB) → `dev` prices `1900 + 50×50¢ = 4400¢`: dev's base plus the 50 GB the cluster still carries. Reconciliation must **converge** on that configuration rather than repeatedly attempting an impossible downgrade. (Priced storage is the RECORDED `storage_gb`, not the volume on disk: the driver floors every PVC at `minVolumeGB` = 10Gi, so a `dev` service records and is billed 0 GB while physically holding 10Gi. That floor is an unbilled implementation detail and this row does not make it billable.) Verbatim: *"Storage is a ratchet. A PostgreSQL volume may grow but must not physically shrink… Pricing must reflect the resulting effective configuration."* **Scope of this ruling — read before citing it.** It settles the ARITHMETIC only. Option (a) as this file previously recorded it was *"keep the arithmetic **and surface a breakdown + consent before the PATCH commits**"*; the second half was **not** ruled and is **not** built — `PATCH /services/{id}` still reprices and commits in one call with no estimate and no breakdown (verified: the handler takes no `estimate_id`; only the hard spend cap stands between the request and the row). That gap now has an owner, **US-3.15**; it is open, not declined. (b) refuse-the-downgrade and (c) decouple-storage-from-base are declined. And this section is ADR-ratified pricing (ADR-041, in `docs/product/18-philosophy/decisions.md`): recording a founder ruling here is the mechanism available to an implementer, but promoting it into the ADR log is a founder action — that file changes by human decision only. | **founder 2026-08-25** (arithmetic) · US-3.15 (consent, open) |
| **What a postgres SIZE is, in vCPU and RAM** | — | ❓ **NEEDS FOUNDER INPUT** — blocks US-3.3d. The catalog prices `dev` $19 / `standard` $58 / `performance` $112 and says **nothing** about compute: `shapeSchema["postgres"]` is `{size, storage_gb, ha, connections, pgmq, version}` and `pricing.json` carries only `base_cents` + `included_gb` (the one `memory_cents_per_gb` there is valkey's). So the rendered Cluster declares NO `resources:` — a customer buying `performance` gets the same pod as one buying `dev`, and the plan's cpu/memory ceiling binds as a pod-count proxy rather than as compute. **Proposal** (market-anchored, see US-3.3d): `dev` 0.5 vCPU / 1 GiB · `standard` 2 / 4 GiB · `performance` 4 / 8 GiB — against DigitalOcean's $15 = 1/1 GiB, $61 = 2/4 GiB, ~$120 = 8 GiB. Two consequences to see before ruling: `standard` (2 vCPU) does **not** fit a `free` environment's 1 vCPU envelope at all, and `performance` + `ha` is 12 vCPU / 24 GiB — exactly `business`'s ceiling. Applied as **Guaranteed QoS** (requests == limits), which is CNPG's own recommendation and what keeps the postmaster alive under OOM. |
| **Per-environment resource envelope** — *ruled* | `free` **1 vCPU / 2 GiB / 10 GiB** · `pro` **8 / 16 GiB / 100 GiB** · `business` **12 / 24 GiB / 200 GiB** · `enterprise` **16 / 32 GiB / 250 GiB**. Per ENVIRONMENT, against the four authoritative plan ids (`orgs.plan`'s CHECK constraint — there is no `standard` tier). | **founder 2026-08-23** · US-3.3e |
| — *derived from it* | machine-readable under each plan's `quota` in `plans.json`, which is the ONE definition; the cell-agent holds no copy and is shipped resolved values. Rendered as a **ResourceQuota on `requests.cpu`/`requests.memory`/`requests.storage`** paired with a LimitRange supplying `defaultRequest` only. Requests rather than limits because a `limits.*` quota forces every pod to declare a limit, and the only way to supply one for the CNPG Cluster (which declares no resources until **US-3.3d**) is a LimitRange `default` — which becomes its hard cap. Enforced by the API server's **ResourceQuota admission controller**, which is in Kubernetes' default-enabled plugin list and needs no add-on. | US-3.3e (derived, not ruled) |
| — *what is NOT yet bounded* | `cluster.yaml.tmpl` declares no `resources:`, so every CNPG container is admitted at the LimitRange's `defaultRequest`; until **US-3.3d** makes the Cluster declare its own, the cpu and memory ceilings bind as a **pod-count proxy** rather than as compute the customer paid for. **And `requests.storage` is NOT rendered at all** — see the row below. So today the envelope bounds pod count and nothing else. | US-3.3e (disclosed) |
| — ⚠️ *what your storage number MEANS, once it is enforced* | **`free`'s 10 GiB is exactly ONE `dev` postgres.** A `standard` (32 GiB PVC) does not fit in it at all, and a second `dev` does not fit either. That is arithmetic from the ruled numbers, not a separate decision — but it is the kind of consequence worth seeing before it is switched on, and the number can be changed. It is withheld today for a different reason: the API cannot predict the PVC the cell will create (`estimates.Resolve` returns `storage_gb: 0` for every size while the driver renders 10/32/128 Gi), so it cannot refuse an order that will not fit — and shipping the ceiling without the gate means a create is priced, accepted, billed and then never admitted, invisibly. **US-3.3i** owns turning it on together with the gate. | US-3.3e (disclosed) · US-3.3i |

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

**Default: OFF.** A feature is enabled only when its implementation is complete
and verified (founder 2026-07-19) — no half-built feature is exposed behind a
flag that's on by default. This is already the operating discipline: staged API
conformance via `oapi-server.cfg.yaml` `include-operation-ids` (an op is served
only when its handler exists and is tested). A dedicated runtime flag system, if
one is later wanted for gated rollouts, inherits this default (ship dark, flip on
after verification).

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
