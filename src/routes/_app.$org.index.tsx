import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { SnavOrg } from "@/app/shell/snav-org";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Glyph } from "@/design-system/glyph";
import { Icon } from "@/design-system/icon";
import { healthDotTone, Pill, Stlab } from "@/design-system/pill";
import { useBillingOverview, useOrgs } from "@/features/org/hooks";
import { useProjects } from "@/features/projects/hooks";
import type { Project } from "@/lib/api";
import { relTime } from "@/lib/canon/now";
import { fmtMoney } from "@/lib/fmt";

/** W1 · Projects home — the org's front door. */

function healthLabel(health: Project["health"]): string {
  // ADR-024 status language: degraded, never "running"/"down".
  if (health === "warn") return "degraded";
  if (health === "err") return "failed";
  return "healthy";
}

function regionLabel(region: string): string {
  // UI renders provider-faceted regions with dots: "aws · ap-south-1".
  return region.replace("/", " · ");
}

function ProjectCard({
  org,
  project,
  mtdCents,
  homeRegion,
}: {
  org: string;
  project: Project;
  mtdCents?: number;
  homeRegion: string;
}) {
  const health = project.health ?? "ok";
  return (
    <Link
      to="/$org/$project"
      params={{ org, project: project.name }}
      search={{ env: "production" }}
    >
      <Card
        className={`flex h-full flex-col gap-3 p-4 hover:border-ink3 ${health === "warn" ? "border-steel" : ""}`}
      >
        <div className="flex items-start gap-2.5">
          <div className="flex-1">
            <div className="text-[13.5px] font-semibold">{project.name}</div>
            <div className="mono mt-0.5 text-[10.5px] text-ink3">
              {org}/{project.name} · {regionLabel(homeRegion)}
            </div>
          </div>
          <Stlab tone={healthDotTone(health)}>{healthLabel(health)}</Stlab>
        </div>
        <div className="flex gap-6">
          <span className="metric">
            <span className="mv block">{project.env_count ?? 0}</span>
            <span className="ml">{project.env_count === 1 ? "environment" : "environments"}</span>
          </span>
          <span className="metric">
            <span className="mv mono block">{fmtMoney(project.monthly_cost_cents ?? 0)}</span>
            <span className="ml">/mo estimated</span>
          </span>
          <span className="metric">
            <span className="mv mono block">
              {mtdCents !== undefined ? fmtMoney(mtdCents) : "$0"}
            </span>
            <span className="ml">this month</span>
          </span>
        </div>
        <div className="mt-auto flex items-center gap-2 border-hair border-t pt-2.5 text-[11px] text-ink3">
          {project.last_deployment?.id ? (
            <>
              <span className="mono text-ink2">
                {project.last_deployment.id.replace(/^dep_/, "#")}
              </span>
              deployed {project.last_deployment.at ? relTime(project.last_deployment.at) : ""}
            </>
          ) : (
            <span>no deployments yet</span>
          )}
          <span className="ml-auto">
            {health === "warn" ? <Pill tone="err">1 alert firing</Pill> : null}
          </span>
        </div>
      </Card>
    </Link>
  );
}

function ProjectsHome() {
  const { org } = Route.useParams();
  const navigate = useNavigate();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);
  const projects = useProjects(org);
  const billing = useBillingOverview(org);

  const mtdByProject = new Map(
    (billing.data?.by_project ?? []).map((p) => [p.project_id, p.mtd_cents]),
  );

  return (
    <>
      <SnavOrg
        org={org}
        orgName={orgRecord?.name ?? org}
        projects={projects.data ?? []}
        billing={billing.data}
        active="all-projects"
      />
      <main className="main">
        <div className="pgpad">
          <Pghead
            title="Projects"
            sub={
              projects.data
                ? `${projects.data.length} projects · ${billing.data?.forecast_cents !== undefined ? fmtMoney(billing.data.forecast_cents) : "…"} this month across the organization`
                : "Loading…"
            }
          >
            <Copybit>steloit init</Copybit>
            <Btn variant="p" onClick={() => navigate({ to: "/$org/new-project", params: { org } })}>
              <Icon id="s-plus" />
              New project
            </Btn>
          </Pghead>

          {projects.isPending ? (
            <div className="grid grid-cols-2 gap-3.5">
              {[0, 1, 2, 3].map((i) => (
                <Card key={i} className="h-[150px] animate-pulse bg-surface2" />
              ))}
            </div>
          ) : (projects.data ?? []).length === 0 ? (
            <Card dashed className="flex flex-col items-center gap-3 py-14">
              <Glyph id="s-hex" />
              <div className="text-[13px] font-medium">
                Projects are free — services are priced.
              </div>
              <Btn
                variant="p"
                onClick={() => navigate({ to: "/$org/new-project", params: { org } })}
              >
                <Icon id="s-plus" />
                New project
              </Btn>
              <Copybit>steloit project create my-app</Copybit>
            </Card>
          ) : (
            <div className="grid grid-cols-2 gap-3.5">
              {(projects.data ?? []).map((p) => (
                <ProjectCard
                  key={p.id}
                  org={org}
                  project={p}
                  mtdCents={mtdByProject.get(p.id)}
                  homeRegion={orgRecord?.home_region ?? "aws/ap-south-1"}
                />
              ))}
            </div>
          )}

          <div className="mt-auto flex items-center gap-2.5 text-[11.5px] text-ink3">
            Everything here is one command away:
            <Copybit>steloit project list</Copybit>
          </div>
        </div>
      </main>
    </>
  );
}

export const Route = createFileRoute("/_app/$org/")({
  component: ProjectsHome,
});
