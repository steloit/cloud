import { createFileRoute } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { type SeriesPoint, Spark } from "@/design-system/chart";
import { Eyebrow } from "@/design-system/eyebrow";
import { Metric } from "@/design-system/metric";
import { Dot, Pill } from "@/design-system/pill";
import { planLabel } from "@/features/billing/hooks";
import { useBillingOverview, useOrgs } from "@/features/org/hooks";
import { fmtMoney } from "@/lib/fmt";

/** B1 · Billing overview — one number, then every step of how it's built. */

function sp(values: number[]): SeriesPoint[] {
  return values.map((v, t) => ({ t, v }));
}

/**
 * Per-project display facts the API doesn't carry (dots, vs-Jun deltas,
 * 30-day shapes) — frame-fixed from B1. Money cells come from fixtures.
 */
const PROJECT_DISPLAY: Array<{
  name: string;
  dot?: "warn" | "prov";
  vsJun: string;
  vsTone?: "warn" | "ok" | "prov";
  spark: SeriesPoint[] | null;
  sparkNote?: string;
}> = [
  {
    name: "ecommerce",
    dot: "warn",
    vsJun: "+$24 · db-reports",
    vsTone: "warn",
    spark: sp([3, 3.1, 3.2, 3.1, 3.3, 3.4, 3.6, 4.1, 4.4, 4.6]),
  },
  { name: "analytics", vsJun: "±$0", spark: sp([2, 2.1, 2, 2.1, 2, 2.1, 2, 2.1, 2, 2]) },
  {
    name: "internal-tools",
    vsJun: "−$3",
    vsTone: "ok",
    spark: sp([2.4, 2.4, 2.3, 2.3, 2.2, 2.1, 2.1, 2, 2, 2]),
  },
  {
    name: "mobile-api",
    dot: "prov",
    vsJun: "new · billing since Jul 3",
    vsTone: "prov",
    spark: sp([0, 0, 0, 0, 0, 0, 0.4, 0.9, 1.2, 1.4]),
  },
  { name: "sandbox", vsJun: "—", spark: null, sparkNote: "free tier" },
];

const VS_TONE: Record<NonNullable<(typeof PROJECT_DISPLAY)[number]["vsTone"]>, string> = {
  warn: "text-warn",
  ok: "text-ok",
  prov: "text-prov",
};

/**
 * Daily-accrual drawing — a frame-fixed illustration (B1), not a data series:
 * solid billed days rising to "today · $101", dashed projection to the
 * fixtures' month-end forecast (≈ $482), warn-dashed budget line at $450.
 */
function AccrualChart() {
  const W = 560;
  const H = 150;
  const padL = 36;
  const padR = 54;
  const padT = 14;
  const padB = 16;
  const x = (day: number) => padL + ((day - 1) / 30) * (W - padL - padR);
  const y = (v: number) => padT + (1 - v / 520) * (H - padT - padB);
  const billed: Array<[number, number]> = [
    [1, 14],
    [2, 33],
    [3, 52],
    [4, 71],
    [5, 87],
    [6, 101],
  ];
  const solid = billed
    .map(([d, v], i) => `${i === 0 ? "M" : "L"}${x(d).toFixed(1)},${y(v).toFixed(1)}`)
    .join(" ");
  // ▲ budget alerts at 50 / 80 / 100% of the $450 budget, on the projection.
  const alerts = [225, 360, 450];
  const projDay = (v: number) => 6 + ((v - 101) / (482 - 101)) * 25;
  const mono = "JetBrains Mono Variable, JetBrains Mono, monospace";
  return (
    <svg
      viewBox={`0 0 ${W} ${H}`}
      className="block w-full"
      role="img"
      aria-label="Daily accrual rising to today at $101, projected to about $482 by month end against a $450 budget"
    >
      {[0, 250, 500].map((v) => (
        <g key={v}>
          <line x1={padL} x2={W - padR} y1={y(v)} y2={y(v)} stroke="var(--hair)" strokeWidth="1" />
          <text
            x={padL - 6}
            y={y(v) + 3}
            textAnchor="end"
            fontSize="8.5"
            fill="var(--ink3)"
            fontFamily={mono}
          >
            ${v}
          </text>
        </g>
      ))}
      <line
        x1={padL}
        x2={W - padR}
        y1={y(450)}
        y2={y(450)}
        stroke="var(--warn)"
        strokeWidth="1"
        strokeDasharray="4 3"
      />
      <text x={W - padR + 4} y={y(450) + 3} fontSize="8.5" fill="var(--warn)" fontFamily={mono}>
        budget $450
      </text>
      <path d={solid} fill="none" stroke="var(--steel)" strokeWidth="1.6" />
      <line
        x1={x(6)}
        x2={x(31)}
        y1={y(101)}
        y2={y(482)}
        stroke="var(--steel)"
        strokeWidth="1.4"
        strokeDasharray="4 4"
      />
      {alerts.map((v) => (
        <text
          key={v}
          x={x(projDay(v))}
          y={y(v) + 3}
          textAnchor="middle"
          fontSize="8"
          fill="var(--warn)"
        >
          ▲
        </text>
      ))}
      <circle cx={x(6)} cy={y(101)} r="2.5" fill="var(--steel)" />
      <text x={x(6) + 6} y={y(101) - 6} fontSize="9" fill="var(--ink2)" fontFamily={mono}>
        today · $101
      </text>
      <text x={W - padR + 4} y={y(482) + 3} fontSize="9" fill="var(--ink2)" fontFamily={mono}>
        ≈ $482
      </text>
    </svg>
  );
}

function fmtFeedDate(iso: string): string {
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    timeZone: "Asia/Kolkata",
  }).format(new Date(iso));
}

function fmtDelta(cents: number): string {
  return `${cents < 0 ? "−" : "+"}${fmtMoney(Math.abs(cents))}/mo`;
}

/**
 * Change-feed display, keyed by the fixture row's event_id. The frame's third
 * entry (internal-tools scaled a worker down) has no change_feed row, so it
 * renders from EXTRA_FEED alone. Where the frame disagrees with the fixtures
 * (frame: "Jul 3 · +$38/mo run-rate" for mobile-api; fixture: Jun 12 · +$41),
 * fixtures win — finding.
 */
const FEED_DISPLAY: Record<string, { label: string; note?: string }> = {
  evt_dbreports: { label: "db-reports created", note: "the month's largest change" },
  evt_ma_bill: { label: "mobile-api left provisioning — billing began" },
};

const EXTRA_FEED = [
  { date: "Jun 12", label: "internal-tools scaled a worker down", delta: "−$3/mo" },
];

function BillingOverviewPage() {
  const { org } = Route.useParams();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);
  const billing = useBillingOverview(org);
  const b = billing.data;

  const mtd = b?.mtd_cents ?? 0;
  const planFee = b?.plan_fee_cents ?? 0;
  const byProject = b?.by_project ?? [];
  const resourcesRollupMtd = byProject.reduce((sum, p) => sum + (p.mtd_cents ?? 0), 0);

  return (
    <>
      <SnavSettings
        org={org}
        orgName={orgRecord?.name ?? org}
        project="ecommerce"
        plan={orgRecord ? planLabel(orgRecord.plan) : "Business"}
        active="b-overview"
      />
      <main className="main">
        <div className="pgpad">
          <Pghead
            title="Billing · Overview"
            sub="Acme · Business — one number, then every step of how it's built"
          >
            <Btn variant="s" disabled disabledReason="Budget editing lands in Phase 4">
              Set budget…
            </Btn>
            <Btn variant="s" disabled disabledReason="Budget editing lands in Phase 4">
              Export ↗
            </Btn>
          </Pghead>

          <div className="grid grid-cols-4 gap-3.5">
            <Metric
              label="Month to date · Jul 1–6"
              value={fmtMoney(mtd)}
              mono
              note={`${fmtMoney(mtd - planFee)} resources + ${fmtMoney(planFee)} plan`}
            />
            <Metric
              label="Forecast · Jul"
              value={
                <>
                  {fmtMoney(b?.forecast_cents ?? 0)}
                  <span className="text-12 text-ink3">
                    {" "}
                    ± {fmtMoney(b?.forecast_margin_cents ?? 0)}
                  </span>
                </>
              }
              mono
              note="from live run-rates, not last month"
            />
            <Metric
              label="Budget · Jul"
              value={fmtMoney(b?.budget?.limit_cents ?? 0)}
              mono
              note={
                <span className="flex flex-col gap-1.5">
                  <span className="block h-1.5 overflow-hidden rounded bg-surface2">
                    <span
                      className="block h-full rounded bg-steel"
                      style={{ width: `${Math.min(100, b?.budget?.used_percent ?? 0)}%` }}
                    />
                  </span>
                  {/* Frame says "23% consumed · on pace for 92%" but the fixtures'
                      budget.used_percent is 38.1 — fixtures win. */}
                  {b?.budget?.used_percent ?? 0}% consumed
                </span>
              }
            />
            <Metric
              label="Last invoice · Jun"
              value={fmtMoney(b?.last_invoice?.total_cents ?? 0)}
              mono
              note={
                <span className="flex items-center gap-1.5">
                  <Pill tone="ok">paid</Pill>
                  Visa ·· 4412 · Jul 1
                </span>
              }
            />
          </div>

          <div className="flex items-start gap-3.5">
            <div className="flex min-w-0 flex-1 flex-col gap-3.5">
              <Card className="flex flex-col gap-2 p-4">
                <div className="flex flex-wrap items-baseline justify-between gap-2">
                  <div className="text-12p5 font-semibold">Daily accrual → month-end forecast</div>
                  <div className="text-10p5 text-ink3">
                    solid = billed · dashed = projected · ▲ budget alerts at 50 / 80 / 100%
                  </div>
                </div>
                <AccrualChart />
              </Card>

              <div className="tblwrap">
                <table className="tbl">
                  <thead>
                    <tr>
                      <th>Project</th>
                      <th>MTD</th>
                      <th>Run-rate /mo</th>
                      <th>vs Jun</th>
                      <th>30 d</th>
                    </tr>
                  </thead>
                  <tbody>
                    {PROJECT_DISPLAY.map((d) => {
                      const row = byProject.find((p) => p.name === d.name);
                      return (
                        <tr key={d.name}>
                          <td>
                            <span className="flex items-center gap-2 font-medium">
                              {d.dot ? <Dot tone={d.dot} /> : null}
                              {d.name}
                            </span>
                          </td>
                          <td className="mono">{fmtMoney(row?.mtd_cents ?? 0)}</td>
                          <td className="mono">{fmtMoney(row?.forecast_cents ?? 0)}</td>
                          <td className={d.vsTone ? VS_TONE[d.vsTone] : "text-ink3"}>{d.vsJun}</td>
                          <td>
                            {d.spark ? (
                              <Spark series={d.spark} tone={d.dot === "warn" ? "warn" : "steel"} />
                            ) : (
                              <span className="text-11 text-ink3">{d.sparkNote}</span>
                            )}
                          </td>
                        </tr>
                      );
                    })}
                    <tr>
                      <td className="text-ink2">Resources</td>
                      <td className="mono">{fmtMoney(resourcesRollupMtd)}</td>
                      <td className="mono">{fmtMoney(b?.resources_cents ?? 0)}</td>
                      <td colSpan={2} />
                    </tr>
                    <tr>
                      <td className="text-ink2">Business plan</td>
                      {/* The FRAME prints run-rate "$29" on this row — a stale
                          pre-re-anchor value; canon plan fee is $99. Finding. */}
                      <td className="mono">$99.00</td>
                      <td className="mono">{fmtMoney(planFee)}</td>
                      <td colSpan={2} />
                    </tr>
                    <tr>
                      <td className="font-semibold">Total</td>
                      <td className="mono font-semibold">{fmtMoney(mtd)}</td>
                      <td className="mono font-semibold">{fmtMoney(b?.forecast_cents ?? 0)}</td>
                      <td colSpan={2} className="text-11 text-ink3">
                        the same $482 every sidebar rolls up to — one arithmetic, everywhere
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <div className="flex w-[300px] shrink-0 flex-col gap-3.5">
              <Card className="flex flex-col gap-3 p-4">
                <Eyebrow>Budget & alerts</Eyebrow>
                <div className="flex items-center justify-between gap-2 text-12">
                  <span>Alert at 50 / 80 / 100%</span>
                  <Pill tone="st">in-app + email</Pill>
                </div>
                <div className="flex items-center justify-between gap-2 text-12">
                  <span className="text-ink3">Recipients</span>
                  <span>Owners + Admins</span>
                </div>
                <p className="text-11 text-ink3">
                  Budgets page humans, never stop services — a hard ceiling is a deliberate policy
                  (compute-ceiling, G7), not a billing side effect.
                </p>
              </Card>

              <Card className="flex flex-col gap-2.5 p-4">
                <Eyebrow>What changed this month</Eyebrow>
                {(b?.change_feed ?? []).map((row) => {
                  const display = row.event_id ? FEED_DISPLAY[row.event_id] : undefined;
                  return (
                    <div
                      key={row.event_id ?? row.at}
                      className="flex items-baseline gap-2 text-11p5"
                    >
                      <span className="mono w-12 shrink-0 text-ink3">
                        {row.at ? fmtFeedDate(row.at) : ""}
                      </span>
                      <span className="flex-1">
                        {display?.label ?? row.cause}
                        {display?.note ? (
                          <span className="text-ink3"> — {display.note}</span>
                        ) : null}
                      </span>
                      <span className="mono">{fmtDelta(row.delta_cents ?? 0)}</span>
                    </div>
                  );
                })}
                {EXTRA_FEED.map((row) => (
                  <div key={row.label} className="flex items-baseline gap-2 text-11p5">
                    <span className="mono w-12 shrink-0 text-ink3">{row.date}</span>
                    <span className="flex-1">{row.label}</span>
                    <span className="mono text-ok">{row.delta}</span>
                  </div>
                ))}
                <p className="border-hair border-t pt-2 text-11 text-ink3">
                  Every entry links to the audit event that caused it.
                </p>
              </Card>
            </div>
          </div>
        </div>
      </main>
    </>
  );
}

export const Route = createFileRoute("/_app/$org/billing/overview")({
  component: BillingOverviewPage,
});
