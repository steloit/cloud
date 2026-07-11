import { createFileRoute, Outlet, useChildMatches } from "@tanstack/react-router";
import { SnavProduct } from "@/app/shell/snav-product";
import { useProject } from "@/features/projects/hooks";
import { GatewayPage } from "@/features/services/gateway-page";
import { useServices } from "@/features/services/hooks";
import { resolveEnvKey } from "@/lib/canon-env";

/** Service layout: product snav (variant B) + the selected tab. */
function ServiceLayout() {
  const { org, project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(resolveEnvKey(project, env));
  const projectQuery = useProject(project);
  const childMatches = useChildMatches();

  const svc = services.data?.find((s) => s.name === service || s.id === service);
  // The leaf segment after $service/ is the active snav key; index = overview.
  const leafId = childMatches[childMatches.length - 1]?.routeId ?? "";
  const segment = leafId.split("/$service/")[1] ?? "";
  const active = segment === "" || segment === "/" ? "overview" : segment.replaceAll("/", "");
  if (!svc) {
    if (services.isPending) return <main className="main" />;
    // X1 exemplar — see gateway-page.tsx for the canon-arithmetic finding.
    if (service === "gateway" || service === "ai-gateway") {
      return <GatewayPage org={org} project={project} />;
    }
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

  return (
    <>
      <SnavProduct
        org={org}
        project={project}
        env={env}
        service={svc}
        siblings={(services.data ?? []).filter((s) => s.product === svc.product)}
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
