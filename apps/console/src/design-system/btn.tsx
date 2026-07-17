import type { ComponentPropsWithoutRef } from "react";
import { cn } from "@/lib/utils";

type BtnVariant = "p" | "s" | "a" | "dgr" | "gh";

interface BtnProps extends ComponentPropsWithoutRef<"button"> {
  variant?: BtnVariant;
  /** Required whenever disabled — gated actions carry the stated reason (B6). */
  disabledReason?: string;
}

export function Btn({ variant = "s", disabledReason, className, ...props }: BtnProps) {
  const disabled = props.disabled ?? false;
  return (
    <button
      type="button"
      {...props}
      className={cn("btn", variant, className)}
      title={disabled ? disabledReason : props.title}
      aria-disabled={disabled || undefined}
    />
  );
}
