import { createFileRoute, Outlet, useChildMatches } from "@tanstack/react-router";
import { SnavProduct } from "@/app/shell/snav-product";
import { useProject } from "@/features/projects/hooks";
import { useServices } from "@/features/services/hooks";

/** Service layout: product snav (variant B) + the selected tab. */
function ServiceLayout() {
  const { org, project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(env);
  const projectQuery = useProject(project);
  const childMatches = useChildMatches();

  const svc = services.data?.find((s) => s.name === service || s.id === service);
  if (!svc) {
    if (services.isPending) return <main className="main" />;
    return (
      <main className="main">
        <div className="pgpad">
          <h1 className="h1">Service not found</h1>
          <p className="hsub">
            No service named <span className="mono">{service}</span> in {env}. Check the rail, or
            switch environments via the crumb.
          </p>
        </div>
      </main>
    );
  }

  const leaf = childMatches[childMatches.length - 1]?.routeId ?? "";
  const active = leaf.includes("branches")
    ? "branches"
    : leaf.includes("insights")
      ? "insights"
      : "overview";

  return (
    <>
      <SnavProduct
        org={org}
        project={project}
        env={env}
        service={svc}
        serviceCount={(services.data ?? []).filter((s) => s.product === svc.product).length}
        projectTotalCents={projectQuery.data?.monthly_cost_cents}
        active={active}
      />
      <Outlet />
    </>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service")({
  validateSearch: (search: Record<string, unknown>): { env: string } => ({
    env: typeof search.env === "string" ? search.env : "production",
  }),
  component: ServiceLayout,
});
