import { Link } from "@tanstack/react-router";
import { Btn } from "@/design-system/btn";
import { Icon } from "@/design-system/icon";
import { Dot, Pill } from "@/design-system/pill";
import { cn } from "@/lib/utils";
import {
  isUnread,
  NEEDS_ACTION_COUNT,
  NOTIFICATIONS,
  type NotificationRow,
  useNotificationsStore,
} from "./data";

/**
 * N1 · Bell panel + N3 · Caught-up state — the popover behind the ctx bar's
 * bell. Read state is local (no notifications endpoint — see data.ts).
 */

/** Live unread count for the orchestrator's bell dot. */
export function useUnreadCount(): number {
  const readIds = useNotificationsStore((s) => s.readIds);
  return NOTIFICATIONS.filter((n) => isUnread(n, readIds)).length;
}

/** insights?proposal= — extra key rides the URL past the env-only schema. */
const PROPOSAL_SEARCH = { env: "production", proposal: "prp_7c31a2" };

function ToneDot({ tone }: { tone: NotificationRow["tone"] }) {
  if (tone === "none") return <span className="dot" style={{ background: "var(--hair)" }} />;
  if (tone === "ai") return <span className="dot" style={{ background: "var(--assist)" }} />;
  return <Dot tone={tone} />;
}

function Row({ row, org, unread }: { row: NotificationRow; org: string; unread: boolean }) {
  return (
    <div className="flex gap-2.5 px-4 py-2.5 hover:bg-surface2">
      <span className="pt-1">
        <ToneDot tone={row.tone} />
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="text-[12px] font-medium leading-snug">{row.title}</span>
        <span className="mono truncate text-[10.5px] text-ink3">{row.meta}</span>
        {row.meta2 ? (
          <span className="mono truncate text-[10.5px] text-ink3">{row.meta2}</span>
        ) : null}
        {row.id === "n-approval" ? (
          <span className="mt-1.5 flex gap-2">
            <Link to="/$org/settings/policies" params={{ org }}>
              <Btn variant="s" className="h-6 px-2 text-[10.5px]">
                Review request
              </Btn>
            </Link>
            <Btn
              variant="gh"
              className="h-6 px-2 text-[10.5px]"
              disabled
              disabledReason="Approval threads land in Phase 5"
            >
              Deny…
            </Btn>
          </span>
        ) : null}
        {row.id === "n-proposal" ? (
          <span className="mt-1.5 flex gap-2">
            <Link
              to="/$org/$project/svc/$service/insights"
              params={{ org, project: "ecommerce", service: "db-main" }}
              search={PROPOSAL_SEARCH}
            >
              <Btn variant="a" className="h-6 px-2 text-[10.5px]">
                Review proposal
              </Btn>
            </Link>
          </span>
        ) : null}
      </div>
      {unread ? <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-steel" /> : null}
    </div>
  );
}

function CaughtUp({ org }: { org: string }) {
  return (
    <div className="flex flex-col items-center gap-3 px-6 py-8 text-center">
      <span className="flex h-[30px] w-[30px] items-center justify-center rounded-lg bg-ok-tint">
        <Icon id="s-check" className="h-[15px] w-[15px] text-ok" />
      </span>
      <div className="text-[13px] font-semibold">You're caught up</div>
      <p className="text-[11.5px] leading-relaxed text-ink3">
        Nothing needs you. Alerts, assistant proposals, approval requests and lifecycle events
        (ready · failed · renewed) will land here — the moment one does, the bell gets its dot back.
      </p>
      <div className="flex gap-2">
        <Link
          to="/$org/$project/observe/alerts"
          params={{ org, project: "ecommerce" }}
          search={{ env: "production" }}
        >
          <Btn variant="s" className="h-7 px-2.5 text-[11px]">
            Review alert rules
          </Btn>
        </Link>
        <Link to="/account/notifications">
          <Btn variant="s" className="h-7 px-2.5 text-[11px]">
            Delivery settings
          </Btn>
        </Link>
      </div>
      <div className="mt-1 flex w-full items-center gap-2 border-hair border-t pt-3 text-[10.5px] text-ink3">
        <span>Silence ≠ health — that's what Observe is for</span>
        <span className="ml-auto mono">history kept 90 d</span>
      </div>
    </div>
  );
}

export function BellPanel({
  open,
  onClose,
  org,
}: {
  open: boolean;
  onClose: () => void;
  org: string;
}) {
  const readIds = useNotificationsStore((s) => s.readIds);
  const markAllRead = useNotificationsStore((s) => s.markAllRead);
  const unread = NOTIFICATIONS.filter((n) => isUnread(n, readIds)).length;

  if (!open) return null;

  const needsAction = NOTIFICATIONS.filter((n) => n.group === "needs-action");
  const today = NOTIFICATIONS.filter((n) => n.group === "today");
  const yesterday = NOTIFICATIONS.filter((n) => n.group === "yesterday");

  const section = (label: string, rows: NotificationRow[], warn?: boolean) => (
    <div className="border-hair border-t">
      <div className={cn("eyebrow px-4 pt-2.5 pb-0.5", warn && "text-warn")}>{label}</div>
      {rows.map((r) => (
        <Row key={r.id} row={r} org={org} unread={isUnread(r, readIds)} />
      ))}
    </div>
  );

  return (
    <>
      {/* Click-away scrim — a real button so it stays keyboard-operable. */}
      <button
        type="button"
        className="fixed inset-0 z-40 cursor-default border-none bg-transparent p-0"
        aria-label="Close notifications"
        onClick={onClose}
      />
      <div
        className="card fixed top-[54px] right-4 z-50 flex w-[404px] flex-col overflow-hidden shadow-e2"
        role="dialog"
        aria-label="Notifications"
      >
        <div className="flex items-center gap-2 px-4 py-3">
          <span className="text-[13px] font-semibold">Notifications</span>
          {unread > 0 ? <Pill tone="warn">{unread} unread</Pill> : null}
          <span className="flex-1" />
          {unread > 0 ? (
            <Btn variant="gh" className="h-6 px-2 text-[10.5px]" onClick={markAllRead}>
              Mark all read
            </Btn>
          ) : null}
          <Link
            to="/account/notifications"
            className="icb h-6 w-6"
            aria-label="Notification settings"
            onClick={onClose}
          >
            <Icon id="s-gear" className="!h-3.5 !w-3.5" />
          </Link>
        </div>

        {unread === 0 ? (
          <CaughtUp org={org} />
        ) : (
          <>
            <div className="max-h-[420px] overflow-y-auto">
              {section(`Needs action · ${NEEDS_ACTION_COUNT}`, needsAction, true)}
              {section("Today", today)}
              {section("Yesterday", yesterday)}
            </div>
            <div className="flex items-center gap-2 border-hair border-t px-4 py-2.5">
              <span className="text-[10.5px] text-ink3">↑↓ move · ↵ open · e mark read</span>
              <span className="flex-1" />
              <Link
                to="/$org/notifications"
                params={{ org }}
                className="text-[11px] font-medium text-steel"
                onClick={onClose}
              >
                View all →
              </Link>
            </div>
          </>
        )}
      </div>
    </>
  );
}
