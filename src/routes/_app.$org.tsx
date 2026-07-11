import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Outlet, useLocation, useParams, useSearch } from "@tanstack/react-router";
import { Ctx } from "@/app/shell/ctx";
import { Rail, type RailActive } from "@/app/shell/rail";
import { useOrgs } from "@/features/org/hooks";
import { useEnvironments, useProjects } from "@/features/projects/hooks";
import type { Product } from "@/lib/api";
import { listServicesOptions } from "@/lib/api";
import { resolveEnvKey } from "@/lib/canon-env";

/**
 * The org shell: context bar + icon rail (ADR-011). The rail renders the
 * active project's actual products; env switching never reshuffles it
 * (ADR-013) — env is only ever the ?env= query param.
 */
function OrgShell() {
  const { org: orgParam } = Route.useParams();
  const childParams = useParams({ strict: false }) as { project?: string; service?: string };
  const search = useSearch({ strict: false }) as { env?: string };
  const location = useLocation();

  const orgs = useOrgs();
  const projects = useProjects(orgParam);
  const org = orgs.data?.find((o) => o.slug === orgParam || o.id === orgParam);

  const activeProjectName = childParams.project ?? projects.data?.[0]?.name;
  const activeProject = projects.data?.find((p) => p.name === activeProjectName);
  const environments = useEnvironments(activeProject?.name ?? "");
  const envName = search.env ?? "production";
  const envValid = environments.data?.some((e) => e.name === envName) ?? false;

  const services = useQuery({
    ...listServicesOptions({ path: { env: resolveEnvKey(activeProject?.name, envName) } }),
    select: (r) => r.data ?? [],
    enabled: envValid,
  });

  if (!org) return null;

  const path = location.pathname;
  const activeService = services.data?.find((s) => s.name === childParams.service);
  const active: RailActive = path.includes("/settings")
    ? { kind: "settings" }
    : path.includes("/observe")
      ? { kind: "observe" }
      : path.includes("/deploy")
        ? { kind: "deploy" }
        : path.endsWith("/create")
          ? { kind: "create" }
          : childParams.service && activeService
            ? { kind: "product", product: activeService.product as Product }
            : { kind: "home" };

  return (
    <div className="flex h-screen flex-col bg-canvas">
      <Ctx
        orgs={orgs.data ?? []}
        org={org}
        project={childParams.project ? activeProject : undefined}
        projects={projects.data ?? []}
        environments={environments.data ?? []}
        env={childParams.project ? envName : undefined}
        searchPlaceholder={
          childParams.service ? "Search or jump…" : "Search projects, services, docs…"
        }
        showAssistant={Boolean(childParams.service)}
      />
      <div className="fbody">
        <Rail
          org={org.slug}
          project={activeProject?.name}
          env={childParams.project ? envName : undefined}
          services={services.data ?? []}
          active={active}
        />
        <Outlet />
      </div>
    </div>
  );
}

export const Route = createFileRoute("/_app/$org")({
  component: OrgShell,
});
