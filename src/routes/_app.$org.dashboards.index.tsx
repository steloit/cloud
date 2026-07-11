import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Spark } from "@/design-system/chart";
import { Eyebrow } from "@/design-system/eyebrow";
import { Glyph } from "@/design-system/glyph";
import type { IconId } from "@/design-system/icon";
import { Pill } from "@/design-system/pill";
import { FilterChips } from "@/features/dashboards/filter-chips";
import { NewDashboardModal } from "@/features/dashboards/new-dashboard-modal";
import { toSeries, useMetrics } from "@/features/observe/hooks";
import { useBillingOverview } from "@/features/org/hooks";
import { fmtMoney } from "@/lib/fmt";

/** DB1 · Dashboards home — pinned views + the generated pre-built shelf. */

type PrebuiltPath =
  | "/$org/dashboards/postgres-health"
  | "/$org/dashboards/infrastructure"
  | "/$org/dashboards/cost-usage";

const PREBUILT: Array<{
  icon: IconId;
  name: string;
  sub: string;
  to?: PrebuiltPath;
  reason?: string;
}> = [
  {
    icon: "s-db",
    name: "PostgreSQL Health",
    sub: "4 instances · fleet",
    to: "/$org/dashboards/postgres-health",
  },
  {
    icon: "s-chip",
    name: "Valkey Performance",
    sub: "1 instance",
    reason: "Valkey Performance lands in Phase 4",
  },
  {
    icon: "s-ai",
    name: "AI Gateway",
    sub: "3 models",
    reason: "AI Gateway dashboard lands in Phase 4",
  },
  {
    icon: "s-pulse",
    name: "Infrastructure",
    sub: "7 services",
    to: "/$org/dashboards/infrastructure",
  },
  {
    icon: "s-card",
    name: "Cost & Usage",
    sub: "watches Billing",
    to: "/$org/dashboards/cost-usage",
  },
  {
    icon: "s-deploy",
    name: "Deployments",
    sub: "releases & markers",
    reason: "Deployments dashboard lands in Phase 4",
  },
];

function PrebuiltCard({ org, item }: { org: string; item: (typeof PREBUILT)[number] }) {
  const body = (
    <Card
      className={
        item.to
          ? "flex h-full items-center gap-3 p-3.5 hover:border-ink3"
          : "flex h-full items-center gap-3 p-3.5 opacity-55"
      }
      title={item.reason}
    >
      <Glyph id={item.icon} />
      <div>
        <div className="text-12p5 font-semibold">{item.name}</div>
        <div className="text-10p5 text-ink3">{item.sub}</div>
      </div>
    </Card>
  );
  if (!item.to) return body;
  return (
    <Link to={item.to} params={{ org }}>
      {body}
    </Link>
  );
}

function DashboardsHome() {
  const { org } = Route.useParams();
  const [modalOpen, setModalOpen] = useState(false);
  const billing = useBillingOverview(org);

  // Pinned-card sparklines ride the canon telemetry — the same series the
  // linked dashboards render at full size.
  const p95 = useMetrics("production", "service:api metric:p95");
  const requests = useMetrics("production", "service:api metric:requests");

  return (
    <main className="main">
      <div className="pgpad">
        <Pghead
          title="Dashboards"
          sub="The platform at a glance, composed your way — pre-built views generated from what you run, plus your own. Filters apply to every widget at once; project-scoped dashboards arrive born filtered."
        >
          <Btn variant="p" onClick={() => setModalOpen(true)}>
            New dashboard (DB8)
          </Btn>
        </Pghead>

        <FilterChips />

        <Eyebrow>Pinned</Eyebrow>
        <div className="grid grid-cols-3 gap-3.5">
          <Link to="/$org/dashboards/$dashId" params={{ org, dashId: "checkout-health" }}>
            <Card className="flex h-full flex-col gap-2 p-3.5 hover:border-ink3">
              <div className="flex items-center gap-2">
                <span className="text-12p5 font-semibold">checkout-health</span>
                <Pill tone="mut">personal</Pill>
                <span className="ml-auto text-ink3">★</span>
              </div>
              <Spark series={toSeries(p95.data)} />
              <span className="mono text-11 text-ok">p95 396 ms · DLQ 0</span>
            </Card>
          </Link>
          <Link to="/$org/dashboards/infrastructure" params={{ org }}>
            <Card className="flex h-full flex-col gap-2 p-3.5 hover:border-ink3">
              <div className="flex items-center gap-2">
                <span className="text-12p5 font-semibold">Infrastructure</span>
                <Pill tone="st">pre-built</Pill>
                <span className="ml-auto text-ink3">★</span>
              </div>
              <Spark series={toSeries(requests.data)} tone="ok" />
              <span className="mono text-11 text-ok">all green · 7 services</span>
            </Card>
          </Link>
          <Link to="/$org/dashboards/cost-usage" params={{ org }}>
            <Card className="flex h-full flex-col gap-2 p-3.5 hover:border-ink3">
              <div className="flex items-center gap-2">
                <span className="text-12p5 font-semibold">Cost & Usage</span>
                <Pill tone="st">pre-built</Pill>
                <span className="ml-auto text-ink3">★</span>
              </div>
              <Spark series={toSeries(requests.data)} />
              <span className="mono text-11 text-ink2">
                MTD {billing.data ? fmtMoney(billing.data.mtd_cents ?? 0) : "…"} →{" "}
                {billing.data ? fmtMoney(billing.data.forecast_cents ?? 0) : "…"}
              </span>
            </Card>
          </Link>
        </div>

        <Eyebrow>Pre-built · generated from what you run</Eyebrow>
        <div className="grid grid-cols-3 gap-3.5">
          {PREBUILT.map((item) => (
            <PrebuiltCard key={item.name} org={org} item={item} />
          ))}
        </div>

        <div className="mt-auto text-10p5 text-ink3">
          Pre-built dashboards are views, not files — they update as services come and go. Customize
          one and it forks into a copy under My dashboards.
        </div>
      </div>
      {modalOpen ? <NewDashboardModal org={org} onClose={() => setModalOpen(false)} /> : null}
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/dashboards/")({
  component: DashboardsHome,
});
