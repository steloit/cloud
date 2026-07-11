import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Glyph } from "@/design-system/glyph";
import { Pill } from "@/design-system/pill";
import { useServices } from "@/features/services/hooks";
import { ScheduleDrawer } from "@/features/services/schedule-drawer";
import { listSchedulesOptions } from "@/lib/api";

/**
 * D17 · Schedules — the table rides the real schedules endpoint; publishes,
 * next/last runs are frame-fixed display (the schema carries no run history —
 * finding). Worker services render the same grammar (S5: worker has the same
 * two canon schedules, receipts-daily + cleanup-tmp).
 */

const DISPLAY: Record<string, { cronNote: string; publishes: string; next: string; last: string }> =
  {
    "receipts-daily": {
      cronNote: "20:30 UTC",
      publishes: "order.digest",
      next: "in 11h 23m",
      last: "ok · 02:00 · 3m 41s",
    },
    "cleanup-tmp": {
      cronNote: "hourly",
      publishes: "cleanup.tick",
      next: "in 37 m",
      last: "ok · 14:00 · 12s",
    },
  };

function SchedulesPage() {
  const { service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(env);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const svc = services.data?.find((s) => s.name === service || s.id === service);
  const schedules = useQuery({
    ...listSchedulesOptions({ path: { service: svc?.id ?? "" } }),
    select: (r) => r.data ?? [],
    enabled: Boolean(svc),
  });

  if (!svc) return <main className="main" />;
  // S5: worker services carry schedules too — same grammar, same canon rows.
  if (svc.product !== "queue" && svc.product !== "worker") {
    return (
      <main className="main">
        <div className="pgpad">
          <Card className="p-4 text-[12px] text-ink2">
            Schedules are a Queue and Worker surface — {svc.name} is a {svc.product} service.
          </Card>
        </div>
      </main>
    );
  }

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          before={<Glyph id="s-cron" />}
          title="Schedules"
          sub={
            <span className="mono">
              {svc.name} · scheduled publishes · timezone Asia/Kolkata (org) — UTC shown alongside
            </span>
          }
        >
          <Btn variant="s" onClick={() => setDrawerOpen(true)}>
            New schedule (U4)
          </Btn>
        </Pghead>

        <Card>
          <div className="tblwrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th>Schedule</th>
                  <th>Cron</th>
                  <th>Publishes</th>
                  <th>Next run</th>
                  <th>Last run</th>
                  <th aria-label="Actions" />
                </tr>
              </thead>
              <tbody>
                {(schedules.data ?? []).map((s) => {
                  const d = DISPLAY[s.name];
                  return (
                    <tr key={s.id}>
                      <td>
                        <b className="mono">{s.name}</b>
                      </td>
                      <td className="mono">
                        {s.cron}
                        {d ? ` · ${d.cronNote}` : ""}
                      </td>
                      <td className="mono">{d?.publishes ?? "—"}</td>
                      <td className="mono">{d?.next ?? "—"}</td>
                      <td>
                        {d ? <Pill tone="ok">{d.last}</Pill> : <Pill tone="mut">not yet run</Pill>}
                      </td>
                      <td>
                        <span className="flex justify-end gap-2">
                          <Btn
                            variant="s"
                            className="h-6 px-2.5 text-[10.5px]"
                            disabled
                            disabledReason="No per-schedule resource in the spec (finding)"
                          >
                            Run now
                          </Btn>
                          <Btn
                            variant="s"
                            className="h-6 px-2.5 text-[10.5px]"
                            disabled
                            disabledReason="No per-schedule resource in the spec (finding)"
                          >
                            Pause
                          </Btn>
                        </span>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </Card>

        <p className="text-[10.5px] leading-relaxed text-ink3">
          A schedule that doesn't fire is an alert, not a silence — missed runs page through Observe
          like any other failure. Payload templates support{" "}
          <span className="mono">{"{{date}}"}</span> interpolation.
        </p>
      </div>

      {drawerOpen ? <ScheduleDrawer svc={svc} onClose={() => setDrawerOpen(false)} /> : null}
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/schedules")({
  component: SchedulesPage,
});
