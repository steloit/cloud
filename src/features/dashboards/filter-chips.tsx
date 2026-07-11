import { useState } from "react";
import { cn } from "@/lib/utils";

/**
 * The shared dashboard filter row (DB1–DB4): one filter bar over every widget
 * at once. Display-state chips in canon mode — clicking toggles .on locally;
 * no refetch behind them yet.
 */

const CHIPS = ["All projects ▾", "production ▾", "all regions ▾", "all products ▾"] as const;

export function FilterChips({ defaultOn = ["All projects ▾"] }: { defaultOn?: string[] }) {
  const [on, setOn] = useState<ReadonlySet<string>>(() => new Set(defaultOn));
  return (
    <div className="chiprow">
      {CHIPS.map((chip) => (
        <button
          key={chip}
          type="button"
          className={cn("chip", on.has(chip) && "on")}
          onClick={() =>
            setOn((prev) => {
              const next = new Set(prev);
              if (next.has(chip)) next.delete(chip);
              else next.add(chip);
              return next;
            })
          }
        >
          {chip}
        </button>
      ))}
    </div>
  );
}
