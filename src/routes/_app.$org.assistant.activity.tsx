import { createFileRoute } from "@tanstack/react-router";
import { Fragment, type ReactNode, useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { Icon, type IconId } from "@/design-system/icon";
import { Pill, type PillTone } from "@/design-system/pill";
import { cn } from "@/lib/utils";

/**
 * AI11 · Activity — every AI action in the org, in one ledger. No activity
 * endpoint exists in 08-api — the ledger is frame-fixed (finding); the
 * accountability grammar (who · what · evidence · audited-as) is what the
 * eventual endpoint must reproduce.
 */

type LedgerKind = "asked" | "proposed" | "applied" | "dismissed" | "dry-run";

type LedgerRow = {
  id: string;
  group: "Today" | "Earlier this week";
  icon: IconId;
  iconClass: string;
  title: ReactNode;
  meta: string;
  kind: LedgerKind;
  pill: { tone: PillTone; label: string };
};

const LEDGER: LedgerRow[] = [
  {
    id: "led_apply",
    group: "Today",
    icon: "s-deploy",
    iconClass: "text-ok",
    title: (
      <>
        <b>asha</b> applied the <b>orders index</b> via deploy #143
      </>
    ),
    meta: "prp_7c31a2 · reviewed 3 min · audited as asha, via assistant · 14:50",
    kind: "applied",
    pill: { tone: "ok", label: "applied" },
  },
  {
    id: "led_ask",
    group: "Today",
    icon: "s-ai",
    iconClass: "text-assist",
    title: (
      <>
        <b>you</b> asked “Will it lock the table?”
      </>
    ),
    meta: "in “Why is my database slow?” · 2 follow-ups · 14:14",
    kind: "asked",
    pill: { tone: "ai", label: "asked" },
  },
  {
    id: "led_prop_idx",
    group: "Today",
    icon: "s-ai",
    iconClass: "text-assist",
    title: (
      <>
        <b>assistant</b> proposed the <b>orders index</b>
      </>
    ),
    meta: "evidence: query + plan + metrics · 14:12",
    kind: "proposed",
    pill: { tone: "st", label: "proposed" },
  },
  {
    id: "led_prop_valkey",
    group: "Earlier this week",
    icon: "s-ai",
    iconClass: "text-assist",
    title: (
      <>
        <b>assistant</b> proposed <b>Valkey eviction</b> change
      </>
    ),
    meta: "prp_ca11e0 · open · awaiting review · Mar 4",
    kind: "proposed",
    pill: { tone: "st", label: "proposed" },
  },
  {
    id: "led_dismiss",
    group: "Earlier this week",
    icon: "s-x",
    iconClass: "text-err",
    title: (
      <>
        <b>marco</b> dismissed <b>Worker scale-down</b>
      </>
    ),
    meta: "prp_wk03c1 · reason “peak is unpredictable” · logged · Mar 2",
    kind: "dismissed",
    pill: { tone: "mut", label: "dismissed" },
  },
  {
    id: "led_dryrun",
    group: "Earlier this week",
    icon: "s-doc",
    iconClass: "text-steel",
    title: (
      <>
        <b>asha</b> ran a <b>Storage lifecycle</b> dry-run
      </>
    ),
    meta: "142k objects matched · 0 deleted · Mar 1",
    kind: "dry-run",
    pill: { tone: "mut", label: "dry-run" },
  },
];

const CHIPS = ["All", "Asked", "Proposed", "Applied", "Dismissed"] as const;

function ActivityPage() {
  const [chip, setChip] = useState<(typeof CHIPS)[number]>("All");

  const rows = LEDGER.filter((r) => chip === "All" || r.kind === chip.toLowerCase());
  const groups = (["Today", "Earlier this week"] as const)
    .map((g) => ({ group: g, rows: rows.filter((r) => r.group === g) }))
    .filter((g) => g.rows.length > 0);

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          title="Activity"
          sub={
            'Every AI action in ecommerce — asked, proposed, applied, dismissed. The accountability half of "suggest, don\'t act."'
          }
        >
          <div className="chiprow">
            {CHIPS.map((c) => (
              <button
                key={c}
                type="button"
                aria-pressed={chip === c}
                className={cn("chip", chip === c && "on")}
                style={{ fontFamily: "Inter" }}
                onClick={() => setChip(c)}
              >
                {c}
              </button>
            ))}
          </div>
        </Pghead>

        <div className="tblwrap">
          <table className="tbl">
            <tbody>
              {groups.map(({ group, rows: groupRows }) => (
                <Fragment key={group}>
                  <tr>
                    <td colSpan={3} className="pt-1">
                      <div className="nsec m-0">{group}</div>
                    </td>
                  </tr>
                  {groupRows.map((r) => (
                    <tr key={r.id}>
                      <td className="w-[34px]">
                        <span className="glyph h-6 w-6">
                          <Icon id={r.icon} className={cn("h-[11px] w-[11px]", r.iconClass)} />
                        </span>
                      </td>
                      <td>
                        <div className="text-[11.5px] text-ink1">{r.title}</div>
                        <div className="mt-0.5 text-[9.5px] text-ink3">{r.meta}</div>
                      </td>
                      <td className="w-[90px] text-right">
                        <Pill tone={r.pill.tone}>{r.pill.label}</Pill>
                      </td>
                    </tr>
                  ))}
                </Fragment>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/assistant/activity")({
  component: ActivityPage,
});
