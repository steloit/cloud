import type { ReactNode } from "react";
import { cn } from "@/lib/utils";
import { Card } from "./card";
import { Eyebrow } from "./eyebrow";

interface MetricProps {
  label: string;
  value: ReactNode;
  note?: ReactNode;
  tone?: "warn" | "err";
  mono?: boolean;
}

/** Vitals cell — value in tabular figures, cost cells always mono. */
export function Metric({ label, value, note, tone, mono }: MetricProps) {
  return (
    <Card className="metric p-3">
      <Eyebrow>{label}</Eyebrow>
      <div
        className={cn(
          "mv mt-1",
          mono && "mono",
          tone === "warn" && "text-warn",
          tone === "err" && "text-err",
        )}
      >
        {value}
      </div>
      {note ? <div className="ml">{note}</div> : null}
    </Card>
  );
}
