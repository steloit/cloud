import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { MetricChart } from "@/design-system/chart";
import { Copybit } from "@/design-system/copybit";
import { Eyebrow } from "@/design-system/eyebrow";
import { Glyph } from "@/design-system/glyph";
import { Flabel, Inp } from "@/design-system/inp";
import { Metric } from "@/design-system/metric";
import { Pill } from "@/design-system/pill";
import { toMarkers, toSeries, useMetrics } from "@/features/observe/hooks";
import type { Service } from "@/lib/api";
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
        <span className="text-[10.5px] text-ink3">
          ◆ deploys · ◇ scale — one timeline with Observe
        </span>
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
        <p className="text-[11.5px] text-ink3">
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
              <span className="text-[12.5px] font-semibold">Hit ratio</span>
              <span className="text-[10.5px] text-ink3">threshold &lt; 95% → alert</span>
              <span className="flex-1" />
              <span className="mono text-[10.5px] text-ink2">
                ◆ api deploy #142 · brief dip: new keys
              </span>
            </div>
            <MetricChart
              series={toSeries(hitRate.data)}
              threshold={95}
              markers={toMarkers(hitRate.data)}
              unit="%"
              height={150}
            />
            <div className="border-hair border-t pt-2.5 text-[10.5px] text-ink3">
              brush: 13:40 → now · a deploy that cold-starts the cache is expected — a decay without
              one is not
            </div>
          </Card>

          <div className="grid grid-cols-2 gap-3.5">
            <Card className="flex flex-col gap-2 p-3.5">
              <div className="flex items-center gap-2">
                <span className="text-[11.5px] font-medium text-ink2">Memory vs eviction line</span>
                <span className="flex-1" />
                <span className="mono text-[12px]">612 MB</span>
              </div>
              <MetricChart series={toSeries(memoryProxy.data)} tone="steel" height={72} />
              <span className="text-[10.5px] text-ink3">
                612 MB of 1 GB — headroom before allkeys-lru engages · frag ratio 1.03
              </span>
            </Card>
            <Card className="flex flex-col gap-2 p-3.5">
              <div className="flex items-center gap-2">
                <span className="text-[11.5px] font-medium text-ink2">Ops by command class</span>
                <span className="flex-1" />
                <span className="text-[10.5px] text-ink3">━ reads · ━ writes</span>
              </div>
              <MetricChart series={toSeries(opsProxy.data)} tone="ok" height={72} />
              <span className="text-[10.5px] text-ink3">
                read-heavy shape fits cache mode — the assistant flags a drift toward writes as a
                mode-fit question
              </span>
            </Card>
          </div>

          <div className="flex items-center gap-2.5 text-[10.5px] text-ink3">
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

const MEMORY_CHIPS = ["256 MB", "1 GB · $22", "4 GB · $64"] as const;

export function ValkeySettingsTab({ svc, env }: TabProps) {
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
        <p className="text-[11.5px] leading-relaxed text-ink2">
          stated before apply: +$6/mo · AOF persistence · one restart (~10 s) · existing keys
          preserved
        </p>
      </Card>

      <Card className="grid grid-cols-3 gap-4 p-4">
        <div className="flex flex-col gap-1.5">
          <Flabel>Max memory</Flabel>
          <div className="chiprow">
            {MEMORY_CHIPS.map((c) => (
              <span
                key={c}
                className={cn("chip", c === "1 GB · $22" && "on")}
                title="Memory changes ride updateService.shape — apply flow lands in Phase 5"
              >
                {c}
              </span>
            ))}
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
          <Btn variant="dgr" disabled disabledReason="No flush endpoint in the spec (finding)">
            FLUSHALL…
          </Btn>
          <p className="text-[11px] leading-relaxed text-ink3">
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
    </>
  );
}
