import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { Dot, Pill } from "@/design-system/pill";
import { cn } from "@/lib/utils";

/**
 * The shared Observe page-top block (O1–O6): one time context, five lenses —
 * the range and env persist across every page. The canon window is fixed
 * (13:30 – 14:45, the incident), so time chips are display-state only; env
 * chips are the real ?env= filter (ADR-013).
 */

const TIME_CHIPS = ["15 m", "1 h", "13:30 – 14:45", "24 h", "7 d"];
const ENV_CHIPS = ["production", "staging", "previews"];

export function ObserveChrome({ env, lens }: { env: string; lens: string }) {
  const navigate = useNavigate();
  const [range, setRange] = useState("13:30 – 14:45");

  return (
    <>
      <Pghead
        title={`Observe · ${lens}`}
        sub="One time context, five lenses — the range and filters persist across every Observe page"
      >
        <Pill tone="warn">1 alert firing · 41 min</Pill>
      </Pghead>
      <div className="flex items-center gap-3">
        <div className="chiprow">
          {TIME_CHIPS.map((chip) => (
            <button
              key={chip}
              type="button"
              className={cn("chip", range === chip && "on")}
              onClick={() => setRange(chip)}
            >
              {chip}
            </button>
          ))}
        </div>
        <div className="chiprow">
          {ENV_CHIPS.map((chip) => (
            <button
              key={chip}
              type="button"
              className={cn("chip", env === chip && "on")}
              onClick={() => navigate({ to: ".", search: { env: chip } })}
            >
              {chip}
            </button>
          ))}
        </div>
        <span className="flex-1" />
        <span className="chip">
          <Dot tone="ok" /> live
        </span>
      </div>
    </>
  );
}
