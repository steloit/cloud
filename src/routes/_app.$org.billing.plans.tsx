import { createFileRoute } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Btn } from "@/design-system/btn";
import { Pill } from "@/design-system/pill";
import { planLabel } from "@/features/billing/hooks";
import { useOrgs } from "@/features/org/hooks";

/**
 * B5 · Plans — the four-column matrix, verbatim from the frame. Reached from
 * Payment & plan and Quotas; lives under Payment & plan in the snav.
 */

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
                  <th>Free</th>
                  <th>Pro</th>
                  <th className="bg-steel-tint">
                    <span className="flex items-center gap-2">
                      Business <Pill tone="st">current</Pill>
                    </span>
                  </th>
                  <th>Enterprise</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td className="text-ink3">Price</td>
                  <td className="mono">$0</td>
                  <td className="mono">$29 /mo</td>
                  <td className="mono bg-steel-tint">$99 /mo</td>
                  <td className="mono">custom</td>
                </tr>
                {MATRIX.map(([feature, free, pro, business, enterprise]) => (
                  <tr key={feature}>
                    <td className="font-medium">{feature}</td>
                    <td className="text-ink2">{free}</td>
                    <td className="text-ink2">{pro}</td>
                    <td className="bg-steel-tint">{business}</td>
                    <td className="text-ink2">{enterprise}</td>
                  </tr>
                ))}
                <tr>
                  <td />
                  <td>
                    <div className="flex flex-col items-start gap-1.5">
                      <span className="flex items-center gap-2">
                        <Btn variant="s" disabled disabledReason="12 members > 3 · 4 projects > 1">
                          Downgrade…
                        </Btn>
                        <Pill tone="err">blocked</Pill>
                      </span>
                      <span className="text-[10.5px] text-ink3">
                        12 members &gt; 3 · 4 projects &gt; 1
                      </span>
                    </div>
                  </td>
                  <td>
                    <div className="flex flex-col items-start gap-1.5">
                      <span className="flex items-center gap-2">
                        <Btn
                          variant="s"
                          disabled
                          disabledReason="12 members > 5 · 2 cells need Business"
                        >
                          Downgrade…
                        </Btn>
                        <Pill tone="err">blocked</Pill>
                      </span>
                      <span className="text-[10.5px] text-ink3">
                        12 members &gt; 5 · 2 cells need Business
                      </span>
                    </div>
                  </td>
                  <td className="bg-steel-tint">
                    <Pill tone="st">your plan</Pill>
                  </td>
                  <td>
                    <Btn
                      variant="s"
                      onClick={() => {
                        window.location.href = "mailto:sales@steloit.com";
                      }}
                    >
                      Talk to us
                    </Btn>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <p className="text-[11px] text-ink3">
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
