import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Dot, Pill, type PillTone } from "@/design-system/pill";
import { ObserveChrome } from "@/features/observe/observe-chrome";
import { useEvents } from "@/features/services/hooks";
import type { Event } from "@/lib/api";
import { fmtTime } from "@/lib/fmt";
import { cn } from "@/lib/utils";

/**
 * O4 · Observe — Events: the spine. Every ◆ ◇ ⬖ marker on every chart is one
 * of these rows. Canon rows render the O4 frame's describeEvent strings
 * verbatim (note: the frame credits deploy #142 to priya while the fixtures
 * say asha — copy follows the frame, divergence flagged as a finding).
 */

const KIND_CHIPS: Array<{ label: string; kind?: string }> = [
  { label: "All kinds" },
  { label: "◆ Deploys", kind: "deploy" },
  { label: "◇ Scaling", kind: "scale" },
  { label: "⚑ Alerts", kind: "alert_state" },
  { label: "⬖ Policy", kind: "policy_trigger" },
  { label: "Lifecycle", kind: "lifecycle" },
  { label: "Config", kind: "config" },
];

function kindGlyph(kind: string): string {
  switch (kind) {
    case "deploy":
      return "◆";
    case "scale":
      return "◇";
    case "alert_state":
      return "⚑";
    case "policy_trigger":
      return "⬖";
    default:
      return "·";
  }
}

interface EventLine {
  text: string;
  pill: { tone: PillTone; label: string };
  actor?: string;
}

function describeEvent(e: Event): EventLine {
  switch (e.id) {
    case "evt_dep142":
      return {
        text: "deploy #142 · api · gift-cards @ a41f2c → production",
        pill: { tone: "st", label: "deploy" },
        actor: "priya",
      };
    case "evt_9d21c4":
      return {
        text: "alert p95-slo opened · api p95 812 ms > 800 ms",
        pill: { tone: "warn", label: "alert" },
        actor: "system",
      };
    case "evt_dlq2":
      return {
        text: "jobs: 2 messages dead-lettered · order.paid",
        pill: { tone: "err", label: "alert" },
      };
    case "evt_dash":
      return {
        text: "dashboard checkout-health created",
        pill: { tone: "mut", label: "lifecycle" },
      };
    case "evt_dep143":
      return {
        text: "deploy #143 · api · canary 33%",
        pill: { tone: "st", label: "deploy" },
      };
    case "evt_apply":
      return {
        text: "assistant drafted proposal prp_7c31a2 for the p95 regression",
        pill: { tone: "ai", label: "proposal" },
        actor: "assistant",
      };
    default:
      return { text: e.action ?? e.kind, pill: { tone: "mut", label: e.kind } };
  }
}

function EventRow({ event }: { event: Event }) {
  const line = describeEvent(event);
  return (
    <div className="flex items-center gap-2.5 border-hair border-b py-2 text-11p5 last:border-b-0">
      <span className="mono text-ink3">{fmtTime(event.at)}</span>
      <span className="w-4 text-center text-ink2">{kindGlyph(event.kind)}</span>
      <span className="min-w-0 flex-1 truncate text-ink2">{line.text}</span>
      <Pill tone={line.pill.tone}>{line.pill.label}</Pill>
      {line.actor ? <span className="mono text-10p5 text-ink3">{line.actor}</span> : null}
    </div>
  );
}

function EventsPage() {
  const { org, project } = Route.useParams();
  const { env } = Route.useSearch();
  const events = useEvents(env);
  const [kind, setKind] = useState<string | undefined>(undefined);

  const filtered = [...(events.data ?? [])]
    .filter((e) => (kind ? e.kind === kind : true))
    .sort((a, b) => (a.at < b.at ? 1 : -1));
  const recent = filtered.filter((e) => fmtTime(e.at) >= "14:00");
  const earlier = filtered.filter((e) => fmtTime(e.at) < "14:00");

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <ObserveChrome env={env} lens="Events" />

        <div className="flex items-center gap-3">
          <div className="chiprow">
            {KIND_CHIPS.map((chip) => (
              <button
                key={chip.label}
                type="button"
                className={cn("chip", kind === chip.kind && "on")}
                onClick={() => setKind(chip.kind)}
              >
                {chip.label}
              </button>
            ))}
          </div>
          <span className="flex-1" />
          <Copybit>steloit events --env production --since 13:30</Copybit>
        </div>

        <Card className="flex flex-col gap-1 p-4">
          {recent.length > 0 ? (
            <>
              <div className="eyebrow pb-1">14:00 — now</div>
              {recent.map((e) => (
                <EventRow key={e.id} event={e} />
              ))}
            </>
          ) : null}
          {earlier.length > 0 ? (
            <>
              <div className="eyebrow pt-3 pb-1">13:30 — 14:00</div>
              {earlier.map((e) => (
                <EventRow key={e.id} event={e} />
              ))}
            </>
          ) : null}
          <div className="flex flex-col gap-1 border-hair border-t pt-2.5 text-10p5 text-ink3">
            <span>
              This stream is the spine — every ◆ ◇ ⬖ marker on every chart is one of these rows.
            </span>
            <span>
              Ops question → here; compliance question → Audit log (W12). Same truth, two grains.
            </span>
          </div>
        </Card>

        <Card className="flex items-center gap-2.5 p-3.5">
          <Dot tone="warn" />
          <span className="flex-1 text-11p5 text-ink2">
            Correlation strip: select any two events to overlay their window in Metrics — "#142 →
            p95" took four minutes to prove by hand once; now it's two clicks.
          </span>
          <Link to="/$org/$project/observe/metrics" params={{ org, project }} search={{ env }}>
            <Btn variant="s">Overlay in Metrics</Btn>
          </Link>
        </Card>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/observe/events")({
  component: EventsPage,
});
