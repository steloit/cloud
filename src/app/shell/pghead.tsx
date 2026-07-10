import type { ReactNode } from "react";

/**
 * Page header — h1 + hsub, actions after the spacer. No breadcrumb line
 * inside (16-qa); subpage h1 grammar is "Area · Thing".
 */
export function Pghead({
  title,
  sub,
  before,
  children,
}: {
  title: ReactNode;
  sub?: ReactNode;
  /** Leading glyph / status cluster rendered before the title block. */
  before?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <div className="pghead">
      {before}
      <div>
        <h1 className="h1">{title}</h1>
        {sub ? <div className="hsub">{sub}</div> : null}
      </div>
      <span className="sp" />
      {children}
    </div>
  );
}
