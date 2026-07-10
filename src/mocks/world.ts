import type {
  Binding,
  Deployment,
  Environment,
  Event,
  Invite,
  InvitePublic,
  Member,
  Org,
  Project,
  Service,
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
export const events = section<Event[]>("events");
export const billingOverview = section<Record<string, unknown>>("billing_overview");
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

export function findOrg(orgParam: string): Org | undefined {
  return orgs.find((o) => o.slug === orgParam || o.id === orgParam);
}

export function findProject(projectParam: string): Project | undefined {
  return projects.find((p) => p.id === projectParam || p.name === projectParam);
}

export function findService(serviceParam: string): Service | undefined {
  return services.find((s) => s.id === serviceParam || s.name === serviceParam);
}
