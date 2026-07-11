import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Icon } from "@/design-system/icon";
import { Kbd } from "@/design-system/kbd";
import { Pill, type PillTone } from "@/design-system/pill";
import { useServices } from "@/features/services/hooks";

/**
 * D1 · SQL Editor — runs as the binding's role against a selectable target.
 * 08-api has no query-execution endpoint (spec gap, a finding): Run is
 * client-side canon playback — a 650 ms delay, then the frame's results.
 */

const DEFAULT_SQL =
  "SELECT id, customer_id, total, status, created_at\nFROM orders\nWHERE customer_id = 8241\nORDER BY created_at DESC LIMIT 50;";

const STATUS_TONE: Record<string, PillTone> = { paid: "ok", pending: "warn", refunded: "err" };

const RESULT_ROWS: Array<[string, string, string, string, string]> = [
  ["ord_91f2a8", "8241", "₹4,180.00", "paid", "2026-07-06 13:41:08"],
  ["ord_90d773", "8241", "₹1,240.00", "paid", "2026-07-04 19:22:51"],
  ["ord_8ce210", "8241", "₹860.00", "pending", "2026-07-01 09:03:17"],
  ["ord_8721bb", "8241", "₹12,400.00", "paid", "2026-06-27 21:48:02"],
  ["ord_84f0c4", "8241", "₹2,150.00", "refunded", "2026-06-25 11:15:33"],
];

function SqlEditorPage() {
  const { org, project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(env);
  const svc = services.data?.find((s) => s.name === service || s.id === service);
  const [sql, setSql] = useState(DEFAULT_SQL);
  const [phase, setPhase] = useState<"idle" | "running" | "done">("idle");

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

  const run = () => {
    if (phase === "running") return;
    setPhase("running");
    window.setTimeout(() => setPhase("done"), 650);
  };

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          title="SQL Editor"
          sub={
            <span>
              Running as role: <span className="mono">readonly</span> (from your binding) · every
              query is logged
            </span>
          }
        >
          <span className="envpill">
            <Icon id="s-branch" />
            target: preview/pr-142
            <Icon id="s-chevd" className="h-[11px] w-[11px]" />
          </span>
          <Btn variant="s">Explain</Btn>
          <Btn
            variant="p"
            onClick={run}
            disabled={phase === "running"}
            disabledReason="Running against preview/pr-142…"
          >
            Run <Kbd>⌘↵</Kbd>
          </Btn>
        </Pghead>

        <textarea
          className="logwell w-full resize-y"
          rows={5}
          value={sql}
          spellCheck={false}
          onChange={(e) => setSql(e.target.value)}
          onKeyDown={(e) => {
            if (e.metaKey && e.key === "Enter") run();
          }}
          aria-label="SQL editor"
        />

        {phase === "done" ? (
          <>
            <div className="flex items-center gap-2.5">
              <Pill tone="ok">8 rows</Pill>
              <span className="mono text-11p5 text-warn">642 ms</span>
              <Pill tone="warn">plan: Seq Scan on orders</Pill>
              <span className="flex-1" />
              <Link
                to="/$org/$project/svc/$service/insights"
                params={{ org, project, service: svc.name }}
                search={{ env, proposal: "prp_7c31a2" }}
              >
                <Btn variant="a">
                  <Icon id="s-ai" />
                  Assistant found an index opportunity — view proposal
                </Btn>
              </Link>
            </div>

            {/* The strip says 8 rows but the frame renders 5 — source inconsistency, carried verbatim. */}
            <div className="tblwrap">
              <table className="tbl">
                <thead>
                  <tr>
                    <th>id</th>
                    <th>customer_id</th>
                    <th>total</th>
                    <th>status</th>
                    <th>created_at</th>
                  </tr>
                </thead>
                <tbody>
                  {RESULT_ROWS.map(([id, customer, total, status, created]) => (
                    <tr key={id}>
                      <td className="mono text-11">{id}</td>
                      <td className="mono text-11">{customer}</td>
                      <td className="mono text-11">{total}</td>
                      <td>
                        <Pill tone={STATUS_TONE[status] ?? "mut"}>{status}</Pill>
                      </td>
                      <td className="mono text-11 text-ink2">{created}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        ) : null}

        <p className="text-10p5 text-ink3">
          Writes require a read-write role and run against the selected target — pointing the editor
          at a branch is one dropdown, so experiments never touch production.
        </p>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/sql-editor")({
  component: SqlEditorPage,
});
