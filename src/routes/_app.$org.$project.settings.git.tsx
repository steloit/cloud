import { createFileRoute } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Eyebrow } from "@/design-system/eyebrow";
import { Glyph } from "@/design-system/glyph";
import { Pill } from "@/design-system/pill";
import { useOrgs } from "@/features/org/hooks";

/** G3 · Project · Git integration — one org-level GitHub app, no per-repo tokens. */

function ProjectGitPage() {
  const { org, project } = Route.useParams();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);

  return (
    <>
      <SnavSettings org={org} orgName={orgRecord?.name ?? org} project={project} active="p-git" />
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            title="Project · Git integration"
            sub="One org-level GitHub app — no per-repo tokens, repos scoped by asha"
          />

          <Card className="flex max-w-[760px] items-center gap-3.5 p-4">
            <Glyph id="s-branch" />
            <div>
              <div className="text-[13px] font-semibold">GitHub · Acme</div>
              <div className="text-[11px] text-ink3">
                installed by asha · scoped to 3 repositories · webhooks healthy
              </div>
            </div>
            <span className="ml-auto flex items-center gap-2.5">
              <Pill tone="ok">connected</Pill>
              <Btn
                variant="s"
                disabled
                disabledReason="Opens GitHub's app settings — external, not wired in the demo"
              >
                Manage repos ↗
              </Btn>
            </span>
          </Card>

          <div className="tblwrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th>Service</th>
                  <th>Repository</th>
                  <th>Branch</th>
                  <th>Deploys</th>
                  <th>Previews</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td className="mono text-[11.5px]">api</td>
                  <td className="mono text-[11.5px] text-ink2">acme/store</td>
                  <td className="mono text-[11.5px] text-ink2">main</td>
                  <td>
                    <Pill tone="st">on push</Pill>
                  </td>
                  <td>
                    <Pill tone="st">on · policy preview-minimal</Pill>
                  </td>
                </tr>
                <tr>
                  <td className="mono text-[11.5px]">worker</td>
                  <td className="mono text-[11.5px] text-ink2">acme/store</td>
                  <td className="mono text-[11.5px] text-ink2">main</td>
                  <td>
                    <Pill tone="st">on push</Pill>
                  </td>
                  <td>
                    <Pill tone="mut">excluded by policy</Pill>
                  </td>
                </tr>
                <tr>
                  <td className="mono text-[11.5px]">admin</td>
                  <td className="mono text-[11.5px] text-ink2">acme/admin</td>
                  <td className="mono text-[11.5px] text-ink2">main</td>
                  <td>
                    <Pill tone="st">on push</Pill>
                  </td>
                  <td>
                    <Pill tone="mut">off</Pill>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <Card className="flex max-w-[760px] flex-col gap-3 p-4">
            <Eyebrow>Disconnect</Eyebrow>
            <div className="text-[12px] text-ink2">
              Disconnecting stops deploys but never touches running services — the last built images
              keep serving.
            </div>
            <div>
              <Btn variant="dgr" disabled disabledReason="Disconnect lands in Phase 4">
                Disconnect…
              </Btn>
            </div>
          </Card>
        </div>
      </main>
    </>
  );
}

export const Route = createFileRoute("/_app/$org/$project/settings/git")({
  component: ProjectGitPage,
});
