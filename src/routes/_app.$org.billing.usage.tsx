import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Fragment, useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Banner } from "@/design-system/banner";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Pill } from "@/design-system/pill";
import { planLabel, useUsage } from "@/features/billing/hooks";
import { useOrgs } from "@/features/org/hooks";
import type { UsageReport } from "@/lib/api";
import { fmtMoney } from "@/lib/fmt";

/**
 * B2 · Usage — the meter behind the invoice — plus the B8 warning state
 * (egress at 87% of included, so the warn banner renders at the top).
 */

type Meter = NonNullable<UsageReport["meters"]>[number];

/**
 * Per-meter display facts (labels, subs, unit grammar) — frame-fixed from B2.
 * Quantities, unit prices and MTD money come from the fixtures.
 */
const METER_DISPLAY: Record<
  string,
  {
    label: string;
    sub?: string;
    included: (m: Meter) => string;
    used: (m: Meter) => string;
  }
> = {
  compute_hours: {
    label: "Compute · instance-hours",
    sub: "api ×3 · workers · admin",
    included: () => "—",
    used: (m) => `${(m.used ?? 0).toLocaleString("en-US")} h`,
  },
  db_compute_hours: {
    label: "Databases · compute",
    sub: "db-main · db-reports · branches",
    included: () => "—",
    used: (m) => `${(m.used ?? 0).toLocaleString("en-US")} h`,
  },
  db_storage_gb_mo: {
    label: "Database storage",
    included: (m) => `${m.included ?? 0} GB`,
    used: (m) => `${m.used ?? 0} GB-mo`,
  },
  object_storage_gb_mo: {
    label: "Object storage",
    sub: "assets · 48.1 GB · 1.21 M obj",
    included: (m) => `${m.included ?? 0} GB`,
    used: (m) => `${m.used ?? 0} GB-mo billable`,
  },
  egress_gb: {
    label: "Network egress",
    included: (m) => `${m.included ?? 0} GB`,
    used: (m) => `${m.used ?? 0} GB`,
  },
  queue_requests_m: {
    label: "Queue · requests",
    included: (m) => `${m.included ?? 0} M`,
    used: (m) => `${m.used ?? 0} M`,
  },
  backups_gb_mo: {
    label: "Backups & PITR retention",
    included: (m) => `${m.included ?? 0} d`,
    used: (m) => `${m.used ?? 0} d window`,
  },
};

interface BranchDetail {
  label?: string;
  note?: string;
  used_label?: string;
  mtd_cents?: number;
}

function exportCsv(meters: Meter[]) {
  const rows = [
    ["meter", "included", "used", "overage_price", "overage_cents"],
    ...meters.map((m) => [
      m.meter ?? "",
      String(m.included ?? ""),
      String(m.used ?? ""),
      m.overage_price ?? "",
      String(m.overage_cents ?? ""),
    ]),
  ];
  const csv = rows.map((r) => r.map((c) => `"${c.replaceAll('"', '""')}"`).join(",")).join("\n");
  const url = URL.createObjectURL(new Blob([csv], { type: "text/csv" }));
  const a = document.createElement("a");
  a.href = url;
  a.download = "steloit-usage-2026-07.csv";
  a.click();
  URL.revokeObjectURL(url);
}

function UsagePage() {
  const { org } = Route.useParams();
  const navigate = useNavigate();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);
  const usage = useUsage(org);
  const meters = usage.data?.meters ?? [];

  const [showMath, setShowMath] = useState(true);
  const [muted, setMuted] = useState(false);

  const egress = meters.find((m) => m.meter === "egress_gb");
  const egressPct = egress?.included ? Math.round(((egress.used ?? 0) / egress.included) * 100) : 0;
  const egressWarn = egressPct >= 80;

  return (
    <>
      <SnavSettings
        org={org}
        orgName={orgRecord?.name ?? org}
        project="ecommerce"
        plan={orgRecord ? planLabel(orgRecord.plan) : "Business"}
        active="b-usage"
      />
      <main className="main">
        <div className="pgpad">
          {egressWarn && !muted ? (
            <Banner tone="warn">
              <span>
                Egress at {egressPct}% of included — {egress?.used ?? 0} of {egress?.included ?? 0}{" "}
                GB with 9 days left in the cycle. At this pace: ~118 GB.
              </span>
              <span className="ml-auto flex shrink-0 items-center gap-2">
                <Btn variant="s" onClick={() => setShowMath((v) => !v)}>
                  See the math
                </Btn>
                <Btn variant="gh" onClick={() => setMuted(true)}>
                  Mute for this cycle
                </Btn>
              </span>
            </Banner>
          ) : null}

          <Pghead
            title={
              <span className="flex items-center gap-2.5">
                Billing · Usage
                {egressWarn ? <Pill tone="warn">1 warning</Pill> : null}
              </span>
            }
            sub="The meter behind the invoice — every line queryable, exportable, priced in the open"
          >
            <Btn variant="s" onClick={() => exportCsv(meters)}>
              Export CSV
            </Btn>
            <Btn
              variant="s"
              onClick={() => navigate({ to: "/$org/billing/quotas", params: { org } })}
            >
              Quotas & limits (B7)
            </Btn>
          </Pghead>

          <div className="flex flex-wrap items-center gap-2">
            <span className="envpill">‹ July 2026 · in progress ›</span>
            <span className="chip on">All projects</span>
            <span className="chip">ecommerce</span>
            <span className="chip">analytics-pipeline</span>
            <span className="chip">internal-tools</span>
            <span className="chip">mobile-api</span>
            <span className="ml-auto">
              <Copybit>steloit usage export --month jul --format csv</Copybit>
            </span>
          </div>

          {showMath ? (
            <Card className="flex flex-col gap-3 p-4">
              <div className="text-[12.5px] font-semibold">The math, in the open</div>
              {/* Frame says "by Apr 1" / "April invoice" inside a July context —
                  source inconsistency, rendered against the fixtures' July cycle. */}
              <div className="logwell">
                <div>{"pace    87 GB in 21 days → ~4.1 GB/day → ~118 GB by month end"}</div>
                <div>{"overage ~18 GB × $0.09 = ~$1.62 on the next invoice"}</div>
                <div>
                  {"driver  assets bucket · 71% of egress · spiked with the gift-card launch"}
                </div>
              </div>
              <p className="text-[12px]">
                Honest recommendation: do nothing. ~$1.62 of overage is the cheapest option — no
                plan change pays for itself here. If the spike is unwanted, the driver is assets: a
                CDN cache rule would cut it (the assistant drafted one — prp_eg41c7, optional).
              </p>
              <p className="border-hair border-t pt-2.5 text-[11px] text-ink3">
                Upgrade prompts here are calculators, not sales — the console recommends a plan only
                when the math favors you, and shows the math either way.
              </p>
            </Card>
          ) : null}

          <div className="tblwrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th>Meter</th>
                  <th>Included in Business</th>
                  <th>Used · Jul 1–6</th>
                  <th>Unit price</th>
                  <th>MTD</th>
                </tr>
              </thead>
              <tbody>
                {meters.map((m) => {
                  const key = m.meter ?? "";
                  const d = METER_DISPLAY[key];
                  const branch =
                    key === "db_compute_hours"
                      ? ((m.detail?.[0] ?? undefined) as BranchDetail | undefined)
                      : undefined;
                  return (
                    <Fragment key={key}>
                      <tr>
                        <td>
                          <div className="font-medium">{d?.label ?? key}</div>
                          {d?.sub ? (
                            <div className="mt-0.5 text-[10.5px] text-ink3">{d.sub}</div>
                          ) : null}
                        </td>
                        <td className="mono text-ink2">{d ? d.included(m) : (m.included ?? 0)}</td>
                        <td className="mono">{d ? d.used(m) : (m.used ?? 0)}</td>
                        <td className="mono text-ink2">{m.overage_price ?? ""}</td>
                        <td>
                          <span className="flex items-center gap-2">
                            <span className="mono">{fmtMoney(m.overage_cents ?? 0)}</span>
                            {key === "egress_gb" ? <Pill tone="ok">within plan</Pill> : null}
                          </span>
                        </td>
                      </tr>
                      {branch ? (
                        <tr>
                          <td className="pl-7">
                            <span className="flex items-center gap-2 text-[12px] text-ink2">
                              ↳ {branch.label}
                              {branch.note ? <Pill tone="mut">{branch.note}</Pill> : null}
                            </span>
                          </td>
                          <td />
                          <td className="mono text-ink2">{branch.used_label}</td>
                          <td className="mono text-ink2">{m.overage_price ?? ""}</td>
                          <td className="mono text-ink2">{fmtMoney(branch.mtd_cents ?? 0)}</td>
                        </tr>
                      ) : null}
                    </Fragment>
                  );
                })}
                <tr>
                  <td className="font-semibold">Resources · month to date</td>
                  <td colSpan={3} />
                  {/* Canon resources-MTD is $72.42 (B1: mtd − plan fee); the fixture
                      meters sum to $62.42 — frame/fixture arithmetic gap, rendered as
                      the canon rollup and flagged as a finding. */}
                  <td className="mono font-semibold">$72.42</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div className="flex flex-col gap-1 text-[11px] text-ink3">
            <p>
              Meters update within ~5 min of consumption — the invoice is this table, frozen on the
              1st.
            </p>
            <p>
              'Included in Business' draws down first; overage pricing is printed here, not
              discovered on the invoice.
            </p>
            <p className="mono">
              API: GET /v1/usage?month=2026-07 · key cost-export scoped org·read billing (G8)
            </p>
          </div>
        </div>
      </main>
    </>
  );
}

export const Route = createFileRoute("/_app/$org/billing/usage")({
  component: UsagePage,
});
