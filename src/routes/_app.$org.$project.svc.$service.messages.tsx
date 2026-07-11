import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { Banner } from "@/design-system/banner";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Eyebrow } from "@/design-system/eyebrow";
import { Glyph } from "@/design-system/glyph";
import { useServices } from "@/features/services/hooks";
import { cn } from "@/lib/utils";

/**
 * D8 · Queue Messages — 08-api has no messages/DLQ endpoints although D8
 * requires them (finding: spec change needed before the lists can go live);
 * the dead letters below are frame-fixed canon from the incident. The
 * payload body beyond the `"receipt": null` line is canon-adjacent (same
 * user/cart as D3's session detail).
 */

const TABS = ["Ready · 12", "In-flight · 3", "Dead letters · 2"] as const;
type Tab = (typeof TABS)[number];

interface DlqRow {
  message: string;
  firstFailed: string;
  attempts: string;
  lastError: string;
  selected?: boolean;
}

const DLQ: DlqRow[] = [
  {
    message: "msg_9f221 · order.paid",
    firstFailed: "14:03:12",
    attempts: "5/5",
    lastError: "TypeError: receiptUrl undefined",
    selected: true,
  },
  {
    message: "msg_9f224 · order.paid",
    firstFailed: "14:03:41",
    attempts: "5/5",
    lastError: "TypeError: receiptUrl undefined",
  },
];

function MessagesPage() {
  const { org, project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(env);
  const [tab, setTab] = useState<Tab>("Dead letters · 2");

  const svc = services.data?.find((s) => s.name === service || s.id === service);
  if (!svc) return <main className="main" />;
  if (svc.product !== "queue") {
    return (
      <main className="main">
        <div className="pgpad">
          <Card className="p-4 text-[12px] text-ink2">
            Messages are a Queue surface — {svc.name} is a {svc.product} service.
          </Card>
        </div>
      </main>
    );
  }

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          before={<Glyph id="s-queue" />}
          title="Messages"
          sub={
            <span className="mono">
              {svc.name} · {env}
            </span>
          }
        />

        <div className="tabrow">
          {TABS.map((t) => (
            <button
              key={t}
              type="button"
              className={cn("tab", tab === t && "on", t === "Dead letters · 2" && "text-warn")}
              onClick={() => setTab(t)}
            >
              {t}
            </button>
          ))}
        </div>

        {tab !== "Dead letters · 2" ? (
          <p className="text-[11.5px] text-ink3">
            The ready and in-flight lists need a messages endpoint the spec lacks (finding) — dead
            letters carry the incident today.
          </p>
        ) : (
          <>
            <Banner tone="warn">
              <span className="flex-1">
                Both dead letters arrived at 14:03 — one minute after deploy #142. Correlated in
                Observe.
              </span>
              <Link to="/$org/$project/observe/events" params={{ org, project }} search={{ env }}>
                <Btn variant="s" className="h-6 px-2.5 text-[10.5px]">
                  Open in Observe →
                </Btn>
              </Link>
            </Banner>

            <div className="flex gap-3.5">
              <Card className="flex-1 self-start">
                <div className="tblwrap">
                  <table className="tbl">
                    <thead>
                      <tr>
                        <th>Message</th>
                        <th>First failed</th>
                        <th>Attempts</th>
                        <th>Last error</th>
                      </tr>
                    </thead>
                    <tbody>
                      {DLQ.map((m) => (
                        <tr key={m.message}>
                          <td className={cn("mono", m.selected && "bg-steel-tint")}>{m.message}</td>
                          <td className={cn("mono", m.selected && "bg-steel-tint")}>
                            {m.firstFailed}
                          </td>
                          <td className={cn("mono", m.selected && "bg-steel-tint")}>
                            {m.attempts}
                          </td>
                          <td className={cn("mono", m.selected && "bg-steel-tint")}>
                            {m.lastError}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </Card>

              <Card className="flex w-[380px] shrink-0 flex-col gap-2.5 self-start p-4">
                <Eyebrow>Payload · msg_9f221</Eyebrow>
                <div className="logwell">
                  <div>{"{"}</div>
                  <div>&nbsp;&nbsp;"type": "order.paid",</div>
                  <div>&nbsp;&nbsp;"order_id": "ord_9f221",</div>
                  <div>&nbsp;&nbsp;"user_id": 8241,</div>
                  <div>&nbsp;&nbsp;"total": 4180.00,</div>
                  <div>&nbsp;&nbsp;"currency": "INR",</div>
                  <div>
                    &nbsp;&nbsp;<span className="lv-e">"receipt": null</span>
                    &nbsp;&nbsp;&nbsp;<span className="t">← introduced by #142</span>
                  </div>
                  <div>{"}"}</div>
                </div>
              </Card>
            </div>

            <div className="flex items-center gap-2.5">
              <Btn variant="p" disabled disabledReason="No redrive endpoint in the spec (finding)">
                Redrive 2 to jobs
              </Btn>
              <Btn
                variant="dgr"
                disabled
                disabledReason="No redrive endpoint in the spec (finding)"
              >
                Discard…
              </Btn>
              <span className="text-[10.5px] text-ink3">
                Redrive after the fix ships — both actions are logged with the actor and reason.
              </span>
            </div>
          </>
        )}
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/messages")({
  component: MessagesPage,
});
