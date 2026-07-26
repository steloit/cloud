import { useRef, useState } from "react";
import { Btn } from "@/design-system/btn";
import { Icon } from "@/design-system/icon";
import { useOverlay } from "@/design-system/overlay";
import { cn } from "@/lib/utils";

/**
 * A10 · Welcome post-setup feedback widget — a corner widget, not a modal:
 * the console stays usable, every question is skippable, the whole thing is
 * dismissible. Shown once after onboarding completes (localStorage flag).
 * The frame specs one question ("2 of 4"); the other three aren't framed —
 * the widget completes after the canon question (finding: A10's remaining
 * questions are unspecced).
 */

const PENDING_KEY = "steloit-welcome-pending";
const DONE_KEY = "steloit-welcome-done";

export function markWelcomePending(): void {
  if (!localStorage.getItem(DONE_KEY)) localStorage.setItem(PENDING_KEY, "1");
}

const CHIPS = [
  "Web app",
  "API / backend",
  "Data & ETL",
  "AI app",
  "Internal tools",
  "Not sure yet",
];

export function WelcomeWidget() {
  const [visible, setVisible] = useState(
    () => Boolean(localStorage.getItem(PENDING_KEY)) && !localStorage.getItem(DONE_KEY),
  );
  const [selected, setSelected] = useState("API / backend");
  const [done, setDone] = useState(false);

  const dismiss = () => {
    localStorage.removeItem(PENDING_KEY);
    localStorage.setItem(DONE_KEY, "1");
    setVisible(false);
  };

  // Popover recipe: Esc dismisses, Tab not trapped. bottom-16 clears the
  // bottom-right Toaster (mount-collision finding).
  const panel = useRef<HTMLDivElement>(null);
  useOverlay(panel, visible ? dismiss : undefined, { trap: false });

  if (!visible) return null;

  return (
    <div
      ref={panel}
      className="card fixed right-4 bottom-16 z-40 flex w-[340px] flex-col gap-3 p-4 shadow-e2"
    >
      <div className="flex items-center gap-2.5">
        <b className="text-13">Welcome to Steloit 🎉</b>
        <span className="mono text-10 text-ink3">2 of 4 · all optional</span>
        <button
          type="button"
          className="icb ml-auto h-6 w-6"
          aria-label="Dismiss"
          onClick={dismiss}
        >
          <Icon id="s-x" className="h-3 w-3" />
        </button>
      </div>
      <div className="flex gap-1.5" aria-hidden="true">
        {[0, 1, 2, 3].map((i) => (
          <span
            key={i}
            className={cn("h-1 flex-1 rounded-full", i < 2 ? "bg-steel" : "bg-surface2")}
          />
        ))}
      </div>
      {done ? (
        <p className="text-12 text-ink2">Thanks — that's all we'll ask.</p>
      ) : (
        <>
          <div className="text-12p5 font-medium">What are you planning to build?</div>
          <div className="flex flex-wrap gap-1.5">
            {CHIPS.map((chip) => (
              <button
                type="button"
                key={chip}
                className={cn("chip", selected === chip && "on")}
                onClick={() => setSelected(chip)}
              >
                {chip}
              </button>
            ))}
          </div>
        </>
      )}
      <div className="flex items-center gap-2 border-hair border-t pt-2.5">
        <span className="text-10 text-ink3">
          Informs roadmap — never billing or support priority
        </span>
        <span className="ml-auto flex gap-2">
          <Btn variant="gh" className="h-6 px-2 text-11" onClick={dismiss}>
            Skip
          </Btn>
          <Btn
            variant="p"
            className="h-6 px-2 text-11"
            onClick={() => (done ? dismiss() : setDone(true))}
          >
            {done ? "Done" : "Next →"}
          </Btn>
        </span>
      </div>
    </div>
  );
}
