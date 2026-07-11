import { cn } from "@/lib/utils";

/**
 * The shared dashboard filter row (DB1–DB4): one filter bar over every widget
 * at once. Rendered as STATIC context chips (the frame shows them as context)
 * — refetch-backed filtering needs query params the dashboards spec lacks
 * (finding), and a toggle that filters nothing would be a dead affordance.
 */

const CHIPS = ["All projects", "production", "all regions", "all products"] as const;

export function FilterChips({ defaultOn = ["All projects"] }: { defaultOn?: string[] }) {
  const on = new Set(defaultOn);
  return (
    <div className="chiprow">
      {CHIPS.map((chip) => (
        <span key={chip} className={cn("chip", on.has(chip) && "on")}>
          {chip}
        </span>
      ))}
    </div>
  );
}
