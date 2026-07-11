import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { MetricChart } from "@/design-system/chart";
import { TypedConfirmModal } from "@/design-system/confirm";
import { Copybit } from "@/design-system/copybit";
import { Eyebrow } from "@/design-system/eyebrow";
import { Glyph } from "@/design-system/glyph";
import { Flabel, Inp } from "@/design-system/inp";
import { Metric } from "@/design-system/metric";
import { Pill } from "@/design-system/pill";
import { RenameModal } from "@/features/common/rename-modal";
import { toMarkers, toSeries, useMetrics } from "@/features/observe/hooks";
import { EditShapeDrawer, type ShapeOption } from "@/features/services/edit-shape-drawer";
import type { Service, UpdateServiceData } from "@/lib/api";
import { errorMessage, updateServiceMutation } from "@/lib/api";
import { cn } from "@/lib/utils";

/**
 * D13 · Valkey Metrics and D14 · Valkey Settings — the per-service metrics
 * grammar (same engine as Observe, scoped to one instance) and the mode-first
 * settings surface. Tile values are frame-fixed canon; the charts ride the
 * real telemetry queries.
 */

export interface TabProps {
  svc: Service;
  org: string;
  project: string;
  env: string;
}

// --------------------------------------------------------------------------
// D13 · Metrics
// --------------------------------------------------------------------------

const RANGES = ["30m", "1h", "6h", "24h", "7d"] as const;
const PANES = ["Overview", "Commands", "Memory", "Keyspace", "Clients"] as const;

export function ValkeyMetricsTab({ svc, env }: TabProps) {
  const [range, setRange] = useState<(typeof RANGES)[number]>("1h");
  const [pane, setPane] = useState<(typeof PANES)[number]>("Overview");

  const hitRate = useMetrics(env, "service:cache metric:hit_rate");
  // The canon telemetry lacks a memory series — the jobs queue_depth series
  // rides as the closest canon proxy for the memory-vs-eviction pane.
  const memoryProxy = useMetrics(env, "service:jobs metric:queue_depth");
  const opsProxy = useMetrics(env, "service:api metric:requests");

  return (
    <>
      <Pghead
        before={<Glyph id="s-chip" />}
        title="Metrics"
        sub={
          <span className="mono">
            {svc.name} · {env}
          </span>
        }
      >
        <Pill tone="ok">live · 10 s</Pill>
        <Btn
          variant="s"
          disabled
          disabledReason="Alert rules compose in Observe → Alerts (U8) — per-pane pre-fill lands later"
        >
          Create alert
        </Btn>
        <Btn variant="s" disabled disabledReason="No export endpoint in the spec (finding)">
          Export
        </Btn>
      </Pghead>

      <div className="flex items-center gap-3">
        <div className="chiprow">
          {RANGES.map((r) => (
            <button
              key={r}
              type="button"
              className={cn("chip", range === r && "on")}
              onClick={() => setRange(r)}
            >
              {r}
            </button>
          ))}
          <span className="chip border-dashed">compare: prev hour</span>
        </div>
        <span className="flex-1" />
        <span className="text-10p5 text-ink3">◆ deploys · ◇ scale — one timeline with Observe</span>
      </div>

      <div className="tabrow">
        {PANES.map((p) => (
          <button
            key={p}
            type="button"
            className={cn("tab", pane === p && "on")}
            onClick={() => setPane(p)}
          >
            {p}
          </button>
        ))}
      </div>

      {pane !== "Overview" ? (
        <p className="text-11p5 text-ink3">
          The {pane} pane needs command-level telemetry the canon lacks — Overview is live.
        </p>
      ) : (
        <>
          <div className="grid grid-cols-5 gap-3">
            <Metric label="Hit ratio" value="98.2%" note="▼ −0.1 pt" />
            <Metric label="p99 latency" value="0.8 ms" note="→ flat" />
            <Metric label="Ops / sec" value="3.4k" note={<span className="text-ok">▲ +4%</span>} />
            <Metric label="Memory" value="612 MB / 1 GB" note="▲ +18 MB" />
            <Metric
              label="Evictions"
              value="0 / h"
              note={<span className="text-ok">→ none this week</span>}
            />
          </div>

          <Card className="flex flex-col gap-2.5 p-4">
            <div className="flex items-center gap-2.5">
              <span className="text-12p5 font-semibold">Hit ratio</span>
              <span className="text-10p5 text-ink3">threshold &lt; 95% → alert</span>
              <span className="flex-1" />
              <span className="mono text-10p5 text-ink2">
                ◆ api deploy #142 · brief dip: new keys
              </span>
            </div>
            <MetricChart
              series={toSeries(hitRate.data)}
              threshold={95}
              markers={toMarkers(hitRate.data)}
              unit="%"
              size="lg"
            />
            <div className="border-hair border-t pt-2.5 text-10p5 text-ink3">
              brush: 13:40 → now · a deploy that cold-starts the cache is expected — a decay without
              one is not
            </div>
          </Card>

          <div className="grid grid-cols-2 gap-3.5">
            <Card className="flex flex-col gap-2 p-3.5">
              <div className="flex items-center gap-2">
                <span className="text-11p5 font-medium text-ink2">Memory vs eviction line</span>
                <span className="flex-1" />
                <span className="mono text-12">612 MB</span>
              </div>
              <MetricChart series={toSeries(memoryProxy.data)} tone="steel" size="sm" />
              <span className="text-10p5 text-ink3">
                612 MB of 1 GB — headroom before allkeys-lru engages · frag ratio 1.03
              </span>
            </Card>
            <Card className="flex flex-col gap-2 p-3.5">
              <div className="flex items-center gap-2">
                <span className="text-11p5 font-medium text-ink2">Ops by command class</span>
                <span className="flex-1" />
                <span className="text-10p5 text-ink3">━ reads · ━ writes</span>
              </div>
              <MetricChart series={toSeries(opsProxy.data)} tone="ok" size="sm" />
              <span className="text-10p5 text-ink3">
                read-heavy shape fits cache mode — the assistant flags a drift toward writes as a
                mode-fit question
              </span>
            </Card>
          </div>

          <div className="flex items-center gap-2.5 text-10p5 text-ink3">
            <span>raw 15 d · downsampled 13 mo · every pane is a query:</span>
            <Copybit>steloit metrics query cache 'hit_ratio' --since 1h</Copybit>
          </div>
        </>
      )}
    </>
  );
}

// --------------------------------------------------------------------------
// D14 · Settings
// --------------------------------------------------------------------------

/** Canon memory shapes — prices from the C3 create block (256 MB · $8, 1 GB · $22, 4 GB · $64). */
const VK_SHAPE_1GB: ShapeOption = {
  id: "1024",
  title: "1 GB",
  price: 22,
  patch: { memory_mb: 1024 },
};
const VK_SHAPES: ShapeOption[] = [
  { id: "256", title: "256 MB", price: 8, patch: { memory_mb: 256 } },
  VK_SHAPE_1GB,
  { id: "4096", title: "4 GB", price: 64, patch: { memory_mb: 4096 } },
];

export function ValkeySettingsTab({ svc, env }: TabProps) {
  const shape = (svc.shape ?? {}) as Record<string, unknown>;
  const currentShape =
    VK_SHAPES.find((s) => s.id === String(shape.memory_mb ?? 1024)) ?? VK_SHAPE_1GB;
  const [editingShape, setEditingShape] = useState(false);
  const [renaming, setRenaming] = useState(false);
  const [flushing, setFlushing] = useState(false);
  const update = useMutation(updateServiceMutation());
  return (
    <>
      <Pghead
        before={<Glyph id="s-chip" />}
        title="Settings"
        sub={
          <span className="mono">
            {svc.name} · {env}
          </span>
        }
      />

      <Card className="flex flex-col gap-2.5 p-4">
        <Eyebrow>Identity</Eyebrow>
        <div className="flex items-center gap-2.5">
          <span className="mono text-12">{svc.name}</span>
          <span className="flex-1" />
          <Btn variant="gh" onClick={() => setRenaming(true)}>
            Rename…
          </Btn>
        </div>
      </Card>

      <Card className="flex flex-col gap-2.5 p-4">
        <Eyebrow>Mode</Eyebrow>
        <div className="flex items-center gap-2.5">
          <Pill tone="mut">cache · data may be evicted</Pill>
          <span className="flex-1" />
          <Btn
            variant="s"
            disabled
            disabledReason="Mode switch rides updateService.shape — apply flow lands in Phase 5"
          >
            Switch to Durable…
          </Btn>
        </div>
        <p className="text-11p5 leading-relaxed text-ink2">
          stated before apply: +$6/mo · AOF persistence · one restart (~10 s) · existing keys
          preserved
        </p>
      </Card>

      <Card className="grid grid-cols-3 gap-4 p-4">
        <div className="flex flex-col gap-1.5">
          <Flabel>Max memory</Flabel>
          <div className="flex items-center gap-2">
            <span className="text-12">{currentShape.title}</span>
            <span className="mono text-11 text-ink3">${currentShape.price}/mo</span>
            <span className="flex-1" />
            <Btn variant="s" className="h-6 px-2.5 text-10p5" onClick={() => setEditingShape(true)}>
              Edit shape…
            </Btn>
          </div>
        </div>
        <div>
          <Flabel htmlFor="valkey-eviction">Eviction policy</Flabel>
          <Inp id="valkey-eviction" className="mono" value="allkeys-lru" readOnly />
        </div>
        <div className="flex flex-col gap-1.5 opacity-75">
          <Flabel htmlFor="valkey-tls">TLS</Flabel>
          <Inp id="valkey-tls" value="enforced" readOnly />
          <span>
            <Pill tone="mut">locked by org policy</Pill>
          </span>
        </div>
      </Card>

      <Card className="flex flex-col gap-3 border-err/40 p-4">
        <Eyebrow className="text-err">Danger zone</Eyebrow>
        <div className="flex items-start gap-3">
          <Btn variant="dgr" onClick={() => setFlushing(true)}>
            FLUSHALL…
          </Btn>
          <p className="text-11 leading-relaxed text-ink3">
            requires write-unlock + typing the instance name · 41k keys · api will cold-start its
            cache — hit ratio dips are expected and annotated in Observe
          </p>
        </div>
        <div className="flex items-center gap-3 border-hair border-t pt-3">
          <Btn
            variant="dgr"
            disabled
            disabledReason="Blocked — 1 binding depends on this instance; detach it first"
          >
            Delete cache…
          </Btn>
          <Pill tone="err">blocked — 1 binding depends on this instance</Pill>
        </div>
      </Card>

      {flushing ? (
        // The B6 grammar moves INTO the overlay: the typed name gates the verb
        // first, then the honest endpoint reason keeps it disabled (reason
        // ladder — type-gate, then endpoint-gate).
        <TypedConfirmModal
          title={`FLUSHALL ${svc.name}`}
          consequence="every key in every db goes — bound consumers reconnect to an empty keyspace, hit ratio starts from zero"
          expected={svc.name}
          verb="FLUSHALL"
          gatedReason="No flush endpoint in the spec (finding)"
          onConfirm={() => undefined}
          onClose={() => setFlushing(false)}
        />
      ) : null}

      {editingShape ? (
        <EditShapeDrawer
          svc={svc}
          env={env}
          options={VK_SHAPES}
          currentId={currentShape.id}
          onClose={() => setEditingShape(false)}
        />
      ) : null}

      {renaming ? (
        <RenameModal
          entity="service"
          current={svc.name}
          consequence="binding env-var names re-derive from the new name at the next deploy — connection strings rotate with it"
          pending={update.isPending}
          onClose={() => setRenaming(false)}
          onSave={(name) =>
            update.mutate(
              {
                path: { service: svc.id },
                // Finding: 08-api's PATCH /services body carries no `name`
                // field — rename needs a spec change; the MSW echo handler
                // accepts it meanwhile, hence the cast.
                body: { name } as unknown as UpdateServiceData["body"],
              },
              {
                // PATCH /services/:service is the echo handler (DB8 precedent) —
                // the rename comes back once, fixtures win on refetch.
                onSuccess: () => {
                  toast.success(`Renamed to ${name} — bindings re-derive at the next deploy`);
                  setRenaming(false);
                },
                onError: (err) => toast.error(errorMessage(err)),
              },
            )
          }
        />
      ) : null}
    </>
  );
}
