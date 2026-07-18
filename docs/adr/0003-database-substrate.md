# ADR-0003 · Database substrate: CNPG + CoW snapshots replaces Neon OSS

**Status:** Proposed · 2026-07-18 · Supersedes architecture.md §3 (customer-database row) and requires INF-001 Amendment A4 (founder ratification — D3 is constitutional).
**Trigger (measured, per ADR-0001's rule):** fresh first-principles review requested by founder; research sweep 2026-07-18 found the Neon OSS upstream risk named in INF-001 §7 has **already materialized** — before we wrote a line of data-plane code.

## The three facts that decide it

1. **Neon OSS is not operable by outsiders, by its maintainers' own account.** Self-hosting was never a supported product; a Neon staff engineer states binary/config backward compatibility holds "**about 2 weeks**" (they ship to their own cloud weekly), and the shipped compose "is not intended for deploying a usable system." The control plane — the part that makes it a product — was never open source. The only serious third-party operator (Molnett) self-describes as dev/test-grade and its company was acquired. *(github.com/neondatabase/neon discussions #8691, #1828, #7998; molnett/neon-operator)*
2. **The upstream has gone dark publicly since the Databricks acquisition.** Public release tags stopped **2025-07-29**; commits fell to ~1/month through 2026; "is neondb no longer maintained?" (Oct 2025) drew no official answer; Databricks' product (Lakebase) is cloud-only. INF-001 §7 listed "Neon OSS stagnation" as a risk with a fork option — the risk fired, and a 2-founder team forking a dark, un-semver'd storage engine is not a mitigation, it's a second company. *(github.com/neondatabase/neon releases/commits; discussion #12835)*
3. **The boring alternative got dramatically stronger while Neon got weaker.** CloudNativePG: CNCF, PG18 within a month of GA, declarative everything (Database/Role CRDs, volume-snapshot backup/recovery, in-place major upgrades, hibernation), and a documented production fleet of **450 clusters/31TB at Mirakl**. And since ~April 2026, **Xata OSS (Apache-2.0)** exists: a k8s-native branching platform built on exactly this substrate (CNPG + OpenEBS CoW, branch operator, scale-to-zero gateway) — the reference implementation for our wedge, on our stack. *(cloudnative-pg.io end_users; xata.io/blog/xata-is-now-open-source)*

## What the product actually requires (the review's yardstick)

Branch-per-PR previews with a masked branch of production at delta pricing (`$0.07/day` — the pitch's headline and M4's exit criterion) · one isolated database per project-env (D7) · scale-to-zero economics for a mostly-idle free tier · PITR with restore-to-new-branch, never in place · RPO ≤ 5 min (A1.3) · operable by two founders for years. **Branching is load-bearing today** — it is the wedge, not a future feature. Any recommendation that kills CoW branching changes the company, not the database.

## Candidates

| Candidate | Verdict | Decisive reason |
|---|---|---|
| **Neon OSS (incumbent, D3)** | ❌ Rejected | Facts 1–2. Also: novel failure modes with no community, our entire branch/PITR story hostage to a dark upstream. Its *architecture* remains the ideal — which is why the replacement keeps CoW semantics. Exit if chosen: fork (≈impossible at our size). |
| **Standard PG, hand-rolled pods** | ❌ | Rebuilds an operator badly; Fly.io (retreated from k8s PG entirely) and Heroku (fleeing to Aurora) show managed-PG ops is brutal even for funded teams. No branching, no fleet tooling. |
| **PG + Crunchy PGO** | ❌ | Capable, but: ~8-month release gap around the Snowflake acquisition, production images gated behind registry terms, community momentum clearly at CNPG. No branching. |
| **PG + CNPG alone** | ❌ as sole answer | Best-in-class operations, but previews = full-copy PITR clones — kills instant + delta-priced branching, i.e., the wedge. This is the *substrate*, not the whole answer. |
| **AlloyDB Omni** | ❌ | Paid per-vCPU for production (~$40–81/vCPU/mo), embedding requires a Google OEM deal; violates D1 (reselling) in spirit and our margins in fact. |
| **CockroachDB / Yugabyte** | ❌ | Not Postgres (extensions, semantics, version lag); CRDB proprietary since Nov 2024; nobody sells "Postgres" on either. We sell Postgres. |
| **DBLab Engine (postgres.ai)** | ❌ as primitive | Apache-2.0, battle-tested (GitLab institutionalizes it), but a single-host ZFS appliance for dev/CI clones — wrong shape for a multi-tenant product primitive. Keep in mind for internal CI. |
| **✅ CNPG + OpenEBS ZFS-LocalPV CoW snapshots, branch orchestration in OUR control plane, Xata OSS as reference** | **Adopted** | See below. |

## The decision

**Customer databases:** one **CNPG cluster per project-environment** (single-instance for free/dev tiers; replicas are a paid knob) on GKE, storage class **OpenEBS ZFS-LocalPV** on a storage node pool.
**Branching:** branch = CSI VolumeSnapshot (ZFS CoW, instant, delta-priced) → CNPG `bootstrap: recovery` from snapshot → hibernated-by-default single-instance cluster. Branch metadata, lifecycle, routing, and cleanup live in **our** provisioning module + cell-agent — which is the control plane we were building under Neon anyway; the work moves, it doesn't grow. **Xata OSS is the studied reference** (their branch operator, CNPG-I scale-to-zero plugin pattern, and pgstream for DDL-aware masked branch feeds); adopt components where they fit, own the orchestration either way.
**Scale-to-zero:** CNPG declarative hibernation + idle-detection (Xata's published plugin pattern); wake-on-connect via the cell gateway (fast-follow; alpha may wake via control-plane on preview activity).
**PITR/backups:** WAL archiving + base backups to GCS (`archive_timeout` ≤ 5 min ⇒ **A1.3's RPO by construction**). Stay on CNPG ≤1.30 in-tree Barman until the barman-cloud plugin's open failover/restore issues (#656, #652, #164) are closed — a pinned-version knob, tracked. Restore is always to a **new** branch (spec promise kept natively by `recoveryTarget` → new cluster).
**Masking:** policy-driven masking job on branch creation (pgstream when data-sync feeds are needed) — `masked · policy` stays honest.
**Control-plane DB:** also CNPG (single instance + PITR to GCS) — replacing architecture.md §3's hand-rolled pod+WAL-G; one operator to know everywhere.

### What we consciously give up vs Neon, and why it's acceptable
- **Branch creation ~10–60s** (snapshot + pod start) instead of sub-second — inside a CI cycle that takes minutes; the pitch's economics ($/day, delta-priced) survive; only the "milliseconds" flourish dies.
- **Per-tenant pods instead of pageserver multiplexing** — mitigated by hibernation (idle DB = zvol bytes only); node-local ZFS pins branches to their storage node (acceptable for previews; Ceph RBD is the cluster-wide-CoW knob if a trigger demands it). Cell-cap economics re-verified at Cell-1 as INF-001 already requires.
- **"Bottomless" GCS-native storage** → PD+ZFS with snapshots/WAL to GCS — at alpha scale a rounding error; revisit only on measured storage-cost pressure.

### What we gain
Vanilla Postgres end-to-end (every extension, every DBA runbook, exit = pg_dump/replication to anywhere) · a substrate with a real community and a 450-cluster production precedent · **stronger** isolation story than shared pageservers (D7 per-tenant clusters) · one operator for customer AND control-plane databases · the week-1 existential spike (Pageserver↔GCS generation fencing) replaced by a low-risk one (ZFS snapshot → CNPG recovery e2e — all documented, boring paths) · A3.1's queue design simplifies to standard logical-decoding CDC on vanilla PG.

## Ripple (all documents, no code — this is why the review happened now)

1. **INF-001 Amendment A4** (founder ratification required; proposed text below).
2. `docs/architecture.md` §3/§17 rewrite (after ratification — §3 currently cites D3 as locked).
3. Tasks: T1.0 (spike → ZFS/CNPG branch e2e + hibernation wake), T1.2 (Neon fleet → CNPG operator + storage pool), T3.4 (Neon driver → CNPG driver), E9-4/E9-5 queue notes (CDC on vanilla PG). `contexts/provisioning.md` rewrite of the substrate section.
4. Effort delta: **≈ zero net** (branch orchestration was always our code; substrate risk drops sharply). Migration cost: **zero — nothing is built.**

### Proposed INF-001 Amendment A4 (for founder to apply — 00-sources is human-only)

> **A4 (2026-07-18, ADR-0003).** D3 is amended: the Postgres substrate is **CNPG-operated vanilla PostgreSQL, one cluster per project-environment, with copy-on-write branching via CSI/ZFS volume snapshots orchestrated by the Steloit control plane** (Xata OSS as Apache-2.0 reference implementation). Neon OSS is rejected on measured evidence: unsupported self-hosting with a ~2-week compatibility window, closed control plane, and a public upstream dark since 2025-07 (post-Databricks). The §7 "Neon stagnation" risk is recorded as having fired pre-build. D3's branching *requirement* (branch-per-PR as the category feature) is unchanged and is satisfied at the storage-snapshot layer. The week-1 spike (D3 open item) is redefined accordingly. Grammar isolation (D8) preserved: no substrate name reaches any customer surface — which is precisely what made this swap free.
