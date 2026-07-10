import { Icon } from "@/design-system/icon";
import type { Service } from "@/lib/api";
import { fmtMoneyPerMonth } from "@/lib/fmt";
import { PRODUCT_ICON, PRODUCT_LABEL } from "./rail";
import { Nfoot, NitDisabled, NitLink, Nsec, Snav } from "./snav";

/**
 * Snav variant B — product level (W4/W5/W10): the selected product's
 * workspace. Overview/Metrics/Logs → Browse → Manage, Settings last.
 */
export function SnavProduct({
  org,
  project,
  env,
  service,
  projectTotalCents,
  active,
}: {
  org: string;
  project: string;
  env: string;
  service: Service;
  projectTotalCents?: number;
  active: "overview" | "branches" | "insights";
}) {
  const isPostgres = service.product === "postgres";
  const linkParams = { org, project, service: service.name };
  const search = { env };

  return (
    <Snav>
      <div className="snhead">
        <span className="glyph">
          <Icon id={PRODUCT_ICON[service.product]} />
        </span>
        <div>
          <div className="t">{PRODUCT_LABEL[service.product]}</div>
          <div className="u">
            {service.name} ▾ · {env}
          </div>
        </div>
      </div>
      <NitLink
        to="/$org/$project/svc/$service"
        params={linkParams}
        search={search}
        icon="s-eye"
        label="Overview"
        on={active === "overview"}
      />
      <NitDisabled icon="s-chart" label="Metrics" reason="Metrics (D9) land in Phase 2" />
      <NitDisabled icon="s-doc" label="Logs" reason="Logs (D10) land in Phase 2" />
      {isPostgres ? (
        <>
          <Nsec>Browse</Nsec>
          <NitDisabled icon="s-term" label="SQL Editor" reason="SQL Editor (D1) lands in Phase 2" />
          <NitDisabled
            icon="s-grid"
            label="Table Viewer"
            reason="Table Viewer (D2) lands in Phase 2"
          />
          <NitLink
            to="/$org/$project/svc/$service/insights"
            params={linkParams}
            search={search}
            icon="s-pulse"
            label="Query Insights"
            badge="1"
            on={active === "insights"}
          />
          <Nsec>Manage</Nsec>
          <NitLink
            to="/$org/$project/svc/$service/branches"
            params={linkParams}
            search={search}
            icon="s-branch"
            label="Branches"
            count="4"
            on={active === "branches"}
          />
          <NitDisabled icon="s-shield" label="Backups" reason="Backups (D5) land in Phase 2" />
          <NitDisabled
            icon="s-bind"
            label="Bindings"
            count="3"
            reason="Bindings (D11) land in Phase 2"
          />
          <NitDisabled icon="s-gear" label="Settings" reason="Settings (D12) land in Phase 2" />
        </>
      ) : (
        <>
          <Nsec>Manage</Nsec>
          <NitDisabled icon="s-bind" label="Bindings" reason="Bindings (D11) land in Phase 2" />
          <NitDisabled icon="s-gear" label="Settings" reason="Settings land in Phase 2" />
        </>
      )}
      <Nfoot>
        <div className="flex justify-between px-1.5">
          <span>This service</span>
          <span className="mono">{fmtMoneyPerMonth(service.monthly_estimate_cents ?? 0)}</span>
        </div>
        <div className="flex justify-between px-1.5 pt-1">
          <span>{project} total</span>
          <span className="mono">
            {projectTotalCents !== undefined ? fmtMoneyPerMonth(projectTotalCents) : "…"}
          </span>
        </div>
      </Nfoot>
    </Snav>
  );
}
