import type { ReactNode } from "react";
import { Card } from "@/design-system/card";
import { Eyebrow } from "@/design-system/eyebrow";
import { Icon, type IconId } from "@/design-system/icon";
import { Metric } from "@/design-system/metric";
import { Pill, type PillTone } from "@/design-system/pill";
import { cn } from "@/lib/utils";

/**
 * The shared Overview grammar (W4 · S1–S6, Design-Spec §Product Overview):
 * five zones — identity, vitals (cost always last), Right now, Connect, Act.
 */

export interface VitalCell {
  label: string;
  value: ReactNode;
  note?: ReactNode;
  tone?: "warn" | "err";
  mono?: boolean;
}

export function VitalsStrip({ cells }: { cells: VitalCell[] }) {
  return (
    <div className="grid grid-cols-5 gap-3">
      {cells.map((c) => (
        <Metric
          key={c.label}
          label={c.label}
          value={c.value}
          note={c.note}
          tone={c.tone}
          mono={c.mono}
        />
      ))}
    </div>
  );
}

export function RightNow({
  mood,
  children,
}: {
  /** Activity when calm; Needs attention flips the eyebrow amber. */
  mood: "Activity" | "Needs attention";
  children: ReactNode;
}) {
  return (
    <Card className="flex flex-1 flex-col gap-2.5 p-4">
      <Eyebrow className={mood === "Needs attention" ? "text-warn" : undefined}>{mood}</Eyebrow>
      {children}
    </Card>
  );
}

export interface ConsumerRow {
  icon: IconId;
  name: string;
  pill: { tone: PillTone; label: string };
  extraPill?: { tone: PillTone; label: string };
  note: string;
}

export function ConnectPanel({
  eyebrow = "Connect",
  envPills,
  cli,
  consumersEyebrow,
  consumers,
  footer,
  children,
}: {
  eyebrow?: string;
  envPills: string[];
  cli?: ReactNode;
  consumersEyebrow: string;
  consumers: ConsumerRow[];
  footer: string;
  children?: ReactNode;
}) {
  return (
    <Card className="flex w-[380px] shrink-0 flex-col gap-2.5 p-4">
      <Eyebrow>{eyebrow}</Eyebrow>
      <div className="flex flex-wrap items-center gap-2">
        {envPills.map((p) => (
          <span key={p} className="bindpill">
            <Icon id="s-bind" />
            {p}
          </span>
        ))}
      </div>
      {cli}
      <Eyebrow>{consumersEyebrow}</Eyebrow>
      <div className="flex flex-col gap-1.5">
        {consumers.map((c) => (
          <div key={`${c.name}-${c.note}`} className="flex items-center gap-2 text-[11.5px]">
            <Icon id={c.icon} className="h-3.5 w-3.5 text-ink3" />
            <b>{c.name}</b>
            <Pill tone={c.pill.tone}>{c.pill.label}</Pill>
            {c.extraPill ? <Pill tone={c.extraPill.tone}>{c.extraPill.label}</Pill> : null}
            <span className="ml-auto text-[10.5px] text-ink3">{c.note}</span>
          </div>
        ))}
      </div>
      {children}
      <p className="border-hair border-t pt-2 text-[10.5px] leading-relaxed text-ink3">{footer}</p>
    </Card>
  );
}

export interface ActTile {
  title: string;
  sub: string;
  danger?: boolean;
  onClick?: () => void;
  disabledReason?: string;
}

export function ActRow({ tiles }: { tiles: ActTile[] }) {
  return (
    <div className="grid grid-cols-4 gap-3">
      {tiles.map((t) => {
        const disabled = Boolean(t.disabledReason) && !t.onClick;
        return (
          <Card
            key={t.title}
            className={cn(
              "flex h-full flex-col gap-1 p-3.5",
              t.onClick && "cursor-pointer hover:border-ink3",
              disabled && "opacity-55",
            )}
            title={disabled ? t.disabledReason : undefined}
            onClick={t.onClick}
          >
            <b className={cn("text-[12.5px]", t.danger && "text-err")}>{t.title}</b>
            <span className="text-[10.5px] text-ink3">{t.sub}</span>
          </Card>
        );
      })}
    </div>
  );
}

/** Ambient facts strip at the bottom of a Right-now panel. */
export function FloorStrip({ children }: { children: ReactNode }) {
  return (
    <div className="mt-auto flex flex-wrap items-center gap-x-4 gap-y-1.5 border-hair border-t pt-2.5 text-[10.5px] text-ink3">
      {children}
    </div>
  );
}
