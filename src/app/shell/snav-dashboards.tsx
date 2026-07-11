import { Icon } from "@/design-system/icon";
import { Nfoot, NitDisabled, NitLink, Nsec, Snav } from "./snav";

/**
 * Snav variant for the Dashboards plane (DB1–DB8): pre-built views generated
 * from the org's topology, plus yours. Phase-4 surfaces render disabled with
 * the stated reason — visible-but-disabled, never hidden (16-qa).
 */
export type DashboardsActive =
  | "overview"
  | "postgres-health"
  | "infrastructure"
  | "cost-usage"
  | "mine"
  | "layouts";

export function SnavDashboards({ org, active }: { org: string; active: DashboardsActive }) {
  const params = { org };
  return (
    <Snav>
      <div className="snhead">
        <span className="glyph">
          <Icon id="s-chart" />
        </span>
        <div>
          <div className="t">Dashboards</div>
          <div className="u">Acme · all projects</div>
        </div>
      </div>
      <NitLink to="/$org/dashboards" params={params} label="Overview" on={active === "overview"} />
      <Nsec>Pre-built</Nsec>
      <NitLink
        to="/$org/dashboards/postgres-health"
        params={params}
        icon="s-db"
        label="PostgreSQL Health"
        on={active === "postgres-health"}
      />
      <NitDisabled
        icon="s-chip"
        label="Valkey Performance"
        reason="Valkey Performance lands in Phase 4"
      />
      <NitDisabled icon="s-ai" label="AI Gateway" reason="AI Gateway dashboard lands in Phase 4" />
      <NitLink
        to="/$org/dashboards/infrastructure"
        params={params}
        icon="s-pulse"
        label="Infrastructure"
        on={active === "infrastructure"}
      />
      <NitLink
        to="/$org/dashboards/cost-usage"
        params={params}
        icon="s-card"
        label="Cost & Usage"
        on={active === "cost-usage"}
      />
      <NitDisabled
        icon="s-deploy"
        label="Deployments"
        reason="Deployments dashboard lands in Phase 4"
      />
      <Nsec>Yours</Nsec>
      <NitLink
        to="/$org/dashboards/mine"
        params={params}
        icon="s-user"
        label="My dashboards"
        count="3"
        on={active === "mine"}
      />
      <NitDisabled icon="s-grid" label="Shared" count="4" reason="rendered inside My & shared" />
      <NitLink
        to="/$org/dashboards/layouts"
        params={params}
        icon="s-doc"
        label="Templates"
        count="5"
        on={active === "layouts"}
      />
      <Nfoot>
        <div className="px-1.5">Pre-built are generated — always current</div>
      </Nfoot>
    </Snav>
  );
}
