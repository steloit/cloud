import { createFileRoute } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";

/**
 * DB6 · Dashboard templates — arrangements only; a template binds to your
 * services when instantiated. Not the same asset as Settings → Templates.
 */

const LAYOUTS = [
  {
    name: "Golden Signals",
    widgets: 8,
    desc: "Latency, errors, throughput, saturation per service — the SRE classic, wired to whatever you run.",
  },
  {
    name: "Release Review",
    widgets: 6,
    desc: "Deploy markers, error delta before/after, rollback shortcut — for the hour after you ship.",
  },
  {
    name: "On-call Handoff",
    widgets: 7,
    desc: "Firing alerts, open AI insights, DLQ depths, last 24 h anomalies — what the next person needs.",
  },
  {
    name: "Postgres Deep-dive",
    widgets: 9,
    desc: "Connections, locks, slow queries, WAL, vacuum — one instance, everything that matters.",
  },
  {
    name: "Team Weekly",
    widgets: 6,
    desc: "Cost trend, deploy count, incident minutes, SLO budgets — the manager view, honestly labeled.",
  },
];

function DashboardTemplates() {
  return (
    <main className="main">
      <div className="pgpad">
        <Pghead
          title="Dashboard templates"
          sub="Starting layouts — a template is the arrangement only; it binds to your services when instantiated. Not the same asset as Settings → Templates (those are infrastructure shapes)."
        />

        <div className="grid grid-cols-3 gap-3.5">
          {LAYOUTS.map((layout) => (
            <Card key={layout.name} className="flex flex-col gap-2 p-3.5">
              <div className="flex items-center gap-2">
                <span className="text-[12.5px] font-semibold">{layout.name}</span>
                <span className="mono ml-auto text-[10.5px] text-ink3">
                  {layout.widgets} widgets
                </span>
              </div>
              <div className="flex-1 text-[11px] leading-relaxed text-ink2">{layout.desc}</div>
              <div>
                <Btn variant="s" disabled disabledReason="Instantiation lands in Phase 4">
                  Use layout
                </Btn>
              </div>
            </Card>
          ))}
        </div>

        <div className="mt-auto text-[10.5px] text-ink3">
          Instantiating never provisions anything — a dashboard reads, it doesn't create. Personas
          differ; that's the point of five layouts instead of one.
        </div>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/dashboards/layouts")({
  component: DashboardTemplates,
});
