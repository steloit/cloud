import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Eyebrow } from "@/design-system/eyebrow";
import { Glyph } from "@/design-system/glyph";
import { Icon } from "@/design-system/icon";
import { Kbd } from "@/design-system/kbd";
import { Metric } from "@/design-system/metric";
import { Dot, Pill, statusDotTone } from "@/design-system/pill";
import { Skeleton } from "@/design-system/skeleton";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { useBillingOverview } from "@/features/org/hooks";
import { useEnvironments, useProject } from "@/features/projects/hooks";
import { useEvents, useServices } from "@/features/services/hooks";
import type { Binding, Event, Service } from "@/lib/api";
import { listBindingsOptions } from "@/lib/api";
import { fmtMoney, fmtTime } from "@/lib/fmt";
import { cn } from "@/lib/utils";

/**
 * W3 · Project overview (canvas rework) — vitals, environments, full-width
 * topology with the incident path, attention + event spine. Sidebar-less
 * canvas shape (ADR-018); env arrives via ?env= only (ADR-013).
 */

/** Node positions for the canon topology (W3). Unknown services fall back to a grid. */
const NODE_POS: Record<string, { x: number; y: number }> = {
  api: { x: 30, y: 70 },
  worker: { x: 30, y: 210 },
  cache: { x: 330, y: 10 },
  "db-main": { x: 360, y: 120 },
  jobs: { x: 330, y: 240 },
  assets: { x: 640, y: 30 },
  "db-reports": { x: 660, y: 190 },
};

/** Node subtitle microcopy — verbatim from the W3 frame for canon services. */
const NODE_SUB: Record<string, string> = {
  api: "Web · p95 812 ms · 3 inst",
  worker: "Worker · 42 jobs/min",
  jobs: "Queue · depth 12 · DLQ 2",
  "db-main": "PostgreSQL 16 · conns 96%",
  cache: "Valkey · hit 98.2%",
  assets: "Storage · 48.1 GB",
  "db-reports": "PostgreSQL 16 · fresh · $24/mo",
};

/** Edge labels — verbatim from the W3 frame, keyed by canon binding id. */
const EDGE_LABEL: Record<string, string> = {
  bnd_api_dbmain: "role: rw · 642 ms here",
  bnd_worker_dbmain: "role: rw",
  bnd_worker_dbreports: "role: ro · bnd_worker_dbreports",
  bnd_api_cache: "role: cache",
  bnd_api_assets: "role: write",
  bnd_api_jobs: "publish",
  bnd_worker_jobs: "consume",
};

const NODE_W = 158;
const NODE_H = 50;

function nodePos(name: string, index: number) {
  return NODE_POS[name] ?? { x: 40 + (index % 4) * 220, y: 30 + Math.floor(index / 4) * 110 };
}

function Topology({
  services,
  bindings,
  env,
}: {
  services: Service[];
  bindings: Binding[];
  env: string;
}) {
  const byId = new Map(services.map((s) => [s.id, s]));
  const indexOf = new Map(services.map((s, i) => [s.name, i]));

  return (
    <Card className="relative flex-1 overflow-hidden p-4">
      <div className="flex items-center gap-2.5">
        <Eyebrow>Topology · {env}</Eyebrow>
        <Pill tone="mut">bindings are edges</Pill>
        <Pill tone="warn">alert riding the api → db-main edge</Pill>
      </div>
      <div className="relative mt-3 h-[300px]">
        <svg className="absolute inset-0 h-full w-full" aria-hidden="true">
          {bindings.map((b) => {
            const src = byId.get(b.source_id);
            const dst = byId.get(b.target_id);
            if (!src || !dst) return null;
            const a = nodePos(src.name, indexOf.get(src.name) ?? 0);
            const c = nodePos(dst.name, indexOf.get(dst.name) ?? 0);
            const x1 = a.x + NODE_W;
            const y1 = a.y + NODE_H / 2;
            const x2 = c.x;
            const y2 = c.y + NODE_H / 2;
            const incident = b.id === "bnd_api_dbmain";
            return (
              <g key={b.id}>
                <line
                  x1={x1}
                  y1={y1}
                  x2={x2}
                  y2={y2}
                  stroke={incident ? "var(--warn)" : "var(--hair)"}
                  strokeWidth={incident ? 1.8 : 1.2}
                />
                <text
                  x={(x1 + x2) / 2}
                  y={(y1 + y2) / 2 - 5}
                  fill={incident ? "var(--warn)" : "var(--ink3)"}
                  fontSize="9"
                  fontFamily="JetBrains Mono Variable, JetBrains Mono, monospace"
                  textAnchor="middle"
                >
                  {EDGE_LABEL[b.id] ?? `role: ${b.scope === "read_only" ? "ro" : "rw"}`}
                </text>
              </g>
            );
          })}
        </svg>
        {services.map((s, i) => {
          const pos = nodePos(s.name, i);
          return (
            <div key={s.id} className="node" style={{ left: pos.x, top: pos.y }}>
              <div className="nn">
                <Dot tone={statusDotTone(s.status)} />
                {s.name}
              </div>
              <div className="nt">{NODE_SUB[s.name] ?? `${s.product} · ${s.status}`}</div>
            </div>
          );
        })}
      </div>
      <p className="mt-2 text-10p5 text-ink3">
        Nodes are the rail's services; click one to enter its domain. The amber path is today's
        incident: ◆ #142 13:58 → p95 14:02 → dead letters 14:03.
      </p>
    </Card>
  );
}

/** Event spine lines — verbatim W3 strings for canon events, derived otherwise. */
function describeEvent(e: Event): React.ReactNode {
  switch (e.id) {
    case "evt_dep143":
      return (
        <>
          ◆ deploy <b>#143</b> · api · {e.actor} · canary 33%
        </>
      );
    case "evt_apply":
      return (
        <>
          proposal <span className="mono">prp_7c31a2</span> applied · {e.actor}, via assistant
        </>
      );
    case "evt_dash":
      return (
        <>
          dashboard <b>checkout-health</b> created · {e.actor}
        </>
      );
    case "evt_dlq2":
      return <>2 messages → DLQ · order.paid</>;
    case "evt_9d21c4":
      return (
        <>
          alert <b>p95-slo</b> opened
        </>
      );
    case "evt_dep142":
      return (
        <>
          ◆ deploy <b>#142</b> · api · {e.actor}
        </>
      );
    default:
      return e.action ?? e.kind;
  }
}

/** E1 · Empty project — nothing here yet, by design; shown when zero services exist. */
function EmptyProject({ org, project, env }: { org: string; project: string; env: string }) {
  return (
    <>
      <Card dashed className="flex flex-col items-center gap-3 py-14">
        <Glyph id="s-hex" />
        <div className="text-14 font-semibold">Nothing here yet — by design</div>
        <p className="max-w-[440px] text-center text-11p5 leading-relaxed text-ink3">
          Add your first service or start from a template. You'll see the monthly estimate before
          anything exists, and backups, metrics and alerts come included.
        </p>
        <div className="flex gap-2">
          <Link to="/$org/create" params={{ org }} search={{ env }}>
            <Btn variant="p">
              <Icon id="s-plus" />
              Add service
            </Btn>
          </Link>
          <Btn variant="s" disabled disabledReason="Templates gallery (C5) — see New project">
            Start from template
          </Btn>
        </div>
        <Copybit>{`steloit db create db-main --project ${project}`}</Copybit>
      </Card>
      <div className="grid grid-cols-3 gap-3.5">
        <Card className="flex flex-col gap-2 p-4">
          <div className="text-12p5 font-semibold">No deployments yet</div>
          <p className="text-11p5 leading-relaxed text-ink3">
            Connect a repo and every push becomes an immutable release.
          </p>
          <span className="mono mt-auto text-10p5 text-ink3">steloit init</span>
        </Card>
        <Card className="flex flex-col gap-2 p-4">
          <div className="text-12p5 font-semibold">Nothing to observe</div>
          <p className="text-11p5 leading-relaxed text-ink3">
            Dashboards, logs and default alerts appear with your first service — no setup.
          </p>
          <span className="mt-auto text-10p5 text-ink3">included, not configured</span>
        </Card>
        <Card className="flex flex-col gap-2 p-4">
          <div className="text-12p5 font-semibold">No branches yet</div>
          <p className="text-11p5 leading-relaxed text-ink3">
            Database branches unlock once PostgreSQL exists — one per preview, in seconds.
          </p>
          <span className="mono mt-auto text-10p5 text-ink3">steloit db branch create</span>
        </Card>
      </div>
    </>
  );
}

/**
 * W3 pending — the populated layout's shape, no claims: until the services
 * query resolves we can't assert vitals, emptiness or a topology (the shell
 * previously rendered with an empty canvas — a lie).
 */
function ProjectSkeleton() {
  return (
    <>
      <div className="grid grid-cols-5 gap-3" aria-hidden="true">
        {Array.from({ length: 5 }, (_, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: placeholder cells have no identity
          <Skeleton key={i} className="h-[64px]" />
        ))}
      </div>
      <div className="grid grid-cols-4 gap-3" aria-hidden="true">
        {Array.from({ length: 4 }, (_, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: placeholder cells have no identity
          <Skeleton key={i} className="h-[88px]" />
        ))}
      </div>
      <div className="flex gap-3.5" aria-hidden="true">
        <Skeleton className="h-[396px] flex-1" />
        <div className="flex w-[352px] shrink-0 flex-col gap-3">
          <Skeleton className="h-[196px]" />
          <Skeleton className="h-[188px]" />
        </div>
      </div>
    </>
  );
}

function ProjectOverview() {
  const { org, project } = Route.useParams();
  const { env } = Route.useSearch();
  const projectQuery = useProject(project);
  const environments = useEnvironments(project);
  const services = useServices(env);
  const events = useEvents(env);
  const billing = useBillingOverview(org);
  const apiService = services.data?.find((s) => s.name === "api");
  const bindings = useQuery({
    ...listBindingsOptions({ path: { service: apiService?.id ?? "" } }),
    select: (r) => r.data ?? [],
    enabled: Boolean(apiService),
  });
  const workerService = services.data?.find((s) => s.name === "worker");
  const workerBindings = useQuery({
    ...listBindingsOptions({ path: { service: workerService?.id ?? "" } }),
    select: (r) => r.data ?? [],
    enabled: Boolean(workerService),
  });

  const allBindings = [
    ...(bindings.data ?? []),
    ...(workerBindings.data ?? []).filter((b) => !(bindings.data ?? []).some((x) => x.id === b.id)),
  ];

  const svcList = services.data ?? [];
  const degraded = svcList.filter((s) => s.status === "degraded").length;
  const mtd = billing.data?.by_project?.find((p) => p.name === project)?.mtd_cents;
  const estCents = projectQuery.data?.monthly_cost_cents ?? 0;
  const orgForecast = billing.data?.forecast_cents;
  const latestEvents = [...(events.data ?? [])].sort((a, b) => (a.at < b.at ? 1 : -1)).slice(0, 4);

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          title={project}
          sub={
            services.isSuccess
              ? svcList.length > 0
                ? `${svcList.length} services · ${degraded} degraded · viewing ${env} via the crumb ▾ · ${mtd !== undefined ? fmtMoney(mtd) : "$0"} so far this month · ${fmtMoney(estCents)}/mo estimated across environments`
                : "No services yet — every price is shown before anything exists."
              : "…"
          }
        >
          <span className="chip">
            <Kbd>⌘K</Kbd> jump anywhere
          </span>
          <Link to="/$org/create" params={{ org }} search={{ env }}>
            <Btn variant="p">
              <Icon id="s-plus" />
              Add service
            </Btn>
          </Link>
        </Pghead>

        {/* Four-state grammar (16-qa) on the services query, which gates the
            whole canvas: skeleton while unknown, failure card on error, E1
            empty project, else the populated shell. */}
        {services.isError ? (
          <ApiFailureCard
            title="Services didn't load"
            error={services.error}
            requestLine={`GET /envs/${env}/services`}
            onRetry={() => services.refetch()}
          />
        ) : services.isPending ? (
          <ProjectSkeleton />
        ) : svcList.length === 0 ? (
          <EmptyProject org={org} project={project} env={env} />
        ) : (
          <>
            {/* Vitals — the four telemetry cells are frame-fixed canon (W3: the
            incident numbers); the env-scoped metrics live under Observe; per-project vitals stay frame-fixed. */}
            <div className="grid grid-cols-5 gap-3">
              <Metric label="Requests" value="214/s" note="+6% vs yesterday" />
              <Metric label="p95" value="812 ms" tone="warn" note="SLO 800 ms · ▲ since #142" />
              <Metric label="Error rate" value="0.4%" note="30 m window" />
              <Metric label="Queue depth" value="12" tone="warn" note="DLQ 2 · order.paid" />
              <Metric
                label="Cost"
                mono
                value={`${mtd !== undefined ? fmtMoney(mtd) : "$0"} mtd`}
                note={`${fmtMoney(estCents)}/mo est · of org ${orgForecast !== undefined ? fmtMoney(orgForecast) : "…"}`}
              />
            </div>

            <div className="grid grid-cols-4 gap-3">
              {/* The env cards are query-backed: skeletons while pending, a
                  warn line if the query fails — the rest of the canvas stands
                  on the services query and needn't fall with this one. */}
              {environments.isPending ? (
                Array.from({ length: 3 }, (_, i) => (
                  // biome-ignore lint/suspicious/noArrayIndexKey: placeholder cards have no identity
                  <Skeleton key={i} className="h-[88px]" />
                ))
              ) : environments.isError ? (
                <div className="col-span-3 flex items-center text-10p5 text-warn">
                  environments didn't load — switch via the env crumb ▾, or reload
                </div>
              ) : null}
              {(environments.data ?? []).map((e) => {
                const flagged = (e.policy_flags ?? []).length > 0;
                const isHome = e.name === "production";
                return (
                  <Card
                    key={e.id}
                    className={cn("flex flex-col gap-1.5 p-3", flagged && "border-warn/40")}
                  >
                    <div className="flex items-center gap-2">
                      <span className="text-12p5 font-semibold">{e.name}</span>
                      {isHome ? <Pill tone="mut">home</Pill> : null}
                      {flagged ? <Pill tone="warn">policy-flagged</Pill> : null}
                    </div>
                    <div className="mono text-13">{fmtMoney(e.monthly_cost_cents ?? 0)}/mo</div>
                    <div className="text-10p5 text-ink3">
                      {e.region.replace("/", " · ")} · {svcList.length} services
                      {e.name === "production" && degraded > 0 ? " · 1 alert firing" : ""}
                      {e.kind === "preview" && e.expires_at ? " · expires in 5 d · marco" : ""}
                    </div>
                  </Card>
                );
              })}
              <Link to="/$org/$project/new-env" params={{ org, project }}>
                <Card
                  dashed
                  className="flex h-full flex-col items-center justify-center gap-1 p-3 hover:border-ink3"
                >
                  <span className="text-12p5 font-medium text-ink2">⊕ New environment</span>
                  <span className="text-10p5 text-ink3">or manage via the env crumb ▾</span>
                </Card>
              </Link>
            </div>

            <div className="flex gap-3.5">
              {/* The topology's edges ride the bindings queries — a canvas of
                  nodes with silently-missing edges would misstate the wiring,
                  so it holds a skeleton until they resolve (isLoading, not
                  isPending: the queries are disabled until api/worker exist). */}
              {bindings.isLoading || workerBindings.isLoading ? (
                <Skeleton className="h-[396px] flex-1" />
              ) : (
                <Topology services={svcList} bindings={allBindings} env={env} />
              )}
              <div className="flex w-[352px] shrink-0 flex-col gap-3">
                <Card className="flex flex-col gap-2.5 p-3.5">
                  <Eyebrow className="text-warn">Needs attention · 4</Eyebrow>
                  <div className="flex items-center gap-2 text-11p5">
                    <span className="flex-1">
                      <b>api</b> p95 812 ms — 4 min after deploy #142
                    </span>
                    <Link
                      to="/$org/$project/observe/health"
                      params={{ org, project }}
                      search={{ env }}
                    >
                      <Btn variant="s" className="h-6 px-2 text-10p5">
                        Observe
                      </Btn>
                    </Link>
                  </div>
                  <div className="flex items-center gap-2 text-11p5">
                    <span className="flex-1">
                      <b>jobs</b>: 2 dead letters · <span className="mono">"receipt": null</span>
                    </span>
                    {/* Pass-6: the DLQ view exists — the jobs Messages tab (D8);
                        the stale "lands in Phase 3" gate was a lie. */}
                    <Link
                      to="/$org/$project/svc/$service/messages"
                      params={{ org, project, service: "jobs" }}
                      search={{ env }}
                    >
                      <Btn variant="s" className="h-6 px-2 text-10p5">
                        Open DLQ
                      </Btn>
                    </Link>
                  </div>
                  <div className="flex items-center gap-2 text-11p5">
                    <span className="flex-1">
                      branch <span className="mono">preview/pr-142</span> flagged by{" "}
                      <span className="mono">branch-data-masking</span>
                    </span>
                    {/* Pass-6: honest gate — the org rulebook page exists, but the
                        spec has no per-policy view to open branch-data-masking on
                        (in canon it's the G7 draft, not yet a listed policy). */}
                    <Btn
                      variant="gh"
                      className="h-6 px-2 text-10p5"
                      disabled
                      disabledReason="No per-policy view in the spec (finding) — the rulebook lives in Settings → Policies"
                    >
                      Policy
                    </Btn>
                  </div>
                  <div className="flex items-center gap-2 text-11p5">
                    <span className="flex-1">
                      assistant drafted <span className="mono">prp_7c31a2</span> — the index fix
                    </span>
                    <Link
                      to="/$org/$project/svc/$service/insights"
                      params={{ org, project, service: "db-main" }}
                      search={{ env }}
                    >
                      <Btn variant="p" className="h-6 px-2 text-10p5">
                        Review
                      </Btn>
                    </Link>
                  </div>
                </Card>
                <Card className="flex flex-col gap-2 p-3.5">
                  <Eyebrow>Latest events</Eyebrow>
                  {latestEvents.map((e) => (
                    <div key={e.id} className="flex gap-2.5 text-11p5">
                      <span className="mono text-ink3">{fmtTime(e.at)}</span>
                      <span className="text-ink2">{describeEvent(e)}</span>
                    </div>
                  ))}
                  <Link
                    to="/$org/$project/observe/events"
                    params={{ org, project }}
                    search={{ env }}
                    className="border-hair border-t pt-2 text-11 font-medium text-steel"
                  >
                    All events → Observe
                  </Link>
                </Card>
              </div>
            </div>
          </>
        )}
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/")({
  validateSearch: (search: Record<string, unknown>): { env: string } => ({
    env: typeof search.env === "string" ? search.env : "production",
  }),
  component: ProjectOverview,
});
