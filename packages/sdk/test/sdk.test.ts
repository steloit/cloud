// The alpha loop, scriptable from Node with typed responses (T5.6 AC),
// against a scripted fetch — no invented shapes, wire bodies asserted.
import { describe, expect, it } from "vitest";
import {
  ConflictError,
  fmtMoney,
  QuotaExceededError,
  Steloit,
} from "../src/index.ts";

type Call = { url: string; method: string; body?: unknown };

function scriptedFetch(calls: Call[], routes: Record<string, (req: { url: URL; body?: any }) => Response>) {
  return (async (input: any, init?: any) => {
    const url = new URL(typeof input === "string" ? input : input.url);
    const method = (init?.method ?? input?.method ?? "GET").toUpperCase();
    let raw: unknown;
    if (init?.body) raw = JSON.parse(init.body as string);
    else if (typeof input?.text === "function") {
      const t = await input.clone().text();
      raw = t ? JSON.parse(t) : undefined;
    }
    calls.push({ url: url.pathname + url.search, method, body: raw });
    const key = `${method} ${url.pathname}`;
    const handler = routes[key];
    if (!handler) return new Response(JSON.stringify({ title: "not found", status: 404 }), { status: 404 });
    return handler({ url, body: raw });
  }) as typeof fetch;
}

const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

describe("the alpha loop", () => {
  it("estimate → create carries the accepted estimate id; context is carried", async () => {
    const calls: Call[] = [];
    const client = new Steloit({
      apiUrl: "https://api.test",
      token: "stp_t",
      org: "org_1",
      env: "env_1",
      fetch: scriptedFetch(calls, {
        "POST /v1/estimates": () =>
          json(200, { id: "est_1", monthly_total_cents: 2400, lines: [], expires_at: "2026-07-19T01:00:00Z" }),
        "POST /v1/envs/env_1/services": ({ body }) =>
          body.estimate_id === "est_1"
            ? json(201, { id: "svc_1", env_id: "env_1", name: "db", product: "postgres", status: "provisioning" })
            : json(409, { title: "Conflict", status: 409, reasons: ["estimate missing"] }),
      }),
    });

    const est = await client.estimates.create({
      services: [{ product: "postgres", name: "db", shape: { size: "dev", storage_gb: 10 } }],
    });
    expect(est.monthly_total_cents).toBe(2400);
    expect(fmtMoney(est.monthly_total_cents)).toBe("$24");

    const svc = await client.services.create({
      product: "postgres",
      name: "db",
      shape: { size: "dev", storage_gb: 10 },
      estimateId: est.id,
    });
    expect(svc.status).toBe("provisioning");
    // the wire body used snake_case estimate_id and the env came from context
    const create = calls.find((c) => c.url.startsWith("/v1/envs/env_1/services"))!;
    expect((create.body as any).estimate_id).toBe("est_1");
    // the estimate call inherited the context env
    const estCall = calls.find((c) => c.url === "/v1/estimates")!;
    expect((estCall.body as any).env).toBe("env_1");
  });

  it("pagination is an iterator over {data, next_cursor}", async () => {
    let page = 0;
    const client = new Steloit({
      apiUrl: "https://api.test",
      token: "stp_t",
      org: "org_1",
      fetch: scriptedFetch([], {
        "GET /v1/orgs/org_1/projects": () =>
          json(200, ++page === 1
            ? { data: [{ id: "prj_a", org_id: "org_1", name: "a" }], next_cursor: "c1" }
            : { data: [{ id: "prj_b", org_id: "org_1", name: "b" }], next_cursor: null }),
      }),
    });
    const seen: string[] = [];
    for await (const p of client.projects.list()) seen.push(p.id);
    expect(seen).toEqual(["prj_a", "prj_b"]);
  });

  it("errors are typed problem+json with remediation preserved", async () => {
    const client = new Steloit({
      apiUrl: "https://api.test",
      token: "stp_t",
      org: "org_1",
      fetch: scriptedFetch([], {
        "POST /v1/orgs/org_1/projects": () =>
          json(402, { title: "Plan required", status: 402, required_plan: "pro", remediation: "Upgrade to the pro plan." }),
        "POST /v1/orgs/org_1/invites": () => json(409, { title: "Conflict", status: 409, reasons: ["a", "b"] }),
      }),
    });
    const err = await client.projects.create({ name: "second" }).catch((e) => e);
    expect(err).toBeInstanceOf(QuotaExceededError);
    expect(err.requiredPlan).toBe("pro");
    expect(err.remediation).toContain("Upgrade");
    expect(err.status).toBe(402);
    expect(ConflictError.name).toBe("ConflictError");
  });

  it("reveal-once: the secret exists in the returned object and nowhere else", async () => {
    const client = new Steloit({
      apiUrl: "https://api.test",
      token: "stp_t",
      fetch: scriptedFetch([], {
        "POST /v1/me/tokens": () =>
          json(201, { id: "tok_1", token: "stp_secret", shown_once: true, prefix: "stp_secret", hash_stored: true }),
      }),
    });
    const t = await client.tokens.create({ name: "ci" });
    expect(t.token).toBe("stp_secret");
    // nothing on the client caches it
    expect(JSON.stringify(client).includes("stp_secret")).toBe(false);
  });

  it("events.stream resumes from the last frame's cursor", async () => {
    const cursors: (string | null)[] = [];
    let conn = 0;
    const sse = (frames: string) =>
      new Response(frames, { status: 200, headers: { "Content-Type": "text/event-stream" } });
    const controller = new AbortController();
    const client = new Steloit({
      apiUrl: "https://api.test",
      token: "stp_t",
      env: "env_1",
      fetch: (async (input: any, init?: any) => {
        const url = new URL(typeof input === "string" ? input : input.url);
        cursors.push(url.searchParams.get("cursor"));
        conn++;
        if (conn === 1) return sse('id: cur_1\nevent: deploy\ndata: {"n":1}\n\n');
        controller.abort();
        return sse('id: cur_2\nevent: deploy\ndata: {"n":2}\n\n');
      }) as typeof fetch,
    });
    const got: number[] = [];
    for await (const f of client.events.stream({ signal: controller.signal })) {
      got.push((f.data as any).n);
      if (got.length === 2) break;
    }
    expect(got).toEqual([1, 2]);
    expect(cursors).toEqual([null, "cur_1"]); // reconnect carried the cursor
  });
});

describe("money", () => {
  it("is integer cents with the one arithmetic", () => {
    expect(fmtMoney(5800)).toBe("$58");
    expect(fmtMoney(20800)).toBe("$208");
    expect(fmtMoney(162)).toBe("$1.62");
    expect(() => fmtMoney(1.5)).toThrow();
  });
});
