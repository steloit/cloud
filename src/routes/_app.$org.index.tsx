import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { SnavOrg } from "@/app/shell/snav-org";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Glyph } from "@/design-system/glyph";
import { Icon } from "@/design-system/icon";
import { healthDotTone, Pill, Stlab } from "@/design-system/pill";
import { ApiFailureCard } from "@/features/errors/failure-states";
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
            <div className="text-13 font-semibold">{project.name}</div>
            <div className="mono mt-0.5 text-10p5 text-ink3">
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
        <div className="mt-auto flex items-center gap-2 border-hair border-t pt-2.5 text-11 text-ink3">
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
                ? projects.data.length === 0
                  ? "Your organization is ready — nothing is provisioned, nothing is billing"
                  : `${projects.data.length} projects · ${billing.data?.forecast_cents !== undefined ? fmtMoney(billing.data.forecast_cents) : "…"} this month across the organization`
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
          ) : projects.isError ? (
            // A failed query is an error, never "you have no projects".
            <ApiFailureCard
              title="Projects didn't load"
              error={projects.error}
              requestLine={`GET /orgs/${org}/projects`}
              onRetry={() => projects.refetch()}
            />
          ) : (projects.data ?? []).length === 0 ? (
            // E5 · Zero projects — the org's front door before anything exists.
            <>
              <Card dashed className="flex flex-col items-center gap-3 py-14">
                <Glyph id="s-hex" />
                <div className="text-14 font-semibold">Create your first project</div>
                <p className="max-w-[440px] text-center text-11p5 leading-relaxed text-ink3">
                  A project is your application. Describe what you're building and review a
                  recommended stack — with the monthly estimate shown before anything exists.
                </p>
                <div className="flex gap-2">
                  <Btn
                    variant="p"
                    onClick={() => navigate({ to: "/$org/new-project", params: { org } })}
                  >
                    <Icon id="s-plus" />
                    New project
                  </Btn>
                  <Btn
                    variant="s"
                    disabled
                    disabledReason="Templates gallery (C5) — see New project"
                  >
                    Start from template
                  </Btn>
                </div>
                <Copybit>steloit project create my-app</Copybit>
              </Card>
              <div className="grid grid-cols-3 gap-3.5">
                <Card className="flex flex-col gap-2 p-4">
                  <div className="text-12p5 font-semibold">What's a project?</div>
                  <p className="text-11p5 leading-relaxed text-ink3">
                    Services, environments, deployments and cost all roll up to it — one picture of
                    your whole app, no vendor seams.
                  </p>
                </Card>
                <Card className="flex flex-col gap-2 p-4">
                  <div className="text-12p5 font-semibold">Invite your team</div>
                  <p className="text-11p5 leading-relaxed text-ink3">
                    Teammates get roles, not passwords — every action lands in the audit log.
                  </p>
                  <Link
                    to="/$org/settings/members"
                    params={{ org }}
                    className="mt-auto text-11 font-medium text-steel"
                  >
                    Invite members → Settings
                  </Link>
                </Card>
                <Card className="flex flex-col gap-2 p-4">
                  <div className="text-12p5 font-semibold">Prefer IaC?</div>
                  <p className="text-11p5 leading-relaxed text-ink3">
                    The Terraform provider maps 1:1 to the same primitives — define the project in
                    code instead.
                  </p>
                  <span className="mono mt-auto text-10p5 text-ink3">
                    terraform init · provider "steloit"
                  </span>
                </Card>
              </div>
            </>
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

          <div className="mt-auto flex items-center gap-2.5 text-11p5 text-ink3">
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
