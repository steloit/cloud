import { createFileRoute, Link } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Banner } from "@/design-system/banner";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { EmptyState } from "@/design-system/empty-state";
import { Pill } from "@/design-system/pill";
import { SkeletonRows } from "@/design-system/skeleton";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { useOrgs } from "@/features/org/hooks";
import { usePolicies } from "@/features/settings/hooks";

/** G7 · Organization · Policies — the rulebook; versioned, enforced at the platform. */

// Scope labels are frame-fixed (G7): Policy.scope only distinguishes org vs project,
// while the frame names the governed surface per rule.
const SCOPE_LABELS: Record<string, string> = {
  "allowed-regions": "environments",
  "ai-assistant": "organization",
  "public-buckets": "storage",
  "credential-rotation": "bindings",
  "compute-ceiling": "compute",
  "mfa-required": "identity",
};

// Last-changed is rendered from the frame: the schema carries last_change_event but
// not the actor/relative-date strings G7 shows.
const LAST_CHANGED: Record<string, string> = {
  "allowed-regions": "asha · yesterday · evt_44be",
  "ai-assistant": "asha · Mar 3 · evt_31c0 · open →",
  "public-buckets": "asha · 6 mo",
  "credential-rotation": "asha · 6 mo",
  "compute-ceiling": "priya · 2 mo",
  "mfa-required": "asha · today",
};

function enforcementPill(enforcement?: string) {
  if (enforcement === "warn") return <Pill tone="warn">warn → enforce Jul 20</Pill>;
  if (enforcement === "enabled") return <Pill tone="ok">enabled</Pill>;
  return <Pill tone="st">enforce</Pill>;
}

function OrgPoliciesPage() {
  const { org } = Route.useParams();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);
  const policies = usePolicies(org);
  const orgPolicies = (policies.data ?? []).filter((p) => p.scope?.project_id == null);

  // Four-state grammar (16-qa): pending → skeleton, error → failure card,
  // empty → EmptyState, else the table (canon carries 6 org policies).
  const isEmpty = policies.isSuccess && orgPolicies.length === 0;

  return (
    <>
      <SnavSettings
        org={org}
        orgName={orgRecord?.name ?? org}
        project="ecommerce"
        active="o-policies"
      />
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            title="Organization · Policies"
            sub="The rulebook — versioned, enforced at the platform, every trigger audited"
          >
            <Link to="/$org/settings/policies/new" params={{ org }}>
              <Btn variant="p">New policy</Btn>
            </Link>
          </Pghead>

          <Banner tone="warn">
            <span>
              <b>1 pending request:</b> marco asks for public CDN on assets under public-buckets ·
              yesterday 16:40
            </span>
            <span className="ml-auto flex flex-shrink-0 items-center gap-2">
              <Btn variant="p" disabled disabledReason="Approval threads land in Phase 4">
                Review
              </Btn>
              <Btn variant="s" disabled disabledReason="Approval threads land in Phase 4">
                Deny…
              </Btn>
            </span>
          </Banner>

          {policies.isError ? (
            <ApiFailureCard
              title="Policies didn't load"
              error={policies.error}
              requestLine={`GET /orgs/${org}/policies`}
              onRetry={() => policies.refetch()}
            />
          ) : isEmpty ? (
            <EmptyState
              compact
              icon="s-shield"
              title="No policies yet"
              meaning={
                <>
                  policies gate actions before they run — approval is a design primitive, not a
                  favor; the first rule turns the platform's defaults into your rulebook
                </>
              }
              cta={
                <Link to="/$org/settings/policies/new" params={{ org }}>
                  <Btn variant="p">New policy</Btn>
                </Link>
              }
              cli="steloit policy create allowed-regions"
            />
          ) : (
            <div className="tblwrap">
              <table className="tbl">
                <thead>
                  <tr>
                    <th>Policy</th>
                    <th>Rule</th>
                    <th>Scope</th>
                    <th>Enforcement</th>
                    <th>Last changed</th>
                  </tr>
                </thead>
                <tbody>
                  {policies.isPending ? (
                    <SkeletonRows cols={5} />
                  ) : (
                    orgPolicies.map((p) => (
                      <tr key={p.id}>
                        <td>
                          <span className="flex items-center gap-2">
                            <span className="mono text-11p5">{p.key}</span>
                            {p.key === "ai-assistant" ? <Pill tone="ai">AI</Pill> : null}
                          </span>
                        </td>
                        <td className="text-ink2">{p.description}</td>
                        <td className="text-ink2">{SCOPE_LABELS[p.key] ?? "organization"}</td>
                        <td>{enforcementPill(p.enforcement)}</td>
                        <td className="mono text-11 text-ink3">{LAST_CHANGED[p.key] ?? "—"}</td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          )}

          <Card className="flex flex-col gap-2.5 border-assist/40 p-4">
            <div>
              <Pill tone="ai">Law 4</Pill>
            </div>
            <div className="text-11p5 leading-relaxed text-ink2">
              Set ai-assistant to disabled and every AI surface disappears — the Assistant page, ⌘K
              "ask", describe-to-provision (AI1), and the per-product panels. No workflow breaks:
              the create grid, Observe, Deploy and manual tuning all keep working exactly as before.
              AI is an enhancement, never a dependency — the platform is whole without it. See the
              full policy (AI3).
            </div>
          </Card>

          <p className="text-11 text-ink3">
            Enforcement modes: warn (allowed, logged, nagged) → enforce (denied with the policy
            named — E3's 403 grammar). Projects may tighten these; nothing below the org may loosen
            them.
          </p>
        </div>
      </main>
    </>
  );
}

export const Route = createFileRoute("/_app/$org/settings/policies/")({
  component: OrgPoliciesPage,
});
