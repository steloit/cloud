import { cn } from "@/lib/utils";

/**
 * Loading skeletons (16-qa four-state contract): pulsing blocks on
 * --surface2, shaped like the content they stand in for. Never a spinner,
 * never a blank region.
 */

export function Skeleton({ className }: { className?: string }) {
  return (
    <div aria-hidden="true" className={cn("animate-pulse rounded-md bg-surface2", className)} />
  );
}

/** Table-loading rows — render inside the <tbody> while the query is pending. */
export function SkeletonRows({ cols, rows = 3 }: { cols: number; rows?: number }) {
  return (
    <>
      {Array.from({ length: rows }, (_, r) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: placeholder rows have no identity
        <tr key={r}>
          {Array.from({ length: cols }, (_, c) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: placeholder cells have no identity
            <td key={c}>
              <Skeleton className={cn("h-3", c === 0 ? "w-[85%]" : "w-[60%]")} />
            </td>
          ))}
        </tr>
      ))}
    </>
  );
}

/** Stacked line placeholders for card/list bodies. */
export function SkeletonLines({ lines = 3, className }: { lines?: number; className?: string }) {
  return (
    <div className={cn("flex flex-col gap-2.5", className)} aria-hidden="true">
      {Array.from({ length: lines }, (_, i) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: placeholder lines have no identity
        <Skeleton key={i} className={cn("h-3", i === lines - 1 ? "w-[55%]" : "w-full")} />
      ))}
    </div>
  );
}
