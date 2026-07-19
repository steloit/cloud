// @steloit/sdk — the ergonomics layer (20-clients/sdk.md §2/§3): thin,
// handwritten, wrapping the GENERATED core. It may wrap the core; it may
// never bypass it or add semantics the API doesn't have.

import { createClient, createConfig, type Client } from "./generated/client";
import * as gen from "./generated/sdk.gen.ts";
import type {
  Estimate,
  ServiceShapeInput,
  TokenCreated,
} from "./generated/types.gen.ts";

// ---- typed errors (problem+json, remediation preserved — §3) ----------------

export interface ProblemFields {
  title?: string;
  detail?: string;
  status?: number;
  remediation?: string;
  denied_by?: string;
  reasons?: string[];
  overage_price_cents?: number;
  required_plan?: string;
  retry_after_s?: number;
}

export class SteloitError extends Error {
  readonly status: number;
  readonly remediation?: string;
  readonly problem: ProblemFields;
  constructor(problem: ProblemFields, status: number) {
    super(problem.title ? `${problem.title}${problem.detail ? " — " + problem.detail : ""}` : `request failed (${status})`);
    this.name = "SteloitError";
    this.status = status;
    this.remediation = problem.remediation;
    this.problem = problem;
  }
}

export class PermissionDeniedError extends SteloitError {
  readonly deniedBy?: string;
  constructor(p: ProblemFields, status: number) {
    super(p, status);
    this.name = "PermissionDeniedError";
    this.deniedBy = p.denied_by;
  }
}

export class ConflictError extends SteloitError {
  readonly reasons: string[];
  constructor(p: ProblemFields, status: number) {
    super(p, status);
    this.name = "ConflictError";
    this.reasons = p.reasons ?? [];
  }
}

export class QuotaExceededError extends SteloitError {
  readonly overagePriceCents?: number;
  readonly requiredPlan?: string;
  constructor(p: ProblemFields, status: number) {
    super(p, status);
    this.name = "QuotaExceededError";
    this.overagePriceCents = p.overage_price_cents;
    this.requiredPlan = p.required_plan;
  }
}

export class NotFoundError extends SteloitError {
  constructor(p: ProblemFields, status: number) {
    super(p, status);
    this.name = "NotFoundError";
  }
}

export class RateLimitedError extends SteloitError {
  readonly retryAfterS?: number;
  constructor(p: ProblemFields, status: number) {
    super(p, status);
    this.name = "RateLimitedError";
    this.retryAfterS = p.retry_after_s;
  }
}

function toError(body: unknown, status: number): SteloitError {
  const p = (body ?? {}) as ProblemFields;
  switch (status) {
    case 401:
    case 403:
      return new PermissionDeniedError(p, status);
    case 404:
      return new NotFoundError(p, status);
    case 409:
      return new ConflictError(p, status);
    case 402:
      return new QuotaExceededError(p, status);
    case 429:
      return new RateLimitedError(p, status);
    default:
      return new SteloitError(p, status);
  }
}

// unwrap turns the generated {data, error, response} result into data-or-throw.
function unwrap<T>(r: { data?: T; error?: unknown; response?: Response }): T {
  if (r.response?.ok && r.data !== undefined) return r.data;
  throw toError(r.error, r.response?.status ?? 0);
}

// ---- money (integer cents, one arithmetic — §3) -----------------------------

export function fmtMoney(cents: number): string {
  if (!Number.isInteger(cents)) throw new TypeError("money is integer cents (ADR-025)");
  const sign = cents < 0 ? "-" : "";
  const abs = Math.abs(cents);
  return abs % 100 === 0
    ? `${sign}$${abs / 100}`
    : `${sign}$${Math.floor(abs / 100)}.${String(abs % 100).padStart(2, "0")}`;
}

// ---- the client -------------------------------------------------------------

export interface SteloitOptions {
  apiUrl?: string;
  token: string;
  /** context carried, not repeated (mirrors --org/--project/--env) */
  org?: string;
  project?: string;
  env?: string;
  fetch?: typeof fetch;
}

async function* paginate<Item>(
  fetchPage: (cursor?: string) => Promise<{ data?: Item[]; next_cursor?: string | null }>,
): AsyncGenerator<Item> {
  let cursor: string | undefined;
  for (;;) {
    const page = await fetchPage(cursor);
    for (const item of page.data ?? []) yield item;
    if (!page.next_cursor) return;
    cursor = page.next_cursor;
  }
}

export class Steloit {
  readonly core: Client;
  private readonly ctx: { org?: string; project?: string; env?: string };
  private readonly apiUrl: string;
  private readonly token: string;
  private readonly fetchImpl: typeof fetch;

  constructor(opts: SteloitOptions) {
    this.apiUrl = (opts.apiUrl ?? "https://api.steloit.com").replace(/\/$/, "");
    this.token = opts.token;
    this.ctx = { org: opts.org, project: opts.project, env: opts.env };
    this.fetchImpl = opts.fetch ?? fetch;
    this.core = createClient(
      createConfig({
        baseUrl: this.apiUrl + "/v1",
        fetch: this.fetchImpl,
        headers: { Authorization: `Bearer ${opts.token}` },
      }),
    );
  }

  private need(name: "org" | "project" | "env", override?: string): string {
    const v = override ?? this.ctx[name];
    if (!v) throw new Error(`steloit: no ${name} in context — pass {${name}} or set it at construction`);
    return v;
  }

  // -- auth --
  readonly auth = {
    session: async () => unwrap(await gen.getSession({ client: this.core })),
  };

  // -- projects --
  readonly projects = {
    create: async (body: { name: string; region?: string }, ctx?: { org?: string }) =>
      unwrap(await gen.createProject({ client: this.core, path: { org: this.need("org", ctx?.org) }, body })),
    list: (ctx?: { org?: string }) =>
      paginate(async () =>
        unwrap(await gen.listProjects({ client: this.core, path: { org: this.need("org", ctx?.org) } })),
      ),
    get: async (project?: string) =>
      unwrap(await gen.getProject({ client: this.core, path: { project: this.need("project", project) } })),
  };

  // -- environments --
  readonly environments = {
    create: async (body: { name: string; region?: string }, ctx?: { project?: string }) =>
      unwrap(
        await gen.createEnvironment({ client: this.core, path: { project: this.need("project", ctx?.project) }, body }),
      ),
    list: (ctx?: { project?: string }) =>
      paginate(async () =>
        unwrap(await gen.listEnvironments({ client: this.core, path: { project: this.need("project", ctx?.project) } })),
      ),
  };

  // -- estimates (the gate: create → inspect → pass — no shortcut exists) --
  readonly estimates = {
    create: async (body: { env?: string; services?: ServiceShapeInput[] }): Promise<Estimate> =>
      unwrap(await gen.createEstimate({ client: this.core, body: { env: body.env ?? this.ctx.env, services: body.services } })),
  };

  // -- services (estimate-before-provision IS the method signature — §3) --
  readonly services = {
    create: async (
      body: ServiceShapeInput & { name: string; estimateId: string },
      ctx?: { env?: string },
    ) =>
      unwrap(
        await gen.createService({
          client: this.core,
          path: { env: this.need("env", ctx?.env) },
          body: {
            name: body.name,
            product: body.product,
            intent: body.intent,
            shape: body.shape,
            estimate_id: body.estimateId,
          },
        }),
      ),
    list: (ctx?: { env?: string }) =>
      paginate(async () =>
        unwrap(await gen.listServices({ client: this.core, path: { env: this.need("env", ctx?.env) } })),
      ),
    get: async (service: string) =>
      unwrap(await gen.getService({ client: this.core, path: { service } })),
    delete: async (service: string) => {
      const r = await gen.deleteService({ client: this.core, path: { service } });
      if (!r.response?.ok) throw toError(r.error, r.response?.status ?? 0);
    },
  };

  // -- bindings --
  readonly bindings = {
    create: async (source: string, body: { target: string; scope?: "read_only" | "read_write" }) =>
      unwrap(await gen.createBinding({ client: this.core, path: { service: source }, body })),
    delete: async (binding: string) => {
      const r = await gen.deleteBinding({ client: this.core, path: { binding } });
      if (!r.response?.ok) throw toError(r.error, r.response?.status ?? 0);
    },
  };

  // -- deployments --
  readonly deployments = {
    create: async (body: { service: string; gitSha?: string }, ctx?: { env?: string }) =>
      unwrap(
        await gen.createDeployment({
          client: this.core,
          path: { env: this.need("env", ctx?.env) },
          body: { service: body.service, git_sha: body.gitSha },
        }),
      ),
    list: (ctx?: { env?: string }) =>
      paginate(async () =>
        unwrap(await gen.listDeployments({ client: this.core, path: { env: this.need("env", ctx?.env) } })),
      ),
    rollback: async (dep: string) =>
      unwrap(await gen.rollbackDeployment({ client: this.core, path: { dep } })),
  };

  // -- tokens (reveal-once survives the SDK: the secret exists in the
  //    returned object and nowhere else — never cached, never logged) --
  readonly tokens = {
    create: async (body: { name: string; scope?: "read_only" | "full" }): Promise<TokenCreated> =>
      unwrap(await gen.createPersonalToken({ client: this.core, body })),
    list: () => paginate(async () => unwrap(await gen.listPersonalTokens({ client: this.core }))),
    revoke: async (tok: string) => {
      const r = await gen.revokePersonalToken({ client: this.core, path: { tok } });
      if (!r.response?.ok) throw toError(r.error, r.response?.status ?? 0);
    },
  };

  // -- events (list + SSE stream with cursor resume — §3 streaming) --
  readonly events = {
    list: async (ctx?: { env?: string; kind?: string }) =>
      unwrap(
        await gen.listEvents({
          client: this.core,
          path: { env: this.need("env", ctx?.env) },
          query: ctx?.kind ? { kind: ctx.kind } : undefined,
        }),
      ),
    stream: (opts?: { env?: string; kind?: string; signal?: AbortSignal }) =>
      this.streamEvents(opts),
  };

  private async *streamEvents(opts?: { env?: string; kind?: string; signal?: AbortSignal }) {
    const env = this.need("env", opts?.env);
    let cursor: string | undefined;
    for (;;) {
      const qs = new URLSearchParams();
      if (opts?.kind) qs.set("kind", opts.kind);
      if (cursor) qs.set("cursor", cursor);
      const url = `${this.apiUrl}/v1/envs/${env}/events${qs.size ? "?" + qs : ""}`;
      const resp = await this.fetchImpl(url, {
        headers: { Accept: "text/event-stream", Authorization: `Bearer ${this.token}` },
        signal: opts?.signal,
      });
      if (!resp.ok || !resp.body) {
        let problem: unknown;
        try {
          problem = await resp.json();
        } catch {
          problem = undefined;
        }
        throw toError(problem, resp.status);
      }
      const reader = resp.body.getReader();
      const decoder = new TextDecoder();
      let buf = "";
      let frame: { id?: string; event?: string; data?: string } = {};
      try {
        for (;;) {
          const { done, value } = await reader.read();
          if (done) break;
          buf += decoder.decode(value, { stream: true });
          let nl: number;
          while ((nl = buf.indexOf("\n")) >= 0) {
            const line = buf.slice(0, nl);
            buf = buf.slice(nl + 1);
            if (line.startsWith("id: ")) frame.id = line.slice(4);
            else if (line.startsWith("event: ")) frame.event = line.slice(7);
            else if (line.startsWith("data: ")) frame.data = line.slice(6);
            else if (line === "" && frame.data) {
              if (frame.id) cursor = frame.id;
              yield { id: frame.id, event: frame.event, data: JSON.parse(frame.data) };
              frame = {};
            }
          }
        }
      } finally {
        reader.releaseLock();
      }
      if (opts?.signal?.aborted) return;
      await new Promise((r) => setTimeout(r, 1000)); // reconnect, resuming from cursor
    }
  }
}
