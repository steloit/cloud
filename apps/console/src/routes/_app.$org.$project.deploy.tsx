import { createFileRoute, Outlet, useChildMatches } from "@tanstack/react-router";
import { SnavDeploy } from "@/app/shell/snav-deploy";

/** Deploy layout: domain snav + the selected leaf (DP1/DP2/DP3). */
function DeployLayout() {
  const { org, project } = Route.useParams();
  const { env } = Route.useSearch();
  const childMatches = useChildMatches();

  const leaf = childMatches[childMatches.length - 1]?.routeId ?? "";
  const active = leaf.includes("previews") ? ("previews" as const) : ("deployments" as const);

  return (
    <>
      <SnavDeploy org={org} project={project} env={env} active={active} />
      <Outlet />
    </>
  );
}

export const Route = createFileRoute("/_app/$org/$project/deploy")({
  validateSearch: (search: Record<string, unknown>): { env: string } => ({
    env: typeof search.env === "string" ? search.env : "production",
  }),
  component: DeployLayout,
});
