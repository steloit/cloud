import { createFileRoute, Outlet, useChildMatches } from "@tanstack/react-router";
import { type DashboardsActive, SnavDashboards } from "@/app/shell/snav-dashboards";

/**
 * Dashboards layout (DB1–DB8): the plane's snav + the selected page. A viewed
 * dashboard ($dashId, DB7) highlights "My dashboards" — that's where
 * checkout-health lives in the tree.
 */
function DashboardsLayout() {
  const { org } = Route.useParams();
  const childMatches = useChildMatches();

  const leaf = childMatches[childMatches.length - 1]?.routeId ?? "";
  const active: DashboardsActive = leaf.includes("postgres-health")
    ? "postgres-health"
    : leaf.includes("infrastructure")
      ? "infrastructure"
      : leaf.includes("cost-usage")
        ? "cost-usage"
        : leaf.includes("layouts")
          ? "layouts"
          : leaf.includes("mine") || leaf.includes("$dashId")
            ? "mine"
            : "overview";

  return (
    <>
      <SnavDashboards org={org} active={active} />
      <Outlet />
    </>
  );
}

export const Route = createFileRoute("/_app/$org/dashboards")({
  component: DashboardsLayout,
});
