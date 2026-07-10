import { Dot, healthDotTone } from "@/design-system/pill";
import type { BillingOverview, Project } from "@/lib/api";
import { fmtMoney } from "@/lib/fmt";
import { NitDisabled, NitLink, Nsec, Snav } from "./snav";

/** Snav variant A — org level (W1/W2): All projects · Dashboards · Projects · New project. */
export function SnavOrg({
  org,
  orgName,
  projects,
  billing,
  active,
}: {
  org: string;
  orgName: string;
  projects: Project[];
  billing?: BillingOverview;
  active: "all-projects" | "new-project";
}) {
  const orgTotal =
    billing?.forecast_cents !== undefined ? `${fmtMoney(billing.forecast_cents)}/mo` : "…";

  return (
    <Snav>
      <div className="snhead">
        <span
          className="cav h-6 w-6 rounded-md"
          style={{ background: "linear-gradient(135deg,#E36C4B,#B34A2E)" }}
        >
          {orgName.slice(0, 1).toUpperCase()}
        </span>
        <div>
          <div className="t">{orgName}</div>
          <div className="u">
            {projects.length} projects · {orgTotal}
          </div>
        </div>
      </div>
      <NitLink
        to="/$org"
        params={{ org }}
        icon="s-grid"
        label="All projects"
        on={active === "all-projects"}
      />
      <NitDisabled
        icon="s-chart"
        label="Dashboards"
        count="7"
        reason="Dashboards land in Phase 2"
      />
      <Nsec>Projects</Nsec>
      {projects.map((p) => (
        <NitLink
          key={p.id}
          to="/$org/$project"
          params={{ org, project: p.name }}
          dot={<Dot tone={healthDotTone(p.health ?? "ok")} />}
          label={p.name}
        />
      ))}
      <div className="my-2 h-px bg-hair" />
      <NitLink
        to="/$org/new-project"
        params={{ org }}
        icon="s-plus"
        label="New project"
        steel
        on={active === "new-project"}
      />
    </Snav>
  );
}
