import { createFileRoute, Link } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { MetricChart } from "@/design-system/chart";
import { Copybit } from "@/design-system/copybit";
import { Pill } from "@/design-system/pill";
import { Skeleton } from "@/design-system/skeleton";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { toMarkers, toSeries, useMetrics } from "@/features/observe/hooks";
import { ObserveChrome } from "@/features/observe/observe-chrome";
import { cn } from "@/lib/utils";

/**
 * O2 · Observe — Metrics: every pane is a query in the shared language.
 * Compute is the frame-specced tab; the Databases/Queue/Network/Cost
 * category tabs are unspecced by frames (a finding per tab) — their panes
 * ride the canon series honestly rather than faking new telemetry.
 *
 * NOTE: the O2 gallery frame shows a "Cost · $/hour" small pane, but the
 * canon telemetry (src/mocks/telemetry.ts) has no cost series — data follows
 * fixtures, so the third small pane renders db-main connections (a real
 * query) instead. The cost pane needs a cost series the canon lacks.
 */

const TABS = ["Compute", "Databases", "Queue", "Network", "Cost"];

/** Category-pane shell — the Compute small-pane grammar at md height. */
function ChartPane({
  title,
  stat,
  pending,
  children,
}: {
  title: string;
  stat: ReactNode;
  pending: boolean;
  children: ReactNode;
}) {
  return (
    <Card className="flex flex-col gap-1.5 p-3.5">
      <div className="flex items-center gap-2">
        <span className="text-11p5 font-medium text-ink2">{title}</span>
        <span className="flex-1" />
        {stat}
      </div>
      {pending ? <Skeleton className="h-[120px]" /> : children}
    </Card>
  );
}

/** One failure card per tab — its panes share the /metrics endpoint. */
function PanesFailure({
  env,
  panes,
}: {
  env: string;
  panes: Array<ReturnType<typeof useMetrics>>;
}) {
  const failed = panes.find((pane) => pane.isError);
  if (!failed) return null;
  return (
    <ApiFailureCard
      title="The panes didn't load"
      error={failed.error}
      requestLine={`GET /envs/${env}/metrics`}
      onRetry={() => {
        for (const pane of panes) if (pane.isError) pane.refetch();
      }}
    />
  );
}

// Finding: the Databases category tab is unspecced by frames — the charts are
// the canon db-main/cache series; the p95-read chips are the O1 frame stats.
function DatabasesTab({ env }: { env: string }) {
  const connections = useMetrics(env, "service:db-main metric:connections");
  const hitRate = useMetrics(env, "service:cache metric:hit_rate");
  const panes = [connections, hitRate];
  if (panes.some((pane) => pane.isError)) return <PanesFailure env={env} panes={panes} />;
  return (
    <>
      <div className="chiprow">
        <span className="chip">
          db-main conns <span className="mono">192/200</span>
        </span>
        <span className="chip">
          p95 read · db-main <span className="mono">2.1 ms</span>
        </span>
        <span className="chip">
          p95 read · db-reports <span className="mono">1.4 ms</span>
        </span>
        <span className="chip">
          cache hit <span className="mono">98.2%</span>
        </span>
      </div>
      <div className="grid grid-cols-2 gap-3.5">
        <ChartPane
          title="db-main · connections"
          stat={
            <span className="mono text-12 text-warn">
              {connections.isPending ? "…" : "192/200"}
            </span>
          }
          pending={connections.isPending}
        >
          <MetricChart
            series={toSeries(connections.data)}
            threshold={200}
            markers={toMarkers(connections.data)}
            tone="warn"
            size="md"
          />
        </ChartPane>
        <ChartPane
          title="cache · hit rate"
          stat={<span className="mono text-12">{hitRate.isPending ? "…" : "98.2%"}</span>}
          pending={hitRate.isPending}
        >
          <MetricChart series={toSeries(hitRate.data)} unit="%" tone="ok" size="md" />
        </ChartPane>
      </div>
    </>
  );
}

// Finding: the Queue category tab is unspecced by frames — both charts are
// canon jobs series; the chips are the frame-fixed queue stats (D8/O1).
function QueueTab({ env }: { env: string }) {
  const depth = useMetrics(env, "service:jobs metric:queue_depth");
  const dlq = useMetrics(env, "service:jobs metric:dlq_depth");
  const panes = [depth, dlq];
  if (panes.some((pane) => pane.isError)) return <PanesFailure env={env} panes={panes} />;
  return (
    <>
      <div className="chiprow">
        <span className="chip">
          depth <span className="mono">12</span>
        </span>
        <span className="chip">
          oldest ready msg <span className="mono">41 s</span>
        </span>
        <span className="chip">
          DLQ <span className="mono">2</span>
        </span>
      </div>
      <div className="grid grid-cols-2 gap-3.5">
        <ChartPane
          title="jobs · queue depth"
          stat={<span className="mono text-12 text-warn">{depth.isPending ? "…" : "12 ▲"}</span>}
          pending={depth.isPending}
        >
          <MetricChart
            series={toSeries(depth.data)}
            markers={toMarkers(depth.data)}
            tone="warn"
            size="md"
          />
        </ChartPane>
        <ChartPane
          title="jobs · DLQ depth"
          stat={<span className="mono text-12 text-warn">{dlq.isPending ? "…" : "2"}</span>}
          pending={dlq.isPending}
        >
          <MetricChart
            series={toSeries(dlq.data)}
            markers={toMarkers(dlq.data)}
            tone="warn"
            size="md"
          />
        </ChartPane>
      </div>
    </>
  );
}

// Finding: the Network category tab is unspecced by frames — both charts are
// canon api series. No egress pane: the canon has no egress series and this
// surface references no B-canon egress figure, so none is invented.
function NetworkTab({ env }: { env: string }) {
  const requests = useMetrics(env, "service:api metric:requests");
  const errorRate = useMetrics(env, "service:api metric:error_rate");
  const panes = [requests, errorRate];
  if (panes.some((pane) => pane.isError)) return <PanesFailure env={env} panes={panes} />;
  return (
    <>
      <div className="chiprow">
        <span className="chip">
          requests <span className="mono">214/s</span>
        </span>
        <span className="chip">
          error rate <span className="mono">0.4%</span>
        </span>
      </div>
      <div className="grid grid-cols-2 gap-3.5">
        <ChartPane
          title="api · requests"
          stat={<span className="mono text-12">{requests.isPending ? "…" : "214/s"}</span>}
          pending={requests.isPending}
        >
          <MetricChart
            series={toSeries(requests.data)}
            markers={toMarkers(requests.data)}
            tone="steel"
            size="md"
          />
        </ChartPane>
        <ChartPane
          title="api · error rate"
          stat={<span className="mono text-12">{errorRate.isPending ? "…" : "0.4%"}</span>}
          pending={errorRate.isPending}
        >
          <MetricChart series={toSeries(errorRate.data)} unit="%" tone="steel" size="md" />
        </ChartPane>
      </div>
    </>
  );
}

// Finding: the Cost category tab is unspecced by frames AND the canon has no
// cost series — the trend pane rides the steady request-rate shape as the
// honest stand-in (the DB4 cost-usage precedent); the chips are the canon
// $ figures every sidebar rolls up to.
function CostTab({ env }: { env: string }) {
  const requests = useMetrics(env, "service:api metric:requests");
  if (requests.isError) return <PanesFailure env={env} panes={[requests]} />;
  return (
    <>
      <div className="chiprow">
        <span className="chip">
          project ecommerce <span className="mono">$208/mo</span>
        </span>
        <span className="chip">
          org forecast <span className="mono">≈ $482</span>
        </span>
      </div>
      <div className="grid grid-cols-2 gap-3.5">
        <ChartPane
          title="cost · trend"
          stat={<span className="mono text-12">{requests.isPending ? "…" : "$208/mo"}</span>}
          pending={requests.isPending}
        >
          <MetricChart series={toSeries(requests.data)} tone="steel" size="md" />
        </ChartPane>
      </div>
      <p className="text-10p5 text-ink3">cost meters land with the billing telemetry (finding)</p>
    </>
  );
}

function MetricsPage() {
  const { org, project } = Route.useParams();
  const { env } = Route.useSearch();
  const [tab, setTab] = useState("Compute");

  const p95 = useMetrics(env, "service:api metric:p95");
  const errorRate = useMetrics(env, "service:api metric:error_rate");
  const queueDepth = useMetrics(env, "service:jobs metric:queue_depth");
  const connections = useMetrics(env, "service:db-main metric:connections");

  const smallPanes = [errorRate, queueDepth, connections];
  const smallPaneError = smallPanes.find((pane) => pane.isError);

  const linkParams = { org, project };
  const search = { env };

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <ObserveChrome env={env} lens="Metrics" />

        <div className="tabrow">
          {TABS.map((t) => (
            <button
              key={t}
              type="button"
              className={cn("tab", tab === t && "on")}
              onClick={() => setTab(t)}
            >
              {t}
            </button>
          ))}
        </div>

        {tab === "Databases" ? (
          <DatabasesTab env={env} />
        ) : tab === "Queue" ? (
          <QueueTab env={env} />
        ) : tab === "Network" ? (
          <NetworkTab env={env} />
        ) : tab === "Cost" ? (
          <CostTab env={env} />
        ) : (
          <>
            <div className="flex items-center gap-3">
              <Copybit>{"p95(http.request.duration){service=api,env=production}"}</Copybit>
              <span className="chip">compare: yesterday ▾</span>
            </div>

            <Card className="flex flex-col gap-2.5 p-4">
              <div className="flex items-center gap-2.5">
                <span className="text-12p5 font-semibold">api · p95 latency</span>
                <Pill tone="warn">812 ms · +37% vs yesterday</Pill>
                <span className="flex-1" />
                <span className="text-10p5 text-ink3">split by: route ▾ · ghost = yesterday</span>
              </div>
              {p95.isError ? (
                <ApiFailureCard
                  title="Metrics couldn't load"
                  error={p95.error}
                  requestLine="GET /metrics → 502 upstream · req_19fa4c · 2 retries attempted"
                  onRetry={() => p95.refetch()}
                />
              ) : null}
              <MetricChart
                series={toSeries(p95.data)}
                threshold={800}
                markers={toMarkers(p95.data)}
                unit="ms"
                tone="warn"
                size="lg"
              />
              <div className="flex items-center gap-4 text-11">
                <span className="text-ink2">— GET /orders (the regression)</span>
                <span className="text-steel">— all other routes</span>
              </div>
              <div className="flex items-center gap-2 border-hair border-t pt-2.5">
                <span className="text-10p5 text-ink3">
                  brush to zoom — the range updates every tab
                </span>
                <span className="flex-1" />
                <Link to="/$org/$project/observe/logs" params={linkParams} search={search}>
                  <Btn variant="s" className="h-6 px-2.5 text-10p5">
                    Logs for this range →
                  </Btn>
                </Link>
                <Btn
                  variant="s"
                  className="h-6 px-2.5 text-10p5"
                  disabled
                  disabledReason="The rule drawer (U8) lands in Phase 3"
                >
                  ⚑ Alert on this query
                </Btn>
              </div>
            </Card>

            {/* The three small panes hit the same /metrics endpoint as the hero,
                so they fail together — one shared failure card for the strip
                (four-state grammar, 16-qa); pending panes show a chart-height
                skeleton, never a blank strip. */}
            {smallPaneError ? (
              <ApiFailureCard
                title="The small panes didn't load"
                error={smallPaneError.error}
                requestLine={`GET /envs/${env}/metrics`}
                onRetry={() => {
                  for (const pane of smallPanes) if (pane.isError) pane.refetch();
                }}
              />
            ) : (
              <div className="grid grid-cols-3 gap-3.5">
                <Card className="flex flex-col gap-1.5 p-3.5">
                  <div className="flex items-center gap-2">
                    <span className="text-11p5 font-medium text-ink2">
                      Error rate · all services
                    </span>
                    <span className="flex-1" />
                    <span className="mono text-12">{errorRate.isPending ? "…" : "0.4%"}</span>
                  </div>
                  {errorRate.isPending ? (
                    <Skeleton className="h-20" />
                  ) : (
                    <MetricChart series={toSeries(errorRate.data)} tone="steel" size="sm" />
                  )}
                </Card>
                <Card className="flex flex-col gap-1.5 p-3.5">
                  <div className="flex items-center gap-2">
                    <span className="text-11p5 font-medium text-ink2">jobs · queue depth</span>
                    <span className="flex-1" />
                    <span className="mono text-12 text-warn">
                      {queueDepth.isPending ? "…" : "12 ▲"}
                    </span>
                  </div>
                  {queueDepth.isPending ? (
                    <Skeleton className="h-20" />
                  ) : (
                    <MetricChart series={toSeries(queueDepth.data)} tone="warn" size="sm" />
                  )}
                </Card>
                <Card className="flex flex-col gap-1.5 p-3.5">
                  <div className="flex items-center gap-2">
                    <span className="text-11p5 font-medium text-ink2">db-main · connections</span>
                    <span className="flex-1" />
                    <span className="mono text-12 text-warn">
                      {connections.isPending ? "…" : "192/200"}
                    </span>
                  </div>
                  {connections.isPending ? (
                    <Skeleton className="h-20" />
                  ) : (
                    <MetricChart series={toSeries(connections.data)} tone="warn" size="sm" />
                  )}
                </Card>
              </div>
            )}

            <div className="flex flex-col gap-1 text-10p5 text-ink3">
              <span>Every pane is a query — ⋯ copies it for CLI and alerts.</span>
              <span>Raw 15 d · downsampled 13 mo.</span>
              <span>
                Service pages (D9, D13) are these charts scoped to one instance — same engine, same
                grammar.
              </span>
            </div>
          </>
        )}
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/observe/metrics")({
  component: MetricsPage,
});
