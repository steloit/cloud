import { Icon } from "@/design-system/icon";
import type { Service } from "@/lib/api";
import { fmtMoney, fmtMoneyPerMonth } from "@/lib/fmt";
import { PRODUCT_ICON, PRODUCT_LABEL } from "./rail";
import { Nfoot, NitDisabled, NitLink, Nsec, Snav } from "./snav";

/**
 * Snav variant B — the selected product's workspace. Item sets match the
 * per-product coverage matrix exactly, in the order Overview/Metrics/Logs →
 * Browse → Manage, Settings always last (Design-Spec §nav rule).
 */

interface MatrixItem {
  label: string;
  count?: string;
  /** Frame that owns this destination — used in the disabled reason. */
  frame: string;
  /** Live route key when the surface is built. */
  live?: "overview" | "branches" | "insights";
  badge?: string;
}

interface ProductMatrix {
  browse: MatrixItem[];
  manage: MatrixItem[];
}

const MATRIX: Record<Service["product"], ProductMatrix> = {
  postgres: {
    browse: [
      { label: "SQL Editor", frame: "D1" },
      { label: "Table Viewer", frame: "D2" },
      { label: "Query Insights", frame: "D4", live: "insights", badge: "1" },
    ],
    manage: [
      { label: "Branches", frame: "W5", live: "branches", count: "4" },
      { label: "Backups", frame: "D5" },
      { label: "Bindings", frame: "D11", count: "3" },
      { label: "Settings", frame: "D12" },
    ],
  },
  valkey: {
    browse: [
      { label: "Data Browser", frame: "D3" },
      { label: "CLI Console", frame: "D6" },
    ],
    manage: [
      { label: "Bindings", frame: "D11", count: "1" },
      { label: "Settings", frame: "D14" },
    ],
  },
  storage: {
    browse: [{ label: "Object Browser", frame: "D7" }],
    manage: [
      { label: "Lifecycle rules", frame: "D15", count: "2" },
      { label: "Bindings", frame: "D11", count: "2" },
      { label: "Settings", frame: "D16" },
    ],
  },
  queue: {
    browse: [
      { label: "Messages", frame: "D8" },
      { label: "Dead letters", frame: "D8", count: "2" },
    ],
    manage: [
      { label: "Schedules", frame: "D17", count: "1" },
      { label: "Bindings", frame: "D11", count: "3" },
      { label: "Settings", frame: "D18" },
    ],
  },
  web: {
    browse: [
      { label: "Deployments", frame: "D19" },
      { label: "Shell", frame: "D20" },
    ],
    manage: [
      { label: "Domains", frame: "D21", count: "2" },
      { label: "Scaling", frame: "D22" },
      { label: "Bindings", frame: "D11", count: "3" },
      { label: "Settings", frame: "D23" },
    ],
  },
  worker: {
    browse: [
      { label: "Deployments", frame: "D19" },
      { label: "Shell", frame: "D20" },
    ],
    manage: [
      { label: "Schedules", frame: "S5", count: "2" },
      { label: "Scaling", frame: "D22" },
      { label: "Bindings", frame: "D11", count: "3" },
      { label: "Settings", frame: "D23" },
    ],
  },
  "gpu-worker": {
    browse: [{ label: "Shell", frame: "D20" }],
    manage: [{ label: "Settings", frame: "D23" }],
  },
  "ai-gateway": {
    browse: [],
    manage: [{ label: "Settings", frame: "X1" }],
  },
};

const LIVE_ICON = { insights: "s-pulse", branches: "s-branch" } as const;

function MatrixNit({
  item,
  linkParams,
  search,
  active,
}: {
  item: MatrixItem;
  linkParams: { org: string; project: string; service: string };
  search: { env: string };
  active: string;
}) {
  if (item.live === "insights" || item.live === "branches") {
    return (
      <NitLink
        to={`/$org/$project/svc/$service/${item.live}`}
        params={linkParams}
        search={search}
        icon={LIVE_ICON[item.live]}
        label={item.label}
        count={item.count}
        badge={item.badge}
        on={active === item.live}
      />
    );
  }
  return (
    <NitDisabled
      label={item.label}
      count={item.count}
      reason={`${item.label} (${item.frame}) lands in a later phase`}
    />
  );
}

export function SnavProduct({
  org,
  project,
  env,
  service,
  serviceCount,
  projectTotalCents,
  active,
}: {
  org: string;
  project: string;
  env: string;
  service: Service;
  /** Instances of this product in the project — switcher chevron only at n>1. */
  serviceCount: number;
  projectTotalCents?: number;
  active: "overview" | "branches" | "insights";
}) {
  const matrix = MATRIX[service.product];
  const linkParams = { org, project, service: service.name };
  const search = { env };
  const usageBased = service.product === "storage";
  const costLabel = usageBased
    ? `${fmtMoney(914)} mtd`
    : fmtMoneyPerMonth(service.monthly_estimate_cents ?? 0);

  return (
    <Snav>
      <div className="snhead">
        <span className="glyph">
          <Icon id={PRODUCT_ICON[service.product]} />
        </span>
        <div>
          <div className="t">{PRODUCT_LABEL[service.product]}</div>
          <div className="u">
            {service.name}
            {serviceCount > 1 ? " ▾" : ""} · {env}
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
      <NitDisabled
        icon="s-chart"
        label="Metrics"
        reason="Per-service metrics (D9) land in a later phase"
      />
      <NitDisabled
        icon="s-doc"
        label="Logs"
        reason="Per-service logs (D10) land in a later phase"
      />
      {matrix.browse.length > 0 ? (
        <>
          <Nsec>Browse</Nsec>
          {matrix.browse.map((item) => (
            <MatrixNit
              key={item.label}
              item={item}
              linkParams={linkParams}
              search={search}
              active={active}
            />
          ))}
        </>
      ) : null}
      <Nsec>Manage</Nsec>
      {matrix.manage.map((item) => (
        <MatrixNit
          key={item.label}
          item={item}
          linkParams={linkParams}
          search={search}
          active={active}
        />
      ))}
      <Nfoot>
        <div className="flex justify-between px-1.5">
          <span>This service</span>
          <span className="mono">{costLabel}</span>
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
