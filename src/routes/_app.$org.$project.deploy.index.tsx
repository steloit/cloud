import { createFileRoute, Link } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { EmptyState } from "@/design-system/empty-state";
import { Eyebrow } from "@/design-system/eyebrow";
import { Dot, Pill } from "@/design-system/pill";
import { Skeleton, SkeletonRows } from "@/design-system/skeleton";
import { useRollback } from "@/features/deploy/hooks";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { useDeployments } from "@/features/services/hooks";
import type { Deployment } from "@/lib/api";

/**
 * DP1 · Deploy · Deployments — the cross-service promotion lane and history.
 * Rows come from GET /envs/{env}/deployments; the frame-fixed display strings
 * (change lines, durations) are a per-number map merged with the live data.
 */

interface RowDisplay {
  change: (dep: Deployment) => ReactNode;
  status: ReactNode;
  duration: string;
  /** By-column override; defaults to the actor from data. */
  by?: ReactNode;
  highlight?: boolean;
}

const DISPLAY: Record<number, RowDisplay> = {
  143: {
    change: () => <>fix/orders-query · migration 0142_add_idx (from prp_7c31a2)</>,
    // The gallery shows a prov-toned pill; the design system's pill vocabulary
    // is ok/warn/err/st/ai/mut (no prov) — st is the closest existing tone.
    status: <Pill tone="st">rolling out · 33%</Pill>,
    duration: "2 m 10 s",
    highlight: true,
  },
  142: {
    change: (dep) => (
      <>
        gift-cards @ a41f2c · <span className="text-warn">{dep.annotation}</span>
      </>
    ),
    status: <Pill tone="ok">live · 13:58</Pill>,
    duration: "58 s",
  },
  140: {
    change: () => <>checkout-retry @ 88ba02</>,
    status: <Pill tone="err">rolled back · auto</Pill>,
    duration: "3 m 02 s",
    by: <span className="mono text-warn">gate: error rate 2.4×</span>,
  },
};

function DeploymentsPage() {
  const { org, project } = Route.useParams();
  const { env } = Route.useSearch();
  const deployments = useDeployments(env);
  const rollback = useRollback();
  const [armed, setArmed] = useState(false);

  const rows = [...(deployments.data ?? [])].sort((a, b) => (b.number ?? 0) - (a.number ?? 0));
  // Four-state grammar (16-qa): pending → skeleton, error → failure card,
  // empty → EmptyState, else the lane + history. The promotion lane and the
  // staging rows are frame-fixed canon for a project that deploys — a
  // zero-deploy project's DP1 has nothing to promote, so they live only in
  // the populated branch (and pending shows their shape, not their claims).
  const isEmpty = deployments.isSuccess && rows.length === 0;

  const action = (dep: Deployment): ReactNode => {
    switch (dep.number) {
      case 143:
        return (
          <Link
            to="/$org/$project/deploy/$dep"
            params={{ org, project, dep: dep.id }}
            search={{ env }}
          >
            <Btn variant="s">Watch rollout</Btn>
          </Link>
        );
      case 142:
        return (
          <Btn
            variant={armed ? "dgr" : "s"}
            className={armed ? undefined : "text-err"}
            onClick={() => {
              if (!armed) {
                setArmed(true);
                return;
              }
              rollback.mutate({ path: { dep: dep.id } });
            }}
          >
            {armed ? "Confirm rollback" : "Rollback…"}
          </Btn>
        );
      case 140:
        return (
          <Btn variant="gh" title="auto-abort gate: error rate 2.4× baseline — see the rollout log">
            Why?
          </Btn>
        );
      default:
        return null;
    }
  };

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          title="Deploy · Deployments"
          sub="Cross-service: promotion moves a coordinated set through environments — per-service history lives on each service page (D19)"
        >
          <Copybit>steloit deploy --env staging</Copybit>
        </Pghead>

        {deployments.isError ? (
          <ApiFailureCard
            title="Deployments didn't load"
            error={deployments.error}
            requestLine={`GET /envs/${env}/deployments`}
            onRetry={() => deployments.refetch()}
          />
        ) : isEmpty ? (
          <EmptyState
            compact
            icon="s-deploy"
            title="No deployments yet"
            meaning={
              <>
                the first git push creates #1 — every push becomes an immutable release, promoted
                through environments with gates
              </>
            }
            cli="git push steloit main"
          />
        ) : (
          <>
            {deployments.isPending ? (
              <Skeleton className="h-[118px]" />
            ) : (
              <Card className="flex flex-col gap-3 p-4">
                <Eyebrow>
                  Promotion lane — same image, next environment's config, progressive rollout with
                  gates
                </Eyebrow>
                <div className="flex items-center gap-4">
                  <Card className="flex-1 bg-surface2 p-3">
                    <div className="flex items-center gap-2">
                      <Dot tone="ok" />
                      <b className="text-12p5">staging</b>
                      <span className="mono ml-auto text-12">#143</span>
                    </div>
                    <div className="mt-1.5 text-11 text-ink3">
                      fix/orders-query · checks 4/4 ✓ · soaked 9 min
                    </div>
                  </Card>
                  <div className="flex flex-col items-center gap-1.5">
                    <Link
                      to="/$org/$project/deploy/$dep"
                      params={{ org, project, dep: "dep_143" }}
                      search={{ env }}
                    >
                      <Btn variant="p">Promoting →</Btn>
                    </Link>
                    <span className="mono text-10p5 text-prov">canary 33% · gates healthy</span>
                  </div>
                  <Card className="flex-1 bg-surface2 p-3">
                    <div className="flex items-center gap-2">
                      <Dot tone="warn" />
                      <b className="text-12p5">production</b>
                      <span className="mono ml-auto text-12">#142 → #143</span>
                    </div>
                    <div className="mt-1.5 text-11 text-ink3">
                      #142 live · p95 alert open · #143 carries the index fix
                    </div>
                  </Card>
                </div>
              </Card>
            )}

            <div className="tblwrap">
              <table className="tbl">
                <thead>
                  <tr>
                    <th>Deploy</th>
                    <th>Change</th>
                    <th>Env</th>
                    <th>Status</th>
                    <th>Duration</th>
                    <th>By</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {deployments.isPending ? (
                    <SkeletonRows cols={7} rows={5} />
                  ) : (
                    <>
                      {rows.map((dep) => {
                        const display = dep.number !== undefined ? DISPLAY[dep.number] : undefined;
                        return (
                          <tr
                            key={dep.id}
                            className={display?.highlight ? "bg-steel-tint" : undefined}
                          >
                            <td className="mono">#{dep.number}</td>
                            <td>{display?.change(dep) ?? dep.git_sha}</td>
                            <td>production</td>
                            <td>{display?.status ?? dep.state}</td>
                            <td className="mono">{display?.duration}</td>
                            <td>{display?.by ?? dep.actor}</td>
                            <td className="text-right">{action(dep)}</td>
                          </tr>
                        );
                      })}
                      {/* Staging rows are frame-fixed (DP1 gallery) — fixtures only
                          define production deployments, so these two render static. */}
                      <tr>
                        <td className="mono">#143</td>
                        <td>fix/orders-query</td>
                        <td>staging</td>
                        <td>
                          <Pill tone="ok">live · 14:40</Pill>
                        </td>
                        <td className="mono">41 s</td>
                        <td>priya</td>
                        <td />
                      </tr>
                      <tr>
                        <td className="mono">#139</td>
                        <td>checkout-retry</td>
                        <td>staging</td>
                        <td>
                          <Pill tone="ok">live · Jul 3</Pill>
                        </td>
                        <td className="mono">47 s</td>
                        <td>marco</td>
                        <td />
                      </tr>
                    </>
                  )}
                </tbody>
              </table>
            </div>

            <div className="flex flex-col gap-1.5 text-11 leading-relaxed text-ink3">
              <p>
                Rollback = redeploy of the previous image, same gates, &lt; 60 s. Database
                migrations don't auto-revert — expand-contract keeps old code compatible, and each
                migration states its reverse.
              </p>
              <p>Every row is an ◆ event on every Observe chart.</p>
            </div>
          </>
        )}
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/deploy/")({
  component: DeploymentsPage,
});
