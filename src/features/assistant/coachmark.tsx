import { useState } from "react";
import { Btn } from "@/design-system/btn";
import { Icon } from "@/design-system/icon";
import { Kbd } from "@/design-system/kbd";
import { Pill } from "@/design-system/pill";

/**
 * AI5 · the one-time Assistant coachmark — a single popover near the ctx
 * bar's Assistant button (fixed top-right; the orchestrator mounts it).
 * Any close persists to localStorage; it never comes back.
 */

const SEEN_KEY = "steloit-coachmark-seen";

export function Coachmark() {
  const [open, setOpen] = useState(() => {
    try {
      return localStorage.getItem(SEEN_KEY) === null;
    } catch {
      return false;
    }
  });

  if (!open) return null;

  const dismiss = () => {
    try {
      localStorage.setItem(SEEN_KEY, "1");
    } catch {
      // storage unavailable — the popover still closes for this session
    }
    setOpen(false);
  };

  return (
    <div
      role="dialog"
      aria-label="Meet your Assistant"
      className="fixed right-11 top-[60px] z-50 w-[320px] rounded-[13px] border bg-surface p-[16px_17px] shadow-e2"
      style={{ borderColor: "color-mix(in srgb, var(--assist) 40%, var(--hair))" }}
    >
      <div className="mb-2 flex items-center gap-2">
        <span className="glyph h-[26px] w-[26px] bg-assist-tint">
          <Icon id="s-ai" className="h-[13px] w-[13px] text-assist" />
        </span>
        <b className="text-[13px]">Meet your Assistant</b>
        <Pill tone="st">new</Pill>
        <span className="sp flex-1" />
        <button
          type="button"
          aria-label="Close"
          className="cursor-pointer text-[11px] text-ink3"
          onClick={dismiss}
        >
          ✕
        </button>
      </div>
      <div className="text-[11.5px] leading-relaxed text-ink2">
        Ask why something's slow, explain an error, or find a cheaper config — from anywhere. It
        reads your metrics, logs and traces to answer <b>with evidence</b>, then proposes a change
        you approve. It never acts on its own.
      </div>
      <div className="mt-2.5 flex items-center gap-2 rounded-lg bg-assist-tint p-[8px_10px]">
        <Kbd>⌘J</Kbd>
        <span className="text-[10.5px] text-ink2">
          opens it anywhere — or click <b className="text-assist">Assistant</b> up here
        </span>
      </div>
      <div className="mt-3 flex items-center gap-2">
        <Btn variant="a" className="h-[30px] text-[11.5px]" onClick={dismiss}>
          Got it
        </Btn>
        <Btn variant="gh" className="h-[30px] text-[11.5px]" onClick={dismiss}>
          Don't show again
        </Btn>
        <span className="sp flex-1" />
        <span className="text-[9px] text-ink3">1 of 1</span>
      </div>
      <div className="mt-2.5 border-hair border-t pt-2 text-[9px] text-ink3">
        Optional by design — your org can disable it in <b>Policies</b>. The platform works fully
        without it.
      </div>
    </div>
  );
}
