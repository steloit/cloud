import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { EmptyState } from "@/design-system/empty-state";
import { Inp } from "@/design-system/inp";
import { Pill } from "@/design-system/pill";
import { Skeleton, SkeletonLines } from "@/design-system/skeleton";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { minutesOfDay, useLogs } from "@/features/observe/hooks";
import { ObserveChrome } from "@/features/observe/observe-chrome";
import type { LogEntry } from "@/lib/api";

/**
 * O3 · Observe — Logs: one query language, one stream. The trace id on a
 * line stitches api → jobs → db — the same incident from three services.
 * Facet counts fall back to the O3 frame's canon values when the API result
 * carries no facet for a section (routes are not faceted by the mock).
 */

const T_DEP142 = 13 * 60 + 58;

const timeFmt = new Intl.DateTimeFormat("en-GB", {
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  timeZone: "Asia/Kolkata",
});

interface FacetRow {
  label: string;
  count: string;
  tone?: "warn" | "err";
}

const FACET_TONE: Record<string, "warn" | "err"> = {
  api: "warn",
  jobs: "err",
  error: "err",
  warn: "warn",
};

const CANON_SERVICE_FACETS: FacetRow[] = [
  { label: "api", count: "312", tone: "warn" },
  { label: "worker", count: "41" },
  { label: "jobs", count: "2", tone: "err" },
  { label: "db-main", count: "18" },
];

const CANON_LEVEL_FACETS: FacetRow[] = [
  { label: "error", count: "2", tone: "err" },
  { label: "warn", count: "371", tone: "warn" },
];

const CANON_ROUTE_FACETS: FacetRow[] = [{ label: "GET /orders", count: "297", tone: "warn" }];

function facetRows(record: Record<string, number> | undefined, fallback: FacetRow[]): FacetRow[] {
  if (!record) return fallback;
  return Object.entries(record)
    .sort(([, a], [, b]) => b - a)
    .map(([label, count]) => {
      const tone = FACET_TONE[label];
      return tone ? { label, count: String(count), tone } : { label, count: String(count) };
    });
}

function FacetSection({ title, rows }: { title: string; rows: FacetRow[] }) {
  return (
    <div className="flex flex-col gap-1">
      <div className="text-10 font-semibold text-ink3 uppercase tracking-wide">{title}</div>
      {rows.map((row) => (
        <div key={row.label} className="flex items-center gap-2 text-11">
          <span className="flex-1 truncate text-ink2">{row.label}</span>
          <span
            className={
              row.tone === "err" ? "mono text-err" : row.tone === "warn" ? "mono text-warn" : "mono"
            }
          >
            {row.count}
          </span>
        </div>
      ))}
    </div>
  );
}

function Histogram({
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
    if (minutesOfDay(b.at ?? "") <= T_DEP142) markerIndex = i;
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

function LogRow({
  entry,
  expanded,
  onToggle,
}: {
  entry: LogEntry;
  expanded: boolean;
  onToggle: () => void;
}) {
  const level = entry.level ?? "info";
  const lv = level === "error" ? "lv-e" : level === "warn" ? "lv-w" : "lv-i";
  const isJobsError = entry.source === "jobs" && level === "error";
  return (
    <div>
      <button
        type="button"
        className="flex w-full items-baseline gap-2.5 text-left"
        onClick={isJobsError ? onToggle : undefined}
      >
        <span className="t">{entry.at ? timeFmt.format(new Date(entry.at)) : "—"}</span>
        <span className={lv}>{level.toUpperCase().padEnd(5, " ")}</span>
        <span className="t">{entry.source}</span>
        <span className="min-w-0 flex-1">{entry.message}</span>
      </button>
      {isJobsError && expanded ? (
        <div className="flex flex-col gap-1.5 py-1 pl-[74px]">
          <div className="t">msg_9f22 · attempt 3/3 · "receipt": null</div>
          <div className="flex items-center gap-2">
            <Pill tone="st">trace tr_8814 → api</Pill>
            <Btn
              variant="gh"
              className="h-5 px-2 text-10"
              disabled
              disabledReason="Log context windows land in Phase 3"
            >
              Context ± 5 s
            </Btn>
            <Btn
              variant="gh"
              className="h-5 px-2 text-10"
              disabled
              disabledReason="The queue DLQ view (D8) lands in Phase 3"
            >
              Open in DLQ (D8)
            </Btn>
            <Btn
              variant="gh"
              className="h-5 px-2 text-10"
              disabled
              disabledReason="The rule drawer (U8) lands in Phase 3"
            >
              ⚑ Alert on this pattern
            </Btn>
          </div>
        </div>
      ) : null}
    </div>
  );
}

function LogsPage() {
  const { env } = Route.useSearch();
  const logs = useLogs(env);
  const [expanded, setExpanded] = useState(true);

  const facets = logs.data?.facets as
    | { level?: Record<string, number>; source?: Record<string, number> }
    | undefined;

  // Four-state grammar (16-qa): the histogram and the logwell ride the logs
  // query; the search bar and facet rail (Pass 6 scope) stay put.
  const entries = logs.data?.data ?? [];
  const isEmpty = logs.isSuccess && entries.length === 0;

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <ObserveChrome env={env} lens="Logs" />

        <div className="flex items-center gap-3">
          <div className="w-[360px]">
            <Inp className="mono" defaultValue="env:production level:warn+ since:13:30" />
          </div>
          <span className="text-10p5 text-ink3">
            — same language as ⌘K, the CLI, and alert rules
          </span>
          <span className="flex-1" />
          <span className="chip">Saved: incident-triage ▾</span>
          <Btn variant="s" disabled disabledReason="The rule drawer (U8) lands in Phase 3">
            ⚑ Alert on this query
          </Btn>
        </div>

        {/* Four-state grammar: the failure card replaces everything below the
            bar — facets are computed on the result, so a failed result has no
            facets to show either. */}
        {logs.isError ? (
          <ApiFailureCard
            title="Logs didn't load"
            error={logs.error}
            requestLine={`GET /envs/${env}/logs`}
            onRetry={() => logs.refetch()}
          />
        ) : (
          <>
            <Card className="flex flex-col gap-1.5 p-3.5">
              <div className="flex items-center gap-2">
                <span className="text-11p5 font-medium text-ink2">Volume</span>
                <span className="flex-1" />
                <span className="text-10p5 text-ink3">the spike is the navigation</span>
              </div>
              {logs.isPending ? (
                <Skeleton className="h-[70px]" />
              ) : (
                <Histogram buckets={logs.data?.histogram ?? []} />
              )}
            </Card>

            <div className="flex gap-3.5">
              <Card className="flex w-[170px] shrink-0 flex-col gap-3 p-3">
                <FacetSection
                  title="Service"
                  rows={facetRows(facets?.source, CANON_SERVICE_FACETS)}
                />
                <FacetSection title="Level" rows={facetRows(facets?.level, CANON_LEVEL_FACETS)} />
                <FacetSection title="Route" rows={CANON_ROUTE_FACETS} />
                <p className="border-hair border-t pt-2 text-10 text-ink3">
                  facets are computed on the result, so they're always clickable truths
                </p>
              </Card>

              <div className="flex min-w-0 flex-1 flex-col gap-2">
                {logs.isPending ? (
                  <div className="logwell">
                    <SkeletonLines lines={8} />
                  </div>
                ) : isEmpty ? (
                  <EmptyState
                    compact
                    icon="s-doc"
                    title="No log lines in this window"
                    meaning={
                      <>
                        nothing matched this query in the selected range — widen the window with the
                        time chips above, or loosen the query
                      </>
                    }
                  />
                ) : (
                  <div className="logwell">
                    {entries.map((entry) => (
                      <LogRow
                        key={`${entry.at}-${entry.source}-${entry.message}`}
                        entry={entry}
                        expanded={expanded}
                        onToggle={() => setExpanded((v) => !v)}
                      />
                    ))}
                  </div>
                )}
                <p className="text-10p5 text-ink3">
                  j/k move · e expand · c context · one trace id stitches api → jobs → db — the same
                  incident from three services, one stream
                </p>
              </div>
            </div>
          </>
        )}
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/observe/logs")({
  component: LogsPage,
});
