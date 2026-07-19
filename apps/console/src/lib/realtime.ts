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

  const setMode = (m: RealtimeMode) => {
    if (mode !== m) {
      mode = m;
      opts.onModeChange?.(m);
    }
  };

  const deliver = (e: SpineEvent) => {
    if (e.id) cursor = e.id; // both paths advance the resume cursor
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
    source.onopen = () => setMode("sse");
    source.onmessage = (msg) => {
      try {
        deliver(JSON.parse(msg.data) as SpineEvent);
      } catch {
        /* a malformed frame never kills the channel */
      }
    };
    source.onerror = () => {
      // EventSource retries on transient blips itself; a hard failure (canon
      // mode, proxy without SSE) surfaces here — degrade to polling.
      source?.close();
      source = null;
      degrade();
    };
  };

  const degrade = () => {
    if (closed || mode === "poll") return;
    setMode("poll");
    pollDelay = POLL_START_MS;
    schedulePoll();
    // keep trying to climb back to SSE (recovery, not a hot loop)
    sseRetryTimer = setTimer(() => {
      if (closed || mode === "sse") return;
      stopPolling();
      startSSE();
      // if SSE fails again, onerror re-degrades immediately.
    }, SSE_RETRY_MS);
  };

  const stopPolling = () => {
    if (pollTimer) {
      clearTimer(pollTimer);
      pollTimer = null;
    }
  };

  const schedulePoll = () => {
    if (closed) return;
    pollTimer = setTimer(async () => {
      try {
        const page = await fetchPage(opts.env, cursor);
        for (const e of page.events) deliver(e);
        if (page.cursor) cursor = page.cursor;
      } catch {
        /* a failed poll just backs off — never throws out of the channel */
      }
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
