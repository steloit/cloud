import { createFileRoute } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { MetricChart } from "@/design-system/chart";
import { Pill } from "@/design-system/pill";
import { FilterChips } from "@/features/dashboards/filter-chips";
import { toMarkers, toSeries, useMetrics } from "@/features/observe/hooks";

/** DB3 · Infrastructure Overview — golden signals, deploy markers everywhere. */

function InfrastructureOverview() {
  const p95 = useMetrics("production", "service:api metric:p95");
  const errorRate = useMetrics("production", "service:api metric:error_rate");
  const requests = useMetrics("production", "service:api metric:requests");
  const connections = useMetrics("production", "service:db-main metric:connections");
  const dlq = useMetrics("production", "service:jobs metric:queue_depth");

  return (
    <main className="main">
      <div className="pgpad">
        <Pghead
          title={
            <span className="flex items-center gap-2.5">
              Infrastructure Overview <Pill tone="st">pre-built</Pill>
            </span>
          }
          sub={`Golden signals across every service — the "Production Health" view that ends tab-jumping. Deploys drawn on every chart.`}
        >
          <Btn variant="s" disabled disabledReason="Forking lands in Phase 4">
            Customize — forks a copy
          </Btn>
        </Pghead>

        <FilterChips defaultOn={["All projects ▾", "production ▾"]} />

        <div className="grid grid-cols-2 gap-3.5">
          <Card className="flex flex-col gap-2 p-3.5">
            <span className="text-[12.5px] font-semibold">Latency · p95 by service</span>
            <MetricChart
              series={toSeries(p95.data)}
              threshold={800}
              markers={toMarkers(p95.data)}
              unit="ms"
              height={120}
            />
            <span className="text-[10.5px] text-ink3">api spiked at #142, settled after #143</span>
          </Card>
          <Card className="flex flex-col gap-2 p-3.5">
            <span className="text-[12.5px] font-semibold">Error rate</span>
            <MetricChart
              series={toSeries(errorRate.data)}
              markers={toMarkers(errorRate.data)}
              tone="warn"
              height={120}
            />
            <span className="text-[10.5px] text-ink3">order.paid failures during the incident</span>
          </Card>
        </div>

        <div className="grid grid-cols-3 gap-3.5">
          <Card className="flex flex-col gap-2 p-3.5">
            <span className="text-[12.5px] font-semibold">Throughput · req/s</span>
            <MetricChart series={toSeries(requests.data)} tone="ok" height={80} />
            <span className="text-[10.5px] text-ink3">steady</span>
          </Card>
          <Card className="flex flex-col gap-2 p-3.5">
            <span className="text-[12.5px] font-semibold">Saturation</span>
            <MetricChart series={toSeries(connections.data)} tone="warn" height={80} />
            <span className="text-[10.5px] text-ink3">
              valkey memory 95% — the assistant's open insight
            </span>
          </Card>
          <Card className="flex flex-col gap-2 p-3.5">
            <span className="text-[12.5px] font-semibold">Queue depth · DLQ</span>
            <MetricChart series={toSeries(dlq.data)} height={80} />
            <span className="text-[10.5px] text-ink3">2 dead letters resolved via #143</span>
          </Card>
        </div>

        <Card className="p-3.5 text-[11.5px] text-ink2">
          Deploys · production: <span className="mono">#143</span> idempotent gift-card receipts ·
          Jul 2 — markers on every widget above, so "what changed?" is never a second investigation.
        </Card>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/dashboards/infrastructure")({
  component: InfrastructureOverview,
});
