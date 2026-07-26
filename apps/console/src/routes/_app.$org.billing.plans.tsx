import { createFileRoute, Link } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Btn } from "@/design-system/btn";
import { Pill } from "@/design-system/pill";
import { Skeleton } from "@/design-system/skeleton";
import { planLabel, useSubscription } from "@/features/billing/hooks";
import { useOrgs } from "@/features/org/hooks";
import type { Plan } from "@/lib/api";

/**
 * B5 · Plans — the four-column matrix, verbatim from the frame. Reached from
 * Payment & plan and Quotas; lives under Payment & plan in the snav.
 * The current column derives from GET /orgs/{org}/subscription (the B4
 * source) — canon acme resolves to Business, exactly the frame's column.
 */

const UPGRADE_REASON =
  "No upgrade flow in the frames — trial→Pro via the confirm page (B11) is the one wired path (finding)";

/** Column order doubles as plan rank — below current downgrades, above upgrades. */
const COLS: Array<{ key: Plan; label: string; price: string }> = [
  { key: "free", label: "Free", price: "$0" },
  { key: "pro", label: "Pro", price: "$29 /mo" },
  { key: "business", label: "Business", price: "$99 /mo" },
  { key: "enterprise", label: "Enterprise", price: "custom" },
];

const MATRIX: Array<[string, string, string, string, string]> = [
  ["Members", "3", "5 then $7/seat", "20 then $7/seat", "custom + SCIM"],
  ["Projects", "1", "3", "unlimited", "unlimited"],
  ["Preview environments", "—", "3 concurrent", "10 concurrent", "custom"],
  ["Egress included", "10 GB", "50 GB then $0.09/GB", "100 GB then $0.09/GB", "custom"],
  ["Observability events", "1M", "10M then $1.20/M", "50M then $1.20/M", "custom"],
  [
    "AI assistant requests",
    "ask-only · 500",
    "5k then $2/1k",
    "25k · org-wide insights",
    "custom · private upstreams",
  ],
  ["Build minutes", "300", "2,400 then $0.01/min", "6,000", "custom"],
  ["Log retention", "3 days", "14 days", "30 days", "90 days + export"],
  ["PITR window", "—", "7 days", "30 days", "30 days"],
  ["Custom roles (RBAC)", "—", "—", "✓", "✓ + SCIM"],
  ["SAML SSO", "—", "—", "✓", "✓"],
  ["BYOC cells", "—", "—", "✓ control-plane fee per cell", "✓ + dedicated cells & private regions"],
  ["Audit log export", "—", "—", "—", "✓"],
  ["Support · SLA", "community", "business hours", "priority", "24×7 · 99.95% SLA"],
];

function PlansPage() {
  const { org } = Route.useParams();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);
  const sub = useSubscription(org);
  // Subscription is the source (B4's); the org record backstops while it
  // loads. Undefined (both pending/failed) marks no column current — the
  // matrix itself is frame content and renders regardless.
  const plan: Plan | undefined = sub.data?.plan ?? orgRecord?.plan;
  const rank = plan ? COLS.findIndex((c) => c.key === plan) : -1;

  /** The CTA row, per column — current pill · downgrade link · gated upgrade. */
  const cta = (col: (typeof COLS)[number], i: number) => {
    if (!plan) return <Skeleton className="h-6 w-24" />;
    if (col.key === plan) return <Pill tone="mut">Current plan</Pill>;
    if (col.key === "enterprise") {
      return (
        <Btn
          variant="s"
          onClick={() => {
            window.location.href = "mailto:sales@steloit.com";
          }}
        >
          Talk to us
        </Btn>
      );
    }
    if (i < rank) {
      // B5 frame truth: downgrades from canon Business are blocked and "the
      // button says why, like every blocked action here" — reasons verbatim
      // from the frame. The unblocked flow lives on Payment & plan (B4).
      const reason =
        col.key === "free"
          ? "blocked — 12 members > Free's 3 · 4 projects > Free's 1"
          : "blocked — 12 members > 5 · 2 cells need Business";
      return (
        <span className="flex items-center gap-2">
          <Btn variant="s" disabled disabledReason={reason}>
            {col.key === "free" ? "Downgrade to Free…" : "Downgrade…"}
          </Btn>
          <Pill tone="err">blocked</Pill>
        </span>
      );
    }
    return (
      <Btn variant="s" disabled disabledReason={UPGRADE_REASON}>
        Upgrade…
      </Btn>
    );
  };

  return (
    <>
      <SnavSettings
        org={org}
        orgName={orgRecord?.name ?? org}
        project="ecommerce"
        plan={orgRecord ? planLabel(orgRecord.plan) : "Business"}
        active="b-payment"
      />
      <main className="main">
        <div className="pgpad">
          <Pghead
            title="Billing · Plans"
            sub="The hybrid model in one sentence: subscription = platform capabilities, pay-as-you-go = infrastructure, overage = beyond included quotas. Safety is never gated."
          />

          <div className="tblwrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th />
                  {COLS.map((col) => (
                    <th key={col.key} className={col.key === plan ? "bg-steel-tint" : undefined}>
                      {col.key === plan ? (
                        <span className="flex items-center gap-2">
                          {col.label} <Pill tone="st">current</Pill>
                        </span>
                      ) : (
                        col.label
                      )}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td className="text-ink3">Price</td>
                  {COLS.map((col) => (
                    <td key={col.key} className={col.key === plan ? "mono bg-steel-tint" : "mono"}>
                      {col.price}
                    </td>
                  ))}
                </tr>
                {MATRIX.map(([feature, ...cells]) => (
                  <tr key={feature}>
                    <td className="font-medium">{feature}</td>
                    {COLS.map((col, i) => (
                      <td
                        key={col.key}
                        className={col.key === plan ? "bg-steel-tint" : "text-ink2"}
                      >
                        {cells[i]}
                      </td>
                    ))}
                  </tr>
                ))}
                <tr>
                  <td />
                  {COLS.map((col, i) => (
                    <td key={col.key} className={col.key === plan ? "bg-steel-tint" : undefined}>
                      {cta(col, i)}
                    </td>
                  ))}
                </tr>
              </tbody>
            </table>
          </div>

          <p className="text-11 text-ink3">
            Infrastructure (databases, caches, storage, compute, AI Gateway tokens) is pay-as-you-go
            on every tier and never appears in this table. Never gated on any plan: TLS, backups,
            MFA, policies, alerts, dunning protections, deleting your own data.
          </p>
        </div>
      </main>
    </>
  );
}

export const Route = createFileRoute("/_app/$org/billing/plans")({
  component: PlansPage,
});
