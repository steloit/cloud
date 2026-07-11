import { createFileRoute, Link } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Spark } from "@/design-system/chart";
import { Eyebrow } from "@/design-system/eyebrow";
import { Pill } from "@/design-system/pill";
import { FilterChips } from "@/features/dashboards/filter-chips";
import { toSeries, useMetrics } from "@/features/observe/hooks";
import { useBillingOverview } from "@/features/org/hooks";
import { fmtMoney } from "@/lib/fmt";

/**
 * DB4 · Cost & Usage — spend as telemetry. The numbers come from the same
 * billing overview every sidebar renders (canon: $171.42 MTD → $482 forecast).
 */

function CostUsage() {
  const { org } = Route.useParams();
  const billing = useBillingOverview(org);
  // Cost sparklines ride canon telemetry — the fixtures have no cost series,
  // so the trend lines borrow the steady request-rate shape.
  const requests = useMetrics("production", "service:api metric:requests");

  return (
    <main className="main">
      <div className="pgpad">
        <Pghead
          title={
            <span className="flex items-center gap-2.5">
              Cost & Usage <Pill tone="st">pre-built</Pill>
            </span>
          }
          sub="Spend as telemetry — burn, trend and meters, watched like latency. The ledger and the contract live in Billing (B1); this page watches them."
        >
          <Link to="/$org/billing/overview" params={{ org }}>
            <Btn variant="s">Open Billing →</Btn>
          </Link>
        </Pghead>

        <FilterChips />

        <div className="grid grid-cols-3 gap-3.5">
          <Card className="flex flex-col gap-1.5 p-3.5">
            <Eyebrow>Month to date</Eyebrow>
            <span className="mono text-[19px] font-semibold">
              {billing.data ? fmtMoney(billing.data.mtd_cents ?? 0) : "…"}
            </span>
            <span className="text-[10.5px] text-ink3">$72.42 resources + $99 plan</span>
            <Spark series={toSeries(requests.data)} />
          </Card>
          <Card className="flex flex-col gap-1.5 p-3.5">
            <Eyebrow>Forecast · Jul</Eyebrow>
            <span className="mono text-[19px] font-semibold">
              {billing.data ? fmtMoney(billing.data.forecast_cents ?? 0) : "…"}{" "}
              <span className="text-[11px] font-normal text-ink3">± $6</span>
            </span>
            <span className="text-[10.5px] text-ink3">same arithmetic as every sidebar</span>
            <Spark series={toSeries(requests.data)} />
          </Card>
          <Card className="flex flex-col gap-1.5 p-3.5">
            <Eyebrow>Egress · included</Eyebrow>
            <span className="mono text-[19px] font-semibold text-warn">87%</span>
            <span className="text-[10.5px] text-ink3">
              87 of 100 GB — the B8 warning, as a widget
            </span>
            <div className="h-1.5 w-full overflow-hidden rounded-full bg-surface2">
              <div className="h-full rounded-full bg-warn" style={{ width: "87%" }} />
            </div>
          </Card>
        </div>

        <div className="grid grid-cols-2 gap-3.5">
          <Card className="flex flex-col gap-2 p-3.5">
            <span className="text-[12.5px] font-semibold">By project · trend</span>
            <Spark series={toSeries(requests.data)} width={200} height={36} />
            <span className="text-[10.5px] text-ink3">
              ecommerce $208 · analytics $96 · internal $41 · mobile-api $38
            </span>
          </Card>
          <Card className="flex flex-col gap-2 p-3.5">
            <span className="text-[12.5px] font-semibold">Anomaly watch</span>
            <span className="mono text-[19px] font-semibold text-ok">✓ quiet</span>
            <span className="text-[10.5px] text-ink3">
              nothing unusual — assets egress spike explained by the gift-card launch
            </span>
          </Card>
        </div>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/dashboards/cost-usage")({
  component: CostUsage,
});
