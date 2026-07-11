import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { Banner } from "@/design-system/banner";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { EmptyState } from "@/design-system/empty-state";
import { Eyebrow } from "@/design-system/eyebrow";
import { Glyph } from "@/design-system/glyph";
import { useServices } from "@/features/services/hooks";
import { resolveEnvKey } from "@/lib/canon-env";
import { cn } from "@/lib/utils";

/**
 * D8 · Queue Messages — 08-api has no messages/DLQ endpoints although D8
 * requires them (finding: spec change needed before the lists can go live);
 * the dead letters below are frame-fixed canon from the incident. The
 * payload body beyond the `"receipt": null` line is canon-adjacent (same
 * user/cart as D3's session detail).
 */

const TABS = ["Ready · 12", "In-flight · 3", "Dead letters · 2"] as const;

/** A fresh queue's honest counts — nothing enqueued, nothing dead yet. */
const TABS_FRESH = ["Ready · 0", "In-flight · 0", "Dead letters · 0"] as const;

interface DlqRow {
  message: string;
  firstFailed: string;
  attempts: string;
  lastError: string;
}

const DLQ: DlqRow[] = [
  {
    message: "msg_9f221 · order.paid",
    firstFailed: "14:03:12",
    attempts: "5/5",
    lastError: "TypeError: receiptUrl undefined",
  },
  {
    message: "msg_9f224 · order.paid",
    firstFailed: "14:03:41",
    attempts: "5/5",
    lastError: "TypeError: receiptUrl undefined",
  },
];

/** "msg_9f221 · order.paid" → "msg_9f221" — the row's id is the message string's first token. */
const msgId = (m: DlqRow) => m.message.split(" · ")[0] ?? m.message;

function MessagesPage() {
  const { org, project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(resolveEnvKey(project, env));
  const [tabIdx, setTabIdx] = useState(2);
  // Pass-5: the rows are selectable — the detail panel renders whichever dead
  // letter is picked. msg_9f221 is the canon exemplar and starts selected.
  const [selectedId, setSelectedId] = useState("msg_9f221");
  const selectedMsg = DLQ.find((r) => msgId(r) === selectedId);

  const svc = services.data?.find((s) => s.name === service || s.id === service);
  if (!svc) return <main className="main" />;
  if (svc.product !== "queue") {
    return (
      <main className="main">
        <div className="pgpad">
          <Card className="p-4 text-12 text-ink2">
            Messages are a Queue surface — {svc.name} is a {svc.product} service.
          </Card>
        </div>
      </main>
    );
  }

  // The dead letters (and the 12/3/2 counts) are `jobs`' frame-fixed canon
  // from the Jul 2 incident — any other queue starts honest: empty lists
  // until a message exhausts its retries.
  const canon = svc.name === "jobs";
  const tabs = canon ? TABS : TABS_FRESH;

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
          {tabs.map((t, i) => (
            <button
              key={t}
              type="button"
              className={cn("tab", tabIdx === i && "on", t === "Dead letters · 2" && "text-warn")}
              onClick={() => setTabIdx(i)}
            >
              {t}
            </button>
          ))}
        </div>

        {tabIdx !== 2 ? (
          <p className="text-11p5 text-ink3">
            {canon
              ? "The ready and in-flight lists need a messages endpoint the spec lacks (finding) — dead letters carry the incident today."
              : "The ready and in-flight lists need a messages endpoint the spec lacks (finding)."}
          </p>
        ) : !canon ? (
          <EmptyState
            icon="s-queue"
            title="DLQ is empty"
            meaning="messages land here after max retries · redrive returns them to the main queue"
            cta={
              <>
                <Btn
                  variant="p"
                  disabled
                  disabledReason="No redrive endpoint in the spec (finding)"
                >
                  Redrive
                </Btn>
                <Btn
                  variant="dgr"
                  disabled
                  disabledReason="No redrive endpoint in the spec (finding)"
                >
                  Discard…
                </Btn>
              </>
            }
          />
        ) : (
          <>
            <Banner tone="warn">
              <span className="flex-1">
                Both dead letters arrived at 14:03 — one minute after deploy #142. Correlated in
                Observe.
              </span>
              <Link to="/$org/$project/observe/events" params={{ org, project }} search={{ env }}>
                <Btn variant="s" className="h-6 px-2.5 text-10p5">
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
                      {DLQ.map((m) => {
                        const selected = msgId(m) === selectedId;
                        return (
                          // Row-click selection — the invoices-list idiom (B3).
                          // aria-selected, not aria-pressed: rows carry the row
                          // role, which only supports the former (a11y lint).
                          <tr
                            key={m.message}
                            className="cursor-pointer"
                            aria-selected={selected}
                            onClick={() => setSelectedId(msgId(m))}
                          >
                            <td className={cn("mono", selected && "bg-steel-tint")}>{m.message}</td>
                            <td className={cn("mono", selected && "bg-steel-tint")}>
                              {m.firstFailed}
                            </td>
                            <td className={cn("mono", selected && "bg-steel-tint")}>
                              {m.attempts}
                            </td>
                            <td className={cn("mono", selected && "bg-steel-tint")}>
                              {m.lastError}
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              </Card>

              {selectedId === "msg_9f221" ? (
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
              ) : selectedMsg ? (
                // msg_9f224's payload never made the canon fixtures — the panel
                // renders the row's own metadata and says so (finding).
                <Card className="flex w-[380px] shrink-0 flex-col gap-2.5 self-start p-4">
                  <Eyebrow>Message · {msgId(selectedMsg)}</Eyebrow>
                  <div className="flex flex-col gap-1.5">
                    <div className="flex justify-between text-11p5">
                      <span className="text-ink3">Type</span>
                      <span className="mono">{selectedMsg.message.split(" · ")[1]}</span>
                    </div>
                    <div className="flex justify-between text-11p5">
                      <span className="text-ink3">Attempts</span>
                      <span className="mono">{selectedMsg.attempts}</span>
                    </div>
                    <div className="flex justify-between text-11p5">
                      <span className="text-ink3">First failed</span>
                      <span className="mono">{selectedMsg.firstFailed}</span>
                    </div>
                    <div className="flex justify-between text-11p5">
                      <span className="text-ink3">Last error</span>
                      <span className="mono text-err">{selectedMsg.lastError}</span>
                    </div>
                    {/* No canon clock on this surface — age renders as the row's
                        own absolute since-time, Jul 2 per the incident. */}
                    <div className="flex justify-between text-11p5">
                      <span className="text-ink3">Age</span>
                      <span className="mono">
                        in the DLQ since {selectedMsg.firstFailed} · Jul 2
                      </span>
                    </div>
                  </div>
                  <p className="border-hair border-t pt-2.5 text-10p5 leading-relaxed text-ink3">
                    payload not captured in the canon fixtures — msg_9f221 is the exemplar (finding)
                  </p>
                </Card>
              ) : null}
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
              <span className="text-10p5 text-ink3">
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
