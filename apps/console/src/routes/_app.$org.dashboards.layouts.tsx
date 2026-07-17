import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { NewDashboardModal } from "@/features/dashboards/new-dashboard-modal";

/**
 * DB6 · Dashboard templates — arrangements only; a template binds to your
 * services when instantiated. Not the same asset as Settings → Templates.
 * "Use layout" opens the DB8 modal prefilled with the layout's slug — the
 * frame speccs no instantiation flow of its own (finding), and only
 * dsh_tpl_release_review exists in the fixtures, so the modal's "Start from"
 * options stay DB8's regardless of which layout launched it (finding).
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

/** "Golden Signals" → "golden-signals" — dashboard names are kebab across canon. */
function slug(name: string): string {
  return name.toLowerCase().replace(/\s+/g, "-");
}

function DashboardTemplates() {
  const { org } = Route.useParams();
  // Strictly one overlay: the DB8 modal — prefilled from a layout card,
  // blank-default from the header CTA.
  const [modal, setModal] = useState<{ initialName?: string } | null>(null);

  return (
    <main className="main">
      <div className="pgpad">
        {/* Finding: frame DB6 titles this pane "Dashboard templates" — the design-system
            "Area · Thing" h1 grammar wins per the audit's P1 ruling. */}
        <Pghead
          title="Dashboards · Templates"
          sub="Starting layouts — a template is the arrangement only; it binds to your services when instantiated. Not the same asset as Settings → Templates (those are infrastructure shapes)."
        >
          <Btn variant="p" onClick={() => setModal({})}>
            New dashboard (DB8)
          </Btn>
        </Pghead>

        <div className="grid grid-cols-3 gap-3.5">
          {LAYOUTS.map((layout) => (
            <Card key={layout.name} className="flex flex-col gap-2 p-3.5">
              <div className="flex items-center gap-2">
                <span className="text-12p5 font-semibold">{layout.name}</span>
                <span className="mono ml-auto text-10p5 text-ink3">{layout.widgets} widgets</span>
              </div>
              <div className="flex-1 text-11 leading-relaxed text-ink2">{layout.desc}</div>
              <div>
                <Btn variant="s" onClick={() => setModal({ initialName: slug(layout.name) })}>
                  Use layout
                </Btn>
              </div>
            </Card>
          ))}
        </div>

        <div className="mt-auto text-10p5 text-ink3">
          Instantiating never provisions anything — a dashboard reads, it doesn't create. Personas
          differ; that's the point of five layouts instead of one.
        </div>
      </div>
      {modal ? (
        <NewDashboardModal
          org={org}
          initialName={modal.initialName}
          onClose={() => setModal(null)}
        />
      ) : null}
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/dashboards/layouts")({
  component: DashboardTemplates,
});
