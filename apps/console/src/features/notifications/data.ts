import { create } from "zustand";

/**
 * N-series notification rows — frame-fixed canon.
 *
 * The API has no notifications endpoint (08-api/openapi.yaml lists none);
 * state.md says the event stream drives the bell. These rows are the N1/N2
 * frames' canon verbatim, kept here as fixtures — surfaced as a finding.
 */

export type NotificationType = "alert" | "proposal" | "approval" | "deploy" | "lifecycle";
export type NotificationTone = "warn" | "ai" | "ok" | "none";

export interface NotificationRow {
  id: string;
  /** Bell section (N1): needs-action rows float above the day groups. */
  group: "needs-action" | "today" | "yesterday";
  /** Inbox day group (N2): every row lands in Today or Yesterday. */
  day: "today" | "yesterday";
  type: NotificationType;
  tone: NotificationTone;
  title: string;
  /** Mono meta line, verbatim from the frame. */
  meta: string;
  /** Optional second mono line (the proposal row carries its SQL). */
  meta2?: string;
  unread?: boolean;
  /** Descending order within a day group (frame order). */
  ord: number;
}

export const NOTIFICATIONS: NotificationRow[] = [
  // Needs action · 2
  {
    id: "n-approval",
    group: "needs-action",
    day: "yesterday",
    type: "approval",
    tone: "warn",
    title: "Review approval: marco requests public CDN on assets",
    meta: "org policy public-buckets · yesterday 16:40",
    unread: true,
    ord: 100,
  },
  {
    id: "n-proposal",
    group: "needs-action",
    day: "today",
    type: "proposal",
    tone: "ai",
    title: "Assistant proposal ready — index for the slow query",
    meta: "CREATE INDEX CONCURRENTLY idx_orders_customer_created …",
    meta2: "prp_7c31a2 · ecommerce / production · 14:04",
    unread: true,
    ord: 104,
  },
  // Today
  {
    id: "n-p95",
    group: "today",
    day: "today",
    type: "alert",
    tone: "warn",
    title: "api p95 crossed 800 ms — 4 min after deploy #142",
    meta: "alert p95-slo · ecommerce / production · 14:02",
    unread: true,
    ord: 102,
  },
  {
    id: "n-dlq",
    group: "today",
    day: "today",
    type: "alert",
    tone: "warn",
    title: "jobs: 2 messages dead-lettered",
    meta: "order.paid · TypeError receiptUrl · 14:03",
    unread: true,
    ord: 103,
  },
  {
    id: "n-dbreports",
    group: "today",
    day: "today",
    type: "lifecycle",
    tone: "ok",
    title: "db-reports is ready — binding activated, worker restarted",
    meta: "provisioned in 38 s · billing started at ready · 11:20",
    ord: 90,
  },
  {
    id: "n-dep142",
    group: "today",
    day: "today",
    type: "deploy",
    tone: "none",
    title: "Deploy #142 to production by priya",
    meta: "api · gift cards · 13:58",
    ord: 101,
  },
  // Yesterday
  {
    id: "n-cert",
    group: "yesterday",
    day: "yesterday",
    type: "lifecycle",
    tone: "ok",
    title: "Certificate renewed — api.acme-store.com · 90 d",
    meta: "automatic · no action needed",
    ord: 50,
  },
];

// ---------------------------------------------------------------------------
// Local read state — no endpoint to persist to, so "read" lives client-side.
// ---------------------------------------------------------------------------

interface NotificationsState {
  readIds: Record<string, true>;
  /** Realtime badge bumps from the event spine (T8.2). */
  liveUnread: number;
  markRead: (id: string) => void;
  markAllRead: () => void;
  bumpLive: () => void;
  clearLive: () => void;
}

export const useNotificationsStore = create<NotificationsState>()((set) => ({
  readIds: {},
  liveUnread: 0,
  markRead: (id) => set((s) => ({ readIds: { ...s.readIds, [id]: true } })),
  markAllRead: () =>
    set(() => ({
      liveUnread: 0,
      readIds: Object.fromEntries(
        NOTIFICATIONS.filter((n) => n.unread).map((n) => [n.id, true as const]),
      ),
    })),
  /** T8.2: the spine stream bumps the badge live; opening the bell clears it. */
  bumpLive: () => set((s) => ({ liveUnread: s.liveUnread + 1 })),
  clearLive: () => set({ liveUnread: 0 }),
}));

export function isUnread(row: NotificationRow, readIds: Record<string, true>): boolean {
  return Boolean(row.unread) && !readIds[row.id];
}

export const NEEDS_ACTION_COUNT = NOTIFICATIONS.filter((n) => n.group === "needs-action").length;
