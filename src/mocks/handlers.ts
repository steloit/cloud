import { HttpResponse, http } from "msw";
import { logResult, metricResult } from "./telemetry";
import * as world from "./world";

/**
 * Canon-mode API: serves 19-canon fixtures through the real contract in
 * 08-api/openapi.yaml — list envelopes are {data, next_cursor}, errors are
 * problem+json with required remediation. No endpoint exists here that the
 * spec lacks (hard rule).
 */

function list<T>(data: T[]) {
  return HttpResponse.json({ data, next_cursor: null });
}

function problem(status: number, title: string, detail: string, remediation: string) {
  return HttpResponse.json(
    { title, status, detail, remediation },
    { status, headers: { "Content-Type": "application/problem+json" } },
  );
}

const notFound = (what: string) =>
  problem(
    404,
    "Not found",
    `${what} does not exist in this organization.`,
    "Check the id, or list the collection to find the right one.",
  );

export const handlers = [
  // orgs
  http.get("/v1/orgs", () => list(world.orgs)),
  http.post("/v1/orgs", async ({ request }) => {
    const body = (await request.json()) as { name: string; home_region?: string };
    const slug = body.name.toLowerCase().replace(/[^a-z0-9-]/g, "-");
    return HttpResponse.json(
      {
        id: `org_${slug.replace(/-/g, "_")}`,
        slug,
        name: body.name,
        home_region: body.home_region ?? "aws/ap-south-1",
        plan: "free",
        created_at: world.CANON_NOW.toISOString(),
      },
      { status: 201 },
    );
  }),
  http.get("/v1/orgs/:org", ({ params }) => {
    const org = world.findOrg(String(params.org));
    return org ? HttpResponse.json(org) : notFound(`Organization ${String(params.org)}`);
  }),
  http.get("/v1/orgs/:org/members", () =>
    HttpResponse.json({
      data: world.members,
      next_cursor: null,
      seats: { included: 20, used: 12, overage_price_cents: 700 },
    }),
  ),
  http.patch("/v1/orgs/:org/members/:member", async ({ params, request }) => {
    const body = (await request.json()) as { role: string };
    const member = world.members.find((m) => m.id === params.member);
    if (!member) return notFound(`Member ${String(params.member)}`);
    return HttpResponse.json({ ...member, role: body.role });
  }),
  http.delete("/v1/orgs/:org/members/:member", () => HttpResponse.json({ flagged_resources: [] })),

  // invites
  http.get("/v1/orgs/:org/invites", () => list(world.pendingInvites)),
  http.post("/v1/orgs/:org/invites", async ({ request }) => {
    const body = (await request.json()) as { email: string; role: string };
    return HttpResponse.json(
      {
        id: `inv_${Math.abs(hash(body.email)).toString(16).slice(0, 6)}`,
        email: body.email,
        role: body.role,
        status: "pending",
        expires_at: new Date(world.CANON_NOW.getTime() + 7 * 86_400_000).toISOString(),
      },
      { status: 201 },
    );
  }),
  http.get("/v1/invites/:invite", ({ params }) => {
    const invite = world.invitePublics[String(params.invite)];
    if (!invite) {
      return problem(
        404,
        "This link isn't valid",
        "Most common reason: it was already used. If you accepted earlier — even on another device — you're a member; just sign in.",
        "Sign in — you may already be a member. Otherwise ask the inviter for a new link.",
      );
    }
    if (invite.failure === "expired") {
      return problem(
        410,
        "This invitation has expired",
        "Invitations are valid for 7 days — this one expired 2 days ago.",
        "Request a new invitation — the inviter gets a bell and an email.",
      );
    }
    return HttpResponse.json(invite);
  }),
  http.post("/v1/invites/:invite", ({ params }) => {
    const invite = world.invitePublics[String(params.invite)];
    if (!invite || invite.failure) return notFound(`Invitation ${String(params.invite)}`);
    return HttpResponse.json({ ...invite, status: "accepted" });
  }),
  http.delete("/v1/invites/:invite", () => new HttpResponse(null, { status: 204 })),
  http.post("/v1/invites/:invite/renew", () =>
    HttpResponse.json({ renewed: true, notified: true }, { status: 202 }),
  ),

  // projects
  http.get("/v1/orgs/:org/projects", ({ params }) => {
    const org = world.findOrg(String(params.org));
    if (!org) return notFound(`Organization ${String(params.org)}`);
    return list(org.id === "org_acme" ? world.projects : []);
  }),
  http.post("/v1/orgs/:org/projects", async ({ request }) => {
    const body = (await request.json()) as { name: string };
    return HttpResponse.json(
      {
        id: `prj_${body.name.replace(/-/g, "_")}`,
        org_id: "org_acme",
        name: body.name,
        health: "ok",
        monthly_cost_cents: 0,
        env_count: 1,
        created_at: world.CANON_NOW.toISOString(),
      },
      { status: 201 },
    );
  }),
  http.get("/v1/projects/:project", ({ params }) => {
    const project = world.findProject(String(params.project));
    return project ? HttpResponse.json(project) : notFound(`Project ${String(params.project)}`);
  }),
  http.get("/v1/projects/:project/envs", ({ params }) => {
    const project = world.findProject(String(params.project));
    if (!project) return notFound(`Project ${String(params.project)}`);
    return list(project.id === "prj_ecommerce" ? world.environments : []);
  }),

  // estimates — estimate-before-provision; W2's rail is canon est_w2_demo
  http.post("/v1/estimates", () => HttpResponse.json(world.estimateExample, { status: 201 })),

  // services (env-as-filter: shape is per-project; canon fixes env_prod rows)
  http.post("/v1/envs/:env/services", async ({ params, request }) => {
    const body = (await request.json()) as {
      product: string;
      name: string;
      shape?: Record<string, unknown>;
      estimate_id?: string;
    };
    if (!body.estimate_id) {
      return problem(
        422,
        "Validation failed",
        "estimate_id is required — nothing provisions without an accepted estimate.",
        "POST /estimates first, then pass the returned est_… id.",
      );
    }
    const created = {
      id: `svc_${body.name.replace(/-/g, "_")}`,
      env_id: String(params.env),
      name: body.name,
      product: body.product,
      status: "provisioning",
      shape: body.shape ?? {},
      region: "aws/ap-south-1",
      monthly_estimate_cents: 2400,
      provisioning_steps: [
        { step: "Allocate compute", status: "done" },
        { step: `Configure ${body.product}`, status: "done" },
        { step: "Private network & scoped credentials", status: "active" },
        { step: "First backup & verification", status: "pending" },
        {
          step: "Ready — binding activates, consumers restart with config injected",
          status: "pending",
        },
      ],
      created_at: new Date().toISOString(),
    } as (typeof world.services)[number];
    createdServices.push(created);
    return HttpResponse.json(created, { status: 201 });
  }),
  http.get("/v1/envs/:env/services", ({ params }) => {
    const env = String(params.env);
    const known = world.environments.some((e) => e.id === env || e.name === env);
    if (!known) return list([]);
    return list([...world.services, ...createdServices.map(withProvisioningProgress)]);
  }),
  http.get("/v1/services/:service", ({ params }) => {
    const key = String(params.service);
    const created = createdServices.find((s) => s.id === key || s.name === key);
    if (created) return HttpResponse.json(withProvisioningProgress(created));
    const service = world.findService(key);
    return service ? HttpResponse.json(service) : notFound(`Service ${key}`);
  }),
  http.get("/v1/services/:service/bindings", ({ params }) => {
    const service = world.findService(String(params.service));
    if (!service) return notFound(`Service ${String(params.service)}`);
    return list(
      world.bindings.filter((b) => b.source_id === service.id || b.target_id === service.id),
    );
  }),

  // deploy + observe (W3 spine)
  http.get("/v1/envs/:env/deployments", () => list(world.deployments)),
  http.post("/v1/deployments/:dep/rollback", ({ params }) => {
    const dep = world.deployments.find((d) => d.id === params.dep);
    if (!dep) return notFound(`Deployment ${String(params.dep)}`);
    return HttpResponse.json({ ...dep, state: "rolling_back" }, { status: 202 });
  }),
  http.get("/v1/envs/:env/events", () => list(world.events)),

  // observe — canon telemetry anchored on the incident numbers
  http.get("/v1/envs/:env/metrics", ({ request }) => {
    const query = new URL(request.url).searchParams.get("query") ?? "";
    return HttpResponse.json(metricResult(query));
  }),
  http.get("/v1/envs/:env/logs", ({ request }) => {
    const query = new URL(request.url).searchParams.get("query") ?? undefined;
    return HttpResponse.json(logResult(query));
  }),
  http.get("/v1/envs/:env/traces/:trace", ({ params }) => {
    if (params.trace !== world.trace.id) return notFound(`Trace ${String(params.trace)}`);
    return HttpResponse.json(world.trace);
  }),
  http.get("/v1/projects/:project/alert-rules", () => list(world.alertRules)),

  // governance: api keys, personal tokens (reveal-once), policies
  http.get("/v1/orgs/:org/api-keys", () => list(world.apiKeys)),
  http.post("/v1/orgs/:org/api-keys", async ({ request }) => {
    const body = (await request.json()) as { name: string; expires_in_days?: number };
    return HttpResponse.json(
      {
        id: `key_${body.name.replace(/-/g, "_")}`,
        token: "stk_live_Bq72Xw4nLm9cVd…",
        shown_once: true,
        prefix: "stk_live_Bq72…",
        hash_stored: true,
        expires_at: new Date(
          world.CANON_NOW.getTime() + (body.expires_in_days ?? 90) * 86_400_000,
        ).toISOString(),
      },
      { status: 201 },
    );
  }),
  http.get("/v1/me/tokens", () => list(world.personalTokens)),
  http.post("/v1/me/tokens", async ({ request }) => {
    const body = (await request.json()) as { name: string; expires_in_days?: number };
    return HttpResponse.json(
      {
        id: `tok_${body.name.replace(/-/g, "_")}`,
        token: "stp_Ax91K3q8pR2vTz…",
        shown_once: true,
        prefix: "stp_Ax91…",
        hash_stored: true,
        expires_at: new Date(
          world.CANON_NOW.getTime() + (body.expires_in_days ?? 90) * 86_400_000,
        ).toISOString(),
      },
      { status: 201 },
    );
  }),
  http.delete("/v1/me/tokens/:tok", () => new HttpResponse(null, { status: 204 })),
  http.get("/v1/orgs/:org/policies", () => list(world.policies)),
  http.post("/v1/orgs/:org/policies", async ({ request }) => {
    const url = new URL(request.url);
    const body = (await request.json()) as Record<string, unknown>;
    if (url.searchParams.get("dry_run") === "true") {
      // Impact preview — preview before enforce (G9's live rail)
      return HttpResponse.json({
        affected: [
          {
            id: "branches_future",
            name: "New branches",
            effect: "masked automatically — zero developer effort",
          },
          {
            id: "env_pr142",
            name: "preview/pr-142",
            effect: "carries unmasked production data — will be flagged, never deleted",
          },
        ],
        conflicts: [],
        members_affected: 12,
      });
    }
    return HttpResponse.json(
      {
        ...body,
        id: `pol_${String(body.key ?? "new").replace(/-/g, "_")}`,
        version: 1,
        last_change_event: "evt_51aa02",
        violation_count_30d: 0,
      },
      { status: 201 },
    );
  }),

  // billing + governance
  http.get("/v1/orgs/:org/billing/overview", () => HttpResponse.json(world.billingOverview)),
  http.get("/v1/orgs/:org/billing/quotas", () => list(world.quotas)),
  http.get("/v1/orgs/:org/billing/usage", () => HttpResponse.json(world.usageReport)),
  http.get("/v1/orgs/:org/billing/invoices", () => list(world.invoices)),
  http.get("/v1/orgs/:org/payment-methods", () => list(world.paymentMethods)),
  http.get("/v1/orgs/:org/subscription", ({ params }) => {
    const org = world.findOrg(String(params.org));
    if (!org) return notFound(`Organization ${String(params.org)}`);
    if (org.id === "org_borealis") {
      return HttpResponse.json(borealisSubscription ?? world.borealisLifecycle.trial);
    }
    return HttpResponse.json({ plan: org.plan, status: "current", anchor_day: 1 });
  }),
  http.post("/v1/orgs/:org/subscription", async ({ params, request }) => {
    const body = (await request.json()) as { plan: string };
    const org = world.findOrg(String(params.org));
    if (!org) return notFound(`Organization ${String(params.org)}`);
    if (org.id === "org_acme" && body.plan === "free") {
      return HttpResponse.json(
        {
          title: "Downgrade blocked",
          status: 409,
          detail: "12 members > Free's 3 · 4 projects > Free's 1",
          remediation: "Reduce members and projects to Free limits, or stay on Business.",
          reasons: ["12 members > Free's 3", "4 projects > Free's 1"],
        },
        { status: 409, headers: { "Content-Type": "application/problem+json" } },
      );
    }
    if (org.id === "org_borealis") {
      borealisSubscription = {
        plan: body.plan,
        status: "current",
        anchor_day: 1,
        trial_ends_at: null,
        dunning: { state: "current", day: null, next_retry_at: null },
        wind_down: null,
      } as (typeof world.borealisLifecycle)["trial"];
      return HttpResponse.json(borealisSubscription);
    }
    return HttpResponse.json({ plan: body.plan, status: "current", anchor_day: 1 });
  }),
  http.delete("/v1/orgs/:org/subscription", ({ params }) => {
    const org = world.findOrg(String(params.org));
    if (!org) return notFound(`Organization ${String(params.org)}`);
    const cancelled = {
      plan: org.plan,
      status: "cancelled_at_anchor",
      anchor_day: 1,
      wind_down: { plan_ends_at: "2026-08-01T00:00:00+05:30", resources_unaffected: true },
    } as (typeof world.borealisLifecycle)["trial"];
    if (org.id === "org_borealis") borealisSubscription = cancelled;
    return HttpResponse.json(cancelled);
  }),
  http.get("/v1/orgs/:org/templates", () => list(world.templates)),
  http.get("/v1/templates/:tpl", ({ params }) => {
    const tpl = world.templates.find((t) => t.id === params.tpl || t.name === params.tpl);
    return tpl ? HttpResponse.json(tpl) : notFound(`Template ${String(params.tpl)}`);
  }),
  http.get("/v1/orgs/:org/cells", () => list(world.cells)),
  http.get("/v1/orgs/:org/dashboards", () => list(world.dashboards)),
  http.get("/v1/dashboards/:dash", ({ params }) => {
    const dash = world.dashboards.find((d) => d.id === params.dash || d.name === params.dash);
    return dash ? HttpResponse.json(dash) : notFound(`Dashboard ${String(params.dash)}`);
  }),
  http.get("/v1/orgs/:org/audit", () => list(world.auditEvents)),
];

/** Borealis subscription transitions in-memory (B11 confirm / B12 cancel are real flows). */
let borealisSubscription: (typeof world.borealisLifecycle)["trial"] | undefined;

/** Session-created services (canon mode is in-memory; refresh resets the demo). */
const createdServices: (typeof world.services)[number][] = [];

/** Provisioning flips to ready ~40s after create — metering starts at ready (C4). */
function withProvisioningProgress(svc: (typeof world.services)[number]) {
  const created = new Date(svc.created_at ?? 0).getTime();
  if (svc.status !== "provisioning" || Date.now() - created < 40_000) return svc;
  return {
    ...svc,
    status: "ready" as const,
    provisioning_steps: (svc.provisioning_steps ?? []).map((s) => ({
      ...s,
      status: "done" as const,
    })),
  };
}

function hash(input: string): number {
  let h = 0;
  for (let i = 0; i < input.length; i++) h = (h * 31 + input.charCodeAt(i)) | 0;
  return h;
}
