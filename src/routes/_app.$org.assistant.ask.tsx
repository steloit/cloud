import { createFileRoute, Link } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Eyebrow } from "@/design-system/eyebrow";
import { Icon } from "@/design-system/icon";
import { Inp } from "@/design-system/inp";
import { Pill } from "@/design-system/pill";
import { useCreateThread, usePostMessage } from "@/features/assistant/hooks";
import { errorMessage } from "@/lib/api";

/**
 * AI2 · Ask — the full Assistant page: conversation rail, the canon thread
 * ("Why is my database slow?"), a working composer, and the sources rail.
 * Every assistant claim cites (Law 2); the proposal buttons route to W10 —
 * the assistant proposes, the human applies.
 */

const CANON_THREAD = "thr_db_slow";

// The W10 insights route validates only { env }; ?proposal= rides along as
// the frame's deep-link param (finding: not in the route's search schema).
const W10_SEARCH = { env: "production", proposal: "prp_7c31a2" } as { env: string };

type ChatMsg = { id: string; role: "user" | "assistant"; body: ReactNode };

function AssistantBubble({ children }: { children: ReactNode }) {
  return (
    <div className="my-2 flex gap-2.5">
      <span
        className="glyph h-6 w-6 shrink-0"
        style={{ background: "var(--assist-tint)" }}
        aria-hidden
      >
        <Icon id="s-ai" className="h-3 w-3 text-assist" />
      </span>
      <div
        className="flex-1 rounded-[12px_12px_12px_3px] border p-[11px_13px] text-11p5 leading-relaxed text-ink1"
        style={{
          background: "color-mix(in srgb, var(--assist) 4%, var(--surface))",
          borderColor: "color-mix(in srgb, var(--assist) 28%, var(--hair))",
        }}
      >
        {children}
      </div>
    </div>
  );
}

function UserBubble({ children }: { children: ReactNode }) {
  return (
    <div className="my-2 flex justify-end">
      <div className="max-w-[76%] rounded-[12px_12px_3px_12px] border border-hair bg-surface2 p-[9px_12px] text-11p5 text-ink1">
        {children}
      </div>
    </div>
  );
}

/** The canon thread, verbatim from AI2 — seeds the local thread state. */
function seedMessages(org: string): ChatMsg[] {
  return [
    {
      id: "msg_seed_1",
      role: "assistant",
      body: (
        <>
          Your <b>api</b> p95 crossed 800 ms at 14:02, four minutes after deploy{" "}
          <span className="mono text-10p5">#142</span>. I traced it to an unindexed query on{" "}
          <span className="mono text-10p5">orders</span> — a sequential scan over 1.2M rows taking
          642 ms.
          <div className="chiprow mt-2">
            <Pill tone="ai">3 sources</Pill>
            <Pill tone="st">p95 812ms</Pill>
            <Pill tone="warn">642ms SeqScan</Pill>
          </div>
          <div
            className="mt-2.5 rounded-lg border bg-surface p-2.5"
            style={{ borderColor: "color-mix(in srgb, var(--assist) 28%, var(--hair))" }}
          >
            <div className="mb-1.5 flex items-center gap-1.5">
              <b className="text-10p5">Proposed fix</b>
              <Pill tone="ai">prp_7c31a2</Pill>
            </div>
            <div className="mono rounded-[5px] bg-mono-bg p-[7px_9px] text-10 text-ink1">
              CREATE INDEX CONCURRENTLY idx_orders_customer_created ON orders (customer_id,
              created_at);
            </div>
            <div className="mt-2">
              <Link
                to="/$org/$project/svc/$service/insights"
                params={{ org, project: "ecommerce", service: "db-main" }}
                search={W10_SEARCH}
              >
                <Btn variant="s" className="h-6 border-assist text-10 text-assist">
                  Review proposal (W10)
                </Btn>
              </Link>
            </div>
          </div>
        </>
      ),
    },
    { id: "msg_seed_2", role: "user", body: "Will it lock the table?" },
    {
      id: "msg_seed_3",
      role: "assistant",
      body: (
        <>
          No — <span className="mono text-10p5">CONCURRENTLY</span> builds without an exclusive
          lock, so reads and writes keep flowing. ~40 s on 1.2M rows. Written as migration{" "}
          <span className="mono text-10p5">0142</span>, reversible via{" "}
          <span className="mono text-10p5">DROP INDEX</span>. I won't run it — you approve it in
          Deploy.
        </>
      ),
    },
  ];
}

const EARLIER_THREADS = [
  { title: "Cache strategy for sessions", when: "Mar 4" },
  { title: "Explain the p95 alert", when: "Mar 3" },
  { title: "Which region for EU users?", when: "Mar 1" },
];

function AskPage() {
  const { org } = Route.useParams();
  const createThread = useCreateThread();
  const post = usePostMessage(CANON_THREAD);
  const [messages, setMessages] = useState<ChatMsg[]>(() => seedMessages(org));
  const [draft, setDraft] = useState("");

  const newConversation = () =>
    createThread.mutate(
      { body: { context: { org, project: "ecommerce", env: "production" } } },
      {
        onSuccess: (t) => toast.success(`New conversation started — "${t.title}"`),
        onError: (err) => toast.error(errorMessage(err)),
      },
    );

  const send = () => {
    const content = draft.trim();
    if (!content || post.isPending) return;
    post.mutate(
      { path: { thread: CANON_THREAD }, body: { content } },
      {
        onSuccess: (reply) => {
          setMessages((m) => [
            ...m,
            { id: `msg_u_${m.length}`, role: "user", body: content },
            { id: reply.id, role: "assistant", body: reply.content },
          ]);
          setDraft("");
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );
  };

  return (
    <main className="main">
      <div className="pgpad">
        <Pghead
          title="Ask"
          sub="Conversations scoped to ecommerce / production — I answer with evidence and propose; you approve."
        >
          <Btn variant="s" onClick={newConversation} disabled={createThread.isPending}>
            <Icon id="s-plus" />
            New conversation
          </Btn>
        </Pghead>

        <div className="flex min-h-0 flex-1 gap-3">
          {/* conversation rail */}
          <Card className="flex w-[262px] shrink-0 flex-col overflow-hidden p-0">
            <div className="border-hair border-b p-[11px_12px]">
              <Inp placeholder="Search conversations" className="h-8 text-11" />
            </div>
            <div className="flex-1 overflow-auto p-2">
              <div className="nsec pl-1.5">Today</div>
              <div className="cursor-pointer rounded-lg bg-steel-tint p-[8px_10px]">
                <div className="truncate text-11p5 font-medium text-ink1">
                  Why is my database slow?
                </div>
                <div className="mt-0.5 text-10 text-ink3">2 follow-ups · 14:12</div>
              </div>
              <div className="cursor-pointer rounded-lg p-[8px_10px]">
                <div className="truncate text-11p5 text-ink2">Cheaper config for staging</div>
                <div className="mt-0.5 text-10 text-ink3">11:40</div>
              </div>
              <div className="nsec mt-2 pl-1.5">Earlier this week</div>
              {EARLIER_THREADS.map((t) => (
                <div key={t.title} className="cursor-pointer rounded-lg p-[8px_10px]">
                  <div className="truncate text-11p5 text-ink2">{t.title}</div>
                  <div className="mt-0.5 text-10 text-ink3">{t.when}</div>
                </div>
              ))}
            </div>
          </Card>

          {/* the thread */}
          <Card className="flex min-w-0 flex-1 flex-col overflow-hidden p-0">
            <div className="flex items-center gap-2.5 border-hair border-b p-[11px_15px]">
              <div className="flex-1">
                <div className="text-12p5 font-semibold">Why is my database slow?</div>
                <div className="text-10 text-ink3">
                  started 14:12 · reads metrics, logs, traces within your permissions
                </div>
              </div>
            </div>
            <div className="flex-1 overflow-auto p-[13px_16px]">
              {messages.map((m) =>
                m.role === "assistant" ? (
                  <AssistantBubble key={m.id}>{m.body}</AssistantBubble>
                ) : (
                  <UserBubble key={m.id}>{m.body}</UserBubble>
                ),
              )}
            </div>
            <div className="flex items-center gap-2 border-hair border-t p-[11px_14px]">
              <Inp
                placeholder="Reply — or “turn this into a proposal”…"
                className="h-9 flex-1 text-11p5"
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") send();
                }}
              />
              <Btn variant="p" onClick={send} disabled={post.isPending || !draft.trim()}>
                Send
              </Btn>
            </div>
          </Card>

          {/* sources rail */}
          <Card className="flex w-[248px] shrink-0 flex-col gap-3.5 overflow-auto p-[14px_15px]">
            <div>
              <Eyebrow className="mb-2">Sources I used</Eyebrow>
              <div className="logwell p-[9px_11px]">
                <div>
                  <span className="t">metric&nbsp;&nbsp;</span>api p95 812 ms (O2)
                </div>
                <div>
                  <span className="t">trace&nbsp;&nbsp;&nbsp;</span>tr_8814 · 79% one span (O6)
                </div>
                <div>
                  <span className="t">plan&nbsp;&nbsp;&nbsp;&nbsp;</span>Seq Scan · 1.2M rows
                </div>
              </div>
            </div>
            <div>
              <Eyebrow className="mb-2">From this conversation</Eyebrow>
              <div className="rounded-[9px] border border-hair p-[11px_12px]">
                <div className="mb-1 flex items-center gap-1.5">
                  <Pill tone="ai">proposal</Pill>
                  <span className="mono text-10 text-ink3">prp_7c31a2</span>
                </div>
                <div className="text-10p5 font-medium text-ink1">Add index to orders</div>
                <div className="mt-0.5 text-10 text-ink3">p95 812 → 431 ms</div>
                <div className="mt-2">
                  <Link
                    to="/$org/$project/svc/$service/insights"
                    params={{ org, project: "ecommerce", service: "db-main" }}
                    search={W10_SEARCH}
                  >
                    <Btn
                      variant="s"
                      className="h-6 w-full justify-center border-assist text-10 text-assist"
                    >
                      Review
                    </Btn>
                  </Link>
                </div>
              </div>
            </div>
            <div>
              <Eyebrow className="mb-1.5">Do more</Eyebrow>
              <div className="flex flex-col items-stretch gap-1.5">
                {["Turn into a proposal", "Share conversation", "Export as runbook"].map((c) => (
                  <span key={c} className="chip justify-start" style={{ fontFamily: "Inter" }}>
                    {c}
                  </span>
                ))}
              </div>
            </div>
          </Card>
        </div>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/assistant/ask")({
  component: AskPage,
});
