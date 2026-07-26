/**
 * T8.2 — the state.md realtime contract: SSE-primary on the env event spine
 * (`/envs/{env}/events`, x-streamable), poll fallback at 2s backing off to
 * 10s, cursor resume on both paths, periodic SSE recovery attempts.
 *
 * Canon mode needs no special case: MSW cannot stream, so EventSource errors
 * and the fallback — the SAME endpoint's JSON list — carries the channel.
 * EventSource sends `Accept: text/event-stream` natively (08-api realtime
 * convention); the server's Streamer claims the request pre-strict.
 */

import { listEvents } from "@/lib/api/generated/sdk.gen";

export interface SpineEvent {
  id: string;
  kind: string;
  action?: string;
  actor?: string;
  subject?: string;
  at?: string;
  detail?: Record<string, unknown>;
}

export type RealtimeMode = "sse" | "poll" | "idle";

export interface RealtimeChannel {
  close: () => void;
  /** Current transport — surfaced for tests and (later) a status affordance. */
  mode: () => RealtimeMode;
}

interface ChannelOptions {
  env: string;
  onEvent: (e: SpineEvent) => void;
  onModeChange?: (mode: RealtimeMode) => void;
  /** Injectable seams — tests fake these; production uses the real ones. */
  eventSource?: (url: string) => EventSource;
  setTimer?: (fn: () => void, ms: number) => ReturnType<typeof setTimeout>;
  clearTimer?: (t: ReturnType<typeof setTimeout>) => void;
  fetchPage?: (env: string, cursor?: string) => Promise<{ events: SpineEvent[]; cursor?: string }>;
  baseUrl?: string;
}

/** state.md: poll starts at 2s and backs off to 10s. */
export const POLL_START_MS = 2_000;
export const POLL_MAX_MS = 10_000;
/** How often the fallback re-attempts SSE (recovery, not a hot loop). */
export const SSE_RETRY_MS = 30_000;

async function defaultFetchPage(
  env: string,
  cursor?: string,
): Promise<{ events: SpineEvent[]; cursor?: string }> {
  const res = await listEvents({
    path: { env },
    query: cursor ? { cursor } : undefined,
  });
  const data = (res.data?.data ?? []) as SpineEvent[];
  const next = (res.data as { next_cursor?: string | null } | undefined)?.next_cursor ?? undefined;
  return { events: data, cursor: next };
}

/**
 * One live channel per env. The stream drives toasts, the bell badge, and
 * status-pill flips — the channel itself only transports; consumers decide.
 */
export function createRealtimeChannel(opts: ChannelOptions): RealtimeChannel {
  const makeSource =
    opts.eventSource ?? ((url: string) => new EventSource(url, { withCredentials: true }));
  const setTimer = opts.setTimer ?? ((fn, ms) => setTimeout(fn, ms));
  const clearTimer = opts.clearTimer ?? ((t) => clearTimeout(t));
  const fetchPage = opts.fetchPage ?? defaultFetchPage;
  const base = (opts.baseUrl ?? "").replace(/\/+$/, "");

  let mode: RealtimeMode = "idle";
  let closed = false;
  let cursor: string | undefined;
  let source: EventSource | null = null;
  let pollDelay = POLL_START_MS;
  let pollTimer: ReturnType<typeof setTimeout> | null = null;
  let sseRetryTimer: ReturnType<typeof setTimeout> | null = null;
  /** true from the moment a poll loop starts until it is stopped — the honest
   *  "polling active" flag (review: pollTimer===null is ALSO true mid-fetch, so
   *  a degrade() in that window spawned a second orphan loop). */
  let polling = false;
  /** the poll path PRIMES on its first page (backlog is history, not live) */
  let primed = false;
  /** bounded dedup — the canon MSW handler ignores cursors and replays */
  const seen = new Set<string>();
  const SEEN_CAP = 500;

  const setMode = (m: RealtimeMode) => {
    if (mode !== m) {
      mode = m;
      opts.onModeChange?.(m);
    }
  };

  const deliver = (e: SpineEvent) => {
    if (e.id) {
      if (seen.has(e.id)) return; // replayed page (cursor-ignoring server) — drop
      seen.add(e.id);
      if (seen.size > SEEN_CAP) {
        const first = seen.values().next().value;
        if (first) seen.delete(first);
      }
      cursor = e.id; // both paths advance the resume cursor
    }
    opts.onEvent(e);
  };

  const startSSE = () => {
    if (closed) return;
    const url = `${base}/v1/envs/${opts.env}/events${cursor ? `?cursor=${encodeURIComponent(cursor)}` : ""}`;
    try {
      source = makeSource(url);
    } catch {
      degrade();
      return;
    }
    source.onopen = () => {
      stopPolling(); // recovered — SSE carries the channel again
      setMode("sse");
    };
    source.onmessage = (msg) => {
      try {
        deliver(JSON.parse(msg.data) as SpineEvent);
      } catch {
        /* a malformed frame never kills the channel */
      }
    };
    source.onerror = () => {
      // A hard failure (canon mode, a proxy without SSE, or a dropped stream)
      // surfaces here — close this source and fall back to polling. We do NOT
      // rely on EventSource's own reconnect: the poll path carries the cursor
      // and a 30s timer re-attempts SSE deliberately.
      source?.close();
      source = null;
      degrade();
    };
  };

  // degrade is safe to call from ANY state — including a failed recovery
  // attempt (review blocker: an early-return on mode==="poll" left the channel
  // permanently dead after one failed retry, which is canon mode's steady
  // state). The explicit `polling` flag makes it idempotent WITHOUT the
  // pollTimer===null proxy (which is also true mid-fetch → orphan loops).
  const degrade = () => {
    if (closed) return;
    setMode("poll");
    if (!polling) {
      polling = true;
      pollDelay = POLL_START_MS;
      schedulePoll();
    }
    // keep trying to climb back to SSE (recovery, not a hot loop)
    if (sseRetryTimer) clearTimer(sseRetryTimer);
    sseRetryTimer = setTimer(() => {
      if (closed || mode === "sse") return;
      startSSE(); // polling KEEPS RUNNING during the attempt; onopen stops it
    }, SSE_RETRY_MS);
  };

  const stopPolling = () => {
    polling = false;
    if (pollTimer) {
      clearTimer(pollTimer);
      pollTimer = null;
    }
  };

  const schedulePoll = () => {
    if (closed || !polling) return;
    pollTimer = setTimer(async () => {
      pollTimer = null;
      try {
        const page = await fetchPage(opts.env, cursor);
        if (closed || !polling) return; // stopped (close/recovery) mid-flight: drop
        // prime ONLY the very first page fetched from a cold cursor — a poll
        // that starts AFTER SSE already delivered events (cursor set) must not
        // swallow genuinely new rows (review: live-mode post-SSE gap).
        if (!primed && cursor === undefined) {
          // FIRST page is backlog, not live traffic: prime the cursor + dedup
          // set silently (review: replaying history as toasts is a defect).
          primed = true;
          for (const e of page.events) {
            if (e.id) {
              seen.add(e.id);
              cursor = e.id;
            }
          }
        } else {
          for (const e of page.events) deliver(e);
        }
        if (page.cursor) cursor = page.cursor;
      } catch {
        /* a failed poll just backs off — never throws out of the channel */
      }
      if (closed || !polling) return; // recovered/closed during the fetch
      // state.md backoff: 2s → 10s (and stay there)
      pollDelay = Math.min(pollDelay * 2, POLL_MAX_MS);
      schedulePoll();
    }, pollDelay);
  };

  startSSE();

  return {
    close: () => {
      closed = true;
      source?.close();
      source = null;
      stopPolling();
      if (sseRetryTimer) clearTimer(sseRetryTimer);
      setMode("idle");
    },
    mode: () => mode,
  };
}
