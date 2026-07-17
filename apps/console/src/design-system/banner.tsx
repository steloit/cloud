import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

/** Banner classes are only `default` and `warn` (16-qa consistency checklist). */
export function Banner({
  tone = "default",
  children,
}: {
  tone?: "default" | "warn";
  children: ReactNode;
}) {
  return <div className={cn("banner", tone)}>{children}</div>;
}
