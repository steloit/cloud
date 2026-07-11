import { createFileRoute, Link, useRouter } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Eyebrow } from "@/design-system/eyebrow";
import { Glyph } from "@/design-system/glyph";
import { Icon } from "@/design-system/icon";
import { Inp } from "@/design-system/inp";
import { Pill } from "@/design-system/pill";
import { useProposal, useUpdateInsight } from "@/features/assistant/hooks";
import { useServices } from "@/features/services/hooks";
import { errorMessage } from "@/lib/api";
import { useUIStore } from "@/lib/store";

/**
 * AI6/AI7/AI8 · the per-product AI-insights page — the SAME .prop grammar as
 * W10 (evidence → reasoning → proposed change → impact → human-only apply),
 * one product-specific proposal each: Valkey eviction (prp_ca11e0), Storage
 * lifecycle (prp_st77a4), Queue dead letters (prp_qu20b9). Products without
 * a fixture proposal get an honest empty card, never a fake one.
 */

interface FrameContent {
  prp: string;
  insightId: string;
  /** phead title after "Proposal · " */
  title: ReactNode;
  evidence: ReactNode;
  reasoning: ReactNode;
  changeLabel: string;
  change: ReactNode;
  impact: ReactNode;
}

const T = ({ children }: { children: ReactNode }) => <span className="t">{children}</span>;

const FRAMES: Record<string, FrameContent> = {
  valkey: {
    prp: "prp_ca11e0",
    insightId: "ins_valkey_mem",
    title: (
      <>
        Switch eviction to <span className="mono">allkeys-lru</span>, raise maxmemory
      </>
    ),
    evidence: (
      <>
        <div>
          <T>memory&nbsp;&nbsp;&nbsp;</T>used 1.9 GB of 2.0 GB (95%) · maxmemory-policy ={" "}
          <span className="lv-w">noeviction</span>
        </div>
        <div>
          <T>errors&nbsp;&nbsp;&nbsp;&nbsp;</T>3.1k OOM-command rejects/hr since 14:10 — writes
          failing, not evicting
        </div>
        <div>
          <T>keys&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;</T>hit rate 88% · session:* 61% · cart:* 22% ·
          40% of keys have no TTL
        </div>
      </>
    ),
    reasoning: (
      <>
        With <span className="mono">noeviction</span> and memory full, new writes are rejected (the
        OOM errors), not old keys dropped. <b>session:*</b> and <b>cart:*</b> are cache-safe to
        evict by LRU — the app treats a miss as a cold read. Raising maxmemory to 3 GB restores
        headroom; <span className="mono">allkeys-lru</span> ends the write failures.
      </>
    ),
    changeLabel: "Proposed change",
    change: (
      <>
        <div>
          <T>-</T> maxmemory-policy <span className="lv-w">noeviction</span>
        </div>
        <div>
          <T>+</T> maxmemory-policy <span className="text-ok">allkeys-lru</span>
        </div>
        <div>
          <T>-</T> maxmemory <span className="lv-w">2 GB</span>
        </div>
        <div>
          <T>+</T> maxmemory <span className="text-ok">3 GB</span>
        </div>
      </>
    ),
    impact: (
      <>
        write failures → <b>0</b> · hit rate ≈ unchanged · cost <b>+$11/mo</b> (2→3 GB)
      </>
    ),
  },
  storage: {
    prp: "prp_st77a4",
    insightId: "ins_storage_cold",
    title: <>Lifecycle — expire temp uploads, tier cold originals</>,
    evidence: (
      <>
        <div>
          <T>size&nbsp;&nbsp;&nbsp;&nbsp;</T>48.1 GB · +2.3 GB/wk · $9/mo
        </div>
        <div>
          <T>age&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;</T>31 GB not read in 30d · mostly{" "}
          <span className="mono">uploads/tmp/*</span>
        </div>
        <div>
          <T>access&nbsp;&nbsp;</T>
          <span className="mono">uploads/tmp/*</span> write-once — 0 GETs after 48h ·{" "}
          <span className="mono">originals/*</span> re-read weekly
        </div>
      </>
    ),
    reasoning: (
      <>
        <span className="mono">uploads/tmp/*</span> are written once and never read after two days —
        safe to expire at 7 days. 22 GB of <span className="mono">originals/*</span> older than 30
        days are read rarely; moving them to infrequent-access cuts cost with only a first-read
        latency penalty. No deletes yet — these are rules you review.
      </>
    ),
    changeLabel: "Proposed change",
    change: (
      <>
        <div>
          <T>rule 1&nbsp;&nbsp;</T>prefix <span className="mono">uploads/tmp/</span> →{" "}
          <span className="text-ok">expire after 7 days</span>
        </div>
        <div>
          <T>rule 2&nbsp;&nbsp;</T>prefix <span className="mono">originals/</span> →{" "}
          <span className="text-ok">infrequent-access after 30 days</span>
        </div>
        <div>
          <T>dry-run&nbsp;</T>142k objects match · 0 deleted now
        </div>
      </>
    ),
    impact: (
      <>
        −18 GB active · cost <b>−$3.20/mo</b> · fully reversible (rules only)
      </>
    ),
  },
  queue: {
    prp: "prp_qu20b9",
    insightId: "ins_queue_dlq",
    title: (
      <>
        Fix &amp; replay 2 dead letters (<span className="mono">order.paid</span>)
      </>
    ),
    evidence: (
      <>
        <div>
          <T>dlq&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;</T>2 messages ·
          both <span className="mono">order.paid</span> · since 14:03
        </div>
        <div>
          <T>cause&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;</T>
          <span className="lv-w">TypeError: cannot read 'total' of null</span> —{" "}
          <span className="mono">"receipt":null</span> in payload
        </div>
        <div>
          <T>correlation&nbsp;&nbsp;</T>both from deploy <span className="mono">#142</span> —
          receipt omitted for gift-card orders · retries 5/5 exhausted
        </div>
      </>
    ),
    reasoning: (
      <>
        The two dead letters share one root cause: deploy <b>#142</b> emits{" "}
        <span className="mono">order.paid</span> without a receipt for gift-card orders, and the
        worker assumes one is present. This is a <b>code fix, not a retry problem</b> — replaying
        now would fail again. The fix is in deploy #143; once it ships, these two messages replay
        cleanly.
      </>
    ),
    changeLabel: "Proposed change",
    change: (
      <>
        <div>
          <T>step 1&nbsp;&nbsp;</T>guard null receipt in worker — drafted, ships via{" "}
          <span className="text-prov">deploy #143</span>
        </div>
        <div>
          <T>step 2&nbsp;&nbsp;</T>after #143: <span className="text-ok">replay 2 messages</span>{" "}
          (idempotent — keyed by order_id)
        </div>
        <div>
          <T>step 3&nbsp;&nbsp;</T>add alert: DLQ depth &gt; 0 (Alerts)
        </div>
      </>
    ),
    impact: (
      <>
        unblocks <b>2 paid orders</b> · no retry-config change · replay is idempotent
      </>
    ),
  },
};

const PRODUCT_SUB: Record<string, string> = {
  valkey: "Valkey · AI insights",
  storage: "Object Storage · AI insights",
  queue: "Queue · AI insights",
};

function DismissLogged({ insightId, prp }: { insightId: string; prp: string }) {
  const update = useUpdateInsight();
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");

  const dismiss = () =>
    update.mutate(
      { path: { ins: insightId }, body: { status: "dismissed", reason } },
      {
        onSuccess: () => {
          setOpen(false);
          toast.success(`Dismissed — reason logged as you (${prp})`);
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );

  return (
    <span className="flex items-center gap-2">
      <Btn variant="gh" className="text-ink3" onClick={() => setOpen((v) => !v)}>
        Dismiss (logged)
      </Btn>
      {open ? (
        <>
          <Inp
            placeholder="Reason — logged"
            className="h-8 w-[200px] text-[11px]"
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
            Confirm
          </Btn>
        </>
      ) : null}
    </span>
  );
}

function ProposalPanel({
  org,
  project,
  service,
  env,
  product,
  frame,
}: {
  org: string;
  project: string;
  service: string;
  env: string;
  product: string;
  frame: FrameContent;
}) {
  const router = useRouter();
  const proposal = useProposal(frame.prp);
  const openAssistant = useUIStore((s) => s.openAssistant);
  const applied = proposal.data?.status === "applied";

  // The Lifecycle rules leaf (D15) is owned by the product wave — a raw href
  // keeps this file decoupled from that route's registration at typecheck time.
  const goLifecycle = () =>
    router.history.push(`/${org}/${project}/svc/${service}/lifecycle?env=${env}`);

  return (
    <div className="flex gap-3.5">
      <div className="prop flex flex-1 flex-col">
        <div className="phead">
          <Icon id="s-ai" />
          <span>Proposal · {frame.title}</span>
          <span className="ml-auto flex items-center gap-2">
            <button
              type="button"
              className="chip border-assist/40 text-assist"
              style={{ fontFamily: "Inter" }}
              onClick={() => openAssistant(frame.insightId)}
            >
              <Icon id="s-ai" className="h-[11px] w-[11px]" />
              Ask a follow-up
            </button>
            <Pill tone="ai">
              {frame.prp} · {applied ? "applied" : "awaiting your review"}
            </Pill>
          </span>
        </div>
        <div className="flex flex-col gap-3.5 p-4">
          <div>
            <Eyebrow>Evidence — what I read</Eyebrow>
            <div className="logwell mt-2">{frame.evidence}</div>
          </div>
          <div>
            <Eyebrow>Reasoning</Eyebrow>
            <p className="mt-1.5 text-[11.5px] leading-relaxed text-ink2">{frame.reasoning}</p>
          </div>
          <div>
            <Eyebrow>{frame.changeLabel}</Eyebrow>
            <div className="logwell mt-2">{frame.change}</div>
          </div>
          <div>
            <Eyebrow>Impact</Eyebrow>
            <div className="mt-1.5 text-[11px] text-ink2">{frame.impact}</div>
          </div>
        </div>
        <div className="mt-auto flex items-center gap-2 border-hair border-t p-[12px_16px]">
          {product === "valkey" ? (
            <>
              <Btn variant="p" disabled disabledReason="Settings apply flow lands in Phase 5">
                <Icon id="s-gear" />
                Open in Settings (D14)
              </Btn>
              <Btn
                variant="s"
                disabled
                disabledReason="Law 1 — human apply lands with the settings flow"
              >
                Apply in staging first
              </Btn>
            </>
          ) : product === "storage" ? (
            <>
              <Btn variant="p" onClick={goLifecycle}>
                <Icon id="s-doc" />
                Open in Lifecycle rules (D15)
              </Btn>
              <Btn variant="s" onClick={goLifecycle}>
                Run dry-run
              </Btn>
            </>
          ) : (
            <>
              <Link to="/$org/$project/deploy" params={{ org, project }} search={{ env }}>
                <Btn variant="p">
                  <Icon id="s-deploy" />
                  Open the fix in Deploy (DP1)
                </Btn>
              </Link>
              <Btn
                variant="s"
                className="opacity-50"
                disabled
                disabledReason="replay unblocks when #143 is live"
              >
                Replay — after #143
              </Btn>
            </>
          )}
          <DismissLogged insightId={frame.insightId} prp={frame.prp} />
          <span className="ml-auto text-[10.5px] text-ink3">
            Applied as <b>you, via assistant</b> — same permissions, same audit
          </span>
        </div>
      </div>

      <div className="flex w-[300px] shrink-0 flex-col gap-3">
        <Card className="flex flex-col gap-1.5 p-3.5">
          <Eyebrow>The four laws, here</Eyebrow>
          <div className="flex flex-col gap-1.5 text-[11px] leading-normal text-ink2">
            <div>
              <b className="text-assist">Suggest</b> — I drafted this; you apply it
            </div>
            <div>
              <b className="text-assist">Explainable</b> — evidence is shown above
            </div>
            <div>
              <b className="text-assist">In-permission</b> — applied through the normal flow
            </div>
            <div>
              <b className="text-assist">Optional</b> — disable in Policies (AI3)
            </div>
          </div>
        </Card>
        <Card className="flex flex-col gap-1.5 p-3.5">
          <Eyebrow>Same pattern, every product</Eyebrow>
          <p className="text-[11px] leading-relaxed text-ink3">
            Identical to the PostgreSQL index proposal (W10) and the others — one visual +
            interaction grammar for every AI recommendation.
          </p>
        </Card>
      </div>
    </div>
  );
}

function ServiceAiPage() {
  const { org, project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(env);

  const svc = services.data?.find((s) => s.name === service || s.id === service);
  const product = svc?.product ?? "";
  const frame = FRAMES[product];

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          before={<Glyph id="s-ai" />}
          title={service}
          sub={<span className="mono">{PRODUCT_SUB[product] ?? "AI insights"}</span>}
        >
          <span className="envpill">
            <Icon id="s-env" />
            {env}
            <Icon id="s-chevd" className="h-[11px] w-[11px]" />
          </span>
        </Pghead>

        {frame ? (
          <ProposalPanel
            org={org}
            project={project}
            service={service}
            env={env}
            product={product}
            frame={frame}
          />
        ) : (
          <Card className="flex max-w-[560px] flex-col gap-2 p-4">
            <div className="flex items-center gap-2">
              <Glyph id="s-ai" />
              <b className="text-[12.5px]">No AI recommendation for {service} right now</b>
            </div>
            <p className="text-[11.5px] leading-relaxed text-ink2">
              The assistant only surfaces a proposal when the evidence supports one — nothing is
              generated to fill the page. When one exists it renders here in the same grammar as
              every other product: evidence → reasoning → proposed change → impact → your apply.
            </p>
            <div>
              <Link to="/$org/assistant/insights" params={{ org }}>
                <Btn variant="s">See all Insights (AI10)</Btn>
              </Link>
            </div>
          </Card>
        )}
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/ai")({
  component: ServiceAiPage,
});
