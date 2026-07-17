import type { LogResult, MetricResult } from "@/lib/api";

/**
 * Canon telemetry — deterministic series interpolated between the incident's
 * fixed anchor values (19-canon/canon.md): #142 at 13:58 → p95 812 ms at
 * 14:02 (SLO 800) → 2 dead letters 14:03 → #143 canary at 14:50 → p95 431 ms
 * (−47%). No randomness: the same chart renders the same pixels every time.
 * Window: 13:00–15:00 IST, 2026-07-02, 2-minute resolution.
 */

const DAY = "2026-07-02";
const START_MIN = 13 * 60; // 13:00
const END_MIN = 15 * 60; // 15:00
const STEP = 2;

function iso(minOfDay: number): string {
  const h = String(Math.floor(minOfDay / 60)).padStart(2, "0");
  const m = String(minOfDay % 60).padStart(2, "0");
  return `${DAY}T${h}:${m}:00+05:30`;
}

/** Smooth deterministic wobble so lines read as telemetry, not vectors. */
function wobble(minOfDay: number, amp: number): number {
  return Math.sin(minOfDay / 3.1) * amp * 0.6 + Math.sin(minOfDay / 7.7) * amp * 0.4;
}

type Anchor = [minOfDay: number, value: number];

/** Piecewise-linear interpolation through canon anchors + wobble. */
function seriesFrom(anchors: Anchor[], amp: number): Array<[string, number]> {
  const points: Array<[string, number]> = [];
  for (let t = START_MIN; t <= END_MIN; t += STEP) {
    let v = anchors[anchors.length - 1]?.[1] ?? 0;
    for (let i = 0; i < anchors.length - 1; i++) {
      const a = anchors[i];
      const b = anchors[i + 1];
      if (!a || !b) continue;
      if (t >= a[0] && t <= b[0]) {
        v = a[1] + ((t - a[0]) / (b[0] - a[0])) * (b[1] - a[1]);
        break;
      }
      if (t < (anchors[0]?.[0] ?? 0)) v = anchors[0]?.[1] ?? 0;
    }
    points.push([iso(t), Math.max(0, Math.round((v + wobble(t, amp)) * 100) / 100)]);
  }
  return points;
}

const T_DEP142 = 13 * 60 + 58;
const T_ALERT = 14 * 60 + 2;
const T_DLQ = 14 * 60 + 3;
const T_DEP143 = 14 * 60 + 50;

/** The shared incident timeline — the same markers on every chart. */
export const MARKERS: NonNullable<MetricResult["markers"]> = [
  { kind: "deploy", at: iso(T_DEP142), ref: "dep_142", label: "#142" },
  { kind: "alert", at: iso(T_ALERT), ref: "rule_p95slo", label: "p95" },
  { kind: "deploy", at: iso(T_DEP143), ref: "dep_143", label: "#143" },
];

const SERIES: Record<string, { label: string; amp: number; anchors: Anchor[] }> = {
  "service:api metric:p95": {
    label: "api p95 (ms)",
    amp: 14,
    anchors: [
      [START_MIN, 238],
      [T_DEP142, 246],
      [T_ALERT, 812],
      [T_DEP143 - 2, 806],
      [T_DEP143 + 4, 431],
      [END_MIN, 431],
    ],
  },
  "service:api metric:requests": {
    label: "api requests (/s)",
    amp: 9,
    anchors: [
      [START_MIN, 201],
      [END_MIN, 214],
    ],
  },
  "service:api metric:error_rate": {
    label: "api error rate (%)",
    amp: 0.05,
    anchors: [
      [START_MIN, 0.2],
      [T_ALERT, 0.4],
      [END_MIN, 0.4],
    ],
  },
  "service:jobs metric:dlq_depth": {
    label: "jobs DLQ depth",
    amp: 0,
    anchors: [
      [START_MIN, 0],
      [T_DLQ - 1, 0],
      [T_DLQ, 2],
      [END_MIN, 2],
    ],
  },
  "service:jobs metric:queue_depth": {
    label: "jobs queue depth",
    amp: 1.5,
    anchors: [
      [START_MIN, 4],
      [T_ALERT, 12],
      [T_DEP143, 12],
      [END_MIN, 6],
    ],
  },
  "service:db-main metric:connections": {
    label: "db-main connections",
    amp: 3,
    anchors: [
      [START_MIN, 96],
      [T_DEP142, 104],
      [T_ALERT, 192],
      [T_DEP143, 190],
      [END_MIN, 121],
    ],
  },
  "service:cache metric:hit_rate": {
    label: "cache hit rate (%)",
    amp: 0.2,
    anchors: [
      [START_MIN, 98.2],
      [END_MIN, 98.2],
    ],
  },
  "service:api metric:slo_burn": {
    label: "api slo burn (x)",
    amp: 0.2,
    anchors: [
      [START_MIN, 0.6],
      [T_ALERT, 3.1],
      [T_DEP143, 3.1],
      [END_MIN, 1.1],
    ],
  },
};

export function metricResult(query: string): MetricResult {
  const def = SERIES[query] ?? {
    label: query,
    amp: 1,
    anchors: [
      [START_MIN, 10],
      [END_MIN, 10],
    ] as Anchor[],
  };
  return {
    query,
    resolution: "2m",
    series: [{ label: def.label, points: seriesFrom(def.anchors, def.amp) }],
    markers: MARKERS,
  };
}

/** Canon log stream (O3): the 642 ms SeqScan story around the incident. */
const LOG_ENTRIES: NonNullable<LogResult["data"]> = [
  {
    at: iso(T_DEP143 + 2),
    level: "info",
    source: "api",
    message: "GET /orders 200 in 118 ms",
    fields: { route: "GET /orders", status: 200, duration_ms: 118 },
    trace_id: null,
  },
  {
    at: iso(T_DEP143),
    level: "info",
    source: "deploy",
    message: "deployment #143 canary 33% · fix/orders-query",
    fields: { deployment: 143, canary_percent: 33 },
    trace_id: null,
  },
  {
    at: iso(T_DLQ + 20),
    level: "error",
    source: "db-main",
    message: "slow query: SELECT * FROM orders WHERE customer_id = $1 — 642 ms (Seq Scan)",
    fields: {
      duration_ms: 642,
      plan: "Seq Scan on orders",
      rows: 1204318,
      query: "SELECT * FROM orders WHERE customer_id = $1 ORDER BY created_at DESC",
    },
    trace_id: "tr_8814",
  },
  {
    at: iso(T_DLQ + 8),
    level: "error",
    source: "worker",
    message: 'order.paid handler failed: unhandled field "receipt": null',
    fields: { message_type: "order.paid", dead_lettered: true },
    trace_id: "tr_8814",
  },
  {
    at: iso(T_DLQ),
    level: "error",
    source: "jobs",
    message: "2 messages moved to DLQ · order.paid",
    fields: { dlq_depth: 2, message_type: "order.paid" },
    trace_id: null,
  },
  {
    at: iso(T_ALERT),
    level: "warn",
    source: "api",
    message: "alert p95-slo fired · api p95 > 800ms (812 ms)",
    fields: { rule: "p95-slo", value_ms: 812, threshold_ms: 800 },
    trace_id: null,
  },
  {
    at: iso(T_ALERT - 1),
    level: "warn",
    source: "db-main",
    message: "connection pool at 96% (192/200)",
    fields: { connections_used: 192, connections_max: 200 },
    trace_id: null,
  },
  {
    at: iso(T_DEP142),
    level: "info",
    source: "deploy",
    message: "deployment #142 live · gift cards",
    fields: { deployment: 142 },
    trace_id: null,
  },
];

export function logResult(query?: string): LogResult {
  const q = (query ?? "").toLowerCase();
  const data = q
    ? LOG_ENTRIES.filter(
        (e) =>
          (e.message ?? "").toLowerCase().includes(q) ||
          (e.source ?? "").includes(q) ||
          (e.level ?? "").includes(q.replace("level:", "")),
      )
    : LOG_ENTRIES;
  const counts = new Map<string, Record<string, number>>();
  for (const e of LOG_ENTRIES) {
    const bucketMin = Math.floor((minuteOf(e.at ?? "") - START_MIN) / 10) * 10 + START_MIN;
    const bucket = iso(bucketMin);
    const c = counts.get(bucket) ?? {};
    c[e.level ?? "info"] = (c[e.level ?? "info"] ?? 0) + 1;
    counts.set(bucket, c);
  }
  return {
    data,
    next_cursor: null,
    histogram: [...counts.entries()]
      .sort(([a], [b]) => (a < b ? -1 : 1))
      .map(([at, c]) => ({ at, counts: c })),
    facets: {
      level: { error: 3, warn: 2, info: 3 },
      source: { api: 2, "db-main": 2, worker: 1, jobs: 1, deploy: 2 },
      top_patterns: ["SELECT * FROM orders WHERE customer_id = $1", "order.paid"],
    },
    total_matches: data.length,
  };
}

function minuteOf(at: string): number {
  const m = at.match(/T(\d{2}):(\d{2})/);
  if (!m) return START_MIN;
  return Number(m[1]) * 60 + Number(m[2]);
}
