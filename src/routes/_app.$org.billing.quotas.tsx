import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Pill } from "@/design-system/pill";
import { planLabel, useQuotas } from "@/features/billing/hooks";
import { useOrgs } from "@/features/org/hooks";
import type { Quota } from "@/lib/api";

/** B7 · Quotas & included usage — lives under Usage in the snav (active b-usage). */

function fmtM(n: number): string {
  const m = n / 1_000_000;
  return `${m % 1 === 0 ? m : Number(m.toFixed(1))}M`;
}

function fmtQty(n: number): string {
  return n.toLocaleString("en-US");
}

/**
 * Per-quota display grammar — names and unit phrasing are frame-fixed from B7;
 * quantities come from the fixtures. Note: the frame shows preview
 * environments at "8 / 10 concurrent" while the fixtures say used 1 of 10 —
 * fixtures win, rendered from data. Likewise the frame's "6.2M / 50M"
 * observability events render from the fixture's 31.2M used.
 */
const QUOTA_DISPLAY: Record<string, { label: string; used: (q: Quota) => string }> = {
  egress: { label: "Egress", used: (q) => `${q.used} GB / ${q.included} GB` },
  events: { label: "Observability events", used: (q) => `${fmtM(q.used)} / ${fmtM(q.included)}` },
  ai_requests: {
    label: "AI assistant requests",
    used: (q) => `${fmtQty(q.used)} / ${fmtQty(q.included)}`,
  },
  build_minutes: {
    label: "Build minutes",
    used: (q) => `${fmtQty(q.used)} / ${fmtQty(q.included)} min`,
  },
  seats: { label: "Seats", used: (q) => `${q.used} / ${q.included}` },
  preview_environments: {
    label: "Preview environments",
    used: (q) => `${q.used} / ${q.included} concurrent`,
  },
  api_rate: { label: "API requests", used: (q) => `${q.used}/min / ${q.included}/min` },
};

function QuotasPage() {
  const { org } = Route.useParams();
  const navigate = useNavigate();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);
  const quotas = useQuotas(org);

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
          <Pghead
            title="Billing · Quotas & included usage"
            sub="What Business includes, what overage costs, and exactly what happens at each limit — soft limits bill, hard limits stop, and both say so before you hit them."
          >
            <Btn
              variant="s"
              onClick={() => navigate({ to: "/$org/billing/plans", params: { org } })}
            >
              Compare plans
            </Btn>
          </Pghead>

          <div className="tblwrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th>Allowance</th>
                  <th>Used this cycle</th>
                  <th>Type</th>
                  <th>Beyond it</th>
                  <th>At the limit</th>
                </tr>
              </thead>
              <tbody>
                {(quotas.data ?? []).map((q) => {
                  const d = QUOTA_DISPLAY[q.name];
                  const pct = q.included > 0 ? (q.used / q.included) * 100 : 0;
                  const warn = pct >= 80;
                  return (
                    <tr key={q.name}>
                      <td className="font-medium">{d?.label ?? q.name}</td>
                      <td>
                        <div className="mono">{d ? d.used(q) : `${q.used} / ${q.included}`}</div>
                        <div className="mt-1.5 h-1.5 w-[120px] overflow-hidden rounded bg-surface2">
                          <div
                            className={`h-full rounded ${warn ? "bg-warn" : "bg-steel"}`}
                            style={{ width: `${Math.min(100, pct)}%` }}
                          />
                        </div>
                      </td>
                      <td>
                        <Pill tone={q.type === "hard" ? "st" : "mut"}>{q.type}</Pill>
                      </td>
                      <td className="mono text-ink2">{q.price_beyond ?? "—"}</td>
                      <td className="text-[12px] text-ink2">{q.at_limit}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          <div className="grid grid-cols-2 gap-3.5">
            <Card className="p-4">
              <p className="text-[12px] text-ink2">
                Soft limits never break production. (AI Gateway tokens are infrastructure — metered
                like any resource, not a plan quota.) Egress, events, AI requests, builds and seats
                keep working past the line and meter as overage — the limit is a billing threshold,
                not a wall.
              </p>
            </Card>
            <Card className="p-4">
              <p className="text-[12px] text-ink2">
                Hard limits fail loudly and early. Previews queue with a stated reason; the API
                returns 429 with Retry-After. You're warned at 80% (B8) before either happens.
              </p>
            </Card>
          </div>
        </div>
      </main>
    </>
  );
}

export const Route = createFileRoute("/_app/$org/billing/quotas")({
  component: QuotasPage,
});
