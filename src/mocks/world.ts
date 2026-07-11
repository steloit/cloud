import type {
  AlertRule,
  Binding,
  Cell,
  Dashboard,
  Deployment,
  Environment,
  Event,
  Insight,
  Invite,
  InvitePublic,
  Invoice,
  Member,
  Org,
  PaymentMethod,
  Policy,
  Project,
  Proposal,
  Quota,
  Service,
  Subscription,
  Template,
  Token,
  Trace,
} from "@/lib/api";
import fixtures from "@/lib/canon/fixtures.json";
import { CANON_NOW } from "@/lib/canon/now";

/**
 * The canon world (ADR-026: one canon world, no demo data outside 19-canon).
 * fixtures.json is copied verbatim from the handoff; its section keys are
 * annotated ("orgs (Org)"), so this module is the typed accessor.
 */
export { CANON_NOW };

const raw = fixtures as Record<string, unknown>;

function section<T>(prefix: string): T {
  const key = Object.keys(raw).find((k) => k === prefix || k.startsWith(`${prefix} (`));
  if (!key) throw new Error(`Canon fixtures missing section "${prefix}"`);
  return raw[key] as T;
}

export const orgs = section<Org[]>("orgs");
export const members = section<Member[]>("members");
export const projects = section<Project[]>("projects");
export const environments = section<Environment[]>("environments");
export const services = section<Service[]>("services");
export const bindings = section<Binding[]>("bindings");
export const deployments = section<Deployment[]>("deployments");
export const trace = section<Trace>("trace");
export const alertRules = section<AlertRule[]>("alert_rules");
export const events = section<Event[]>("events");
export const billingOverview = section<Record<string, unknown>>("billing_overview");
export const insightsAndProposals =
  section<Array<{ insight: Insight; proposal: Proposal }>>("insights_and_proposals");
export const quotas = section<Quota[]>("quotas");
export const templates = section<Template[]>("templates");
export const cells = section<Cell[]>("cells");
export const dashboards = section<Dashboard[]>("dashboards");
export const borealisLifecycle = section<Record<string, Subscription>>("borealis_lifecycle");
export const estimateExample = section<Record<string, unknown>>("estimate_example");

/**
 * Invitation canon (frames A6/A7 — ids are frame-fixed: inv_9d47c2 live,
 * inv_7d21aa expired). fixtures.json has no invites section, so the frames
 * are the source; surfaced as a finding in the Phase-1 report.
 */
export const invitePublics: Record<string, InvitePublic & { failure?: "expired" }> = {
  inv_9d47c2: {
    inviter_name: "Priya Sharma",
    org_name: "Acme",
    role: "developer",
    role_consequences: [
      "Create and deploy services, read metrics and logs across 4 projects · can't manage members, billing or org policies.",
      "Roles are set by the org — yours can change later without a new invite.",
    ],
    email_hint: "m•••o@acme.dev",
    status: "pending",
  },
  inv_7d21aa: {
    inviter_name: "Priya Sharma",
    org_name: "Acme",
    role: "developer",
    email_hint: "m•••o@acme.dev",
    status: "expired",
    failure: "expired",
  },
};

export const pendingInvites: Invite[] = [
  {
    id: "inv_priya_demo",
    email: "priya@acme.dev",
    role: "admin",
    status: "pending",
    expires_at: "2026-07-09T15:00:00+05:30",
  },
  {
    id: "inv_marco_demo",
    email: "marco@acme.dev",
    role: "developer",
    status: "pending",
    expires_at: "2026-07-09T15:00:00+05:30",
  },
];

/**
 * W12 org audit rows — Event-shaped (the audit endpoint returns EventList).
 * Content is frame-fixed from the W12 gallery frame (fixtures.json has no
 * audit section; finding). Times are the incident day, IST. `detail.target`
 * carries the Target column; `detail.actor_note` the actor pill.
 */
export const auditEvents: Event[] = [
  {
    id: "evt_a91f…",
    kind: "lifecycle",
    at: "2026-07-02T14:19:02+05:30",
    actor: "priya",
    via: "assistant",
    action: "created database branch for proposal prp_7c31a2",
    detail: { target: "ecommerce/db-main" },
  },
  {
    id: "evt_88c2…",
    kind: "alert_state",
    at: "2026-07-02T14:02:11+05:30",
    actor: "system",
    via: "system",
    action: "alert fired · api p95 > 800 ms",
    detail: { target: "ecommerce/production", actor_note: "system" },
  },
  {
    id: "evt_71d0…",
    kind: "deploy",
    at: "2026-07-02T13:58:03+05:30",
    actor: "priya",
    via: "user",
    action: "released deployment #142 (git push)",
    detail: { target: "ecommerce/api" },
  },
  {
    id: "evt_65aa…",
    kind: "lifecycle",
    at: "2026-07-02T13:52:40+05:30",
    actor: "token · gh-actions",
    via: "system",
    action: "created preview environment preview/pr-142 + db branch",
    detail: { target: "ecommerce", actor_note: "token · gh-actions" },
  },
  {
    id: "evt_5202…",
    kind: "lifecycle",
    at: "2026-07-02T11:15:22+05:30",
    actor: "system · rotation",
    via: "system",
    action: "rotated secret STRIPE_KEY v3 → v4",
    detail: { target: "ecommerce/production", actor_note: "system · rotation" },
  },
  {
    id: "evt_39be…",
    kind: "policy_trigger",
    at: "2026-07-02T09:41:07+05:30",
    actor: "asha",
    via: "user",
    action: "updated policy budget-600 · alert threshold 90% → 80%",
    detail: { target: "org/acme", actor_note: "org admin" },
  },
  {
    id: "evt_2f11…",
    kind: "policy_trigger",
    at: "2026-07-02T09:12:55+05:30",
    actor: "marco",
    via: "user",
    action: "denied: attempted cross-env binding staging → production db",
    detail: { target: "ecommerce", denied_by: "policy" },
  },
  {
    id: "evt_0c77…",
    kind: "lifecycle",
    at: "2026-07-02T03:00:12+05:30",
    actor: "system · drill",
    via: "system",
    action: "restore drill completed · verification passed ✓",
    detail: { target: "ecommerce/db-main", actor_note: "system · drill" },
  },
];

/**
 * Billing canon beyond billing_overview — fixtures.json has no usage, invoices,
 * or payment-methods sections (finding); rows are frame-fixed from B2/B3/B4.
 * Known source defects carried as findings, not silently fixed: B2's egress row
 * (41.3 GB) contradicts the quotas fixture (87/100 — fixtures win here); B3's
 * invoice lines don't sum to their own printed subtotal (frame arithmetic error).
 */
export const usageReport = {
  month: "2026-07",
  updated_at: "2026-07-02T14:55:00+05:30",
  meters: [
    {
      meter: "compute_hours",
      included: 0,
      used: 1046,
      overage_price: "$0.031 / S-hr",
      overage_cents: 3290,
      detail: [{ label: "api ×3 · workers · admin" }],
    },
    {
      meter: "db_compute_hours",
      included: 0,
      used: 312,
      overage_price: "$0.058 / hr",
      overage_cents: 1810,
      detail: [
        {
          label: "branches: preview/pr-142 · staging",
          note: "scale-to-zero — billed only while awake",
          used_label: "21 h",
          mtd_cents: 122,
        },
      ],
    },
    {
      meter: "db_storage_gb_mo",
      included: 10,
      used: 92,
      overage_price: "$0.12 / GB-mo",
      overage_cents: 588,
    },
    {
      meter: "object_storage_gb_mo",
      included: 5,
      used: 43.1,
      overage_price: "$0.023 / GB-mo",
      overage_cents: 191,
    },
    {
      meter: "egress_gb",
      included: 100,
      used: 87,
      overage_price: "$0.09 / GB after included",
      overage_cents: 0,
    },
    {
      meter: "queue_requests_m",
      included: 1,
      used: 2.84,
      overage_price: "$0.40 / M after included",
      overage_cents: 74,
    },
    {
      meter: "backups_gb_mo",
      included: 7,
      used: 30,
      overage_price: "$0.021 / GB-mo",
      overage_cents: 167,
    },
  ],
};

export const invoices: Invoice[] = [
  { id: "inv_2026_07", period: "Jul 2026", status: "accruing", total_cents: 48200 },
  {
    id: "inv_2026_06",
    period: "Jun 2026",
    status: "paid",
    total_cents: 47700,
    tax: { kind: "GST 18%", id: "29ABCDE1234F1Z5", cents: 13208 },
    lines: [
      {
        description: "ecommerce — compute · databases · storage · network",
        project_id: "prj_ecommerce",
        cents: 18930,
      },
      { description: "analytics-pipeline", project_id: "prj_internal_tools", cents: 8710 },
      { description: "internal-tools", project_id: "prj_mobile_api", cents: 3952 },
      { description: "Business plan · 20 seats included", project_id: null, cents: 9900 },
    ],
    formats: ["pdf", "csv", "json"],
  },
  { id: "inv_2026_05", period: "May 2026", status: "paid", total_cents: 45944 },
  { id: "inv_2026_04", period: "Apr 2026", status: "paid", total_cents: 46102 },
  { id: "inv_2026_03", period: "Mar 2026", status: "paid", total_cents: 47277 },
  { id: "inv_2026_02", period: "Feb 2026", status: "paid", total_cents: 44719 },
];

export const paymentMethods: PaymentMethod[] = [
  { id: "pm_visa_4412", brand: "Visa", last4: "4412", backup: false, expires: "08/27" },
];

/**
 * Governance canon — fixtures.json has no api-keys, policies, or personal-token
 * sections (finding); rows below are frame-fixed from G7/G8/P5/G4.
 * Note: Token.scope is enum full|read_only while G8 renders richer scope pills
 * ("ecommerce · deploy") — schema gap, flagged; display strings live in the UI.
 */
export const apiKeys: Token[] = [
  {
    id: "key_ci_deploy",
    name: "ci-deploy",
    prefix: "stk_live_8Ff2…",
    scope: "full",
    last_used_at: "2026-07-02T14:01:00+05:30",
    expires_at: "2027-04-02T00:00:00+05:30",
  },
  {
    id: "key_cost_export",
    name: "cost-export",
    prefix: "stk_live_2Kd9…",
    scope: "read_only",
    last_used_at: "2026-07-02T02:00:00+05:30",
    expires_at: "2027-02-02T00:00:00+05:30",
  },
  {
    id: "key_old_terraform",
    name: "old-terraform",
    prefix: "stk_live_9Xa1…",
    scope: "full",
    last_used_at: "2026-05-02T00:00:00+05:30",
  },
];

export const personalTokens: Token[] = [
  {
    id: "tok_laptop_cli",
    name: "laptop-cli",
    prefix: "stp_9m2K…",
    scope: "full",
    last_used_at: "2026-07-02T14:01:00+05:30",
    expires_at: "2027-04-02T00:00:00+05:30",
  },
  {
    id: "tok_notebook_ro",
    name: "notebook-readonly",
    prefix: "stp_4Ha8…",
    scope: "read_only",
    last_used_at: "2026-07-04T00:00:00+05:30",
    expires_at: "2026-08-31T00:00:00+05:30",
  },
];

export const policies: Policy[] = [
  {
    id: "pol_prod_guard",
    key: "prod-guard",
    description: "Developers: production mutations require Admin approval",
    scope: { project_id: "prj_ecommerce" },
    rule: { text: "Developers: production mutations require Admin approval" },
    enforcement: "enforce",
    version: 1,
    last_change_event: "evt_pg01…",
    violation_count_30d: 1,
  },
  {
    id: "pol_preview_minimal",
    key: "preview-minimal",
    description: "Previews: exclude jobs & worker · scale S · auto-expire 7 d",
    scope: { project_id: "prj_ecommerce" },
    rule: { text: "Previews: exclude jobs & worker · scale S · auto-expire 7 d" },
    enforcement: "enforce",
    version: 1,
    last_change_event: "evt_pm01…",
    violation_count_30d: 0,
  },
  {
    id: "pol_allowed_regions",
    key: "allowed-regions",
    description: "APAC cells + aws · us-east-1 (expanded for the US launch)",
    scope: { project_id: null },
    rule: { text: "APAC cells + aws · us-east-1 (expanded for the US launch)" },
    enforcement: "enforce",
    version: 3,
    last_change_event: "evt_44be…",
    violation_count_30d: 0,
  },
  {
    id: "pol_ai_assistant",
    key: "ai-assistant",
    description:
      "Assistant, describe-to-provision & product AI panels — enabled org-wide; developers may opt out per-project",
    scope: { project_id: null },
    rule: {
      text: "Assistant, describe-to-provision & product AI panels — enabled org-wide; developers may opt out per-project",
    },
    enforcement: "enabled",
    version: 1,
    last_change_event: "evt_31c0…",
    violation_count_30d: 0,
  },
  {
    id: "pol_public_buckets",
    key: "public-buckets",
    description: "Public visibility requires named Admin approval",
    scope: { project_id: null },
    rule: { text: "Public visibility requires named Admin approval" },
    enforcement: "enforce",
    version: 1,
    last_change_event: "evt_pb01…",
    violation_count_30d: 0,
  },
  {
    id: "pol_credential_rotation",
    key: "credential-rotation",
    description: "Binding credentials rotate every 90 d",
    scope: { project_id: null },
    rule: { text: "Binding credentials rotate every 90 d" },
    enforcement: "enforce",
    version: 1,
    last_change_event: "evt_cr01…",
    violation_count_30d: 0,
  },
  {
    id: "pol_compute_ceiling",
    key: "compute-ceiling",
    description: "Autoscale ceiling ≤ 6 in production",
    scope: { project_id: null },
    rule: { text: "Autoscale ceiling ≤ 6 in production" },
    enforcement: "enforce",
    version: 1,
    last_change_event: "evt_cc01…",
    violation_count_30d: 0,
  },
  {
    id: "pol_mfa_required",
    key: "mfa-required",
    description: "All members enroll MFA within 14 d",
    scope: { project_id: null },
    rule: { text: "All members enroll MFA within 14 d" },
    enforcement: "warn",
    enforce_from: "2026-07-20T00:00:00+05:30",
    version: 1,
    last_change_event: "evt_mfa1…",
    violation_count_30d: 0,
  },
];

/**
 * Canon domains/lifecycle-rules/schedules — frame-fixed from D21/D15/D17/S5
 * (fixtures.json has no sections for these; finding).
 */
export const domains = [
  {
    id: "dom_api_primary",
    domain: "api.acme-store.com",
    status: "issued" as const,
    records: [
      { type: "CNAME" as const, name: "api", value: "svc-1aa508.edge.steloit.app", verified: true },
    ],
    cert_expires_at: "2026-09-01T00:00:00+05:30",
  },
];

export const lifecycleRules = [
  { id: "rule_tmp_expire", prefix: "tmp/", action: "expire" as const, after_days: 7 },
  { id: "rule_originals_cold", prefix: "originals/", action: "tier_cold" as const, after_days: 90 },
];

export const schedules = [
  { id: "sch_receipts", name: "receipts-daily", cron: "0 2 * * *", tz: "Asia/Kolkata" },
  { id: "sch_cleanup", name: "cleanup-tmp", cron: "0 * * * *", tz: "Asia/Kolkata" },
];

/**
 * internal-tools canon (frames M1/AI5): tools-db is fully frame-specced
 * (svc_4nn810, $19/mo, 14/100 conns); the storage instance's name is not
 * frame-fixed — "files" follows fixtures' own $representative convention.
 * Finding: these services don't reconcile to the project's fixture total
 * ($96) — the gallery's internal-tools card said $41 (the name↔cost swap).
 */
export const internalToolsEnv: Environment = {
  id: "env_it_prod",
  project_id: "prj_internal_tools",
  name: "production",
  region: "aws/ap-south-1",
  kind: "standard",
  monthly_cost_cents: 4100,
  policy_flags: [],
  expires_at: null,
};

export const internalToolsServices: Service[] = [
  {
    id: "svc_4nn810",
    env_id: "env_it_prod",
    name: "tools-db",
    product: "postgres",
    status: "ready",
    shape: {
      version: "16.4",
      size: "dev",
      storage_gb: 10,
      ha: false,
      connections: { used: 14, max: 100 },
    },
    region: "aws/ap-south-1",
    monthly_estimate_cents: 1900,
    created_at: "2026-01-10T11:00:00+05:30",
  },
  {
    id: "svc_it_files",
    env_id: "env_it_prod",
    name: "files",
    product: "storage",
    status: "ready",
    shape: { public: false, versioning: true, $representative: "name not frame-fixed" },
    region: "aws/ap-south-1",
    monthly_estimate_cents: 2200,
    created_at: "2026-01-10T11:00:00+05:30",
  },
];

export function findOrg(orgParam: string): Org | undefined {
  return orgs.find((o) => o.slug === orgParam || o.id === orgParam);
}

export function findProject(projectParam: string): Project | undefined {
  return projects.find((p) => p.id === projectParam || p.name === projectParam);
}

export function findService(serviceParam: string): Service | undefined {
  return services.find((s) => s.id === serviceParam || s.name === serviceParam);
}
