import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Eyebrow } from "@/design-system/eyebrow";
import { Pill, type PillTone } from "@/design-system/pill";
import { useDashboards } from "@/features/dashboards/hooks";
import { NewDashboardModal } from "@/features/dashboards/new-dashboard-modal";

/**
 * DB5 · My dashboards & shared — scope (org-wide vs project) and visibility
 * are separate axes. checkout-health is the one live row (canon fixtures);
 * the rest are frame-fixed.
 */

interface Row {
  name: string;
  sub?: string;
  scope: string;
  scopeTone: PillTone;
  visibility: string;
  visibilityTone: PillTone;
  widgets: string;
  owner: string;
  updated: string;
  pinned: boolean;
  /** Only checkout-health navigates — the only dashboard in canon fixtures. */
  live?: boolean;
}

const MINE_STATIC: Row[] = [
  {
    name: "pg-tuning-notes",
    scope: "ecommerce",
    scopeTone: "mut",
    visibility: "personal",
    visibilityTone: "mut",
    widgets: "4",
    owner: "asha (you)",
    updated: "Jun 28",
    pinned: false,
  },
  {
    name: "cost-deep-dive",
    scope: "org-wide",
    scopeTone: "st",
    visibility: "personal",
    visibilityTone: "mut",
    widgets: "5",
    owner: "asha (you)",
    updated: "Jun 12",
    pinned: false,
  },
];

const SHARED: Row[] = [
  {
    name: "team-weekly",
    scope: "org-wide",
    scopeTone: "st",
    visibility: "org",
    visibilityTone: "st",
    widgets: "8",
    owner: "priya",
    updated: "yesterday",
    pinned: true,
  },
  {
    name: "oncall-handoff",
    scope: "org-wide",
    scopeTone: "st",
    visibility: "org",
    visibilityTone: "st",
    widgets: "7",
    owner: "priya",
    updated: "Jul 4",
    pinned: false,
  },
  {
    name: "release-review",
    scope: "ecommerce",
    scopeTone: "mut",
    visibility: "org",
    visibilityTone: "st",
    widgets: "5",
    owner: "marco",
    updated: "Jul 2",
    pinned: false,
  },
  {
    name: "analytics-load",
    scope: "analytics-pipeline",
    scopeTone: "mut",
    visibility: "restricted",
    visibilityTone: "mut",
    widgets: "4",
    owner: "dev-3",
    updated: "Jun 30",
    pinned: false,
  },
];

function DashTable({ org, rows }: { org: string; rows: Row[] }) {
  return (
    <div className="tblwrap">
      <table className="tbl">
        <thead>
          <tr>
            <th>Dashboard</th>
            <th>Scope</th>
            <th>Visibility</th>
            <th>Widgets</th>
            <th>Owner</th>
            <th>Updated</th>
            <th>Pin</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.name}>
              <td>
                {row.live ? (
                  <Link
                    to="/$org/dashboards/$dashId"
                    params={{ org, dashId: "checkout-health" }}
                    className="mono font-medium hover:underline"
                  >
                    {row.name}
                  </Link>
                ) : (
                  <span className="mono font-medium">{row.name}</span>
                )}
                {row.sub ? <div className="text-10p5 text-ink3">{row.sub}</div> : null}
              </td>
              <td>
                <Pill tone={row.scopeTone}>{row.scope}</Pill>
              </td>
              <td>
                <Pill tone={row.visibilityTone}>{row.visibility}</Pill>
              </td>
              <td className="mono">{row.widgets}</td>
              <td className="text-ink2">{row.owner}</td>
              <td className="text-ink2">{row.updated}</td>
              <td>{row.pinned ? "★" : "☆"}</td>
              <td className="text-right whitespace-nowrap">
                {row.live ? (
                  <Link to="/$org/dashboards/$dashId" params={{ org, dashId: "checkout-health" }}>
                    <Btn variant="gh">Open</Btn>
                  </Link>
                ) : (
                  <Btn variant="gh" disabled disabledReason="only canon dashboards are navigable">
                    Open
                  </Btn>
                )}
                <Btn variant="gh" disabled disabledReason="Duplicate lands in Phase 4">
                  Duplicate
                </Btn>
                <Btn variant="gh" disabled disabledReason="Delete lands in Phase 4">
                  Delete…
                </Btn>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function MyDashboards() {
  const { org } = Route.useParams();
  const [modalOpen, setModalOpen] = useState(false);
  const dashboards = useDashboards(org);

  const checkout = dashboards.data?.find((d) => d.id === "dsh_checkout_health");
  const mine: Row[] = [
    {
      name: checkout?.name ?? "checkout-health",
      sub: "born Jul 2, during the incident — editing in DB7",
      scope: "ecommerce",
      scopeTone: "mut",
      // Fixtures say visibility "org" but the Design Spec records that org
      // badge as the error — rendered personal per spec; surfaced as a finding.
      visibility: "personal",
      visibilityTone: "mut",
      // Fixtures have 5 widgets while the frames say 6 — frame value rendered,
      // divergence noted as a finding.
      widgets: "6",
      owner: "asha (you)",
      updated: "Jul 5",
      pinned: true,
      live: true,
    },
    ...MINE_STATIC,
  ];

  return (
    <main className="main">
      <div className="pgpad">
        <Pghead
          title="My dashboards & shared"
          sub="Yours are personal by default; share to the org or a project when a view is worth standardizing. Scope (org-wide vs project) is a separate axis from visibility — project-scoped dashboards are born filtered and inherit project permissions."
        >
          <Btn variant="p" onClick={() => setModalOpen(true)}>
            New dashboard (DB8)
          </Btn>
        </Pghead>

        <Eyebrow>Mine · 3</Eyebrow>
        <DashTable org={org} rows={mine} />

        <Eyebrow>Shared with you · 4</Eyebrow>
        <DashTable org={org} rows={SHARED} />

        <div className="mt-auto text-10p5 text-ink3">
          Shared dashboards are live for everyone — an edit is an edit for all viewers, logged in
          the audit trail. Duplicate to riff privately.
        </div>
      </div>
      {modalOpen ? <NewDashboardModal org={org} onClose={() => setModalOpen(false)} /> : null}
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/dashboards/mine")({
  component: MyDashboards,
});
