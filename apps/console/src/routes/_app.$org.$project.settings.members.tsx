import { createFileRoute } from "@tanstack/react-router";
import type { ReactNode } from "react";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Eyebrow } from "@/design-system/eyebrow";
import { Pill } from "@/design-system/pill";
import { useOrgs } from "@/features/org/hooks";

/** G2 · Project · Members & roles — org members inherit, project roles only tighten. */

interface ProjectMemberRow {
  member: ReactNode;
  role: string;
  source: ReactNode;
  lastActive: string;
  action: ReactNode;
  invited?: boolean;
}

const CHANGE_REASON =
  "Project-scoped roles land with project-member endpoints the API lacks (finding)";

function ProjectMembersPage() {
  const { org, project } = Route.useParams();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);

  // Frame-fixed rows (G2): the API only models org membership — there is no
  // project-membership endpoint, so this table renders the frame verbatim (finding).
  const rows: ProjectMemberRow[] = [
    {
      member: <b>priya</b>,
      role: "Admin",
      source: "direct",
      lastActive: "now",
      action: (
        <Btn variant="gh" disabled disabledReason={CHANGE_REASON}>
          Change ▾
        </Btn>
      ),
    },
    {
      member: <b>marco</b>,
      role: "Developer",
      source: "direct",
      lastActive: "2 h ago",
      action: (
        <Btn variant="gh" disabled disabledReason={CHANGE_REASON}>
          Change ▾
        </Btn>
      ),
    },
    {
      member: <b>asha</b>,
      role: "Admin",
      source: <Pill tone="mut">inherited · org Owner</Pill>,
      lastActive: "yesterday",
      action: <span className="text-11 text-ink3">managed at org</span>,
    },
    {
      member: <span className="mono text-11p5">dev-3@acme.dev</span>,
      role: "invited · Developer",
      source: "—",
      lastActive: "pending 2 d",
      action: <span className="text-11 text-ink3">—</span>,
      invited: true,
    },
  ];

  return (
    <>
      <SnavSettings
        org={org}
        orgName={orgRecord?.name ?? org}
        project={project}
        active="p-members"
      />
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            title="Project · Members & roles"
            sub="Access to ecommerce · org members inherit, project roles can only tighten"
          >
            <Btn
              variant="p"
              disabled
              disabledReason="Project-scoped invites land with project-member endpoints the API lacks (finding)"
            >
              Invite to project
            </Btn>
          </Pghead>

          <div className="tblwrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th>Member</th>
                  <th>Project role</th>
                  <th>Source</th>
                  <th>Last active</th>
                  <th aria-label="Actions" />
                </tr>
              </thead>
              <tbody>
                {rows.map((r, i) => (
                  // biome-ignore lint/suspicious/noArrayIndexKey: static frame-fixed rows
                  <tr key={i} className={r.invited ? "opacity-70" : undefined}>
                    <td>{r.member}</td>
                    <td>{r.role}</td>
                    <td>{r.source}</td>
                    <td className="text-ink2">{r.lastActive}</td>
                    <td>
                      <span className="flex justify-end">{r.action}</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <Card className="flex max-w-[640px] flex-col gap-2.5 p-4">
            <Eyebrow>Roles</Eyebrow>
            <div className="text-11p5">
              <b>Admin</b>
              <span className="text-ink2"> — everything incl. deletes &amp; policies</span>
            </div>
            <div className="text-11p5">
              <b>Developer</b>
              <span className="text-ink2">
                {" "}
                — full non-production; production actions per policy prod-guard (the 403 in E3 is
                this rule working)
              </span>
            </div>
            <div className="text-11p5">
              <b>Viewer</b>
              <span className="text-ink2"> — read-only, no secrets</span>
            </div>
          </Card>

          <p className="text-11 text-ink3">
            Role changes apply on next request — no re-login — and land in the audit log with before
            → after.
          </p>
        </div>
      </main>
    </>
  );
}

export const Route = createFileRoute("/_app/$org/$project/settings/members")({
  component: ProjectMembersPage,
});
