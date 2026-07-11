import { createFileRoute, Link } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Eyebrow } from "@/design-system/eyebrow";
import { Icon } from "@/design-system/icon";
import { Dot, Stlab } from "@/design-system/pill";
import { useRollback } from "@/features/deploy/hooks";
import { useServices } from "@/features/services/hooks";

/**
 * D19 · Web/Worker · Deployments — the per-service history behind DP1's
 * cross-service lane. Rows #141/#140 here are the frame's web-service history
 * — fixtures' deployments (dep_140/142/143 with different shas and changes)
 * differ; frame-fixed. Rollback targets dep_142, the live deployment.
 */

interface HistoryRow {
  deploy: string;
  /** Muted deploy cell for the non-live rows. */
  mut?: boolean;
  highlight?: boolean;
  change: string;
  sha: string;
  by: string;
  when: string;
  health: { tone: "ok" | "warn"; label: string };
  action: "logs" | "rollback";
}

const ROWS: HistoryRow[] = [
  {
    deploy: "#142 · live",
    highlight: true,
    change: "feat: gift cards at checkout",
    sha: "8f31c02",
    by: "priya",
    when: "13:58",
    health: { tone: "warn", label: "p95 812 ms" },
    action: "logs",
  },
  {
    deploy: "#141",
    mut: true,
    change: "fix: cart total rounding",
    sha: "c02d911",
    by: "marco",
    when: "Jul 4 · 16:21",
    health: { tone: "ok", label: "clean · 22 h live" },
    action: "rollback",
  },
  {
    deploy: "#140",
    mut: true,
    change: "chore: bump node 20.14",
    sha: "a8812fe",
    by: "priya",
    when: "Jul 3 · 11:02",
    health: { tone: "ok", label: "clean" },
    action: "rollback",
  },
];

const INSTANCES = ["i-1 · 41% cpu", "i-2 · 38% cpu", "i-3 · 44% cpu"];

function DeploymentsPage() {
  const { org, project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(env);
  const rollback = useRollback();
  const [armed, setArmed] = useState<string | null>(null);

  const svc = services.data?.find((s) => s.name === service || s.id === service);
  if (!svc) return <main className="main" />;
  if (svc.product !== "web" && svc.product !== "worker") {
    return (
      <main className="main">
        <div className="pgpad">
          <h1 className="h1">Deployments</h1>
          <p className="hsub">
            Deployments (D19) is a web/worker surface — {svc.name} is {svc.product}.
          </p>
        </div>
      </main>
    );
  }

  const action = (row: HistoryRow): ReactNode => {
    if (row.action === "logs") {
      // Per-service logs (D10) has no route yet — the shared Observe logs
      // lens is the destination, filtered by the env search param.
      return (
        <Link to="/$org/$project/observe/logs" params={{ org, project }} search={{ env }}>
          <Btn variant="gh">Logs</Btn>
        </Link>
      );
    }
    const isArmed = armed === row.deploy;
    return (
      <Btn
        variant={isArmed ? "dgr" : "gh"}
        onClick={() => {
          if (!isArmed) {
            setArmed(row.deploy);
            return;
          }
          rollback.mutate({ path: { dep: "dep_142" } });
          setArmed(null);
        }}
      >
        {isArmed ? "Confirm rollback" : "Roll back to"}
      </Btn>
    );
  };

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          title="Deployments"
          sub={
            <span className="mono">
              {svc.name} · {env} · zero-downtime · last 5 images kept warm for instant rollback
            </span>
          }
        >
          <Btn variant="s" onClick={() => rollback.mutate({ path: { dep: "dep_142" } })}>
            <Icon id="s-undo" className="h-3 w-3" /> Roll back to #141
          </Btn>
        </Pghead>

        <div className="tblwrap">
          <table className="tbl">
            <thead>
              <tr>
                <th>Deploy</th>
                <th>Change</th>
                <th>By</th>
                <th>When</th>
                <th>Health</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {ROWS.map((row) => (
                <tr
                  key={row.deploy}
                  style={
                    row.highlight
                      ? { background: "color-mix(in srgb, var(--warn) 6%, transparent)" }
                      : undefined
                  }
                >
                  <td className={row.mut ? "mono text-ink3" : "mono"}>{row.deploy}</td>
                  <td>
                    {row.change} <span className="mono text-ink3">{row.sha}</span>
                  </td>
                  <td>{row.by}</td>
                  <td className="mono">{row.when}</td>
                  <td>
                    <Stlab tone={row.health.tone}>{row.health.label}</Stlab>
                  </td>
                  <td className="text-right">{action(row)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <Card className="flex flex-col gap-2.5 p-4">
          <Eyebrow>Instances</Eyebrow>
          <div className="flex items-center gap-5">
            {INSTANCES.map((i) => (
              <span key={i} className="mono flex items-center gap-1.5 text-11p5">
                <Dot tone="ok" />
                {i}
              </span>
            ))}
          </div>
          <p className="text-11 leading-relaxed text-ink3">
            rollout replaces instances one at a time behind the health check — traffic never sees a
            gap
          </p>
        </Card>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/deployments")({
  component: DeploymentsPage,
});
