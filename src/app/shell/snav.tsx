import { Link } from "@tanstack/react-router";
import type { ReactNode } from "react";
import { Icon, type IconId } from "@/design-system/icon";
import { cn } from "@/lib/utils";

/**
 * The secondary sidebar (.snav). Exactly one active .nit per snav (16-qa).
 * Items that land in later phases render disabled with the stated reason —
 * gated/unbuilt actions are visible-but-disabled, never hidden.
 */

export function Snav({ children }: { children: ReactNode }) {
  return <nav className="snav">{children}</nav>;
}

export function Nsec({ children }: { children: ReactNode }) {
  return <div className="nsec">{children}</div>;
}

export function Nfoot({ children }: { children: ReactNode }) {
  return <div className="nfoot">{children}</div>;
}

interface NitBaseProps {
  icon?: IconId;
  dot?: ReactNode;
  label: string;
  count?: string;
  badge?: string;
  on?: boolean;
  steel?: boolean;
}

function NitBody({ icon, dot, label, count, badge }: NitBaseProps) {
  return (
    <>
      {icon ? <Icon id={icon} /> : dot}
      <span className="truncate">{label}</span>
      {count ? <span className="cnt">{count}</span> : null}
      {badge ? (
        <span className="bdg" style={{ background: "var(--assist)" }}>
          {badge}
        </span>
      ) : null}
    </>
  );
}

export function NitLink(
  props: NitBaseProps & {
    to: string;
    params?: Record<string, string>;
    search?: Record<string, unknown>;
  },
) {
  const { to, params, search, on, steel, ...body } = props;
  return (
    <Link
      to={to}
      params={params}
      search={search}
      className={cn("nit", on && "on", steel && "text-steel")}
    >
      <NitBody {...body} label={props.label} />
    </Link>
  );
}

export function NitDisabled(props: NitBaseProps & { reason: string }) {
  const { reason, on, steel, ...body } = props;
  return (
    <span className={cn("nit cursor-not-allowed opacity-55")} title={reason} aria-disabled="true">
      <NitBody {...body} label={props.label} />
    </span>
  );
}
