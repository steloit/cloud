import { createFileRoute } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { MetricChart } from "@/design-system/chart";
import { Pill } from "@/design-system/pill";
import { Skeleton } from "@/design-system/skeleton";
import { FilterChips } from "@/features/dashboards/filter-chips";
import { ApiFailureCard } from "@/features/errors/failure-states";
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
              {/* Finding: frame DB3 titles this pane "Infrastructure Overview" — the
                  design-system "Area · Thing" h1 grammar wins per the audit's P1 ruling. */}
              Dashboards · Infrastructure <Pill tone="st">pre-built</Pill>
            </span>
          }
          sub={`Golden signals across every service — the "Production Health" view that ends tab-jumping. Deploys drawn on every chart.`}
        >
          <Btn
            variant="s"
            disabled
            disabledReason="Forking lands with a dashboard-fork endpoint the API lacks (finding)"
          >
            Customize — forks a copy
          </Btn>
        </Pghead>

        <FilterChips defaultOn={["All projects", "production"]} />

        {/* Four-state grammar on the chart sections — O2's grouping: one
            failure card per section (the golden-signals pair, then the small
            strip), never one per pane. Per-chart skeletons while pending. */}
        {p95.isError || errorRate.isError ? (
          <ApiFailureCard
            title="Golden signals didn't load"
            error={p95.error ?? errorRate.error}
            requestLine="GET /envs/production/metrics · p95, error_rate"
            onRetry={() => {
              p95.refetch();
              errorRate.refetch();
            }}
          />
        ) : (
          <div className="grid grid-cols-2 gap-3.5">
            <Card className="flex flex-col gap-2 p-3.5">
              <span className="text-12p5 font-semibold">Latency · p95 by service</span>
              {p95.isPending ? (
                <Skeleton className="h-[120px]" />
              ) : (
                <MetricChart
                  series={toSeries(p95.data)}
                  threshold={800}
                  markers={toMarkers(p95.data)}
                  unit="ms"
                  size="md"
                />
              )}
              <span className="text-10p5 text-ink3">api spiked at #142, settled after #143</span>
            </Card>
            <Card className="flex flex-col gap-2 p-3.5">
              <span className="text-12p5 font-semibold">Error rate</span>
              {errorRate.isPending ? (
                <Skeleton className="h-[120px]" />
              ) : (
                <MetricChart
                  series={toSeries(errorRate.data)}
                  markers={toMarkers(errorRate.data)}
                  tone="warn"
                  size="md"
                />
              )}
              <span className="text-10p5 text-ink3">order.paid failures during the incident</span>
            </Card>
          </div>
        )}

        {requests.isError || connections.isError || dlq.isError ? (
          <ApiFailureCard
            title="Throughput, saturation and queue panes didn't load"
            error={requests.error ?? connections.error ?? dlq.error}
            requestLine="GET /envs/production/metrics · requests, connections, queue_depth"
            onRetry={() => {
              requests.refetch();
              connections.refetch();
              dlq.refetch();
            }}
          />
        ) : (
          <div className="grid grid-cols-3 gap-3.5">
            <Card className="flex flex-col gap-2 p-3.5">
              <span className="text-12p5 font-semibold">Throughput · req/s</span>
              {requests.isPending ? (
                <Skeleton className="h-[80px]" />
              ) : (
                <MetricChart series={toSeries(requests.data)} tone="ok" size="sm" />
              )}
              <span className="text-10p5 text-ink3">steady</span>
            </Card>
            <Card className="flex flex-col gap-2 p-3.5">
              <span className="text-12p5 font-semibold">Saturation</span>
              {connections.isPending ? (
                <Skeleton className="h-[80px]" />
              ) : (
                <MetricChart series={toSeries(connections.data)} tone="warn" size="sm" />
              )}
              <span className="text-10p5 text-ink3">
                valkey memory 95% — the assistant's open insight
              </span>
            </Card>
            <Card className="flex flex-col gap-2 p-3.5">
              <span className="text-12p5 font-semibold">Queue depth · DLQ</span>
              {dlq.isPending ? (
                <Skeleton className="h-[80px]" />
              ) : (
                <MetricChart series={toSeries(dlq.data)} size="sm" />
              )}
              <span className="text-10p5 text-ink3">2 dead letters resolved via #143</span>
            </Card>
          </div>
        )}

        <Card className="p-3.5 text-11p5 text-ink2">
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
