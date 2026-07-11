import { createFileRoute } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Eyebrow } from "@/design-system/eyebrow";
import { Flabel, Inp } from "@/design-system/inp";
import { Pill } from "@/design-system/pill";
import { useOrgs } from "@/features/org/hooks";

/** G5 · Organization · General — the boundary of identity, billing and policy. */

function OrgGeneralPage() {
  const { org } = Route.useParams();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);

  return (
    <>
      <SnavSettings
        org={org}
        orgName={orgRecord?.name ?? org}
        project="ecommerce"
        active="o-general"
      />
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            title="Organization · General"
            sub="Acme · Business plan · the boundary of identity, billing and policy"
          />

          <Card className="flex max-w-[640px] flex-col gap-4 p-4">
            <div>
              <Flabel htmlFor="org-name">Organization name</Flabel>
              <Inp id="org-name" defaultValue="Acme" />
            </div>
            <div>
              <Flabel htmlFor="org-slug">Slug</Flabel>
              <Inp id="org-slug" className="mono" defaultValue="acme" readOnly />
            </div>
            <div>
              <Flabel htmlFor="org-region">Default home region</Flabel>
              <Inp id="org-region" className="mono" defaultValue="aws · ap-south-1" readOnly />
              <div className="mt-1.5 text-10p5 text-ink3">
                prefill for new environments — never a constraint
              </div>
            </div>
          </Card>

          <Card className="flex max-w-[640px] flex-col gap-3 p-4">
            <Eyebrow>Ownership</Eyebrow>
            <div className="text-12 text-ink2">
              asha is the owner. Transfer requires the new owner to accept — it never happens to
              someone.
            </div>
            <div>
              <Btn variant="s" disabled disabledReason="Ownership transfer lands in Phase 4">
                Transfer ownership…
              </Btn>
            </div>
          </Card>

          <Card className="flex max-w-[640px] flex-col gap-3 border-err/45 p-4">
            <Eyebrow className="text-err">Danger zone</Eyebrow>
            <div className="flex items-center gap-2.5">
              <Btn
                variant="dgr"
                disabled
                disabledReason="blocked — 4 projects, 12 members, active subscription"
              >
                Delete Acme…
              </Btn>
              <Pill tone="err">blocked — 4 projects, 12 members, active subscription</Pill>
            </div>
            <div className="text-10p5 text-ink3">
              delete or transfer every project first; the final invoice closes the account, and
              audit history is exported before erasure
            </div>
          </Card>
        </div>
      </main>
    </>
  );
}

export const Route = createFileRoute("/_app/$org/settings/general")({
  component: OrgGeneralPage,
});
