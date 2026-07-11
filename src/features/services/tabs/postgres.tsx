import { useMutation } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { MetricChart } from "@/design-system/chart";
import { Copybit } from "@/design-system/copybit";
import { Inp } from "@/design-system/inp";
import { Metric } from "@/design-system/metric";
import { Pill } from "@/design-system/pill";
import { RenameModal } from "@/features/common/rename-modal";
import { minutesOfDay, toMarkers, toSeries, useLogs, useMetrics } from "@/features/observe/hooks";
import { DeleteServiceModal } from "@/features/services/delete-service-modal";
import { EditShapeDrawer, type ShapeOption } from "@/features/services/edit-shape-drawer";
import type { Service, UpdateServiceData } from "@/lib/api";
import { errorMessage, updateServiceMutation } from "@/lib/api";
import { cn } from "@/lib/utils";

/**
 * PostgreSQL depth tabs — D9 Metrics · D10 Logs · D12 Settings. Each tab is a
 * page BODY (its own Pghead, no <main>/pgpad — the dispatcher route renders
 * that chrome). Telemetry rides the same series Observe reads (useMetrics /
 * useLogs); everything frame-fixed below is called out where it is.
 */

export interface PostgresTabProps {
  svc: Service;
  org: string;
  project: string;
  env: string;
}

/* ================================ D9 · Metrics ================================ */

const RANGE_CHIPS = ["30m", "1h", "6h", "24h", "7d", "30d", "custom…"];
const METRIC_CATEGORIES = [
  "Overview",
  "Queries",
  "Connections",
  "Resources",
  "Replication",
  "WAL & I/O",
];

export function PostgresMetricsTab({ svc, env }: PostgresTabProps) {
  const [cat, setCat] = useState("Overview");
  const p95 = useMetrics(env, "service:api metric:p95");
  const connections = useMetrics(env, "service:db-main metric:connections");
  const errorRate = useMetrics(env, "service:api metric:error_rate");

  return (
    <>
      {/* Finding: frames D9/D10/D12 show bare tab titles — the design-system
          "Area · Thing" h1 grammar wins per the audit's P1 ruling. */}
      <Pghead
        title="PostgreSQL · Metrics"
        sub={
          <span className="mono">
            {svc.name} · {env} · the same series alerting and the assistant read
          </span>
        }
      >
        <Pill tone="ok">live · 10 s</Pill>
        <Btn variant="s" disabled disabledReason="U8 pre-fill lands from Observe">
          Create alert
        </Btn>
        <Btn variant="s" disabled disabledReason="No export endpoint in the spec (finding)">
          Export
        </Btn>
      </Pghead>

      <div className="flex flex-wrap items-center gap-2">
        <div className="chiprow">
          {RANGE_CHIPS.map((r) => (
            <span key={r} className={cn("chip", r === "1h" && "on")}>
              {r}
            </span>
          ))}
          <span className="chip border-dashed">compare: prev hour</span>
        </div>
        <span className="mono text-10p5 text-ink3">resolution auto · 15 s</span>
        <span className="flex-1" />
        <span className="text-10p5 text-ink3">
          ◆ deploys · ◇ scale · ▲ alerts — one timeline with Observe
        </span>
      </div>

      <div className="tabrow">
        {METRIC_CATEGORIES.map((c) => (
          <button
            key={c}
            type="button"
            className={cn("tab", cat === c && "on")}
            onClick={() => setCat(c)}
          >
            {c}
          </button>
        ))}
      </div>

      {cat !== "Overview" ? (
        <p className="text-11p5 text-ink3">This category's panes land with more telemetry</p>
      ) : (
        <>
          <div className="grid grid-cols-5 gap-3">
            <Metric label="p95 latency" value="812 ms" tone="warn" note="▲ +540 ms vs prev" />
            <Metric label="Transactions" value="1.4k/s" note="▲ +6%" />
            <Metric label="Connections" value="192 / 200" tone="warn" note="▲ 96% of pool" />
            <Metric label="Cache hit" value="99.1%" note="▼ −0.2 pt" />
            <Metric label="Disk" value="21.4 GB" note="→ flat · autogrow 50" />
          </div>

          <Card className="flex flex-col gap-2.5 p-4">
            <div className="flex items-center gap-2.5">
              <span className="text-12p5 font-semibold">Query latency percentiles</span>
              <span className="flex-1" />
              <span className="mono text-10p5 text-ink2">
                ━ p50 6 ms · ━ p95 812 ms · ━ p99 1.4 s
              </span>
            </div>
            <MetricChart
              series={toSeries(p95.data)}
              threshold={800}
              markers={toMarkers(p95.data)}
              unit="ms"
              tone="warn"
              size="lg"
            />
            <span className="mono text-10p5 text-ink3">
              brush: 13:40 → now · drag to zoom · double-click to reset
            </span>
          </Card>

          <div className="grid grid-cols-2 gap-3.5">
            <Card className="flex flex-col gap-2 p-3.5">
              <div className="flex items-center gap-2">
                <span className="text-11p5 font-medium text-ink2">Connections by consumer</span>
                <span className="flex-1" />
                <Pill tone="warn">alert set · &gt;190</Pill>
              </div>
              <MetricChart
                series={toSeries(connections.data)}
                markers={toMarkers(connections.data)}
                tone="warn"
                size="sm"
              />
              <span className="mono text-10p5 text-ink3">
                api 141 · worker 38 · admin 9 — limit 200 dashed · pool exhaustion is a named alert,
                not a mystery 500
              </span>
            </Card>
            <Card className="flex flex-col gap-2 p-3.5">
              <div className="flex items-center gap-2">
                <span className="text-11p5 font-medium text-ink2">Commits vs rollbacks</span>
              </div>
              <MetricChart series={toSeries(errorRate.data)} tone="steel" size="sm" />
              <span className="mono text-10p5 text-ink3">
                rollbacks tick up post-deploy — the assistant cites this pane in prp_7c31a2
              </span>
            </Card>
          </div>

          <div className="flex items-center gap-2.5 text-10p5 text-ink3">
            <span>raw 15 d · downsampled 13 mo · every pane is a query:</span>
            <Copybit>steloit metrics query db-main 'p95(query_latency)' --since 1h</Copybit>
            <span className="flex-1" />
            <span>panes added per extension — pgvector adds an ANN-recall pane when installed</span>
          </div>
        </>
      )}
    </>
  );
}

/* ================================ D10 · Logs ================================ */

/**
 * The D10 log rows are frame-fixed canon: they are db-scoped postgres lines
 * (slow-query durations, pool warnings) that the shared useLogs(env) stream —
 * which serves the O3 cross-service incident lines — does not carry. The
 * histogram and the NDJSON export do read the live useLogs data.
 */
interface DbLogRow {
  t: string;
  level: "warn" | "info";
  msg: string;
  expandable?: boolean;
}

const DB_LOG_ROWS: DbLogRow[] = [
  {
    t: "14:06:12",
    level: "warn",
    msg: "duration: 642.118 ms  execute: SELECT * FROM orders WHERE customer_id = $1 …",
    expandable: true,
  },
  {
    t: "14:05:58",
    level: "warn",
    msg: "duration: 631.004 ms  execute: SELECT * FROM orders WHERE customer_id = $1 …",
  },
  { t: "14:04:31", level: "warn", msg: "connections approaching limit: 188 of 200" },
  {
    t: "14:03:47",
    level: "warn",
    msg: "duration: 655.911 ms  execute: SELECT * FROM orders WHERE customer_id = $1 …",
  },
  {
    t: "14:02:20",
    level: "info",
    msg: "connection authorized: user=bnd_api_dbmain database=store",
  },
];

function LogHistogram({
  buckets,
}: {
  buckets: Array<{ at?: string; counts?: Record<string, number> }>;
}) {
  const width = 560;
  const height = 70;
  if (buckets.length === 0) return <div style={{ height }} />;
  const totals = buckets.map((b) => Object.values(b.counts ?? {}).reduce((a, n) => a + n, 0));
  const max = Math.max(...totals, 1);
  const slot = width / buckets.length;
  const barW = Math.max(slot - 6, 4);
  let markerIndex = 0;
  buckets.forEach((b, i) => {
    if (minutesOfDay(b.at ?? "") <= 13 * 60 + 58) markerIndex = i;
  });
  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      className="block w-full"
      role="img"
      aria-label="Log volume"
    >
      {buckets.map((b, i) => {
        const total = totals[i] ?? 0;
        const h = (total / max) * (height - 18);
        const counts = b.counts ?? {};
        const fill = counts.error ? "var(--err)" : counts.warn ? "var(--warn)" : "var(--ink3)";
        return (
          <rect
            key={b.at ?? i}
            x={i * slot + 3}
            y={height - 14 - h}
            width={barW}
            height={h}
            rx={1.5}
            fill={fill}
            opacity={0.85}
          />
        );
      })}
      <text
        x={markerIndex * slot + 3 + barW / 2}
        y={height - 3}
        textAnchor="middle"
        fontSize="8.5"
        fill="var(--ink2)"
        fontFamily="JetBrains Mono Variable, JetBrains Mono, monospace"
      >
        ◆ #142
      </text>
    </svg>
  );
}

function LogFacet({
  label,
  count,
  tone,
  highlight,
}: {
  label: string;
  count: string;
  tone?: "warn" | "err";
  highlight?: boolean;
}) {
  return (
    <div className={cn("flex items-center gap-2 text-11", highlight && "font-semibold")}>
      <span className="flex-1 truncate text-ink2">{label}</span>
      <span
        className={tone === "err" ? "mono text-err" : tone === "warn" ? "mono text-warn" : "mono"}
      >
        {count}
      </span>
    </div>
  );
}

export function PostgresLogsTab({ svc, org, project, env }: PostgresTabProps) {
  const logs = useLogs(env);
  const [expanded, setExpanded] = useState(true);

  const exportNdjson = () => {
    const lines = (logs.data?.data ?? []).map((l) => JSON.stringify(l)).join("\n");
    const blob = new Blob([lines], { type: "application/x-ndjson" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${svc.name}-logs.ndjson`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <>
      <Pghead
        title="PostgreSQL · Logs"
        sub={
          <span className="mono">
            {svc.name} · {env} · structured · 14 d retention · streams into Observe
          </span>
        }
      >
        <Pill tone="ok">live tail</Pill>
        <Btn variant="s" onClick={exportNdjson}>
          Export NDJSON
        </Btn>
      </Pghead>

      <div className="flex items-center gap-2">
        <div className="chiprow">
          <span className="chip on">level:warn+</span>
          <span className="chip on">source:postgres</span>
          <span className="chip on">"duration"</span>
        </div>
        <div className="w-[180px]">
          <Inp className="mono" placeholder="add filter…" />
        </div>
        <span className="flex-1" />
        <span className="chip">★ Saved views ▾</span>
        <div className="chiprow">
          {["1h", "24h", "7d"].map((r) => (
            <span key={r} className={cn("chip", r === "1h" && "on")}>
              {r}
            </span>
          ))}
        </div>
      </div>

      <Card className="flex flex-col gap-1.5 p-3.5">
        <LogHistogram buckets={logs.data?.histogram ?? []} />
        <span className="mono text-10p5 text-ink3">
          volume · warn spikes begin at the deploy marker · click a bar or drag to filter the window
          · selected 14:02 → 14:20
        </span>
      </Card>

      <div className="flex gap-3.5">
        <Card className="flex w-[190px] shrink-0 flex-col gap-3 p-3">
          <div className="flex flex-col gap-1">
            <div className="text-10 font-semibold text-ink3 uppercase tracking-wide">Level</div>
            <LogFacet label="error" count="3" tone="err" />
            <LogFacet label="warn" count="214" tone="warn" highlight />
            <LogFacet label="info" count="12.4k" />
          </div>
          <div className="flex flex-col gap-1">
            <div className="text-10 font-semibold text-ink3 uppercase tracking-wide">Source</div>
            <LogFacet label="postgres" count="201" highlight />
            <LogFacet label="pgbouncer" count="13" />
            <LogFacet label="backup" count="0" />
          </div>
          <div className="flex flex-col gap-1.5">
            <div className="text-10 font-semibold text-ink3 uppercase tracking-wide">
              Top patterns
            </div>
            <div className="flex items-center gap-2 text-11">
              <span className="flex-1 truncate text-warn">duration: … SELECT * FROM orders…</span>
              <span className="mono">198×</span>
            </div>
            <Btn
              variant="gh"
              className="h-5 self-start px-2 text-10"
              disabled
              disabledReason="Pre-filling the rule drawer from this chart isn't wired — U8 opens pre-filled from Observe → Metrics ⚑ (finding)"
            >
              alert on this
            </Btn>
          </div>
          <p className="border-hair border-t pt-2 text-10 text-ink3">
            facets are computed on the filtered window
          </p>
        </Card>

        <div className="flex min-w-0 flex-1 flex-col gap-2">
          <div className="logwell">
            {DB_LOG_ROWS.map((row) => {
              const lv = row.level === "warn" ? "lv-w" : "lv-i";
              return (
                <div key={`${row.t}-${row.msg}`}>
                  <button
                    type="button"
                    className="flex w-full items-baseline gap-2.5 text-left"
                    onClick={row.expandable ? () => setExpanded((v) => !v) : undefined}
                  >
                    <span className="t">{row.t}</span>
                    <span className={lv}>{row.level.toUpperCase().padEnd(5, " ")}</span>
                    <span className="min-w-0 flex-1">{row.msg}</span>
                  </button>
                  {row.expandable && expanded ? (
                    <div className="flex flex-col gap-1.5 py-1.5 pl-[74px]">
                      <div className="grid grid-cols-[110px_1fr] gap-x-3 gap-y-1 text-11">
                        <span className="t">timestamp</span>
                        <span>2026-07-06 14:06:12.881</span>
                        <span className="t">level</span>
                        <span>
                          <Pill tone="warn">warn · slow query</Pill>
                        </span>
                        <span className="t">duration_ms</span>
                        <span className="lv-w">642.118</span>
                        <span className="t">user</span>
                        <span>bnd_api_dbmain</span>
                        <span className="t">query</span>
                        <span>
                          SELECT * FROM orders WHERE customer_id = $1 ORDER BY created_at DESC
                        </span>
                        <span className="t">trace</span>
                        <span>trc_8812f4 → request POST /checkout · open in Observe</span>
                      </div>
                      <div className="flex items-center gap-2">
                        <Btn
                          variant="gh"
                          className="h-5 px-2 text-10"
                          disabled
                          disabledReason="Context windows land with an around-timestamp logs query the API lacks (finding)"
                        >
                          Context ±5 s
                        </Btn>
                        <Link
                          to="/$org/$project/svc/$service/insights"
                          params={{ org, project, service: svc.name }}
                          search={{ env }}
                        >
                          <Btn variant="gh" className="h-5 px-2 text-10">
                            Open in Query Insights
                          </Btn>
                        </Link>
                        <Btn
                          variant="gh"
                          className="h-5 px-2 text-10"
                          onClick={() =>
                            navigator.clipboard.writeText(`${row.t} ${row.level} ${row.msg}`)
                          }
                        >
                          Copy
                        </Btn>
                        <Btn
                          variant="gh"
                          className="h-5 px-2 text-10"
                          disabled
                          disabledReason="Pre-filling the rule drawer from this chart isn't wired — U8 opens pre-filled from Observe → Metrics ⚑ (finding)"
                        >
                          Create alert from pattern
                        </Btn>
                      </div>
                    </div>
                  ) : null}
                </div>
              );
            })}
          </div>
          <div className="flex items-center gap-3 text-10p5 text-ink3">
            <span className="mono">214 of 12,617 lines match</span>
            <span>j/k move · e expand · c context</span>
            <span className="flex-1" />
            <span>same query language everywhere: logs, ⌘K, CLI</span>
            <span className="mono">steloit logs db-main -q 'level:warn+ "duration"'</span>
          </div>
        </div>
      </div>
    </>
  );
}

/* =============================== D12 · Settings =============================== */

/**
 * Source inconsistencies carried verbatim per surface (findings, not fixes):
 * D12's danger zone says 3 bindings + 4 branches depend on db-main while the
 * U6 modal says 2 services bind it; D12 shows 21.4 GB used where U6's sub
 * says 12.4 GB of data. Each surface renders its own frame's copy.
 */

/** Canon sizes — chip prices ($19/$58/$112) + specs from the C2 create block. */
const PG_SHAPE_STANDARD: ShapeOption = {
  id: "standard",
  title: "Standard",
  spec: "2 vCPU / 4 GB",
  price: 58,
  patch: { size: "standard" },
};
const PG_SHAPES: ShapeOption[] = [
  { id: "dev", title: "Dev", spec: "1 vCPU / 2 GB", price: 19, patch: { size: "dev" } },
  PG_SHAPE_STANDARD,
  {
    id: "performance",
    title: "Performance",
    spec: "4 vCPU / 8 GB",
    price: 112,
    patch: { size: "performance" },
  },
];

export function PostgresSettingsTab({ svc, env }: PostgresTabProps) {
  const shape = (svc.shape ?? {}) as Record<string, unknown>;
  const currentShape =
    PG_SHAPES.find((s) => s.id === String(shape.size ?? "standard")) ?? PG_SHAPE_STANDARD;
  const [editingShape, setEditingShape] = useState(false);
  const [renaming, setRenaming] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const update = useMutation(updateServiceMutation());

  return (
    <>
      <Pghead
        title="PostgreSQL · Settings"
        sub={
          <span className="mono">
            {svc.name} · {env} · service-scoped — this page never leaves the service (§4.6)
          </span>
        }
      />

      <Card className="flex flex-col gap-3 p-4">
        <div className="text-12p5 font-semibold">Size &amp; storage</div>
        <div className="flex items-center gap-2 text-11p5">
          <span className="text-ink2">Size</span>
          <span>
            {currentShape.title} · {currentShape.spec}
          </span>
          <span className="mono text-ink3">${currentShape.price}/mo</span>
          <span className="flex-1" />
          <Btn variant="s" className="h-6 px-2.5 text-10p5" onClick={() => setEditingShape(true)}>
            Edit shape…
          </Btn>
        </div>
        <div className="flex items-center gap-2 border-hair border-t pt-3 text-11p5">
          <span className="text-ink2">Storage</span>
          <span>50 GB · auto-grows</span>
          <span className="flex-1" />
          <span className="mono text-ink3">21.4 used</span>
        </div>
        <div className="flex items-center gap-2 border-hair border-t pt-3 text-11p5">
          <span className="text-ink2">
            High availability · +$19/mo · recommended for production
          </span>
          <span className="flex-1" />
          <span className="chip">off</span>
          <Pill tone="ai">assistant suggests</Pill>
        </div>
      </Card>

      <Card className="flex flex-col gap-3 p-4">
        <div className="text-12p5 font-semibold">Config</div>
        <div className="flex items-center gap-2 text-11p5">
          <span className="text-ink2">Name</span>
          <span className="flex-1" />
          <span className="mono">{svc.name}</span>
          <Btn variant="gh" className="h-5 px-2 text-10" onClick={() => setRenaming(true)}>
            Rename…
          </Btn>
        </div>
        <div className="flex items-center gap-2 border-hair border-t pt-3 text-11p5">
          <span className="text-ink2">Maintenance window</span>
          <span className="flex-1" />
          <span>Sun 03:00–04:00 IST</span>
        </div>
        <div className="flex items-center gap-2 border-hair border-t pt-3 text-11p5">
          <span className="text-ink2">Slow-query threshold</span>
          <span className="flex-1" />
          <span className="mono">200 ms</span>
        </div>
        <div className="flex items-center gap-2 border-hair border-t pt-3 text-11p5">
          <span className="text-ink2">Extensions</span>
          <span className="flex-1" />
          <span className="chip on">pgvector</span>
          <span className="chip">+ add</span>
        </div>
      </Card>

      <Card className="flex flex-col gap-2.5 border-err/45 p-4">
        <div className="text-12p5 font-semibold text-err">Danger zone</div>
        <div className="flex items-center gap-2.5">
          <Btn variant="dgr" onClick={() => setDeleting(true)}>
            Delete {svc.name}…
          </Btn>
          <Pill tone="err">blocked — 3 bindings and 4 branches depend on this instance</Pill>
        </div>
        <p className="text-10p5 text-ink3">
          detach dependents first · then type the instance name to confirm · a final snapshot is
          kept 30 d
        </p>
      </Card>

      {editingShape ? (
        <EditShapeDrawer
          svc={svc}
          env={env}
          options={PG_SHAPES}
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

      {deleting ? (
        <DeleteServiceModal
          svc={svc}
          sub="PostgreSQL 16 · production · 12.4 GB of data"
          onClose={() => setDeleting(false)}
        />
      ) : null}
    </>
  );
}
