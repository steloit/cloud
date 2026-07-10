import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export function Eyebrow({ className, children }: { className?: string; children: ReactNode }) {
  return <div className={cn("eyebrow", className)}>{children}</div>;
}
