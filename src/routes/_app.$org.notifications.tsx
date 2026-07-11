import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { MetricChart } from "@/design-system/chart";
import { Icon } from "@/design-system/icon";
import { Dot, Pill } from "@/design-system/pill";
import {
  isUnread,
  NEEDS_ACTION_COUNT,
  NOTIFICATIONS,
  type NotificationRow,
  type NotificationType,
  useNotificationsStore,
} from "@/features/notifications/data";
import { toMarkers, toSeries, useMetrics } from "@/features/observe/hooks";
import { cn } from "@/lib/utils";

/**
 * N2 · Notifications inbox — everything that asked for you, one inbox across
 * the org. This route is not in 02-ia's route list — minimal addition so the
 * bell's "View all →" has somewhere to land (finding).
 */

type Tab = "all" | "unread" | "needs-action";
type TypeChip = "all" | NotificationType;

const TYPE_CHIPS: Array<{ id: TypeChip; label: string }> = [
  { id: "all", label: "All types" },
  { id: "alert", label: "Alerts" },
  { id: "proposal", label: "Proposals" },
  { id: "approval", label: "Approvals" },
  { id: "deploy", label: "Deploys" },
  { id: "lifecycle", label: "Lifecycle" },
];

/** insights?proposal= — extra key rides the URL past the env-only schema. */
const PROPOSAL_SEARCH = { env: "production", proposal: "prp_7c31a2" };

function ToneDot({ tone }: { tone: NotificationRow["tone"] }) {
  return <Dot tone={tone} />;
}

/** The generic detail's "relevant link" per row (the p95 alert gets the full pane). */
function GenericDetail({ row, org }: { row: NotificationRow; org: string }) {
  const link = (() => {
    switch (row.id) {
      case "n-approval":
        return (
          <Link to="/$org/settings/policies" params={{ org }}>
            <Btn variant="s">Review request</Btn>
          </Link>
        );
      case "n-proposal":
        return (
          <Link
            to="/$org/$project/svc/$service/insights"
            params={{ org, project: "ecommerce", service: "db-main" }}
            search={PROPOSAL_SEARCH}
          >
            <Btn variant="a">View proposal</Btn>
          </Link>
        );
      case "n-dlq":
        return (
          <Link
            to="/$org/$project/observe/health"
            params={{ org, project: "ecommerce" }}
            search={{ env: "production" }}
          >
            <Btn variant="s">Open in Observe</Btn>
          </Link>
        );
      case "n-dbreports":
        return (
          <Link
            to="/$org/$project/svc/$service"
            params={{ org, project: "ecommerce", service: "db-reports" }}
            search={{ env: "production" }}
          >
            <Btn variant="s">Open service</Btn>
          </Link>
        );
      case "n-dep142":
        return (
          <Link
            to="/$org/$project/deploy"
            params={{ org, project: "ecommerce" }}
            search={{ env: "production" }}
          >
            <Btn variant="s">Open deploys</Btn>
          </Link>
        );
      default:
        return (
          <Link
            to="/$org/$project/observe/events"
            params={{ org, project: "ecommerce" }}
            search={{ env: "production" }}
          >
            <Btn variant="s">Open events</Btn>
          </Link>
        );
    }
  })();

  return (
    <Card className="flex flex-1 flex-col gap-3 p-5">
      <div className="flex items-center gap-2.5">
        <ToneDot tone={row.tone} />
        <span className="text-14 font-semibold">{row.title}</span>
      </div>
      <div className="mono text-11 text-ink3">{row.meta}</div>
      {row.meta2 ? <div className="mono text-11 text-ink3">{row.meta2}</div> : null}
      <div>{link}</div>
    </Card>
  );
}

function P95Detail({ org }: { org: string }) {
  const p95 = useMetrics("production", "service:api metric:p95");
  return (
    <Card className="flex flex-1 flex-col gap-3.5 overflow-y-auto p-5">
      <div className="flex items-center gap-2.5">
        <Dot tone="warn" />
        <span className="text-14 font-semibold">api p95 crossed 800 ms</span>
        <span className="flex-1" />
        <Pill tone="warn">still firing · 41 min</Pill>
      </div>
      <div className="mono text-11 text-ink3">
        alert p95-slo · ecommerce / production · fired 14:02 · evt_9d21c4
      </div>
      <p className="text-12 leading-relaxed text-ink2">
        p95 latency reached 812 ms against a threshold of 800 ms, four minutes after deploy #142.
        The assistant traced the regression to an unindexed query on{" "}
        <span className="mono">orders</span> and drafted proposal{" "}
        <span className="mono">prp_7c31a2</span>.
      </p>
      <MetricChart
        series={toSeries(p95.data)}
        threshold={800}
        markers={toMarkers(p95.data)}
        unit="ms"
        tone="warn"
        size="lg"
      />
      <div className="flex flex-wrap items-center gap-1.5">
        <span className="chip">deploy #142</span>
        <span className="chip">slow query · 642 ms</span>
        <span className="chip">prp_7c31a2</span>
        <span className="chip">jobs · 2 dead letters</span>
        <span className="text-10p5 text-ink3">correlated on the shared timeline</span>
      </div>
      <div className="flex flex-wrap gap-2">
        <Link
          to="/$org/$project/observe/health"
          params={{ org, project: "ecommerce" }}
          search={{ env: "production" }}
        >
          <Btn variant="p">Open in Observe</Btn>
        </Link>
        <Link
          to="/$org/$project/svc/$service/insights"
          params={{ org, project: "ecommerce", service: "db-main" }}
          search={PROPOSAL_SEARCH}
        >
          <Btn variant="a">View proposal</Btn>
        </Link>
        <Btn
          variant="s"
          disabled
          disabledReason="Silences land in Phase 5 — every silence is logged with actor + duration"
        >
          Silence 24 h…
        </Btn>
        <Link
          to="/$org/$project/observe/alerts"
          params={{ org, project: "ecommerce" }}
          search={{ env: "production" }}
        >
          <Btn variant="s">Alert settings</Btn>
        </Link>
      </div>
      <div className="mt-auto flex flex-col gap-1 border-hair border-t pt-3 text-10p5 text-ink3">
        <span>Delivered: in-app · email priya@acme.dev</span>
        <span>Escalation: pages on-call if unacked 15 min (production policy)</span>
        <span>Silences are logged with actor + duration</span>
      </div>
    </Card>
  );
}

function NotificationsInbox() {
  const { org } = Route.useParams();
  const readIds = useNotificationsStore((s) => s.readIds);
  const markAllRead = useNotificationsStore((s) => s.markAllRead);

  const [tab, setTab] = useState<Tab>("all");
  const [typeChip, setTypeChip] = useState<TypeChip>("all");
  const [selectedId, setSelectedId] = useState("n-p95");

  const unreadCount = NOTIFICATIONS.filter((n) => isUnread(n, readIds)).length;

  const filtered = NOTIFICATIONS.filter((n) => {
    if (tab === "unread" && !isUnread(n, readIds)) return false;
    if (tab === "needs-action" && n.group !== "needs-action") return false;
    if (typeChip !== "all" && n.type !== typeChip) return false;
    return true;
  });
  const byOrd = (a: NotificationRow, b: NotificationRow) => b.ord - a.ord;
  const today = filtered.filter((n) => n.day === "today").sort(byOrd);
  const yesterday = filtered.filter((n) => n.day === "yesterday").sort(byOrd);
  const selected = NOTIFICATIONS.find((n) => n.id === selectedId);

  const dayGroup = (label: string, rows: NotificationRow[]) =>
    rows.length === 0 ? null : (
      <div key={label}>
        <div className="eyebrow px-3.5 pt-3 pb-1">{label}</div>
        {rows.map((r) => (
          <button
            type="button"
            key={r.id}
            onClick={() => setSelectedId(r.id)}
            className={cn(
              "flex w-full gap-2.5 px-3.5 py-2.5 text-left hover:bg-surface2",
              r.id === selectedId && "bg-steel-tint",
            )}
          >
            <span className="pt-1">
              <ToneDot tone={r.tone} />
            </span>
            <span className="flex min-w-0 flex-1 flex-col gap-0.5">
              <span className="text-12 font-medium leading-snug">{r.title}</span>
              <span className="mono truncate text-10p5 text-ink3">{r.meta}</span>
            </span>
            {isUnread(r, readIds) ? (
              <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-steel" />
            ) : null}
          </button>
        ))}
      </div>
    );

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          title="Notifications"
          sub="Everything that asked for you — alerts, proposals, approvals and lifecycle events, one inbox across the org"
        >
          <Btn variant="s" onClick={markAllRead}>
            Mark all read
          </Btn>
          <Link to="/account/notifications">
            <Btn variant="s">Notification settings ↗</Btn>
          </Link>
        </Pghead>

        <div className="tabrow">
          <button
            type="button"
            className={cn("tab", tab === "all" && "on")}
            onClick={() => setTab("all")}
          >
            All
          </button>
          <button
            type="button"
            className={cn("tab", tab === "unread" && "on")}
            onClick={() => setTab("unread")}
          >
            Unread · {unreadCount}
          </button>
          <button
            type="button"
            className={cn("tab", tab === "needs-action" && "on")}
            style={{ color: "var(--warn)" }}
            onClick={() => setTab("needs-action")}
          >
            Needs action · {NEEDS_ACTION_COUNT}
          </button>
        </div>

        <div className="flex items-center gap-1.5">
          {TYPE_CHIPS.map((c) => (
            <button
              type="button"
              key={c.id}
              className={cn("chip", typeChip === c.id && "on")}
              onClick={() => setTypeChip(c.id)}
            >
              {c.label}
            </button>
          ))}
          <span className="ml-auto flex gap-1.5">
            <span className="envpill">
              Project: all <Icon id="s-chevd" className="h-[11px] w-[11px]" />
            </span>
            <span className="envpill">
              Env: all <Icon id="s-chevd" className="h-[11px] w-[11px]" />
            </span>
          </span>
        </div>

        <div className="flex min-h-0 flex-1 gap-3.5">
          <Card className="flex w-[400px] shrink-0 flex-col overflow-hidden">
            <div className="flex-1 overflow-y-auto">
              {dayGroup("Today", today)}
              {dayGroup("Yesterday", yesterday)}
            </div>
            <div className="border-hair border-t px-3.5 py-2 text-10p5 text-ink3">
              j/k move · e read · s silence · retained 90 d
            </div>
          </Card>
          {selected?.id === "n-p95" ? (
            <P95Detail org={org} />
          ) : selected ? (
            <GenericDetail row={selected} org={org} />
          ) : null}
        </div>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/notifications")({
  component: NotificationsInbox,
});
