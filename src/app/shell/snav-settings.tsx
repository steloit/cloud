import { Icon } from "@/design-system/icon";
import { Nfoot, NitDisabled, NitLink, Nsec, Snav } from "./snav";

/** Snav variant D — the settings plane (ADR-012: org admin behind the gear). */
export function SnavSettings({ org, orgName }: { org: string; orgName: string }) {
  return (
    <Snav>
      <div className="snhead">
        <Icon id="s-chev" className="h-3.5 w-3.5 rotate-180 text-ink3" />
        <div className="t">Settings</div>
      </div>
      <Nsec>Project · ecommerce</Nsec>
      <NitDisabled label="General" reason="Project settings (G1) land in Phase 2" />
      <NitDisabled label="Members & roles" reason="Project members (G2) land in Phase 2" />
      <NitDisabled label="Git integration" reason="Git integration (G3) lands in Phase 2" />
      <NitDisabled label="Policies" reason="Project policies (G4) land in Phase 2" />
      <Nsec>Organization · {orgName}</Nsec>
      <NitDisabled label="General" reason="Org settings (G5) land in Phase 2" />
      <NitDisabled label="Members" count="12" reason="Org members (G6) land in Phase 2" />
      <NitLink to="/$org/settings/audit" params={{ org }} label="Audit log" on />
      <NitDisabled label="Policies" reason="Org policies (G7) land in Phase 2" />
      <NitDisabled label="API keys" reason="API keys (G8) land in Phase 2" />
      <NitDisabled label="Cells" count="2" reason="Cells (X2) land in Phase 2" />
      <NitDisabled label="Templates" count="3" reason="Templates (T1) land in Phase 2" />
      <Nsec>Billing</Nsec>
      <NitDisabled label="Overview" reason="Billing (B1) lands in Phase 2" />
      <NitDisabled label="Usage" reason="Billing usage (B2) lands in Phase 2" />
      <NitDisabled label="Invoices" reason="Invoices (B3) land in Phase 2" />
      <NitDisabled label="Payment & plan" reason="Payment & plan (B4) lands in Phase 2" />
      <Nfoot>
        <div className="px-1.5">{orgName} · Business plan</div>
      </Nfoot>
    </Snav>
  );
}
