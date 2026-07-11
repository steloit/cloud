import { Icon, type IconId } from "@/design-system/icon";
import { Dot } from "@/design-system/pill";
import { Nfoot, NitDisabled, NitLink, Snav } from "./snav";

/**
 * Snav variant for the Observe suite (O1–O6): one workspace, six lenses over
 * the same time context. Dashboards (O-dash) land in Phase 3 — visible but
 * disabled with the reason, never hidden.
 */

export type ObserveLens = "health" | "metrics" | "logs" | "traces" | "events" | "alerts";

const ITEMS: Array<{ lens: ObserveLens; label: string; icon: IconId; badge?: string }> = [
  { lens: "health", label: "Health", icon: "s-pulse" },
  { lens: "metrics", label: "Metrics", icon: "s-chart" },
  { lens: "logs", label: "Logs", icon: "s-term" },
  { lens: "traces", label: "Traces", icon: "s-branch" },
  { lens: "events", label: "Events", icon: "s-cron" },
  { lens: "alerts", label: "Alerts", icon: "s-bell", badge: "1" },
];

export function SnavObserve({
  org,
  project,
  env,
  active,
}: {
  org: string;
  project: string;
  env: string;
  active: ObserveLens;
}) {
  const params = { org, project };
  const search = { env };

  return (
    <Snav>
      <div className="snhead">
        <span className="glyph">
          <Icon id="s-pulse" />
        </span>
        <div>
          <div className="t">Observe</div>
          <div className="u">{project} · all services</div>
        </div>
      </div>
      {ITEMS.map((item) => (
        <NitLink
          key={item.lens}
          to={`/$org/$project/observe/${item.lens}`}
          params={params}
          search={search}
          icon={item.icon}
          label={item.label}
          badge={item.badge}
          on={active === item.lens}
        />
      ))}
      <NitDisabled icon="s-grid" label="Dashboards" count="3" reason="Dashboards land in Phase 3" />
      <Nfoot>
        <div className="flex items-center justify-between px-1.5">
          <span className="flex items-center gap-1.5 text-warn">
            <Dot tone="warn" /> 1 firing
          </span>
          <span className="mono">{env}</span>
        </div>
        <div className="px-1.5 pt-1">retention: raw 15 d · downsampled 13 mo · logs 30 d</div>
      </Nfoot>
    </Snav>
  );
}
