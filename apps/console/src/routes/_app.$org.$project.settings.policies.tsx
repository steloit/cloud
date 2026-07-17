import { createFileRoute, Link } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Btn } from "@/design-system/btn";
import { EmptyState } from "@/design-system/empty-state";
import { Pill } from "@/design-system/pill";
import { SkeletonRows } from "@/design-system/skeleton";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { useOrgs } from "@/features/org/hooks";
import { usePolicies } from "@/features/settings/hooks";
import type { Policy } from "@/lib/api";

/** G4 · Project · Policies — project rules tighten org rules, never loosen them. */

// The org rows G4 shows as inherited context (the full rulebook lives at G7).
const INHERITED_KEYS = ["credential-rotation", "allowed-regions"];

// Frame-fixed changed-by strings (G4): the schema carries last_change_event only.
const LAST_CHANGED: Record<string, string> = {
  "prod-guard": "priya · 3 mo",
  "preview-minimal": "priya · 3 mo",
  "credential-rotation": "asha · 6 mo",
  "allowed-regions": "asha · yesterday",
};

function policyRow(p: Policy, inherited: boolean) {
  return (
    <tr key={p.id}>
      <td className="mono text-11p5">{p.key}</td>
      <td className="text-ink2">{p.description}</td>
      <td>
        {inherited ? <Pill tone="mut">inherited · org</Pill> : <Pill tone="st">project</Pill>}
      </td>
      <td>
        <Pill tone="st">enforce</Pill>
      </td>
      <td className="mono text-11 text-ink3">{LAST_CHANGED[p.key] ?? "—"}</td>
    </tr>
  );
}

function ProjectPoliciesPage() {
  const { org, project } = Route.useParams();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);
  const policies = usePolicies(org);

  const projectRows = (policies.data ?? []).filter((p) => p.scope?.project_id === "prj_ecommerce");
  const inheritedRows = (policies.data ?? []).filter(
    (p) => p.scope?.project_id == null && INHERITED_KEYS.includes(p.key),
  );

  // Four-state grammar (16-qa): pending → skeleton, error → failure card,
  // empty → EmptyState, else the table. Empty means neither project rules nor
  // inherited org rows — canon-unreachable (credential-rotation always
  // applies), wired anyway.
  const isEmpty = policies.isSuccess && projectRows.length + inheritedRows.length === 0;

  return (
    <>
      <SnavSettings
        org={org}
        orgName={orgRecord?.name ?? org}
        project={project}
        active="p-policies"
      />
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            title="Project · Policies"
            sub="What applies to ecommerce — project rules can tighten org rules, never loosen them"
          >
            {/* The G7 wizard is live at the org level and scopes to a project —
                the honest destination (finding: no project-scoped policy endpoint). */}
            <Link to="/$org/settings/policies/new" params={{ org }}>
              <Btn variant="p">New project policy</Btn>
            </Link>
          </Pghead>

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
                  favor; project rules tighten org rules, never loosen them
                </>
              }
              cta={
                <Link to="/$org/settings/policies/new" params={{ org }}>
                  <Btn variant="p">New project policy</Btn>
                </Link>
              }
              cli="steloit policy create --project ecommerce"
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
                    <>
                      {projectRows.map((p) => policyRow(p, false))}
                      {inheritedRows.map((p) => policyRow(p, true))}
                    </>
                  )}
                </tbody>
              </table>
            </div>
          )}

          <p className="text-11 text-ink3">
            Inherited rows are managed at the organization (G7). Every policy is versioned; every
            change and every triggered denial is an audit event — the same trail W12 shows.
          </p>
        </div>
      </main>
    </>
  );
}

export const Route = createFileRoute("/_app/$org/$project/settings/policies")({
  component: ProjectPoliciesPage,
});
