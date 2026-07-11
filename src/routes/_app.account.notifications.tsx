import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Dot } from "@/design-system/pill";
import { cn } from "@/lib/utils";

/**
 * P6 · Account · Notifications — delivery preferences. Routing is yours;
 * whether something fires is the org's (alert rules + policy).
 */

type CellValue = boolean | "always" | "daily digest" | "failures only";

interface RoutingRow {
  id: string;
  type: string;
  inApp: CellValue;
  email: CellValue;
  push: CellValue;
}

const ROWS: RoutingRow[] = [
  { id: "alerts-prod", type: "Alerts · production", inApp: "always", email: true, push: true },
  { id: "alerts-nonprod", type: "Alerts · non-production", inApp: true, email: false, push: false },
  { id: "proposals", type: "Assistant proposals", inApp: true, email: true, push: false },
  { id: "approvals", type: "Approvals assigned to me", inApp: true, email: true, push: true },
  { id: "deploys", type: "Deploys", inApp: true, email: "daily digest", push: false },
  {
    id: "lifecycle",
    type: "Lifecycle (ready · failed · renewed)",
    inApp: true,
    email: "failures only",
    push: false,
  },
];

const QUIET_SEED = { start: "22:00", end: "07:00" };

function NotificationSettingsPage() {
  // Quiet-hours inputs track dirty so Save can gate honestly (see below).
  const [quiet, setQuiet] = useState(QUIET_SEED);
  const quietDirty = quiet.start !== QUIET_SEED.start || quiet.end !== QUIET_SEED.end;

  // Working local toggles — keyed "{rowId}.{channel}", seeded from the canon rows.
  const [toggles, setToggles] = useState<Record<string, boolean>>(() => {
    const seed: Record<string, boolean> = {};
    for (const row of ROWS) {
      for (const [channel, value] of [
        ["inApp", row.inApp],
        ["email", row.email],
        ["push", row.push],
      ] as const) {
        if (typeof value === "boolean") seed[`${row.id}.${channel}`] = value;
      }
    }
    return seed;
  });

  const cell = (row: RoutingRow, channel: "inApp" | "email" | "push") => {
    const value = row[channel];
    if (value === "always") {
      return <span className="mono text-10p5 text-ink3">always · not a preference</span>;
    }
    if (typeof value === "string") {
      return <span className="text-11p5 text-ink2">{value}</span>;
    }
    const key = `${row.id}.${channel}`;
    const on = toggles[key] ?? false;
    return (
      <button
        type="button"
        className={cn("pill", on ? "st" : "mut")}
        aria-pressed={on}
        onClick={() => setToggles((t) => ({ ...t, [key]: !on }))}
      >
        {on ? "on" : "off"}
      </button>
    );
  };

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        {/* Finding: the frame shows the bare "Notifications" title — the design-system
            "Area · Thing" h1 grammar wins per the audit's P1 ruling. */}
        <Pghead
          title="Account · Notifications"
          sub="How things reach you — routing is yours; whether something fires is the org's"
        />

        <div className="tblwrap">
          <table className="tbl">
            <thead>
              <tr>
                <th>Type</th>
                <th>In-app</th>
                <th>Email</th>
                <th>Mobile push</th>
              </tr>
            </thead>
            <tbody>
              {ROWS.map((row) => (
                <tr key={row.id}>
                  <td className="font-medium">{row.type}</td>
                  <td>{cell(row, "inApp")}</td>
                  <td>{cell(row, "email")}</td>
                  <td>{cell(row, "push")}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <Card className="flex max-w-[640px] items-center gap-4 p-4">
          <div className="flex-1">
            <div className="flex items-baseline gap-2.5">
              <span className="text-13 font-semibold">Quiet hours</span>
              <span className="text-11 text-ink3">email & push only — in-app is never gated</span>
            </div>
          </div>
          <span className="flex items-center gap-2">
            <span className="inp mono !h-8 !w-[76px] justify-center !px-2 text-12">
              <input
                className="text-center"
                value={quiet.start}
                onChange={(e) => setQuiet((q) => ({ ...q, start: e.target.value }))}
                aria-label="Quiet hours start"
              />
            </span>
            <span className="text-ink3">→</span>
            <span className="inp mono !h-8 !w-[76px] justify-center !px-2 text-12">
              <input
                className="text-center"
                value={quiet.end}
                onChange={(e) => setQuiet((q) => ({ ...q, end: e.target.value }))}
                aria-label="Quiet hours end"
              />
            </span>
            <span className="mono text-10p5 text-ink3">Asia/Kolkata</span>
            {/* Pass-6: the inputs edited into nowhere — Save exists, takes the enabled look only once something changed (dirty-tracked), and stays gated with the finding. */}
            <Btn
              variant="p"
              className={cn("h-8", quietDirty && "!opacity-100")}
              disabled
              disabledReason={
                quietDirty
                  ? "No notification-settings endpoint in the spec (finding)"
                  : "Nothing changed yet"
              }
            >
              Save
            </Btn>
          </span>
        </Card>

        <Card className="flex max-w-[640px] gap-3 p-4">
          <span className="pt-1">
            <Dot tone="warn" />
          </span>
          <p className="text-11p5 leading-relaxed text-ink2">
            The escalation boundary: production alert paging (unacked 15 min → on-call) is org
            policy, not a personal preference — quiet hours don't silence it, and that's stated here
            instead of discovered at 3 a.m.
          </p>
        </Card>

        <p className="max-w-[640px] text-11 leading-relaxed text-ink3">
          These are delivery preferences (N-series semantics: mute = a type, per user, here). Which
          alerts exist and when they escalate lives with the alert rules and org policy.
        </p>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/account/notifications")({
  component: NotificationSettingsPage,
});
