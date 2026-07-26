import { cn } from "@/lib/utils";

/**
 * Icons come from the single symbol sprite (01-design-system/icons):
 * 24×24 stroke, referenced by <use>, colored by currentColor, aria-hidden.
 */
export type IconId =
  | "s-hex"
  | "s-chart"
  | "s-pulse"
  | "s-deploy"
  | "s-db"
  | "s-chip"
  | "s-bucket"
  | "s-queue"
  | "s-globe"
  | "s-worker"
  | "s-plus"
  | "s-gear"
  | "s-doc"
  | "s-search"
  | "s-bell"
  | "s-branch"
  | "s-shield"
  | "s-bind"
  | "s-term"
  | "s-grid"
  | "s-eye"
  | "s-env"
  | "s-chevd"
  | "s-chev"
  | "s-copy"
  | "s-ai"
  | "s-undo"
  | "s-check"
  | "s-x"
  | "s-user"
  | "s-users"
  | "s-org"
  | "s-key"
  | "s-card"
  | "s-cron";

export function Icon({ id, className }: { id: IconId; className?: string }) {
  return (
    <svg className={cn("shrink-0", className)} aria-hidden="true">
      <use href={`#${id}`} />
    </svg>
  );
}
