import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { EmptyState } from "@/design-system/empty-state";
import { Glyph } from "@/design-system/glyph";
import { Pill } from "@/design-system/pill";
import { SkeletonRows } from "@/design-system/skeleton";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { useServices } from "@/features/services/hooks";
import { ScheduleDrawer } from "@/features/services/schedule-drawer";
import { listSchedulesOptions } from "@/lib/api";
import { resolveEnvKey } from "@/lib/canon-env";

/**
 * D17 · Schedules — the table rides the real schedules endpoint; publishes,
 * next/last runs are frame-fixed display (the schema carries no run history —
 * finding). Worker services render the same grammar (S5: worker has the same
 * two canon schedules, receipts-daily + cleanup-tmp).
 */

// Keyed on the canon fixture ids (not names) so a non-canon schedule that reuses
// a canon name never inherits canon run history — it degrades to "—" / "not yet
// run", and that degradation is intentional.
const DISPLAY: Record<string, { cronNote: string; publishes: string; next: string; last: string }> =
  {
    sch_receipts: {
      cronNote: "20:30 UTC",
      publishes: "order.digest",
      next: "in 11h 23m",
      last: "ok · 02:00 · 3m 41s",
    },
    sch_cleanup: {
      cronNote: "hourly",
      publishes: "cleanup.tick",
      next: "in 37 m",
      last: "ok · 14:00 · 12s",
    },
  };

function SchedulesPage() {
  const { project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(resolveEnvKey(project, env));
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
          <Card className="p-4 text-12 text-ink2">
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

        {/* Four-state grammar (16-qa): pending → skeleton, error → failure
            card, empty → EmptyState, else table. */}
        {schedules.isError ? (
          <ApiFailureCard
            title="Schedules didn't load"
            error={schedules.error}
            requestLine={`GET /services/${svc.id}/schedules`}
            onRetry={() => schedules.refetch()}
          />
        ) : !schedules.isPending && (schedules.data ?? []).length === 0 ? (
          <EmptyState
            compact
            icon="s-cron"
            title="No schedules yet"
            meaning={
              <>
                cron with a paper trail — every run is logged and timed, drift is checked, and a
                missed run pages like any other failure
              </>
            }
            cta={
              <Btn variant="s" onClick={() => setDrawerOpen(true)}>
                New schedule (U4)
              </Btn>
            }
            cli={`steloit schedule create receipts-daily --cron "0 2 * * *" --service ${svc.name}`}
          />
        ) : (
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
                  {schedules.isPending ? (
                    <SkeletonRows cols={6} />
                  ) : (
                    (schedules.data ?? []).map((s) => {
                      const d = DISPLAY[s.id];
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
                            {d ? (
                              <Pill tone="ok">{d.last}</Pill>
                            ) : (
                              <Pill tone="mut">not yet run</Pill>
                            )}
                          </td>
                          <td>
                            <span className="flex justify-end gap-2">
                              <Btn
                                variant="s"
                                className="h-6 px-2.5 text-10p5"
                                disabled
                                disabledReason="No per-schedule resource in the spec (finding)"
                              >
                                Run now
                              </Btn>
                              <Btn
                                variant="s"
                                className="h-6 px-2.5 text-10p5"
                                disabled
                                disabledReason="No per-schedule resource in the spec (finding)"
                              >
                                Pause
                              </Btn>
                            </span>
                          </td>
                        </tr>
                      );
                    })
                  )}
                </tbody>
              </table>
            </div>
          </Card>
        )}

        <p className="text-10p5 leading-relaxed text-ink3">
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
