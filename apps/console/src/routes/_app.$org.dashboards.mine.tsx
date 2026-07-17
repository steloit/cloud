import { useMutation } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { ConfirmModal } from "@/design-system/confirm";
import { EmptyState } from "@/design-system/empty-state";
import { Eyebrow } from "@/design-system/eyebrow";
import { Pill, type PillTone } from "@/design-system/pill";
import { SkeletonRows } from "@/design-system/skeleton";
import { useDashboards } from "@/features/dashboards/hooks";
import { NewDashboardModal } from "@/features/dashboards/new-dashboard-modal";
import { ApiFailureCard } from "@/features/errors/failure-states";
import {
  deleteDashboardMutation,
  errorMessage,
  listDashboardsQueryKey,
  queryClient,
} from "@/lib/api";

/**
 * DB5 · My dashboards & shared — scope (org-wide vs project) and visibility
 * are separate axes. checkout-health is the one live row (canon fixtures);
 * the rest are frame-fixed.
 */

interface Row {
  name: string;
  /** API id where one exists — frame-fixed rows have none; the name stands in. */
  id?: string;
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

function DashTable({
  org,
  rows,
  pending,
  onDelete,
}: {
  org: string;
  rows: Row[];
  pending?: boolean;
  /** Present for Mine — Delete opens the ConfirmModal. Absent (Shared) keeps it honestly disabled. */
  onDelete?: (row: Row) => void;
}) {
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
          {pending ? (
            <SkeletonRows cols={8} />
          ) : (
            rows.map((row) => (
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
                    <Btn
                      variant="gh"
                      disabled
                      disabledReason="Only checkout-health carries canon widgets — the others aren't in the fixtures (finding)"
                    >
                      Open
                    </Btn>
                  )}
                  <Btn
                    variant="gh"
                    disabled
                    disabledReason="No duplicate endpoint in the spec (finding)"
                  >
                    Duplicate
                  </Btn>
                  {onDelete ? (
                    <Btn variant="gh" className="text-err" onClick={() => onDelete(row)}>
                      Delete…
                    </Btn>
                  ) : (
                    <Btn variant="gh" disabled disabledReason="Only the owner deletes a dashboard">
                      Delete…
                    </Btn>
                  )}
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}

function MyDashboards() {
  const { org } = Route.useParams();
  // Strictly one overlay at a time: either the new-dashboard modal or the
  // delete confirm, never both.
  const [modalOpen, setModalOpen] = useState(false);
  const [deleting, setDeleting] = useState<Row | null>(null);
  const dashboards = useDashboards(org);
  const deleteDashboard = useMutation(deleteDashboardMutation());

  const onDelete = (row: Row) =>
    deleteDashboard.mutate(
      // Frame-fixed rows carry no fixture id — the name stands in; MSW 204s either way.
      { path: { dash: row.id ?? row.name } },
      {
        onSuccess: () => {
          setDeleting(null);
          toast.success(`Dashboard ${row.name} deleted`);
          // MSW 204-echoes the delete — fixtures win on refetch (the DB8 add-widget precedent).
          queryClient.invalidateQueries({ queryKey: listDashboardsQueryKey({ path: { org } }) });
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );

  const checkout = dashboards.data?.find((d) => d.id === "dsh_checkout_health");
  // The live row renders only when the API actually carries it — previously a
  // fallback row claimed checkout-health existed even with zero dashboards.
  const mine: Row[] = [
    ...(checkout
      ? [
          {
            name: checkout.name ?? "checkout-health",
            id: checkout.id,
            sub: "born Jul 2, during the incident — editing in DB7",
            scope: "ecommerce",
            scopeTone: "mut" as PillTone,
            // Fixtures say visibility "org" but the Design Spec records that org
            // badge as the error — rendered personal per spec; surfaced as a finding.
            visibility: "personal",
            visibilityTone: "mut" as PillTone,
            // Fixtures have 5 widgets while the frames say 6 — frame value rendered,
            // divergence noted as a finding.
            widgets: "6",
            owner: "asha (you)",
            updated: "Jul 5",
            pinned: true,
            live: true,
          },
        ]
      : []),
    ...MINE_STATIC,
  ];
  // Four-state grammar (16-qa) on the dashboards query. Empty means zero user
  // dashboards — the frame-fixed Mine rows describe asha's canon world and
  // don't survive into an honestly-empty one; the Shared table below is
  // frame-fixed and stays.
  const isEmpty = dashboards.isSuccess && (dashboards.data ?? []).length === 0;

  return (
    <main className="main">
      <div className="pgpad">
        {/* Finding: frame DB5 titles this pane "My dashboards & shared" — the design-system
            "Area · Thing" h1 grammar wins per the audit's P1 ruling. */}
        <Pghead
          title="Dashboards · My dashboards"
          sub="Yours are personal by default; share to the org or a project when a view is worth standardizing. Scope (org-wide vs project) is a separate axis from visibility — project-scoped dashboards are born filtered and inherit project permissions."
        >
          <Btn variant="p" onClick={() => setModalOpen(true)}>
            New dashboard (DB8)
          </Btn>
        </Pghead>

        <Eyebrow>Mine · {dashboards.isSuccess ? (isEmpty ? 0 : mine.length) : "…"}</Eyebrow>
        {dashboards.isError ? (
          <ApiFailureCard
            title="Your dashboards didn't load"
            error={dashboards.error}
            requestLine={`GET /orgs/${org}/dashboards`}
            onRetry={() => dashboards.refetch()}
          />
        ) : isEmpty ? (
          <EmptyState
            compact
            icon="s-grid"
            title="No dashboards yet"
            meaning={
              <>
                pin what you watch — one grid composes widgets from metrics, logs, cost and deploys
              </>
            }
            cta={
              <Btn variant="p" onClick={() => setModalOpen(true)}>
                New dashboard (DB8)
              </Btn>
            }
          />
        ) : (
          <DashTable
            org={org}
            rows={mine}
            pending={dashboards.isPending}
            onDelete={(row) => setDeleting(row)}
          />
        )}

        <Eyebrow>Shared with you · 4</Eyebrow>
        <DashTable org={org} rows={SHARED} />

        <div className="mt-auto text-10p5 text-ink3">
          Shared dashboards are live for everyone — an edit is an edit for all viewers, logged in
          the audit trail. Duplicate to riff privately.
        </div>
      </div>
      {modalOpen ? <NewDashboardModal org={org} onClose={() => setModalOpen(false)} /> : null}
      {deleting ? (
        <ConfirmModal
          title={`Delete ${deleting.name}`}
          consequence="Widgets and layout go — the queries behind them are untouched."
          verb="Delete dashboard"
          pending={deleteDashboard.isPending}
          onConfirm={() => onDelete(deleting)}
          onClose={() => setDeleting(null)}
        />
      ) : null}
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/dashboards/mine")({
  component: MyDashboards,
});
