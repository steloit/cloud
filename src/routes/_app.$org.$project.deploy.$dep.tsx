import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { type DeployMarker, MetricChart, type SeriesPoint } from "@/design-system/chart";
import { Eyebrow } from "@/design-system/eyebrow";
import { Icon } from "@/design-system/icon";
import { Dot, Pill, Stlab } from "@/design-system/pill";
import { Skeleton } from "@/design-system/skeleton";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { queryMetricsOptions } from "@/lib/api";

/**
 * DP2 · Rollout detail — the live view of #143 promoting to production.
 * The canary-vs-baseline chart reads the canon p95 telemetry; everything else
 * is frame-fixed DP2 microcopy (the rollout state machine lands in Phase 3).
 */

const GUARD_REASON = "Pause/abort are guarded — they land with live rollouts in Phase 3";

/** ISO "…T14:50…" → minute-of-day for the chart's x-domain. */
function minuteOf(at: string): number {
  const m = at.match(/T(\d{2}):(\d{2})/);
  if (!m) return 0;
  return Number(m[1]) * 60 + Number(m[2]);
}

function RolloutPage() {
  const { org, project, dep } = Route.useParams();
  const { env } = Route.useSearch();

  const metrics = useQuery(
    queryMetricsOptions({ path: { env }, query: { query: "service:api metric:p95" } }),
  );

  if (dep !== "dep_143" && dep !== "143") {
    return (
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Card className="flex flex-col items-start gap-2.5 p-4">
            <b className="text-13">
              Only in-flight rollouts have a live view — #143 is the one promoting
            </b>
            <Link to="/$org/$project/deploy" params={{ org, project }} search={{ env }}>
              <Btn variant="s">Back to Deployments</Btn>
            </Link>
          </Card>
        </div>
      </main>
    );
  }

  const points = metrics.data?.series?.[0]?.points ?? [];
  const series: SeriesPoint[] = points.map(([at, v]) => ({ t: minuteOf(at), v }));
  const markers: DeployMarker[] = (metrics.data?.markers ?? []).map((m) => ({
    t: minuteOf(m.at ?? ""),
    label: m.label ?? "",
  }));

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          title="#143 · fix/orders-query → production"
          sub={
            <>
              Progressive rollout · started 14:50 by priya · same image that soaked 9 min in staging
              · <span className="mono">c92e11</span>
            </>
          }
        >
          <Pill tone="st">canary · 33% of traffic</Pill>
          <Btn variant="s" disabled disabledReason={GUARD_REASON}>
            Pause
          </Btn>
          <Btn variant="dgr" disabled disabledReason={GUARD_REASON}>
            Abort…
          </Btn>
        </Pghead>

        {/* Step strip — the five rollout stages, left to right. */}
        <Card className="grid grid-cols-5 gap-3 p-4">
          <div className="flex items-start gap-2 text-11p5">
            <Icon id="s-check" className="h-3 w-3 text-ok" />
            <span>
              <b>Build</b> — <span className="text-ink3">41 s · image st-143</span>
            </span>
          </div>
          <div className="flex items-start gap-2 text-11p5">
            <Icon id="s-check" className="h-3 w-3 text-ok" />
            <span>
              <b>Migrate</b> —{" "}
              <span className="text-ink3">0142_add_idx · CONCURRENTLY · reverse stated</span>
            </span>
          </div>
          <div className="flex items-start gap-2 text-11p5">
            <span className="pt-1">
              <Dot tone="prov" />
            </span>
            <b>Canary 33% — 1 of 3 instances · 2 min elapsed</b>
          </div>
          <div className="flex items-start gap-2 text-11p5 text-ink3">
            <span>·</span>
            <span>Full rollout — waits for gates · no fixed timer</span>
          </div>
          <div className="flex items-start gap-2 text-11p5 text-ink3">
            <span>·</span>
            <span>Verify — 10 min watch, then #142 instances released</span>
          </div>
        </Card>

        <div className="flex gap-3.5">
          <div className="flex min-w-0 flex-1 flex-col gap-3.5">
            <Card className="flex flex-col gap-2.5 p-4">
              <Eyebrow>
                Instances — replacement, not restart: old serves until new is proven
              </Eyebrow>
              <div className="flex items-center gap-2 text-11p5">
                <Dot tone="prov" />
                <span className="mono">inst-a · #143</span>
                <span className="text-ink3">— canary · 33% traffic · p95 431 ms</span>
              </div>
              <div className="flex items-center gap-2 text-11p5">
                <Dot tone="ok" />
                <span className="mono">inst-b · #142</span>
                <span className="text-ink3">— serving · 34% · next to replace</span>
              </div>
              <div className="flex items-center gap-2 text-11p5">
                <Dot tone="ok" />
                <span className="mono">inst-c · #142</span>
                <span className="text-ink3">— serving · 33%</span>
              </div>
            </Card>

            <Card className="flex flex-col gap-2.5 p-4">
              <div className="flex items-baseline gap-3">
                <Eyebrow>Canary vs baseline · live</Eyebrow>
                <span className="mono ml-auto text-10p5 text-ink3">
                  <span className="text-warn">—</span> #143 canary ·{" "}
                  <span className="text-ink3">—</span> #142 baseline
                </span>
              </div>
              {/* Four-state grammar (16-qa) on the only query-backed region of
                  DP2 — the chart; no empty state, the canon telemetry always
                  has the p95 series once it loads. */}
              {metrics.isPending ? (
                <Skeleton className="h-[120px]" />
              ) : metrics.isError ? (
                <ApiFailureCard
                  title="Canary telemetry didn't load"
                  error={metrics.error}
                  requestLine={`GET /envs/${env}/metrics · service:api metric:p95`}
                  onRetry={() => metrics.refetch()}
                />
              ) : (
                <MetricChart
                  series={series}
                  threshold={800}
                  markers={markers}
                  unit="ms"
                  tone="warn"
                />
              )}
              <p className="text-11 leading-relaxed text-ink3">
                The fix visible before it's fully shipped — idx_orders_customer_created (proposal
                prp_7c31a2, applied as a reversible migration).
              </p>
            </Card>
          </div>

          <div className="flex w-[400px] shrink-0 flex-col gap-3.5">
            <Card className="flex flex-col gap-2 p-4">
              <Eyebrow>Health gates — the rollout advances only past these</Eyebrow>
              <table className="tbl">
                <thead>
                  <tr>
                    <th>Gate</th>
                    <th>Canary</th>
                    <th>Baseline</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  <tr>
                    <td>p95 latency</td>
                    <td className="mono text-ok">431 ms</td>
                    <td className="mono">812 ms</td>
                    <td>
                      <Stlab tone="ok">passing</Stlab>
                    </td>
                  </tr>
                  <tr>
                    <td>Error rate</td>
                    <td className="mono">0.2%</td>
                    <td className="mono">0.4%</td>
                    <td>
                      <Stlab tone="ok">passing</Stlab>
                    </td>
                  </tr>
                  <tr>
                    <td>5xx</td>
                    <td className="mono">0</td>
                    <td className="mono">0</td>
                    <td>
                      <Stlab tone="ok">passing</Stlab>
                    </td>
                  </tr>
                </tbody>
              </table>
              <p className="border-hair border-t pt-2 text-10p5 leading-relaxed text-ink3">
                Auto-abort: error rate &gt; 2× baseline for 2 min, or any 5xx spike → traffic
                returns to #142 instances in &lt; 10 s. The migration stays — expand-contract means
                #142 runs happily on the new schema. #140 was aborted by exactly this gate.
              </p>
            </Card>

            <Card className="flex flex-col gap-2.5 p-4">
              <Eyebrow>Rollout log</Eyebrow>
              <div className="logwell">
                <div>
                  <span className="t">14:50:02</span> promotion started · plan: 33 → 100
                </div>
                <div>
                  <span className="t">14:50:04</span> migration 0142_add_idx · CONCURRENTLY · 1.8 s
                  ✓
                </div>
                <div>
                  <span className="t">14:50:41</span> inst-a replaced · #143 · health ✓
                </div>
                <div>
                  <span className="t">14:50:52</span> canary receiving 33% · gates armed
                </div>
                <div className="lv-w">
                  <span className="t">14:52:00</span> watching · p95 canary 431 ms (−47%) · alert
                  p95-slo trending to resolve
                </div>
              </div>
              <p className="text-10p5 leading-relaxed text-ink3">
                Every line lands in Observe · Events; the deploy is one ◆ across all charts.
              </p>
            </Card>
          </div>
        </div>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/deploy/$dep")({
  component: RolloutPage,
});
