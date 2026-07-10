import type { ReactNode } from "react";
import type { ServiceStatus } from "@/lib/api";
import { cn } from "@/lib/utils";

export type PillTone = "ok" | "warn" | "err" | "st" | "ai" | "mut";
export type DotTone = "ok" | "warn" | "err" | "prov" | "sus";

/** Pills always carry text — never color-only (16-qa). */
export function Pill({ tone, children }: { tone: PillTone; children: ReactNode }) {
  return <span className={cn("pill", tone)}>{children}</span>;
}

export function Dot({ tone }: { tone: DotTone }) {
  return <span className={cn("dot", tone)} />;
}

/** Status label: dot + words, the six-mark vocabulary paired with text. */
export function Stlab({ tone, children }: { tone: DotTone; children: ReactNode }) {
  const color =
    tone === "ok"
      ? "text-ok"
      : tone === "warn"
        ? "text-warn"
        : tone === "err"
          ? "text-err"
          : tone === "prov"
            ? "text-prov"
            : "text-ink3";
  return (
    <span className={cn("stlab", color)}>
      <Dot tone={tone} />
      {children}
    </span>
  );
}

/** ADR-024 vocabulary: ready/deleting — never running/deleted. */
export function statusDotTone(status: ServiceStatus): DotTone {
  switch (status) {
    case "ready":
      return "ok";
    case "degraded":
      return "warn";
    case "failed":
      return "err";
    case "provisioning":
      return "prov";
    case "suspended":
      return "sus";
    case "deleting":
      return "sus";
  }
}

export function healthDotTone(health: string): DotTone {
  if (health === "ok") return "ok";
  if (health === "warn") return "warn";
  if (health === "err") return "err";
  return "prov";
}
