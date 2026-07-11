import { useQuery } from "@tanstack/react-query";
import {
  createFileRoute,
  Link,
  Outlet,
  useLocation,
  useParams,
  useSearch,
} from "@tanstack/react-router";
import { Ctx } from "@/app/shell/ctx";
import { Rail, type RailActive } from "@/app/shell/rail";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Skeleton, SkeletonLines } from "@/design-system/skeleton";
import { ApiFailureCard } from "@/features/errors/failure-states";
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

  // Four-state shell: skeleton while orgs load, an API-failure card on error,
  // a designed not-found for an unknown slug — never a blank screen.
  if (orgs.isPending) {
    return (
      <div className="flex h-screen flex-col bg-canvas">
        <header className="ctx">
          <Skeleton className="h-5 w-44" />
          <span className="flex-1" />
          <Skeleton className="h-5 w-64" />
        </header>
        <div className="fbody">
          <nav className="rail" aria-label="Products">
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} className="mt-2 h-7 w-7 rounded-lg" />
            ))}
          </nav>
          <main className="main">
            <div className="pgpad">
              <SkeletonLines lines={4} className="max-w-[520px]" />
            </div>
          </main>
        </div>
      </div>
    );
  }
  if (orgs.isError) {
    return (
      <div className="flex h-screen flex-col items-center justify-center bg-canvas">
        <div className="w-[560px]">
          <ApiFailureCard
            title="Your organizations didn't load"
            error={orgs.error}
            requestLine="GET /orgs"
            onRetry={() => orgs.refetch()}
          />
        </div>
      </div>
    );
  }
  if (!org) {
    return (
      <div className="flex h-screen flex-col items-center justify-center bg-canvas">
        <Card className="flex w-[480px] flex-col gap-3 p-6">
          <b className="text-14">
            No organization named <span className="mono">{orgParam}</span>
          </b>
          <p className="text-11p5 leading-relaxed text-ink3">
            Check the URL — org slugs are lowercase. If you were invited here, the invite email
            carries the right link.
          </p>
          <div>
            <Link to="/">
              <Btn variant="p">Go to your organization</Btn>
            </Link>
          </div>
        </Card>
      </div>
    );
  }

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
          : childParams.service === "gateway" || childParams.service === "ai-gateway"
            ? { kind: "product", product: "ai-gateway" }
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
