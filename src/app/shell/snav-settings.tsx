import { Icon } from "@/design-system/icon";
import { Nfoot, NitDisabled, NitLink, Nsec, Snav } from "./snav";

/**
 * Snav variant D — the settings plane (ADR-012: org admin behind the gear).
 * Three groups: Project · Organization · Billing; footer states the plan.
 */
export type SettingsActive =
  | "p-general"
  | "p-members"
  | "p-git"
  | "p-policies"
  | "o-general"
  | "o-members"
  | "audit"
  | "o-policies"
  | "api-keys"
  | "templates"
  | "b-overview"
  | "b-usage"
  | "b-invoices"
  | "b-payment";

export function SnavSettings({
  org,
  orgName,
  project,
  plan = "Business",
  active,
}: {
  org: string;
  orgName: string;
  /** The active project for the Project group (the shell's convention). */
  project: string;
  plan?: string;
  active: SettingsActive;
}) {
  const p = { org, project };
  const o = { org };
  return (
    <Snav>
      <div className="snhead">
        <Icon id="s-chev" className="h-3.5 w-3.5 rotate-180 text-ink3" />
        <div className="t">Settings</div>
      </div>
      <Nsec>Project · {project}</Nsec>
      <NitLink
        to="/$org/$project/settings/general"
        params={p}
        label="General"
        on={active === "p-general"}
      />
      <NitLink
        to="/$org/$project/settings/members"
        params={p}
        label="Members & roles"
        on={active === "p-members"}
      />
      <NitLink
        to="/$org/$project/settings/git"
        params={p}
        label="Git integration"
        on={active === "p-git"}
      />
      <NitLink
        to="/$org/$project/settings/policies"
        params={p}
        label="Policies"
        on={active === "p-policies"}
      />
      <Nsec>Organization · {orgName}</Nsec>
      <NitLink to="/$org/settings/general" params={o} label="General" on={active === "o-general"} />
      <NitLink
        to="/$org/settings/members"
        params={o}
        label="Members"
        count="12"
        on={active === "o-members"}
      />
      <NitLink to="/$org/settings/audit" params={o} label="Audit log" on={active === "audit"} />
      <NitLink
        to="/$org/settings/policies"
        params={o}
        label="Policies"
        on={active === "o-policies"}
      />
      <NitLink
        to="/$org/settings/api-keys"
        params={o}
        label="API keys"
        on={active === "api-keys"}
      />
      <NitDisabled label="Cells" count="2" reason="Cells (X2) land in Phase 4" />
      <NitLink
        to="/$org/settings/templates"
        params={o}
        label="Templates"
        count="3"
        on={active === "templates"}
      />
      <Nsec>Billing</Nsec>
      <NitLink
        to="/$org/billing/overview"
        params={o}
        label="Overview"
        on={active === "b-overview"}
      />
      <NitLink to="/$org/billing/usage" params={o} label="Usage" on={active === "b-usage"} />
      <NitLink
        to="/$org/billing/invoices"
        params={o}
        label="Invoices"
        on={active === "b-invoices"}
      />
      <NitLink
        to="/$org/billing/payment"
        params={o}
        label="Payment & plan"
        on={active === "b-payment"}
      />
      <Nfoot>
        <div className="px-1.5">
          {orgName} · {plan} plan
        </div>
      </Nfoot>
    </Snav>
  );
}
