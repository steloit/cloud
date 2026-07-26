import type { ReactNode } from "react";
import { cn } from "@/lib/utils";
import { Card } from "./card";
import { Copybit } from "./copybit";
import { Glyph } from "./glyph";
import type { IconId } from "./icon";

/**
 * The empty-state grammar (16-qa): glyph + one-line meaning + primary CTA +
 * CLI equivalent — never a bare "no data". `compact` is the in-card/in-table
 * variant; the default matches W1's zero-projects hero (dashed card).
 */
export function EmptyState({
  icon = "s-hex",
  title,
  meaning,
  cta,
  cli,
  compact,
  className,
}: {
  icon?: IconId;
  title: string;
  /** One line on what would live here and why it matters. */
  meaning?: ReactNode;
  /** Primary action(s) — pass <Btn>s; disabled ones carry their reason. */
  cta?: ReactNode;
  /** CLI equivalent, rendered as a copy chip. */
  cli?: string;
  compact?: boolean;
  className?: string;
}) {
  return (
    <Card
      dashed
      className={cn("flex flex-col items-center gap-3", compact ? "py-8" : "py-14", className)}
    >
      <Glyph id={icon} />
      <div className={cn("font-semibold", compact ? "text-12p5" : "text-14")}>{title}</div>
      {meaning ? (
        <p className="max-w-[440px] text-center text-11p5 leading-relaxed text-ink3">{meaning}</p>
      ) : null}
      {cta ? <div className="flex gap-2">{cta}</div> : null}
      {cli ? <Copybit>{cli}</Copybit> : null}
    </Card>
  );
}

/** Empty state for inside a .tblwrap — spans the table without a nested card. */
export function EmptyRow({ cols, children }: { cols: number; children: ReactNode }) {
  return (
    <tr>
      <td colSpan={cols} className="py-8 text-center text-11p5 text-ink3">
        {children}
      </td>
    </tr>
  );
}
