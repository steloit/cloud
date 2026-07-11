import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import { useNavigate } from "@tanstack/react-router";
import { menuContentClass, menuItemClass } from "@/app/shell/ctx";
import { Icon, type IconId } from "@/design-system/icon";
import { Dot, statusDotTone } from "@/design-system/pill";
import type { CreatableProduct } from "@/features/create/type-blocks";
import type { Service } from "@/lib/api";
import { fmtMoney, fmtMoneyPerMonth } from "@/lib/fmt";
import { cn } from "@/lib/utils";
import { PRODUCT_ICON, PRODUCT_LABEL } from "./rail";
import { Nfoot, NitDisabled, NitLink, Nsec, Snav } from "./snav";

/**
 * Snav variant B — the selected product's workspace. Item sets match the
 * per-product coverage matrix exactly, in the order Overview/Metrics/Logs →
 * Browse → Manage, Settings always last (Design-Spec §nav rule). The header
 * becomes the SW3 instance switcher when the product runs ×n (n>1).
 */

interface MatrixItem {
  label: string;
  count?: string;
  /** Route path segment under svc/$service; absent = disabled with the frame ref. */
  route?: string;
  frame: string;
  badge?: string;
  icon?: IconId;
}

interface ProductMatrix {
  top: MatrixItem[];
  browse: MatrixItem[];
  manage: MatrixItem[];
}

const METRICS: MatrixItem = { label: "Metrics", frame: "D9", route: "metrics", icon: "s-chart" };
const LOGS_PG: MatrixItem = { label: "Logs", frame: "D10", route: "logs", icon: "s-doc" };
const LOGS_OFF: MatrixItem = { label: "Logs", frame: "D10 grammar", icon: "s-doc" };
const AI_INSIGHTS: MatrixItem = {
  label: "AI insights",
  frame: "AI6–8",
  route: "ai",
  badge: "1",
  icon: "s-ai",
};

const MATRIX: Record<Service["product"], ProductMatrix> = {
  postgres: {
    top: [METRICS, LOGS_PG],
    browse: [
      { label: "SQL Editor", frame: "D1", route: "sql-editor", icon: "s-term" },
      { label: "Table Viewer", frame: "D2", route: "tables", icon: "s-grid" },
      { label: "Query Insights", frame: "D4", route: "insights", badge: "1", icon: "s-pulse" },
    ],
    manage: [
      { label: "Branches", frame: "W5", route: "branches", count: "4", icon: "s-branch" },
      { label: "Backups", frame: "D5", route: "backups", icon: "s-shield" },
      { label: "Bindings", frame: "D11", route: "bindings", count: "3", icon: "s-bind" },
      { label: "Settings", frame: "D12", route: "settings", icon: "s-gear" },
    ],
  },
  valkey: {
    top: [AI_INSIGHTS, METRICS, LOGS_OFF],
    browse: [
      { label: "Data Browser", frame: "D3", route: "data-browser", icon: "s-grid" },
      { label: "CLI Console", frame: "D6", route: "cli", icon: "s-term" },
    ],
    manage: [
      { label: "Bindings", frame: "D11", route: "bindings", count: "1", icon: "s-bind" },
      { label: "Settings", frame: "D14", route: "settings", icon: "s-gear" },
    ],
  },
  storage: {
    top: [AI_INSIGHTS, { ...METRICS, route: undefined, frame: "D9 grammar" }, LOGS_OFF],
    browse: [{ label: "Object Browser", frame: "D7", route: "objects", icon: "s-bucket" }],
    manage: [
      { label: "Lifecycle rules", frame: "D15", route: "lifecycle", count: "2", icon: "s-cron" },
      { label: "Bindings", frame: "D11", route: "bindings", count: "2", icon: "s-bind" },
      { label: "Settings", frame: "D16", route: "settings", icon: "s-gear" },
    ],
  },
  queue: {
    top: [AI_INSIGHTS, { ...METRICS, route: undefined, frame: "D9 grammar" }, LOGS_OFF],
    browse: [
      { label: "Messages", frame: "D8", route: "messages", icon: "s-grid" },
      { label: "Dead letters", frame: "D8", route: "messages", count: "2", icon: "s-x" },
    ],
    manage: [
      { label: "Schedules", frame: "D17", route: "schedules", count: "1", icon: "s-cron" },
      { label: "Bindings", frame: "D11", route: "bindings", count: "3", icon: "s-bind" },
      { label: "Settings", frame: "D18", route: "settings", icon: "s-gear" },
    ],
  },
  web: {
    top: [{ ...METRICS, route: undefined, frame: "D9 grammar" }, LOGS_OFF],
    browse: [
      { label: "Deployments", frame: "D19", route: "deployments", icon: "s-deploy" },
      { label: "Shell", frame: "D20", route: "shell", icon: "s-term" },
    ],
    manage: [
      { label: "Domains", frame: "D21", route: "domains", count: "2", icon: "s-globe" },
      { label: "Scaling", frame: "D22", route: "scaling", icon: "s-chart" },
      { label: "Bindings", frame: "D11", route: "bindings", count: "3", icon: "s-bind" },
      { label: "Settings", frame: "D23", route: "settings", icon: "s-gear" },
    ],
  },
  worker: {
    top: [{ ...METRICS, route: undefined, frame: "D9 grammar" }, LOGS_OFF],
    browse: [
      { label: "Deployments", frame: "D19", route: "deployments", icon: "s-deploy" },
      { label: "Shell", frame: "D20", route: "shell", icon: "s-term" },
    ],
    manage: [
      { label: "Schedules", frame: "S5", route: "schedules", count: "2", icon: "s-cron" },
      { label: "Scaling", frame: "D22", route: "scaling", icon: "s-chart" },
      { label: "Bindings", frame: "D11", route: "bindings", count: "3", icon: "s-bind" },
      { label: "Settings", frame: "D23", route: "settings", icon: "s-gear" },
    ],
  },
  "gpu-worker": {
    top: [],
    browse: [{ label: "Shell", frame: "D20", route: "shell", icon: "s-term" }],
    manage: [{ label: "Settings", frame: "D23", route: "settings", icon: "s-gear" }],
  },
  // X1: a flat capability workspace — routes/models, not instances; no
  // Metrics/Logs pair and no Browse/Manage split in the frame.
  "ai-gateway": {
    top: [
      { label: "Models", frame: "X1", route: "models", count: "4", icon: "s-ai" },
      { label: "Routes", frame: "X1", route: "routes", count: "3", icon: "s-branch" },
      { label: "Usage & cost", frame: "X1", route: "usage", icon: "s-card" },
      { label: "Policies", frame: "X1", route: "policies", icon: "s-shield" },
      { label: "Keys & bindings", frame: "X1", route: "keys", icon: "s-key" },
    ],
    browse: [],
    manage: [{ label: "Settings", frame: "X1", route: "settings", icon: "s-gear" }],
  },
};

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
  if (item.route) {
    return (
      <NitLink
        to={`/$org/$project/svc/$service/${item.route}`}
        params={linkParams}
        search={search}
        icon={item.icon}
        label={item.label}
        count={item.count}
        badge={item.badge}
        on={active === item.route && !(item.route === "messages" && item.label === "Dead letters")}
      />
    );
  }
  return (
    <NitDisabled
      icon={item.icon}
      label={item.label}
      count={item.count}
      reason={`${item.label} (${item.frame}) — no dedicated frame in the gallery; lands with per-product telemetry`}
    />
  );
}

export function SnavProduct({
  org,
  project,
  env,
  service,
  siblings,
  projectTotalCents,
  active,
}: {
  org: string;
  project: string;
  env: string;
  service: Service;
  /** All instances of this product in the env — SW3 appears only when n>1. */
  siblings: Service[];
  projectTotalCents?: number;
  active: string;
}) {
  const navigate = useNavigate();
  const matrix = MATRIX[service.product];
  const linkParams = { org, project, service: service.name };
  const search = { env };
  const usageBased = service.product === "storage";
  const costLabel = usageBased
    ? `${fmtMoney(914)} mtd`
    : fmtMoneyPerMonth(service.monthly_estimate_cents ?? 0);

  const header = (
    <div className="snhead">
      <span className="glyph">
        <Icon id={PRODUCT_ICON[service.product]} />
      </span>
      <div>
        <div className="t">{PRODUCT_LABEL[service.product]}</div>
        <div className="u">
          {service.product === "ai-gateway"
            ? "capability · one endpoint"
            : `${service.name}${siblings.length > 1 ? " ▾" : ""} · ${env}`}
        </div>
      </div>
    </div>
  );

  return (
    <Snav>
      {siblings.length > 1 ? (
        // SW3 — the instance switcher: appears only when n>1; switching keeps the page.
        <DropdownMenu.Root>
          <DropdownMenu.Trigger asChild>
            <button type="button" className="w-full cursor-pointer text-left">
              {header}
            </button>
          </DropdownMenu.Trigger>
          <DropdownMenu.Portal>
            <DropdownMenu.Content
              className={cn(menuContentClass, "min-w-[260px]")}
              align="start"
              sideOffset={4}
            >
              <div className="eyebrow px-3 py-2">
                {PRODUCT_LABEL[service.product]} in {project} · {siblings.length}
              </div>
              <DropdownMenu.Item
                className={menuItemClass}
                onSelect={() =>
                  navigate({
                    to: "/$org/$project/instances/$product",
                    params: { org, project, product: service.product },
                    search,
                  })
                }
              >
                <Icon id="s-grid" className="h-3.5 w-3.5 text-ink3" />
                All instances
                <span className="ml-auto text-10p5 text-ink3">{siblings.length}</span>
              </DropdownMenu.Item>
              {siblings.map((s) => (
                <DropdownMenu.Item
                  key={s.id}
                  className={menuItemClass}
                  onSelect={() =>
                    navigate({
                      to: `/$org/$project/svc/$service/${active === "overview" ? "" : active}`,
                      params: { org, project, service: s.name },
                      search,
                    })
                  }
                >
                  <Dot tone={statusDotTone(s.status)} />
                  <b>{s.name}</b>
                  <span className="mono ml-auto text-10p5 text-ink3">
                    {s.name === "db-main"
                      ? "$58/mo · Standard · 192/200 conns"
                      : s.name === "db-reports"
                        ? "$24/mo · Dev · fresh"
                        : fmtMoneyPerMonth(s.monthly_estimate_cents ?? 0)}
                  </span>
                  {s.id === service.id && <Icon id="s-check" className="h-3.5 w-3.5 text-steel" />}
                </DropdownMenu.Item>
              ))}
              <DropdownMenu.Separator className="my-1 h-px bg-hair" />
              <DropdownMenu.Item
                className={cn(menuItemClass, "text-steel")}
                onSelect={() =>
                  navigate({
                    to: "/$org/create",
                    params: { org },
                    // ai-gateway isn't creatable from the canvas; SW3 only renders at n>1.
                    search: {
                      env,
                      type:
                        service.product === "ai-gateway"
                          ? undefined
                          : (service.product as CreatableProduct),
                    },
                  })
                }
              >
                ⊕ New {PRODUCT_LABEL[service.product]}
              </DropdownMenu.Item>
              <div className="px-3 py-2 text-10p5 text-ink3">
                switching keeps the page — Metrics stays Metrics, scoped to the other instance
              </div>
            </DropdownMenu.Content>
          </DropdownMenu.Portal>
        </DropdownMenu.Root>
      ) : (
        header
      )}
      <NitLink
        to="/$org/$project/svc/$service"
        params={linkParams}
        search={search}
        icon="s-eye"
        label="Overview"
        on={active === "overview"}
      />
      {matrix.top.map((item) => (
        <MatrixNit
          key={item.label}
          item={item}
          linkParams={linkParams}
          search={search}
          active={active}
        />
      ))}
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
      {matrix.manage.length > 0 ? (
        <>
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
        </>
      ) : null}
      <Nfoot>
        <div className="flex justify-between px-1.5">
          <span>This service</span>
          <span className="mono">{costLabel}</span>
        </div>
        {service.product === "ai-gateway" ? (
          // X1: usage-priced capability — no project rollup line (adding it
          // to the $208 would misstate the canon arithmetic).
          <div className="px-1.5 pt-1">usage-priced · no instances to scale</div>
        ) : (
          <div className="flex justify-between px-1.5 pt-1">
            <span>{project} total</span>
            <span className="mono">
              {projectTotalCents !== undefined ? fmtMoneyPerMonth(projectTotalCents) : "…"}
            </span>
          </div>
        )}
      </Nfoot>
    </Snav>
  );
}
