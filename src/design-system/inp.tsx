import type { ComponentPropsWithoutRef, ReactNode } from "react";
import { cn } from "@/lib/utils";

interface FlabelProps {
  children: ReactNode;
  /** Right-aligned affordance, e.g. the "Forgot?" link on A1. */
  aside?: ReactNode;
  htmlFor?: string;
}

export function Flabel({ children, aside, htmlFor }: FlabelProps) {
  return (
    <label className="flabel" htmlFor={htmlFor}>
      <span>{children}</span>
      {aside}
    </label>
  );
}

interface InpProps extends ComponentPropsWithoutRef<"input"> {
  /** Inline field error (422 renders under the field — 08-api error catalog). */
  error?: string;
  prefixNode?: ReactNode;
  suffixNode?: ReactNode;
}

export function Inp({ error, prefixNode, suffixNode, className, ...props }: InpProps) {
  return (
    <>
      <div className={cn("inp", error && "err-state", className)}>
        {prefixNode}
        <input {...props} />
        {suffixNode}
      </div>
      {error ? <div className="ferr">{error}</div> : null}
    </>
  );
}
