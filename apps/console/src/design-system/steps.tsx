import { Fragment } from "react";
import { cn } from "@/lib/utils";
import { Icon } from "./icon";

/**
 * Onboarding progress bar. The in-frame bars are authoritative for order:
 * 1 Organization → 2 Team → 3 First project → 4 Connect.
 */
export function Steps({ labels, current }: { labels: string[]; current: number }) {
  return (
    <div className="steps">
      {labels.map((label, i) => {
        const state = i < current ? "done" : i === current ? "on" : "todo";
        return (
          <Fragment key={label}>
            {i > 0 && <span className="stepbar" />}
            <span className={cn("stepdot", state === "on" && "on", state === "done" && "done")}>
              {state === "done" ? <Icon id="s-check" className="h-2.5 w-2.5" /> : i + 1}
            </span>
            <span className={cn("steplbl", state === "on" && "on")}>{label}</span>
          </Fragment>
        );
      })}
    </div>
  );
}
