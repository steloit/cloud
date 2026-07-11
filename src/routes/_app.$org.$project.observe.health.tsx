import { createFileRoute, Link } from "@tanstack/react-router";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { MetricChart } from "@/design-system/chart";
import { EmptyState } from "@/design-system/empty-state";
import { Eyebrow } from "@/design-system/eyebrow";
import { Dot, type DotTone, Pill } from "@/design-system/pill";
import { Skeleton } from "@/design-system/skeleton";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { toMarkers, toSeries, useMetrics } from "@/features/observe/hooks";
import { ObserveChrome } from "@/features/observe/observe-chrome";
import { useServices } from "@/features/services/hooks";
import { cn } from "@/lib/utils";

/**
 * O1 · Observe — Health: the seven-service scoreboard, the shared timeline,
 * SLOs as rules, cost as a signal, and the one firing alert. Per-service
 * metric lines are frame-fixed canon from the O1 frame.
 */

const HEALTH_LINES: Record<string, { tone: DotTone; l1: string; l2: string }> = {
  api: { tone: "warn", l1: "p95 812 ms ▲", l2: "err 0.4% · 3 inst" },
  worker: { tone: "ok", l1: "lag 1.2 s", l2: "concurrency 8" },
  jobs: { tone: "warn", l1: "depth 12 ▲", l2: "DLQ 2" },
  "db-main": { tone: "ok", l1: "p95 read 2.1 ms", l2: "conns 192/200" },
  "db-reports": { tone: "ok", l1: "p95 read 1.4 ms", l2: "fresh · low load" },
  cache: { tone: "ok", l1: "hit 98.2%", l2: "mem 61%" },
  assets: { tone: "ok", l1: "5xx 0.00%", l2: "egress 1.9 GB/d" },
};

const SLO_ROWS: Array<{ label: string; value: string; tone: "ok" | "warn" }> = [
  { label: "api availability ≥ 99.9%", value: "99.96%", tone: "ok" },
  { label: "api p95 ≤ 800 ms", value: "budget 58% · burning 3.1×", tone: "warn" },
  { label: "jobs delivery ≤ 60 s", value: "99.99%", tone: "ok" },
];

function HealthPage() {
  const { org, project } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(env);
  const p95 = useMetrics(env, "service:api metric:p95");
  const cost = useMetrics(env, "service:api metric:requests");

  const linkParams = { org, project };
  const search = { env };

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <ObserveChrome env={env} lens="Health" />

        {/* Four-state grammar (16-qa) on the scoreboard — the SLO rows, timeline
            and firing card below are frame-fixed canon and stay regardless. */}
        {services.isError ? (
          <ApiFailureCard
            title="Services didn't load"
            error={services.error}
            requestLine={`GET /envs/${env}/services`}
            onRetry={() => services.refetch()}
          />
        ) : services.isPending ? (
          <div className="grid grid-cols-7 gap-3">
            {Array.from({ length: 7 }, (_, i) => (
              // biome-ignore lint/suspicious/noArrayIndexKey: placeholder cards have no identity
              <Card key={i} className="flex flex-col gap-1 p-3">
                <Skeleton className="h-3 w-[70%]" />
                <Skeleton className="h-3 w-[85%]" />
                <Skeleton className="h-2.5 w-[55%]" />
              </Card>
            ))}
          </div>
        ) : (services.data ?? []).length === 0 ? (
          <EmptyState
            compact
            icon="s-pulse"
            title="No services in this environment"
            meaning={
              <>
                the scoreboard fills as services land here — one live card per service: latency,
                errors, saturation, on the shared timeline
              </>
            }
          />
        ) : (
          <div className="grid grid-cols-7 gap-3">
            {(services.data ?? []).map((s) => {
              const line = HEALTH_LINES[s.name];
              return (
                <Card key={s.id} className="flex flex-col gap-1 p-3">
                  <div className="flex items-center gap-1.5 text-12 font-semibold">
                    <Dot tone={line?.tone ?? "ok"} />
                    {s.name}
                  </div>
                  <div className={cn("text-11p5", line?.tone === "warn" && "text-warn")}>
                    {line?.l1 ?? s.product}
                  </div>
                  <div className="text-10p5 text-ink3">{line?.l2 ?? s.status}</div>
                </Card>
              );
            })}
          </div>
        )}

        <div className="flex gap-3.5">
          <div className="flex flex-1 flex-col gap-3.5">
            <Card className="flex flex-col gap-2.5 p-4">
              <div className="flex items-center gap-2.5">
                <Eyebrow>The shared timeline · 13:30 – 14:45</Eyebrow>
                <span className="text-10p5 text-ink3">
                  ◆ deploy · ◇ scale · ⬖ policy · every chart in every tab carries these
                </span>
              </div>
              <MetricChart
                series={toSeries(p95.data)}
                threshold={800}
                markers={toMarkers(p95.data)}
                unit="ms"
                tone="warn"
                size="lg"
              />
              <div className="flex items-center gap-2">
                <Pill tone="st">api · p95</Pill>
                <Pill tone="mut">overlay: jobs depth</Pill>
                <Pill tone="mut">overlay: cost/hr</Pill>
                <span className="flex-1" />
                <Link to="/$org/$project/observe/metrics" params={linkParams} search={search}>
                  <Btn variant="s">Open in Metrics →</Btn>
                </Link>
              </div>
            </Card>

            <div className="grid grid-cols-2 gap-3.5">
              <Card className="flex flex-col gap-2 p-4">
                <Eyebrow>SLOs · 30 d</Eyebrow>
                {SLO_ROWS.map((row) => (
                  <div key={row.label} className="flex items-center gap-2 text-11p5">
                    <span className="flex-1 text-ink2">{row.label}</span>
                    <span className={row.tone === "warn" ? "mono text-warn" : "mono text-ok"}>
                      {row.value}
                    </span>
                  </div>
                ))}
                <p className="border-hair border-t pt-2 text-10p5 text-ink3">
                  Burn rate &gt; 2× for 1 h auto-opens an alert — the SLO is a rule, not a poster.
                </p>
              </Card>
              <Card className="flex flex-col gap-2 p-4">
                <Eyebrow>Cost as a signal · $/day</Eyebrow>
                <MetricChart series={toSeries(cost.data)} tone="steel" size="sm" />
                <div className="text-11 text-ink2">+$0.80/d · db-reports created</div>
                <p className="border-hair border-t pt-2 text-10p5 text-ink3">
                  Cost is an observable here (the step has a cause); the meter and the invoice live
                  in Billing (B2, B3).
                </p>
              </Card>
            </div>
          </div>

          <div className="flex w-[340px] shrink-0 flex-col gap-3.5">
            <Card className="border-warn/40 flex flex-col gap-2 p-3.5">
              <Eyebrow className="text-warn">Firing now · 1</Eyebrow>
              <div className="text-12p5 font-semibold">api p95 &gt; 800 ms</div>
              <div className="mono text-10p5 text-ink3">
                p95-slo · since 14:02 · 41 min · evt_9d21c4
              </div>
              <div className="flex flex-wrap items-center gap-2 pt-1">
                <Link to="/$org/$project/observe/metrics" params={linkParams} search={search}>
                  <Btn variant="p" className="h-6 px-2.5 text-10p5">
                    Investigate
                  </Btn>
                </Link>
                <Link
                  to="/$org/$project/svc/$service/insights"
                  params={{ ...linkParams, service: "db-main" }}
                  search={search}
                >
                  <Btn variant="s" className="mono h-6 px-2.5 text-10p5">
                    prp_7c31a2
                  </Btn>
                </Link>
                <Btn
                  variant="gh"
                  className="h-6 px-2.5 text-10p5"
                  disabled
                  disabledReason="Silences land in Phase 3"
                >
                  Silence…
                </Btn>
              </div>
            </Card>

            <Card className="flex flex-col gap-2 p-3.5">
              <Eyebrow>Latest events</Eyebrow>
              <div className="flex items-center gap-2.5 text-11p5">
                <span className="mono text-ink3">14:03</span>
                <span className="flex items-center gap-1.5 text-ink2">
                  <Dot tone="warn" /> jobs · 2 → DLQ
                </span>
              </div>
              <div className="flex items-center gap-2.5 text-11p5">
                <span className="mono text-ink3">14:02</span>
                <span className="flex items-center gap-1.5 text-ink2">
                  <Dot tone="warn" /> alert p95-slo opened
                </span>
              </div>
              <div className="flex items-center gap-2.5 text-11p5">
                <span className="mono text-ink3">13:58</span>
                <span className="text-ink2">◆ deploy #142 · api · priya</span>
              </div>
              <div className="flex items-center gap-2.5 text-11p5">
                <span className="mono text-ink3">13:41</span>
                <span className="text-ink2">◇ api scaled 2 → 3 · autoscaler</span>
              </div>
              <Link
                to="/$org/$project/observe/events"
                params={linkParams}
                search={search}
                className="border-hair border-t pt-2 text-11 font-medium text-steel"
              >
                All events →
              </Link>
            </Card>
          </div>
        </div>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/observe/health")({
  component: HealthPage,
});
