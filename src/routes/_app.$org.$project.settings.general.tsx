import { createFileRoute } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Eyebrow } from "@/design-system/eyebrow";
import { Flabel, Inp } from "@/design-system/inp";
import { Pill } from "@/design-system/pill";
import { useOrgs } from "@/features/org/hooks";

/** G1 · Project · General — identity, transfer, danger zone. */

function ProjectGeneralPage() {
  const { org, project } = Route.useParams();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);

  return (
    <>
      <SnavSettings
        org={org}
        orgName={orgRecord?.name ?? org}
        project={project}
        active="p-general"
      />
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            title="Project · General"
            sub="ecommerce · created from the store template · 7 months ago"
          />

          <Card className="flex max-w-[640px] flex-col gap-4 p-4">
            <div>
              <Flabel htmlFor="prj-name">Project name</Flabel>
              <Inp id="prj-name" defaultValue="ecommerce" />
            </div>
            <div>
              <Flabel htmlFor="prj-slug">Slug</Flabel>
              <div className="flex items-center gap-2.5">
                <span className="flex-1">
                  <Inp id="prj-slug" className="mono" defaultValue="acme/ecommerce" readOnly />
                </span>
                <Pill tone="mut">renaming updates URLs · old slugs redirect 90 d</Pill>
              </div>
            </div>
            <div>
              <Flabel htmlFor="prj-defenv">Default environment</Flabel>
              <Inp id="prj-defenv" className="mono" defaultValue="production" readOnly />
            </div>
          </Card>

          <Card className="flex max-w-[640px] flex-col gap-3 p-4">
            <Eyebrow>Transfer</Eyebrow>
            <div className="text-[12px] text-ink2">
              Moves services, environments, history and cost — bindings and credentials survive;
              billing switches at the next invoice boundary.
            </div>
            <div>
              <Btn variant="s" disabled disabledReason="Project transfer lands in Phase 4">
                Transfer…
              </Btn>
            </div>
          </Card>

          <Card className="flex max-w-[640px] flex-col gap-3 border-err/45 p-4">
            <Eyebrow className="text-err">Danger zone</Eyebrow>
            <div className="flex items-center gap-2.5">
              <Btn
                variant="dgr"
                disabled
                disabledReason="blocked — 7 services across 3 environments"
              >
                Delete ecommerce…
              </Btn>
              <Pill tone="err">blocked — 7 services across 3 environments</Pill>
            </div>
            <div className="text-[10.5px] text-ink3">
              deletion spins services down in dependency order, keeps final snapshots 30 d, and
              requires typing the project slug
            </div>
          </Card>
        </div>
      </main>
    </>
  );
}

export const Route = createFileRoute("/_app/$org/$project/settings/general")({
  component: ProjectGeneralPage,
});
