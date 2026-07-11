import { createFileRoute, Link } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { MetricChart } from "@/design-system/chart";
import { Pill } from "@/design-system/pill";
import { FilterChips } from "@/features/dashboards/filter-chips";
import { toSeries, useMetrics } from "@/features/observe/hooks";

/**
 * DB2 · PostgreSQL Health — the fleet view, generated from topology.
 * Fleet rows are frame-fixed; only ecommerce/production services exist in
 * fixtures, so the staging and analytics-pipeline rows aren't navigable.
 */

const FLEET: Array<{
  instance: string;
  where: string;
  connections: string;
  connectionsWarn?: boolean;
  p95: string;
  replLag: string;
  storage: string;
  /** Canon service name when the row links into the product page. */
  service?: string;
}> = [
  {
    instance: "db-main",
    where: "ecommerce / production",
    connections: "192 / 200",
    connectionsWarn: true,
    p95: "38 ms",
    replLag: "0.2 s",
    storage: "12.4 GB",
    service: "db-main",
  },
  {
    instance: "db-reports",
    where: "ecommerce / production",
    connections: "14 / 100",
    p95: "122 ms",
    replLag: "—",
    storage: "4.1 GB",
    service: "db-reports",
  },
  {
    instance: "db-main",
    where: "ecommerce / staging",
    connections: "6 / 50",
    p95: "31 ms",
    replLag: "—",
    storage: "1.2 GB",
  },
  {
    instance: "events-db",
    where: "analytics-pipeline / production",
    connections: "22 / 100",
    p95: "54 ms",
    replLag: "0.1 s",
    storage: "38.6 GB",
  },
];

function PostgresHealth() {
  const { org } = Route.useParams();
  const connections = useMetrics("production", "service:db-main metric:connections");
  const p95 = useMetrics("production", "service:api metric:p95");

  return (
    <main className="main">
      <div className="pgpad">
        <Pghead
          title={
            <span className="flex items-center gap-2.5">
              PostgreSQL Health <Pill tone="st">pre-built</Pill>
            </span>
          }
          sub="Every PostgreSQL you run — across projects and environments — on one screen. Generated from your topology; always current."
        >
          <Btn variant="s" disabled disabledReason="Forking lands in Phase 4">
            Customize — forks a copy
          </Btn>
        </Pghead>

        <FilterChips />

        <div className="tblwrap">
          <table className="tbl">
            <thead>
              <tr>
                <th>Instance</th>
                <th>Where</th>
                <th>Connections</th>
                <th>p95 query</th>
                <th>Repl. lag</th>
                <th>Storage</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {FLEET.map((row) => (
                <tr key={`${row.instance}-${row.where}`}>
                  <td className="mono font-medium">{row.instance}</td>
                  <td className="text-ink2">{row.where}</td>
                  <td className={row.connectionsWarn ? "mono text-warn" : "mono"}>
                    {row.connections}
                  </td>
                  <td className="mono">{row.p95}</td>
                  <td className="mono">{row.replLag}</td>
                  <td className="mono">{row.storage}</td>
                  <td className="text-right">
                    {row.service ? (
                      <Link
                        to="/$org/$project/svc/$service"
                        params={{ org, project: "ecommerce", service: row.service }}
                        search={{ env: "production" }}
                      >
                        <Btn variant="gh">Open →</Btn>
                      </Link>
                    ) : (
                      <Btn variant="gh" disabled disabledReason="only canon services are navigable">
                        Open →
                      </Btn>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div className="grid grid-cols-2 gap-3.5">
          <Card className="flex flex-col gap-2 p-3.5">
            <span className="text-12p5 font-semibold">Connections · fleet</span>
            <MetricChart series={toSeries(connections.data)} tone="warn" size="md" />
            <span className="text-10p5 text-ink3">
              db-main nearing its 200 limit — the same signal the product page shows
            </span>
          </Card>
          <Card className="flex flex-col gap-2 p-3.5">
            <span className="text-12p5 font-semibold">p95 query time</span>
            <MetricChart series={toSeries(p95.data)} tone="steel" size="md" unit="ms" />
            <span className="text-10p5 text-ink3">per instance · overlaid</span>
          </Card>
        </div>

        <div className="mt-auto text-10p5 text-ink3">
          Depth stays on the product pages — Open → jumps to the instance. This view exists for the
          question they can't answer: how is the fleet?
        </div>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/dashboards/postgres-health")({
  component: PostgresHealth,
});
