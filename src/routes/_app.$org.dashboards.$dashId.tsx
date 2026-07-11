import { useMutation } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { MetricChart, Spark } from "@/design-system/chart";
import { Flabel, Inp } from "@/design-system/inp";
import { Pill } from "@/design-system/pill";
import { useDashboard } from "@/features/dashboards/hooks";
import { toMarkers, toSeries, useMetrics } from "@/features/observe/hooks";
import { useBillingOverview } from "@/features/org/hooks";
import { useDeployments } from "@/features/services/hooks";
import {
  addWidgetMutation,
  updateDashboardMutation,
  type Widget,
  type WidgetInput,
} from "@/lib/api";
import { fmtMoney } from "@/lib/fmt";
import { cn } from "@/lib/utils";

/**
 * DB7 · checkout-health — the one user dashboard in canon fixtures. The grid
 * renders FROM the API widgets array by source/viz; titles are frame-fixed.
 * Edit mode adds dashed borders, ⋮⋮/× affordances and the add-widget drawer.
 */

const TITLES: Record<string, string> = {
  wdg_p95: "api p95 · production",
  wdg_orderr: "order.paid errors",
  wdg_dlq: "DLQ depth · jobs",
  wdg_spend: "MTD spend",
  wdg_deploys: "Deploys · production",
};

const SPAN: Record<number, string> = {
  4: "col-span-4",
  8: "col-span-8",
  12: "col-span-12",
};

function WidgetShell({
  widget,
  editing,
  children,
}: {
  widget: Widget;
  editing: boolean;
  children: React.ReactNode;
}) {
  return (
    <Card
      className={cn(
        "flex flex-col gap-2 p-3.5",
        SPAN[widget.pos?.w ?? 4] ?? "col-span-4",
        editing && "border-dashed",
      )}
    >
      <div className="flex items-center gap-2">
        {editing ? (
          <span className="mono cursor-grab text-[11px] text-ink3" title="Drag to rearrange">
            ⋮⋮
          </span>
        ) : null}
        <span className="text-[12.5px] font-semibold">{TITLES[widget.id] ?? widget.query}</span>
        <span className="flex-1" />
        {editing ? (
          <Btn
            variant="gh"
            className="h-5 px-1.5"
            disabled
            disabledReason="Widget removal lands in Phase 4"
          >
            ×
          </Btn>
        ) : null}
      </div>
      {children}
    </Card>
  );
}

function MetricWidget({ widget }: { widget: Widget }) {
  // Logs-source widgets need a logs series the canon telemetry lacks —
  // wdg_orderr renders the jobs DLQ depth as the closest canon series.
  const query = widget.source === "logs" ? "service:jobs metric:dlq_depth" : widget.query;
  const metrics = useMetrics("production", query);
  const isP95 = widget.id === "wdg_p95";
  return (
    <MetricChart
      series={toSeries(metrics.data)}
      threshold={isP95 ? 800 : undefined}
      markers={isP95 ? toMarkers(metrics.data) : undefined}
      unit={isP95 ? "ms" : ""}
      tone={widget.source === "logs" ? "warn" : "steel"}
      height={130}
    />
  );
}

function StatWidget({ widget, org }: { widget: Widget; org: string }) {
  const billing = useBillingOverview(org);
  if (widget.id === "wdg_spend") {
    return (
      <>
        <span className="mono text-[19px] font-semibold">
          {billing.data ? fmtMoney(billing.data.mtd_cents ?? 0) : "…"}
        </span>
        <span className="text-[10.5px] text-ink3">
          a Billing widget beside metrics — the reason this isn't under Observe
        </span>
      </>
    );
  }
  return <span className="mono text-[19px] font-semibold">2</span>;
}

function ListWidget() {
  const deploys = useDeployments("production");
  // Frame-fixed lines keyed to the canon deployments the API returns.
  const lines: Record<number, string> = {
    143: "#143 fix/orders-query — applies prp_7c31a2 · Jul 2",
    142: "#142 gift cards · Jul 2",
  };
  const rows = (deploys.data ?? [])
    .filter((d) => d.number !== undefined && d.number in lines)
    .sort((a, b) => (b.number ?? 0) - (a.number ?? 0));
  return (
    <div className="flex flex-col gap-1">
      {rows.map((d) => (
        <span key={d.id} className="mono text-[11px] text-ink2">
          {lines[d.number ?? 0]}
        </span>
      ))}
    </div>
  );
}

function WidgetBody({ widget, org }: { widget: Widget; org: string }) {
  if (widget.viz === "stat") return <StatWidget widget={widget} org={org} />;
  if (widget.viz === "list") return <ListWidget />;
  return <MetricWidget widget={widget} />;
}

const SOURCES = ["Metrics", "Logs", "Cost", "Deploys", "AI insights", "Alerts"] as const;
type SourceChip = (typeof SOURCES)[number];
const SOURCE_TO_API: Record<SourceChip, WidgetInput["source"]> = {
  Metrics: "metrics",
  Logs: "logs",
  Cost: "cost",
  Deploys: "deploys",
  "AI insights": "ai",
  Alerts: "alerts",
};

// The drawer offers bar/log-stream while the API viz enum is area/list —
// a frames↔schema divergence, surfaced as a finding; submissions map across.
const VIZES = ["line", "stat", "bar", "table", "log stream"] as const;
type VizChip = (typeof VIZES)[number];
const VIZ_TO_API: Record<VizChip, WidgetInput["viz"]> = {
  line: "line",
  stat: "stat",
  bar: "area",
  table: "table",
  "log stream": "list",
};

function AddWidgetDrawer({ onClose }: { onClose: () => void }) {
  const [source, setSource] = useState<SourceChip>("Metrics");
  const [query, setQuery] = useState("cache.memory_used{service=cache}");
  const [viz, setViz] = useState<VizChip>("line");
  const add = useMutation(addWidgetMutation());
  // cache.memory_used has no canon telemetry — the preview spark rides the
  // db-main connections warn series as the closest canon stand-in.
  const preview = useMetrics("production", "service:db-main metric:connections");

  const submit = () => {
    // Canon-mode add is echo-only: MSW has no POST /dashboards/:dash/widgets
    // handler, so the mutation fires for the contract's sake and success is
    // toasted optimistically.
    add.mutate({
      path: { dash: "dsh_checkout_health" },
      body: { source: SOURCE_TO_API[source], query, viz: VIZ_TO_API[viz] },
    });
    toast.success("Widget added");
    onClose();
  };

  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: scrim dismiss mirrors Esc
    // biome-ignore lint/a11y/useKeyWithClickEvents: Cancel is the keyboard path
    <div className="fixed inset-0 z-40 bg-[rgba(6,9,12,0.44)]" onClick={onClose}>
      {/* biome-ignore lint/a11y/useKeyWithClickEvents: click only stops scrim dismiss */}
      <div
        className="absolute top-0 right-0 flex h-full w-[424px] flex-col border-hair border-l bg-surface shadow-e2"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-label="Add widget"
      >
        <div className="border-hair border-b px-5 py-4">
          <div className="text-[14px] font-semibold">Add widget</div>
          <div className="mono mt-0.5 text-[10.5px] text-ink3">
            checkout-health · any plane, one grid
          </div>
        </div>

        <div className="flex flex-1 flex-col gap-4 overflow-y-auto p-5">
          <div className="flex flex-col gap-1.5">
            <Flabel>Source</Flabel>
            <div className="chiprow">
              {SOURCES.map((s) => (
                <button
                  key={s}
                  type="button"
                  className={cn("chip", source === s && "on")}
                  onClick={() => setSource(s)}
                >
                  {s}
                </button>
              ))}
            </div>
          </div>

          <div>
            <Flabel htmlFor="widget-query">Query</Flabel>
            <Inp
              id="widget-query"
              className="mono"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Flabel>Visualization</Flabel>
            <div className="chiprow">
              {VIZES.map((v) => (
                <button
                  key={v}
                  type="button"
                  className={cn("chip", viz === v && "on")}
                  onClick={() => setViz(v)}
                >
                  {v}
                </button>
              ))}
            </div>
          </div>

          <Card className="flex flex-col gap-2 p-3.5">
            <div className="flex items-center gap-2">
              <span className="text-[12.5px] font-semibold">Preview</span>
              <Pill tone="ok">live</Pill>
            </div>
            <Spark series={toSeries(preview.data)} tone="warn" width={200} height={36} />
            <span className="mono text-[11px] text-ink2">
              95% — the assistant's open insight, now on your grid
            </span>
          </Card>

          <Card className="p-3.5 text-[11px] leading-relaxed text-ink2">
            Or skip this drawer entirely: the ⚑ Add to dashboard button on any chart in Metrics,
            Logs or Billing lands here pre-filled — same grammar as alert rules (U8).
          </Card>
        </div>

        <div className="flex items-center gap-2 border-hair border-t px-5 py-3.5">
          <span className="text-[10.5px] text-ink3">Widget 7 · reads only, never provisions</span>
          <span className="flex-1" />
          <Btn variant="s" onClick={onClose}>
            Cancel
          </Btn>
          <Btn variant="p" onClick={submit}>
            Add widget
          </Btn>
        </div>
      </div>
    </div>
  );
}

function DashboardDetail() {
  const { org, dashId } = Route.useParams();
  const isCheckout = dashId === "checkout-health" || dashId === "dsh_checkout_health";
  const dashboard = useDashboard("dsh_checkout_health");
  const [editing, setEditing] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const update = useMutation(updateDashboardMutation());

  if (!isCheckout) {
    return (
      <main className="main">
        <div className="pgpad">
          <Card dashed className="flex flex-col items-center gap-3 py-14">
            <div className="text-[13px] font-medium">
              No dashboard named {dashId} — check My dashboards.
            </div>
            <Link to="/$org/dashboards/mine" params={{ org }}>
              <Btn variant="s">My dashboards</Btn>
            </Link>
          </Card>
        </div>
      </main>
    );
  }

  const toggleEdit = () => {
    if (editing) {
      // Canon-mode update is echo-only: MSW has no PATCH /dashboards/:dash
      // handler, so the layout save fires for the contract's sake and success
      // is toasted optimistically.
      update.mutate({
        path: { dash: "dsh_checkout_health" },
        body: { layout: dashboard.data?.layout ?? {} },
      });
      toast.success("Layout saved");
    }
    setEditing((e) => !e);
  };

  return (
    <main className="main">
      <div className="pgpad">
        <Pghead
          title={
            <span className="flex items-center gap-2.5">
              checkout-health <Pill tone="st">ecommerce</Pill> <Pill tone="mut">personal</Pill>
              {editing ? <Pill tone="st">editing</Pill> : null}
            </span>
          }
          sub="Project-scoped to ecommerce — born filtered, env follows the crumb when opened in-project. Created Jul 2, during the incident. Drag ⋮⋮ to rearrange; layout saves on Done."
        >
          <Btn variant="s" disabled disabledReason="Sharing lands in Phase 4">
            Share…
          </Btn>
          <Btn variant="p" onClick={toggleEdit}>
            {editing ? "Done — save layout" : "Edit layout"}
          </Btn>
        </Pghead>

        <div className="grid grid-cols-12 gap-3.5">
          {(dashboard.data?.widgets ?? []).map((widget) => (
            <WidgetShell key={widget.id} widget={widget} editing={editing}>
              <WidgetBody widget={widget} org={org} />
            </WidgetShell>
          ))}
          {editing ? (
            <button
              type="button"
              className="card col-span-4 flex min-h-[90px] items-center justify-center border-dashed bg-transparent text-[12px] text-ink3 hover:border-ink3"
              onClick={() => setDrawerOpen(true)}
            >
              + Add widget
            </button>
          ) : null}
        </div>
      </div>
      {drawerOpen ? <AddWidgetDrawer onClose={() => setDrawerOpen(false)} /> : null}
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/dashboards/$dashId")({
  component: DashboardDetail,
});
