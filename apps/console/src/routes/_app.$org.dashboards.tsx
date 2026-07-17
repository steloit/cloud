import { createFileRoute, Outlet, useChildMatches } from "@tanstack/react-router";
import { type DashboardsActive, SnavDashboards } from "@/app/shell/snav-dashboards";
import { useOrgs } from "@/features/org/hooks";

/**
 * Dashboards layout (DB1–DB8): the plane's snav + the selected page. A viewed
 * dashboard ($dashId, DB7) highlights "My dashboards" — that's where
 * checkout-health lives in the tree.
 */
function DashboardsLayout() {
  const { org } = Route.useParams();
  // Pass-6: the snav header names the org — derived via the standard useOrgs
  // lookup (slug or id), never hardcoded; the slug stands in until it resolves.
  const orgs = useOrgs();
  const orgName = orgs.data?.find((o) => o.slug === org || o.id === org)?.name;
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
      <SnavDashboards org={org} orgName={orgName} active={active} />
      <Outlet />
    </>
  );
}

export const Route = createFileRoute("/_app/$org/dashboards")({
  component: DashboardsLayout,
});
