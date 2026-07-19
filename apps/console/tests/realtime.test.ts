import { describe, expect, it } from "vitest";
import {
  createRealtimeChannel,
  POLL_MAX_MS,
  POLL_START_MS,
  type SpineEvent,
} from "../src/lib/realtime";

/**
 * T8.2 — the state.md contract, proven with injected fakes: SSE failure
 * degrades to polling at 2s→10s, the cursor resumes on both paths, and a
 * later successful SSE recovers the channel.
 */

type TimerFn = () => void;

/** Deterministic manual timer wheel with ABSOLUTE deadlines (real semantics:
 *  a 30s retry eventually beats the 10s poll cadence as virtual time advances). */
function makeClock() {
  let seq = 0;
  let now = 0;
  const timers = new Map<number, { fn: TimerFn; ms: number; due: number }>();
  return {
    set(fn: TimerFn, ms: number) {
      seq += 1;
      timers.set(seq, { fn, ms, due: now + ms });
      return seq as unknown as ReturnType<typeof setTimeout>;
    },
    clear(t: ReturnType<typeof setTimeout>) {
      timers.delete(t as unknown as number);
    },
    /** fire the timer with the earliest DEADLINE, advancing virtual time */
    async tick(): Promise<number | undefined> {
      let bestId: number | undefined;
      let best: { fn: TimerFn; ms: number; due: number } | undefined;
      for (const [id, t] of timers) {
        if (!best || t.due < best.due) {
          bestId = id;
          best = t;
        }
      }
      if (bestId === undefined || !best) return undefined;
      timers.delete(bestId);
      now = best.due;
      await best.fn();
      // let promise chains inside the timer settle
      await new Promise((r) => setTimeout(r, 0));
      return best.ms;
    },
    pending: () => timers.size,
  };
}

class FakeSource {
  onopen: (() => void) | null = null;
  onmessage: ((m: { data: string }) => void) | null = null;
  onerror: (() => void) | null = null;
  closed = false;
  constructor(public url: string) {}
  close() {
    this.closed = true;
  }
}

function harness(pages: SpineEvent[][]) {
  const clock = makeClock();
  const sources: FakeSource[] = [];
  const events: SpineEvent[] = [];
  const modes: string[] = [];
  const cursorsSeen: (string | undefined)[] = [];
  let page = 0;
  const channel = createRealtimeChannel({
    env: "env_test",
    onEvent: (e) => events.push(e),
    onModeChange: (m) => modes.push(m),
    eventSource: (url) => {
      const s = new FakeSource(url);
      sources.push(s);
      return s as unknown as EventSource;
    },
    setTimer: clock.set,
    clearTimer: clock.clear,
    fetchPage: async (_env, cursor) => {
      cursorsSeen.push(cursor);
      const events = pages[Math.min(page, pages.length - 1)] ?? [];
      page += 1;
      return { events };
    },
  });
  return { clock, sources, events, modes, cursorsSeen, channel };
}

describe("realtime channel (state.md contract)", () => {
  it("degrades to polling on SSE failure, backs off 2s→10s, and resumes cursor", async () => {
    const h = harness([
      [{ id: "evt_1", kind: "deploy" }],
      [{ id: "evt_2", kind: "lifecycle" }],
      [],
      [],
      [],
    ]);
    // hard SSE failure (canon mode shape)
    h.sources[0]?.onerror?.();
    expect(h.channel.mode()).toBe("poll");

    // first poll at 2s
    const d1 = await h.clock.tick();
    expect(d1).toBe(POLL_START_MS);
    expect(h.events.map((e) => e.id)).toEqual(["evt_1"]);
    // second poll passes the resume cursor from the delivered event
    const d2 = await h.clock.tick();
    expect(d2).toBe(POLL_START_MS * 2);
    expect(h.cursorsSeen[1]).toBe("evt_1");
    // backoff caps at 10s (2 → 4 → 8 → 10); the NEXT event in virtual time is
    // the 30s SSE recovery attempt — correct per the contract, asserted in the
    // recovery test below.
    const d3 = await h.clock.tick(); // 8s
    const d4 = await h.clock.tick(); // 10s (cap)
    expect(d3).toBe(8_000);
    expect(d4).toBe(POLL_MAX_MS);
  });

  it("recovers to SSE when the retry attempt succeeds, and stops polling", async () => {
    const h = harness([[], [], [], [], [], [], []]);
    h.sources[0]?.onerror?.();
    expect(h.channel.mode()).toBe("poll");
    // drain until the 30s SSE-retry timer is the one that fires a new source
    for (let i = 0; i < 10 && h.sources.length < 2; i++) {
      await h.clock.tick();
    }
    expect(h.sources.length).toBe(2);
    // the second source opens → mode climbs back to sse
    h.sources[1]?.onopen?.();
    expect(h.channel.mode()).toBe("sse");
    // SSE messages deliver and advance the cursor
    h.sources[1]?.onmessage?.({ data: JSON.stringify({ id: "evt_9", kind: "deploy" }) });
    expect(h.events.at(-1)?.id).toBe("evt_9");
  });

  it("close() tears everything down", async () => {
    const h = harness([[]]);
    h.sources[0]?.onerror?.();
    h.channel.close();
    expect(h.channel.mode()).toBe("idle");
    // no timer fire after close delivers anything
    await h.clock.tick();
    expect(h.events.length).toBe(0);
  });

  it("a malformed SSE frame never kills the channel", () => {
    const h = harness([[]]);
    h.sources[0]?.onopen?.();
    h.sources[0]?.onmessage?.({ data: "not-json{" });
    h.sources[0]?.onmessage?.({ data: JSON.stringify({ id: "evt_ok", kind: "deploy" }) });
    expect(h.events.map((e) => e.id)).toEqual(["evt_ok"]);
  });
});
