import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import { useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { EmptyState } from "@/design-system/empty-state";
import { Icon, type IconId } from "@/design-system/icon";
import { Inp } from "@/design-system/inp";
import { Metric } from "@/design-system/metric";
import { Pill, type PillTone } from "@/design-system/pill";
import { SkeletonLines } from "@/design-system/skeleton";
import { useInsights, useUpdateInsight } from "@/features/assistant/hooks";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { errorMessage, type Insight, listInsightsQueryKey } from "@/lib/api";
import { useUIStore } from "@/lib/store";
import { cn } from "@/lib/utils";

/**
 * AI10 · Insights — the recommendations inbox: metric summary, status +
 * category filters, ranked cards. Rows come from GET /assistant/insights
 * joined with a frame-fixed display map; Snooze/Dismiss write through
 * PATCH /assistant/insights/:ins (logged, Law 2).
 *
 * Findings vs fixtures: the frame's chip counts (Open 3 / Applied 1 /
 * Dismissed 1) treat the queue insight as open, but fixtures ship it as
 * status=applied ("fix shipped in #143; replayed") — the working filter
 * follows the live data; the chip labels stay frame-verbatim. The dismissed
 * Worker row (prp_wk03c1) is not in fixtures — rendered statically.
 */

type Display = {
  bar: string;
  icon: IconId;
  title: string;
  desc: ReactNode;
  sevPill: { tone: PillTone; label: string };
  category: "reliability" | "performance" | "cost" | "security";
  evidence: string;
  impact: string;
  prp: string;
  /** Where "Review →" lands: the owning product's AI page, or W10 for pg. */
  review: { service: string; page: "ai" | "insights" };
};

const DISPLAY: Record<string, Display> = {
  ins_queue_dlq: {
    bar: "var(--err)",
    icon: "s-queue",
    title: "Queue — 2 dead letters blocking paid orders",
    desc: (
      <>
        Both <span className="mono text-10">order.paid</span>, from #142’s missing gift-card
        receipt. A code fix, not a retry — ships via #143, then replay.
      </>
    ),
    sevPill: { tone: "err", label: "reliability · high" },
    category: "reliability",
    evidence: "evidence: DLQ + trace",
    impact: "unblocks 2 paid orders",
    prp: "prp_qu20b9",
    review: { service: "jobs", page: "ai" },
  },
  ins_valkey_mem: {
    bar: "var(--warn)",
    icon: "s-chip",
    title: "Valkey — cache full, rejecting writes",
    desc: (
      <>
        Memory 95% with <span className="mono text-10">noeviction</span> — 3.1k OOM rejects/hr.
        Switch to <span className="mono text-10">allkeys-lru</span>, raise maxmemory 2→3 GB.
      </>
    ),
    sevPill: { tone: "warn", label: "reliability · high" },
    category: "reliability",
    evidence: "evidence: memory + errors",
    impact: "write failures → 0 · +$11/mo",
    prp: "prp_ca11e0",
    review: { service: "cache", page: "ai" },
  },
  ins_storage_cold: {
    bar: "var(--steel)",
    icon: "s-bucket",
    title: "Object Storage — temp files never expire",
    desc: (
      <>
        31 GB unread in 30 days; <span className="mono text-10">uploads/tmp/*</span> is write-once.
        Lifecycle rules — no deletes yet, reversible.
      </>
    ),
    sevPill: { tone: "st", label: "cost · medium" },
    category: "cost",
    evidence: "evidence: age + access",
    impact: "−$3.20/mo · −18 GB",
    prp: "prp_st77a4",
    review: { service: "assets", page: "ai" },
  },
  ins_pg_orders: {
    bar: "var(--ok)",
    icon: "s-db",
    title: "PostgreSQL — missing index on orders",
    desc: (
      <>
        Seq scan over 1.2M rows (642 ms) driving the p95 alert. Index drafted, shipping through the
        incident fix.
      </>
    ),
    sevPill: { tone: "st", label: "performance" },
    category: "performance",
    evidence: "evidence: query + plan",
    impact: "p95 812 → 431 ms",
    prp: "prp_7c31a2",
    review: { service: "db-main", page: "insights" },
  },
};

/** Frame order — impact-ranked, not fixture order. */
const ROW_ORDER = ["ins_queue_dlq", "ins_valkey_mem", "ins_storage_cold", "ins_pg_orders"];

const STATUS_CHIPS = [
  { key: "open", label: "Open 3" },
  { key: "applied", label: "Applied 1" },
  { key: "dismissed", label: "Dismissed 1" },
] as const;

const CATEGORY_CHIPS = ["All", "Reliability", "Performance", "Cost", "Security"] as const;

/** House DropdownMenu classes (the ctx-bar pattern), sized for a small verb menu. */
const menuContentClass =
  "z-50 min-w-[176px] rounded-xl border border-hair bg-surface p-1 shadow-e2";
const menuItemClass =
  "flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-12p5 font-medium outline-none data-[highlighted]:bg-surface2";

/**
 * Snooze durations — the PATCH body carries no duration field (finding):
 * every option sends the same status=snoozed mutation; the duration lives
 * only in the menu and the toast.
 */
const SNOOZE_OPTIONS = [
  { label: "1 day", toast: "for 1 day" },
  { label: "7 days", toast: "for 7 days" },
  { label: "until the next deploy", toast: "until the next deploy" },
] as const;

function InsightRow({
  org,
  insight,
  display,
}: {
  org: string;
  insight: Insight;
  display: Display;
}) {
  const queryClient = useQueryClient();
  const update = useUpdateInsight();
  const openAssistant = useUIStore((s) => s.openAssistant);
  const [dismissing, setDismissing] = useState(false);
  const [reason, setReason] = useState("");

  const applied = insight.status === "applied";

  const patchCache = (status: Insight["status"]) =>
    queryClient.setQueryData(
      listInsightsQueryKey(),
      (prev: { data?: Insight[] } | undefined) =>
        prev && {
          ...prev,
          data: (prev.data ?? []).map((i) => (i.id === insight.id ? { ...i, status } : i)),
        },
    );

  const snooze = (duration: string) =>
    update.mutate(
      { path: { ins: insight.id }, body: { status: "snoozed" } },
      {
        onSuccess: () => {
          patchCache("snoozed");
          toast.success(`Snoozed ${duration} — ${display.prp} stays open, out of the ranked list`);
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );

  const dismiss = () =>
    update.mutate(
      { path: { ins: insight.id }, body: { status: "dismissed", reason } },
      {
        onSuccess: () => {
          patchCache("dismissed");
          setDismissing(false);
          toast.success(`Dismissed — reason logged as you (${display.prp})`);
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );

  return (
    <Card className="flex overflow-hidden p-0">
      <div className="w-1 shrink-0" style={{ background: display.bar }} />
      <div className="flex-1 p-[13px_15px]">
        <div className="flex items-center gap-2">
          <span className="glyph h-6 w-6">
            <Icon id={display.icon} className="h-3 w-3" />
          </span>
          <b className="text-12p5">{display.title}</b>
          <Pill tone={display.sevPill.tone}>{display.sevPill.label}</Pill>
          <span className="sp flex-1" />
          <span className="mono text-10 text-ink3">{display.prp}</span>
        </div>
        <div className="mt-1.5 text-11 leading-relaxed text-ink2">{display.desc}</div>
        <div className="mt-2.5 flex items-center gap-2">
          <Pill tone="ai">{display.evidence}</Pill>
          <span className="text-10p5 text-ink2">{display.impact}</span>
          <span className="sp flex-1" />
          {applied ? <Pill tone="ok">applied · #143</Pill> : null}
          <Link
            to={
              display.review.page === "ai"
                ? "/$org/$project/svc/$service/ai"
                : "/$org/$project/svc/$service/insights"
            }
            params={{ org, project: "ecommerce", service: display.review.service }}
            search={{ env: "production" }}
          >
            <Btn variant="s" className="h-[26px] text-10p5">
              Review →
            </Btn>
          </Link>
          <Btn
            variant="gh"
            className="h-[26px] text-10p5 text-assist"
            onClick={() => openAssistant(insight.id)}
          >
            <Icon id="s-ai" />
            Ask a follow-up
          </Btn>
          {!applied ? (
            <>
              <DropdownMenu.Root>
                <DropdownMenu.Trigger asChild>
                  <Btn
                    variant="gh"
                    className="h-[26px] text-10p5"
                    disabled={update.isPending}
                    disabledReason="Snoozing…"
                  >
                    Snooze
                    <Icon id="s-chevd" className="h-[9px] w-[9px]" />
                  </Btn>
                </DropdownMenu.Trigger>
                <DropdownMenu.Portal>
                  <DropdownMenu.Content className={menuContentClass} align="end" sideOffset={6}>
                    {SNOOZE_OPTIONS.map((o) => (
                      <DropdownMenu.Item
                        key={o.label}
                        className={menuItemClass}
                        onSelect={() => snooze(o.toast)}
                      >
                        {o.label}
                      </DropdownMenu.Item>
                    ))}
                  </DropdownMenu.Content>
                </DropdownMenu.Portal>
              </DropdownMenu.Root>
              <Btn
                variant="gh"
                className="h-[26px] text-10p5 text-ink3"
                onClick={() => setDismissing((v) => !v)}
              >
                Dismiss
              </Btn>
            </>
          ) : null}
        </div>
        {dismissing ? (
          <div className="mt-2.5 flex items-center gap-2 border-hair border-t pt-2.5">
            <Inp
              placeholder="Reason — logged with the dismissal"
              className="h-8 flex-1 text-11"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
            />
            <Btn
              variant="s"
              className="h-8"
              onClick={dismiss}
              disabled={!reason.trim() || update.isPending}
              disabledReason="Dismissals carry a logged reason"
            >
              Dismiss (logged)
            </Btn>
          </div>
        ) : null}
      </div>
    </Card>
  );
}

/** prp_wk03c1 is frame-only — not in fixtures; rendered statically, dimmed. */
function DismissedWorkerRow() {
  return (
    <Card className="flex overflow-hidden p-0 opacity-60">
      <div className="w-1 shrink-0" style={{ background: "var(--ink3)" }} />
      <div className="flex-1 p-[13px_15px]">
        <div className="flex items-center gap-2">
          <span className="glyph h-6 w-6">
            <Icon id="s-worker" className="h-3 w-3" />
          </span>
          <b className="text-12p5">Worker — over-provisioned off-peak</b>
          <Pill tone="mut">cost · low</Pill>
          <span className="sp flex-1" />
          <span className="mono text-10 text-ink3">prp_wk03c1</span>
        </div>
        <div className="mt-1.5 text-11 leading-relaxed text-ink2">
          CPU &lt; 12% between 01:00–06:00. Suggested scale-to-1 — dismissed Mar 2.
        </div>
        <div className="mt-2.5 flex items-center gap-2">
          <Pill tone="ai">evidence: utilization</Pill>
          <span className="text-10p5 text-ink2">−$7/mo (dismissed)</span>
        </div>
      </div>
    </Card>
  );
}

function InsightsPage() {
  const { org } = Route.useParams();
  const insights = useInsights();
  const [status, setStatus] = useState<"open" | "applied" | "dismissed">("open");
  const [category, setCategory] = useState<(typeof CATEGORY_CHIPS)[number]>("All");

  const rows = ROW_ORDER.map((id) => {
    const insight = (insights.data ?? []).find((i) => i.id === id);
    const display = DISPLAY[id];
    return insight && display ? { insight, display } : null;
  })
    .filter((r) => r != null)
    .filter((r) => r.insight.status === status)
    .filter((r) => category === "All" || r.display.category === category.toLowerCase());

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          title="Insights"
          sub="Recommendations across ecommerce, ranked by impact — you review and approve each one."
        >
          <div className="chiprow">
            {STATUS_CHIPS.map((c) => (
              <button
                key={c.key}
                type="button"
                aria-pressed={status === c.key}
                className={cn("chip", status === c.key && "on")}
                style={{ fontFamily: "Inter" }}
                onClick={() => setStatus(c.key)}
              >
                {c.label}
              </button>
            ))}
          </div>
        </Pghead>

        <div className="grid grid-cols-4 gap-3">
          <Metric label="Open" value="3" note="2 high · 1 medium" mono />
          <Metric label="Reliability fixes" value="2" note="queue · cache" mono />
          <Metric label="Cost identified" value="−$3/mo" note="net of +$11 reliability" mono />
          <Metric label="Applied · 7d" value="1" note="the orders index" mono />
        </div>

        <div className="chiprow">
          {CATEGORY_CHIPS.map((c) => (
            <button
              key={c}
              type="button"
              aria-pressed={category === c}
              className={cn("chip", category === c && "on")}
              style={{ fontFamily: "Inter" }}
              onClick={() => setCategory(c)}
            >
              {c}
            </button>
          ))}
        </div>

        {/* Four-state grammar (16-qa): the ranked list rides GET /assistant/insights;
            the metric tiles and chip counts above are frame-fixed (AI10) and stay. */}
        <div className="flex flex-col gap-2.5">
          {insights.isPending ? (
            Array.from({ length: 3 }, (_, i) => (
              // biome-ignore lint/suspicious/noArrayIndexKey: placeholder cards have no identity
              <Card key={i} className="p-4">
                <SkeletonLines lines={2} />
              </Card>
            ))
          ) : insights.isError ? (
            <ApiFailureCard
              title="Insights didn't load"
              error={insights.error}
              requestLine="GET /assistant/insights"
              onRetry={() => insights.refetch()}
            />
          ) : (
            <>
              {rows.map(({ insight, display }) => (
                <InsightRow key={insight.id} org={org} insight={insight} display={display} />
              ))}
              {status === "dismissed" && (category === "All" || category === "Cost") ? (
                <DismissedWorkerRow />
              ) : null}
              {rows.length === 0 && status !== "dismissed" ? (
                <EmptyState
                  compact
                  icon="s-ai"
                  title={`Nothing ${status} in this view`}
                  meaning={
                    <>
                      insights arrive as telemetry accumulates — the assistant ranks them by impact,
                      and each one waits for your review; switch the status or category chips above
                      to see the rest
                    </>
                  }
                />
              ) : null}
            </>
          )}
        </div>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/assistant/insights")({
  component: InsightsPage,
});
