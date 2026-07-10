import { HttpResponse, http } from "msw";
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
  http.get("/v1/orgs/:org/members", () => list(world.members)),

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
  http.get("/v1/envs/:env/services", ({ params }) => {
    const env = String(params.env);
    const known = world.environments.some((e) => e.id === env || e.name === env);
    return list(known ? world.services : []);
  }),
  http.get("/v1/services/:service", ({ params }) => {
    const service = world.findService(String(params.service));
    return service ? HttpResponse.json(service) : notFound(`Service ${String(params.service)}`);
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

  // billing + governance
  http.get("/v1/orgs/:org/billing/overview", () => HttpResponse.json(world.billingOverview)),
  http.get("/v1/orgs/:org/audit", () => list(world.auditEvents)),
];

function hash(input: string): number {
  let h = 0;
  for (let i = 0; i < input.length; i++) h = (h * 31 + input.charCodeAt(i)) | 0;
  return h;
}
