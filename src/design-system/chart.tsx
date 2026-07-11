import { useId } from "react";
import { cn } from "@/lib/utils";

/**
 * The observability chart grammar (O2/D9): line series on --mono-bg-adjacent
 * surfaces, an SLO threshold line, ◆ deploy markers on every chart, tabular
 * axis labels in mono. Functional motion only — no decorative animation.
 */

export interface SeriesPoint {
  /** Minutes on the chart's x-domain (arbitrary unit, monotonic). */
  t: number;
  v: number;
}

export interface DeployMarker {
  t: number;
  label: string;
}

interface MetricChartProps {
  series: SeriesPoint[];
  /** SLO/threshold line value, drawn in --warn. */
  threshold?: number;
  markers?: DeployMarker[];
  unit?: string;
  height?: number;
  /** Series stroke: steel by default; warn for breaching series. */
  tone?: "steel" | "warn" | "ok" | "assist";
  className?: string;
}

const TONE: Record<NonNullable<MetricChartProps["tone"]>, string> = {
  steel: "var(--steel)",
  warn: "var(--warn)",
  ok: "var(--ok)",
  assist: "var(--assist)",
};

export function MetricChart({
  series,
  threshold,
  markers = [],
  unit = "",
  height = 120,
  tone = "steel",
  className,
}: MetricChartProps) {
  const gradientId = useId();
  if (series.length < 2) return <div style={{ height }} className={className} />;

  const width = 560;
  const pad = { top: 10, right: 8, bottom: 18, left: 40 };
  const first = series[0];
  const last = series[series.length - 1];
  if (!first || !last) return null;
  const tMin = first.t;
  const tMax = last.t;
  const vValues = series.map((p) => p.v).concat(threshold !== undefined ? [threshold] : []);
  const vMax = Math.max(...vValues) * 1.15;
  const vMin = 0;

  const x = (t: number) => pad.left + ((t - tMin) / (tMax - tMin)) * (width - pad.left - pad.right);
  const y = (v: number) =>
    pad.top + (1 - (v - vMin) / (vMax - vMin)) * (height - pad.top - pad.bottom);

  const path = series
    .map((p, i) => `${i === 0 ? "M" : "L"}${x(p.t).toFixed(1)},${y(p.v).toFixed(1)}`)
    .join(" ");
  const area = `${path} L${x(tMax).toFixed(1)},${y(0)} L${x(tMin).toFixed(1)},${y(0)} Z`;

  const yTicks = [0, 0.5, 1].map((f) => vMin + f * (vMax - vMin));

  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      className={cn("block w-full", className)}
      role="img"
      aria-label={`Time series${unit ? ` (${unit})` : ""}`}
    >
      <defs>
        <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={TONE[tone]} stopOpacity="0.18" />
          <stop offset="100%" stopColor={TONE[tone]} stopOpacity="0" />
        </linearGradient>
      </defs>
      {yTicks.map((v) => (
        <g key={v}>
          <line
            x1={pad.left}
            x2={width - pad.right}
            y1={y(v)}
            y2={y(v)}
            stroke="var(--hair)"
            strokeWidth="1"
          />
          <text
            x={pad.left - 6}
            y={y(v) + 3}
            textAnchor="end"
            fontSize="8.5"
            fill="var(--ink3)"
            fontFamily="JetBrains Mono Variable, JetBrains Mono, monospace"
          >
            {formatTick(v)}
            {unit}
          </text>
        </g>
      ))}
      {threshold !== undefined ? (
        <g>
          <line
            x1={pad.left}
            x2={width - pad.right}
            y1={y(threshold)}
            y2={y(threshold)}
            stroke="var(--warn)"
            strokeWidth="1"
            strokeDasharray="4 3"
          />
          <text
            x={width - pad.right}
            y={y(threshold) - 4}
            textAnchor="end"
            fontSize="8.5"
            fill="var(--warn)"
            fontFamily="JetBrains Mono Variable, JetBrains Mono, monospace"
          >
            SLO {formatTick(threshold)}
            {unit}
          </text>
        </g>
      ) : null}
      <path d={area} fill={`url(#${gradientId})`} />
      <path d={path} fill="none" stroke={TONE[tone]} strokeWidth="1.6" />
      {markers.map((m) => (
        <g key={`${m.t}-${m.label}`}>
          <line
            x1={x(m.t)}
            x2={x(m.t)}
            y1={pad.top}
            y2={height - pad.bottom}
            stroke="var(--ink3)"
            strokeWidth="1"
            strokeDasharray="2 3"
          />
          <text
            x={x(m.t)}
            y={height - 6}
            textAnchor="middle"
            fontSize="8.5"
            fill="var(--ink2)"
            fontFamily="JetBrains Mono Variable, JetBrains Mono, monospace"
          >
            ◆ {m.label}
          </text>
        </g>
      ))}
    </svg>
  );
}

function formatTick(v: number): string {
  if (v >= 1000) return `${(v / 1000).toFixed(v % 1000 === 0 ? 0 : 1)}k`;
  if (v >= 100) return String(Math.round(v));
  if (v >= 1) return v % 1 === 0 ? String(v) : v.toFixed(1);
  return v.toFixed(2);
}

/** Small inline sparkline (.spark) for cards and health rows. */
export function Spark({
  series,
  tone = "steel",
  width = 96,
  height = 24,
}: {
  series: SeriesPoint[];
  tone?: MetricChartProps["tone"];
  width?: number;
  height?: number;
}) {
  if (series.length < 2) return null;
  const first = series[0];
  const last = series[series.length - 1];
  if (!first || !last) return null;
  const vMax = Math.max(...series.map((p) => p.v)) || 1;
  const x = (t: number) => ((t - first.t) / (last.t - first.t)) * width;
  const y = (v: number) => height - 2 - (v / vMax) * (height - 4);
  const path = series
    .map((p, i) => `${i === 0 ? "M" : "L"}${x(p.t).toFixed(1)},${y(p.v).toFixed(1)}`)
    .join(" ");
  return (
    <svg width={width} height={height} className="spark" aria-hidden="true">
      <path d={path} fill="none" stroke={TONE[tone ?? "steel"]} strokeWidth="1.4" />
    </svg>
  );
}
