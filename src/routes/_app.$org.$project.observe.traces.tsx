import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Inp } from "@/design-system/inp";
import { Pill } from "@/design-system/pill";
import { SkeletonLines } from "@/design-system/skeleton";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { useTrace } from "@/features/observe/hooks";
import { ObserveChrome } from "@/features/observe/observe-chrome";
import { cn } from "@/lib/utils";

/**
 * O6 · Observe — Traces: slow and broken are never sampled away. Only
 * tr_8814 exists in the canon fixtures; the other list rows are frame-fixed
 * from the O6 frame. Rows are selectable — tr_8814 renders its canon
 * waterfall, the others an honest card (no spans in the fixtures, a finding).
 */

interface TraceListRow {
  id: string;
  route: string;
  ms: string;
  warn?: boolean;
  spans: string;
}

const TRACE_ROWS: TraceListRow[] = [
  { id: "tr_8814", route: "GET /orders · api", ms: "812 ms", warn: true, spans: "5 spans" },
  { id: "tr_8823", route: "GET /orders · api", ms: "831 ms", warn: true, spans: "5 spans" },
  { id: "tr_8819", route: "GET /orders · api", ms: "798 ms", warn: true, spans: "5 spans" },
  { id: "tr_8791", route: "POST /checkout · api", ms: "204 ms", spans: "9 spans" },
  { id: "tr_8788", route: "GET /products · api", ms: "61 ms", spans: "4 spans" },
];

function spanColor(service: string | undefined, isRoot: boolean): string {
  if (isRoot) return "var(--steel)";
  if (service === "db-main") return "var(--warn)";
  if (service === "jobs") return "var(--assist)";
  return "var(--ok)";
}

function TracesPage() {
  const { org, project } = Route.useParams();
  const { env } = Route.useSearch();
  const trace = useTrace(env, "tr_8814");
  // tr_8814 is the default selection (the frame's) and the only trace with
  // spans in the fixtures — the query above stays pinned to it.
  const [selectedId, setSelectedId] = useState("tr_8814");
  const selectedRow = TRACE_ROWS.find((row) => row.id === selectedId) ?? TRACE_ROWS[0];

  const total = trace.data?.duration_ms ?? 812;
  const linkParams = { org, project };
  const search = { env };

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <ObserveChrome env={env} lens="Traces" />

        <div className="flex items-center gap-3">
          <div className="w-[360px]">
            <Inp className="mono" defaultValue="route:/orders duration:>500ms" />
          </div>
          <span className="text-10p5 text-ink3">— or paste a trace id from any log line</span>
          <span className="flex-1" />
          <span className="chip">p95 of matches: 811 ms</span>
          <Btn variant="s" disabled disabledReason="The rule drawer (U8) lands in Phase 3">
            ⚑ Alert on this query
          </Btn>
        </div>

        <div className="flex gap-3.5">
          <Card className="flex w-[390px] shrink-0 flex-col p-3">
            <div className="px-1 pb-2 text-11 font-medium text-ink2">
              Slowest first · 13:30 – 14:45
            </div>
            {TRACE_ROWS.map((row) => (
              <button
                key={row.id}
                type="button"
                aria-pressed={row.id === selectedId}
                onClick={() => setSelectedId(row.id)}
                className={cn(
                  "flex items-center gap-2.5 rounded-md px-2 py-2 text-left text-11p5",
                  row.id === selectedId && "bg-steel/10",
                )}
              >
                <span className="mono font-semibold">{row.id}</span>
                <span className="flex-1 truncate text-ink3">{row.route}</span>
                <span className={cn("mono", row.warn && "text-warn")}>{row.ms}</span>
                <span className="text-10p5 text-ink3">{row.spans}</span>
              </button>
            ))}
            <p className="border-hair mt-2 border-t px-1 pt-2 text-10 text-ink3">
              sampling: 100% of errors and &gt;SLO requests · 1% baseline — slow and broken are
              never sampled away
            </p>
          </Card>

          <div className="flex min-w-0 flex-1 flex-col gap-3.5">
            {selectedId !== "tr_8814" && selectedRow ? (
              /* The other list rows are frame-fixed with no spans behind them —
                 an honest card with the row's known stats, never a fake
                 waterfall. */
              <Card className="flex flex-col gap-2.5 p-4">
                <div className="flex items-center gap-2.5">
                  <span className="mono text-13 font-bold">{selectedRow.id}</span>
                  <Pill tone={selectedRow.warn ? "warn" : "st"}>{selectedRow.ms}</Pill>
                  <span className="text-10p5 text-ink3">
                    {selectedRow.route} · {selectedRow.spans}
                  </span>
                </div>
                <p className="text-11p5 leading-relaxed text-ink3">
                  Spans for <span className="mono">{selectedRow.id}</span> aren't in the canon
                  fixtures — tr_8814 is the exemplar (finding).
                </p>
              </Card>
            ) : (
              <>
                <Card className="flex flex-col gap-2.5 p-4">
                  <div className="flex items-center gap-2.5">
                    <span className="mono text-13 font-bold">tr_8814</span>
                    <Pill tone="warn">812 ms</Pill>
                    <span className="text-10p5 text-ink3">
                      14:02:11 · api → cache → db-main → jobs · 4 min after ◆ #142
                    </span>
                    <span className="flex-1" />
                    <Link to="/$org/$project/observe/logs" params={linkParams} search={search}>
                      <Btn variant="gh" className="h-6 px-2.5 text-10p5">
                        Logs for this trace
                      </Btn>
                    </Link>
                  </div>

                  {/* Four-state grammar: the waterfall rides the trace query — the
                  header line above is frame-fixed (O6) and stays. */}
                  {trace.isPending ? (
                    <SkeletonLines lines={5} />
                  ) : trace.isError ? (
                    <ApiFailureCard
                      title="Trace didn't load"
                      error={trace.error}
                      requestLine={`GET /envs/${env}/traces/tr_8814`}
                      onRetry={() => trace.refetch()}
                    />
                  ) : (
                    <div className="flex flex-col gap-1.5">
                      {(trace.data?.spans ?? []).map((span) => {
                        const isRoot = span.parent_id == null;
                        const left = ((span.start_ms ?? 0) / total) * 100;
                        const width = Math.max(((span.duration_ms ?? 0) / total) * 100, 0.8);
                        const isDbMain = span.service === "db-main";
                        return (
                          <div key={span.id} className="flex items-center gap-2.5 text-10p5">
                            <span className="mono w-[210px] shrink-0 truncate text-ink2">
                              {span.service} · {span.name}
                            </span>
                            <span className="relative h-3.5 flex-1 rounded-sm bg-[var(--mono-bg)]">
                              <span
                                className="absolute top-0 h-full rounded-sm"
                                style={{
                                  left: `${left}%`,
                                  width: `${width}%`,
                                  background: spanColor(span.service, isRoot),
                                  opacity: 0.85,
                                }}
                              />
                            </span>
                            <span
                              className={cn(
                                "mono w-[90px] shrink-0 text-right",
                                isDbMain && "text-warn",
                              )}
                            >
                              {span.duration_ms} ms{isDbMain ? " · 79%" : ""}
                            </span>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </Card>

                <Card className="border-warn/40 flex flex-col gap-2.5 p-4">
                  <p className="text-12 leading-relaxed text-ink2">
                    79% of this trace is one span — the unindexed orders query. The slowest span is
                    the navigation: it links straight to the query's own page and the drafted fix.
                  </p>
                  <div className="flex items-center gap-2">
                    <Btn
                      variant="s"
                      className="h-6 px-2.5 text-10p5"
                      disabled
                      disabledReason="Query insights (D4) lands in Phase 3"
                    >
                      Query insights (D4)
                    </Btn>
                    <Link
                      to="/$org/$project/svc/$service/insights"
                      params={{ ...linkParams, service: "db-main" }}
                      search={search}
                    >
                      <Btn variant="s" className="mono h-6 px-2.5 text-10p5">
                        prp_7c31a2
                      </Btn>
                    </Link>
                  </div>
                </Card>
              </>
            )}

            <div className="flex items-center gap-3 text-10p5 text-ink3">
              <span className="flex-1">
                Trace ids appear on every log line (O3) and in binding-aware clients automatically —
                no SDK ceremony for platform services.
              </span>
              <span className="mono">retention 15 d</span>
            </div>
          </div>
        </div>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/observe/traces")({
  component: TracesPage,
});
