import { Link, useParams } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";
import { toast } from "sonner";
import { Btn } from "@/design-system/btn";
import { Icon } from "@/design-system/icon";
import { Inp } from "@/design-system/inp";
import { Dot, Pill } from "@/design-system/pill";
import { errorMessage } from "@/lib/api";
import { useUIStore } from "@/lib/store";
import { usePostMessage } from "./hooks";

/**
 * AI4 · the quick drawer (+ AI9 when an insight is attached). Mounted by the
 * shell (the orchestrator wires the mount and ⌘J); this component reads
 * assistantOpen / attachedInsight from the UI store. The drawer converses on
 * the canon thread — the same conversation the full page shows; expanding
 * loses nothing.
 */

const CANON_THREAD = "thr_db_slow";

// ?proposal= rides along on the W10 deep link; the route validates { env } only.
const W10_SEARCH = { env: "production", proposal: "prp_7c31a2" } as { env: string };

/** AI9 frames the valkey case; the other fixture insights get the same banner shape. */
const ATTACHED: Record<
  string,
  {
    icon: "s-chip" | "s-bucket" | "s-queue" | "s-db";
    title: ReactNode;
    sub: string;
    service: string;
  }
> = {
  ins_valkey_mem: {
    icon: "s-chip",
    title: (
      <>
        Valkey eviction → <span className="mono text-[10px]">allkeys-lru</span>
      </>
    ),
    sub: "prp_ca11e0 · cache · awaiting review",
    service: "cache",
  },
  ins_storage_cold: {
    icon: "s-bucket",
    title: "Storage lifecycle — expire temp uploads",
    sub: "prp_st77a4 · assets · awaiting review",
    service: "assets",
  },
  ins_queue_dlq: {
    icon: "s-queue",
    title: "Queue — fix & replay 2 dead letters",
    sub: "prp_qu20b9 · jobs · applied",
    service: "jobs",
  },
  ins_pg_orders: {
    icon: "s-db",
    title: "PostgreSQL — add index to orders",
    sub: "prp_7c31a2 · db-main · applied",
    service: "db-main",
  },
};

type ChatMsg = { id: string; role: "user" | "assistant"; body: ReactNode };

function AssistantBubble({ children }: { children: ReactNode }) {
  return (
    <div className="my-2 flex gap-2">
      <span className="glyph h-[22px] w-[22px] shrink-0 bg-assist-tint" aria-hidden>
        <Icon id="s-ai" className="h-[11px] w-[11px] text-assist" />
      </span>
      <div
        className="flex-1 rounded-[11px_11px_11px_3px] border p-[10px_12px] text-[11.5px] leading-relaxed text-ink1"
        style={{
          background: "color-mix(in srgb, var(--assist) 5%, var(--surface))",
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
      <div className="max-w-[80%] rounded-[11px_11px_3px_11px] border border-hair bg-surface2 p-[8px_11px] text-[11.5px] text-ink1">
        {children}
      </div>
    </div>
  );
}

function DrawerConversation({ org, attached }: { org: string; attached: string | null }) {
  const close = useUIStore((s) => s.closeAssistant);
  const post = usePostMessage(CANON_THREAD);
  const [draft, setDraft] = useState("");
  const [messages, setMessages] = useState<ChatMsg[]>(() =>
    attached
      ? [
          {
            id: "msg_seed_attached",
            role: "assistant",
            body: "I've loaded this recommendation's evidence and reasoning — ask anything about it.",
          },
        ]
      : [
          { id: "msg_seed_q", role: "user", body: "Why is my database slow?" },
          {
            id: "msg_seed_a",
            role: "assistant",
            body: (
              <>
                <div>
                  Short answer: an unindexed lookup on{" "}
                  <span className="mono text-[10.5px]">orders</span> after deploy{" "}
                  <span className="mono text-[10.5px]">#142</span>. p95 hit 812 ms; the scan alone
                  is 642 ms.
                </div>
                <div className="mt-1.5 flex flex-wrap gap-1.5">
                  <Pill tone="ai">3 sources</Pill>
                  <Pill tone="st">p95 812ms</Pill>
                  <Pill tone="warn">642ms SeqScan</Pill>
                </div>
                <div className="mt-2 flex gap-1.5">
                  <Link
                    to="/$org/$project/svc/$service/insights"
                    params={{ org, project: "ecommerce", service: "db-main" }}
                    search={W10_SEARCH}
                    onClick={close}
                  >
                    <Btn variant="s" className="h-6 border-assist text-[10px] text-assist">
                      See the fix (W10)
                    </Btn>
                  </Link>
                  <Link to="/$org/assistant/ask" params={{ org }} onClick={close}>
                    <Btn variant="gh" className="h-6 text-[10px]">
                      Open full Assistant ⤢
                    </Btn>
                  </Link>
                </div>
              </>
            ),
          },
        ],
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
    <>
      <div className="flex-1 overflow-auto p-[12px_14px]">
        {messages.map((m) =>
          m.role === "assistant" ? (
            <AssistantBubble key={m.id}>{m.body}</AssistantBubble>
          ) : (
            <UserBubble key={m.id}>{m.body}</UserBubble>
          ),
        )}
      </div>
      <div className="border-hair border-t p-[10px_13px]">
        <div className="flex items-center gap-2">
          <Inp
            placeholder="Ask about api / production…"
            className="h-8 flex-1 text-[11px]"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") send();
            }}
          />
          <Btn
            variant="p"
            className="h-8"
            onClick={send}
            disabled={post.isPending || !draft.trim()}
          >
            Send
          </Btn>
        </div>
        <div className="mt-1.5 text-center text-[9px] text-ink3">
          Same conversation expands to the full <b>Assistant</b> page — nothing is lost.
        </div>
      </div>
    </>
  );
}

export function AssistantDrawer() {
  const open = useUIStore((s) => s.assistantOpen);
  const attached = useUIStore((s) => s.attachedInsight);
  const close = useUIStore((s) => s.closeAssistant);
  const params = useParams({ strict: false }) as { org?: string };
  const org = params.org ?? "acme";

  if (!open) return null;
  const banner = attached ? ATTACHED[attached] : undefined;

  return (
    <aside
      className="fixed inset-y-0 right-0 z-40 flex w-[396px] flex-col border-hair border-l bg-surface shadow-e2"
      aria-label="Assistant"
    >
      <div className="flex items-center gap-2 border-hair border-b p-[12px_15px]">
        <span className="glyph h-6 w-6 bg-assist-tint">
          <Icon id="s-ai" className="h-3 w-3 text-assist" />
        </span>
        <div className="flex-1">
          <div className="text-[12.5px] font-semibold text-ink1">Assistant</div>
          <div className="text-[9px] text-ink3">quick help · doesn't act on its own</div>
        </div>
        <Link
          to="/$org/assistant/ask"
          params={{ org }}
          onClick={close}
          className="icb h-[26px] w-[26px]"
          title="Expand to full page"
        >
          <Icon id="s-grid" className="h-3.5 w-3.5" />
        </Link>
        <button
          type="button"
          className="icb h-[26px] w-[26px]"
          title="Close"
          aria-label="Close assistant"
          onClick={close}
        >
          <Icon id="s-x" className="h-[13px] w-[13px]" />
        </button>
      </div>

      <div className="flex items-center gap-2 border-hair border-b bg-steel-tint p-[8px_13px]">
        <Dot tone="ok" />
        <span className="text-[10px] text-ink2">
          Context: <b>api</b> · ecommerce / production
        </span>
        <span className="sp flex-1" />
        <span className="text-[9px] text-steel">change ▾</span>
      </div>

      {banner ? (
        <div
          className="border-hair border-b p-[10px_13px]"
          style={{ background: "color-mix(in srgb, var(--assist) 5%, transparent)" }}
        >
          <div className="mb-1.5 flex items-center gap-1.5">
            <Icon id="s-ai" className="h-[11px] w-[11px] text-assist" />
            <span className="text-[9px] font-semibold uppercase tracking-[.05em] text-assist">
              Attached insight
            </span>
            <span className="sp flex-1" />
            <span className="text-[9px] text-ink3">1 of 3 open ▾</span>
          </div>
          <div className="flex items-center gap-2">
            <span className="glyph h-5 w-5">
              <Icon id={banner.icon} className="h-2.5 w-2.5" />
            </span>
            <div className="flex-1">
              <div className="text-[11px] font-medium text-ink1">{banner.title}</div>
              <div className="text-[9px] text-ink3">{banner.sub}</div>
            </div>
            <Link
              to={
                banner.service === "db-main"
                  ? "/$org/$project/svc/$service/insights"
                  : "/$org/$project/svc/$service/ai"
              }
              params={{ org, project: "ecommerce", service: banner.service }}
              search={{ env: "production" }}
              onClick={close}
            >
              <Btn variant="gh" className="h-[22px] text-[9.5px]">
                View
              </Btn>
            </Link>
          </div>
        </div>
      ) : null}

      {/* keyed so switching between default and seeded mode re-seeds the thread */}
      <DrawerConversation key={attached ?? "default"} org={org} attached={attached} />
    </aside>
  );
}
