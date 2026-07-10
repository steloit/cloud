import type { ComponentPropsWithoutRef } from "react";
import { cn } from "@/lib/utils";

interface CardProps extends ComponentPropsWithoutRef<"div"> {
  /** Dashed border = add/locked affordance (design-system contract). */
  dashed?: boolean;
}

export function Card({ dashed, className, ...props }: CardProps) {
  return (
    <div {...props} className={cn("card", dashed && "border-dashed bg-transparent", className)} />
  );
}
