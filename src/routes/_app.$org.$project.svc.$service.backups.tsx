import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { EmptyState } from "@/design-system/empty-state";
import { Icon } from "@/design-system/icon";
import { Flabel, Inp } from "@/design-system/inp";
import { Drawer } from "@/design-system/overlay";
import { Pill } from "@/design-system/pill";
import { useServices } from "@/features/services/hooks";
import { resolveEnvKey } from "@/lib/canon-env";
import { cn } from "@/lib/utils";

/**
 * D5 · Backups — PITR + verified snapshots + the restore drill. 08-api has no
 * backups endpoint (spec gap, a finding): every action here is disabled with
 * that stated reason, and the snapshots below are frame-fixed canon.
 */

const NO_ENDPOINT = "No backups endpoint in the spec — spec change first (finding)";

const SNAPSHOTS: Array<[string, string, string]> = [
  ["snap_0706_0200", "today · 02:00", "21.4 GB"],
  ["snap_0705_0200", "yesterday · 02:00", "21.2 GB"],
  ["snap_0704_0200", "Jul 4 · 02:00", "21.2 GB"],
];

function BackupsPage() {
  const { project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(resolveEnvKey(project, env));
  const [restoring, setRestoring] = useState(false);
  const svc = services.data?.find((s) => s.name === service || s.id === service);

  if (!svc) return <main className="main" />;
  if (svc.product !== "postgres") {
    return (
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Card className="p-4 text-12 text-ink2">This tab belongs to PostgreSQL instances.</Card>
        </div>
      </main>
    );
  }

  // Wrong-instance fix: the PITR window, WAL lag, snapshots, and drill below
  // are db-main's D5 canon — every other postgres instance keeps the page
  // chrome (and its disabled-with-reason actions, per B6) but renders honest
  // instance-scoped states.
  const canon = svc.name === "db-main";

  if (!canon) {
    return (
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            title="Backups"
            sub={
              <span className="mono">
                {svc.name} · point-in-time recovery · {env}
              </span>
            }
          >
            <Btn variant="s" disabled disabledReason={NO_ENDPOINT}>
              Take snapshot now
            </Btn>
          </Pghead>

          <Card className="flex flex-col gap-3 p-4">
            <div className="flex items-center gap-2.5">
              <Pill tone="ok">continuous · PITR</Pill>
              <span className="text-11p5 text-ink2">
                PITR window starts accruing now — restore to <b>any second</b> once WAL history
                builds
              </span>
            </div>
            <div className="flex items-center gap-2.5">
              <Btn variant="p" disabled disabledReason={NO_ENDPOINT}>
                Restore to a new branch…
              </Btn>
              <span className="text-10p5 text-ink3">
                restores always land in a branch — production is never overwritten in place
              </span>
            </div>
          </Card>

          <EmptyState
            compact
            icon="s-db"
            title="No snapshots yet"
            meaning={
              <>the first nightly snapshot lands at 02:00 IST — PITR covers 7 days from then</>
            }
          />

          <Card className="flex items-center gap-3 p-4">
            <Icon id="s-shield" className="h-4 w-4 shrink-0 text-ink3" />
            <p className="text-11p5 text-ink2">
              No restore drill has run yet — a backup you haven't restored is a rumor.
            </p>
            <span className="flex-1" />
            <Btn variant="s" disabled disabledReason={NO_ENDPOINT}>
              Run drill
            </Btn>
          </Card>
        </div>
      </main>
    );
  }

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          title="Backups"
          sub={
            <span className="mono">
              {svc.name} · point-in-time recovery · {env}
            </span>
          }
        >
          <Btn variant="s" disabled disabledReason={NO_ENDPOINT}>
            Take snapshot now
          </Btn>
        </Pghead>

        <Card className="flex flex-col gap-3 p-4">
          <div className="flex items-center gap-2.5">
            <Pill tone="ok">continuous · PITR</Pill>
            <span className="text-11p5 text-ink2">
              Restore to <b>any second</b> in the last 7 days
            </span>
            <span className="flex-1" />
            <span className="mono text-11 text-ink3">WAL lag 0.8s</span>
          </div>

          <div className="flex flex-col gap-1">
            <div className="mono pl-[72%] text-10 text-steel">Jul 5 · 14:00 — pre-deploy #142</div>
            <div className="relative h-2 rounded-full border border-hair bg-surface2">
              <span
                className="absolute top-[-3px] h-3 w-[3px] rounded-sm bg-steel"
                style={{ left: "72%" }}
              />
            </div>
            <div className="mono flex justify-between text-10 text-ink3">
              <span>Jun 29</span>
              <span>now</span>
            </div>
          </div>

          <div className="flex items-center gap-2.5">
            <Btn variant="p" onClick={() => setRestoring(true)}>
              Restore to a new branch…
            </Btn>
            <span className="text-10p5 text-ink3">
              restores always land in a branch — production is never overwritten in place
            </span>
          </div>
        </Card>

        <div className="tblwrap">
          <table className="tbl">
            <thead>
              <tr>
                <th>Snapshot</th>
                <th>Taken</th>
                <th>Size</th>
                <th>Verified</th>
                <th aria-label="Actions" />
              </tr>
            </thead>
            <tbody>
              {SNAPSHOTS.map(([id, taken, size]) => (
                <tr key={id}>
                  <td className="mono text-11">{id}</td>
                  <td className="text-ink2">{taken}</td>
                  <td className="mono text-11">{size}</td>
                  <td>
                    <Pill tone="ok">restore-tested</Pill>
                  </td>
                  <td>
                    <span className="flex justify-end">
                      <Btn
                        variant="s"
                        className="h-6 px-2.5 text-10p5"
                        disabled
                        disabledReason={NO_ENDPOINT}
                      >
                        Restore to branch
                      </Btn>
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <Card className="flex items-center gap-3 p-4">
          <Icon id="s-shield" className="h-4 w-4 shrink-0 text-ink3" />
          <p className="text-11p5 text-ink2">
            Last restore drill: <b>Jul 2</b> → branch{" "}
            <span className="mono">restore-drill-jul</span> · verified in <b>4m 12s</b> · a backup
            you haven't restored is a rumor.
          </p>
          <span className="flex-1" />
          <Btn variant="s" disabled disabledReason={NO_ENDPOINT}>
            Run drill
          </Btn>
        </Card>

        {restoring ? (
          <RestoreDrawer svcName={svc.name} env={env} onClose={() => setRestoring(false)} />
        ) : null}
      </div>
    </main>
  );
}

/**
 * Restore to a new branch — the 424 sheet over db-main's canon branch. The
 * verb stays gated with the page's finding (no backups endpoint in the spec);
 * the flow itself is the honest shape of the restore contract.
 */
function RestoreDrawer({
  svcName,
  env,
  onClose,
}: {
  svcName: string;
  env: string;
  onClose: () => void;
}) {
  const [point, setPoint] = useState("snap_0706_0200");
  // The canon incident moment — the natural PITR target in the demo world.
  const [pitrAt, setPitrAt] = useState("2026-07-02T14:02:00+05:30");
  // Default branch name from the canon world day (the 0706 snapshot).
  const [branch, setBranch] = useState("restore-2026-07-06");
  return (
    <Drawer
      title="Restore to a new branch"
      sub={
        <>
          {svcName} · {env}
        </>
      }
      onClose={onClose}
      footer={
        <>
          <span className="flex-1" />
          <Btn variant="s" onClick={onClose}>
            Cancel
          </Btn>
          <Btn variant="p" disabled disabledReason={NO_ENDPOINT}>
            Restore to branch
          </Btn>
        </>
      }
    >
      <div className="flex flex-col gap-1.5">
        <div className="eyebrow">Restore point</div>
        {SNAPSHOTS.map(([id, taken, size]) => (
          <label
            key={id}
            className={cn(
              "flex cursor-pointer items-center gap-2.5 rounded-lg border border-hair px-3 py-2",
              point === id && "border-steel",
            )}
          >
            <input
              type="radio"
              name="restore-point"
              checked={point === id}
              onChange={() => setPoint(id)}
            />
            <span className="mono text-11p5">{id}</span>
            <span className="text-11 text-ink3">{taken}</span>
            <span className="flex-1" />
            <span className="mono text-10p5 text-ink3">{size}</span>
          </label>
        ))}
        <label
          className={cn(
            "flex cursor-pointer flex-col gap-2 rounded-lg border border-hair px-3 py-2",
            point === "pitr" && "border-steel",
          )}
        >
          <span className="flex items-center gap-2.5">
            <input
              type="radio"
              name="restore-point"
              checked={point === "pitr"}
              onChange={() => setPoint("pitr")}
            />
            <span className="text-11p5">PITR — any second in the last 7 days</span>
          </span>
          <Inp
            className="mono"
            value={pitrAt}
            aria-label="Restore moment"
            onChange={(e) => setPitrAt(e.target.value)}
            onFocus={() => setPoint("pitr")}
          />
        </label>
      </div>

      <div>
        <Flabel htmlFor="restore-branch">Target branch</Flabel>
        <Inp
          id="restore-branch"
          className="mono"
          value={branch}
          onChange={(e) => setBranch(e.target.value)}
        />
      </div>

      <Card className="p-3.5">
        <p className="text-11p5 leading-relaxed text-ink2">
          restores into a NEW branch — production is never touched by a restore
        </p>
      </Card>
    </Drawer>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/backups")({
  component: BackupsPage,
});
