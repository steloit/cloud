import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { MetricChart } from "@/design-system/chart";
import { Copybit } from "@/design-system/copybit";
import { Pill } from "@/design-system/pill";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { toMarkers, toSeries, useMetrics } from "@/features/observe/hooks";
import { ObserveChrome } from "@/features/observe/observe-chrome";
import { cn } from "@/lib/utils";

/**
 * O2 · Observe — Metrics: every pane is a query in the shared language.
 * Only the Compute tab has telemetry behind it today; the other categories
 * say so honestly rather than faking panes.
 *
 * NOTE: the O2 gallery frame shows a "Cost · $/hour" small pane, but the
 * canon telemetry (src/mocks/telemetry.ts) has no cost series — data follows
 * fixtures, so the third small pane renders db-main connections (a real
 * query) instead. The cost pane needs a cost series the canon lacks.
 */

const TABS = ["Compute", "Databases", "Queue", "Network", "Cost"];

function MetricsPage() {
  const { org, project } = Route.useParams();
  const { env } = Route.useSearch();
  const [tab, setTab] = useState("Compute");

  const p95 = useMetrics(env, "service:api metric:p95");
  const errorRate = useMetrics(env, "service:api metric:error_rate");
  const queueDepth = useMetrics(env, "service:jobs metric:queue_depth");
  const connections = useMetrics(env, "service:db-main metric:connections");

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

        {tab !== "Compute" ? (
          <p className="text-11p5 text-ink3">This category's panes land with more telemetry</p>
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

            <div className="grid grid-cols-3 gap-3.5">
              <Card className="flex flex-col gap-1.5 p-3.5">
                <div className="flex items-center gap-2">
                  <span className="text-11p5 font-medium text-ink2">Error rate · all services</span>
                  <span className="flex-1" />
                  <span className="mono text-12">0.4%</span>
                </div>
                <MetricChart series={toSeries(errorRate.data)} tone="steel" size="sm" />
              </Card>
              <Card className="flex flex-col gap-1.5 p-3.5">
                <div className="flex items-center gap-2">
                  <span className="text-11p5 font-medium text-ink2">jobs · queue depth</span>
                  <span className="flex-1" />
                  <span className="mono text-12 text-warn">12 ▲</span>
                </div>
                <MetricChart series={toSeries(queueDepth.data)} tone="warn" size="sm" />
              </Card>
              <Card className="flex flex-col gap-1.5 p-3.5">
                <div className="flex items-center gap-2">
                  <span className="text-11p5 font-medium text-ink2">db-main · connections</span>
                  <span className="flex-1" />
                  <span className="mono text-12 text-warn">192/200</span>
                </div>
                <MetricChart series={toSeries(connections.data)} tone="warn" size="sm" />
              </Card>
            </div>

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
