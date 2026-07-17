import { useQuery } from "@tanstack/react-query";
import type { DeployMarker, SeriesPoint } from "@/design-system/chart";
import type { MetricResult } from "@/lib/api";
import {
  getTraceOptions,
  listAlertRulesOptions,
  queryLogsOptions,
  queryMetricsOptions,
} from "@/lib/api";

/**
 * Observe data layer — thin wrappers over the generated query helpers.
 * Metrics and logs poll-adjacent surfaces get a 5 s staleTime (state.md:
 * 5 s metrics); env rides in the query key via the {env} path param
 * (env-as-filter, ADR-013).
 */

export function useMetrics(env: string, query: string) {
  return useQuery({
    ...queryMetricsOptions({ path: { env }, query: { query } }),
    staleTime: 5_000,
  });
}

export function useLogs(env: string, query?: string) {
  return useQuery({
    ...queryLogsOptions(query ? { path: { env }, query: { query } } : { path: { env } }),
    staleTime: 5_000,
  });
}

export function useTrace(env: string, traceId: string) {
  return useQuery(getTraceOptions({ path: { env, trace: traceId } }));
}

export function useAlertRules(project: string) {
  return useQuery({
    ...listAlertRulesOptions({ path: { project } }),
    select: (r) => r.data ?? [],
  });
}

/** "2026-07-02T14:02:00+05:30" → 842 (minutes of day) — the chart's t axis. */
export function minutesOfDay(at: string): number {
  const m = at.match(/T(\d{2}):(\d{2})/);
  if (!m) return 0;
  return Number(m[1]) * 60 + Number(m[2]);
}

/** MetricResult → the chart grammar's series points (t in minutes of day). */
export function toSeries(result: MetricResult | undefined): SeriesPoint[] {
  return (result?.series?.[0]?.points ?? []).map(([at, v]) => ({ t: minutesOfDay(at), v }));
}

/** MetricResult markers → chart markers — the shared incident timeline. */
export function toMarkers(result: MetricResult | undefined): DeployMarker[] {
  return (result?.markers ?? []).map((m) => ({
    t: minutesOfDay(m.at ?? ""),
    label: m.label ?? "",
  }));
}
