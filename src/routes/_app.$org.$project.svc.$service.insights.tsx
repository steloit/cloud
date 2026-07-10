import { createFileRoute } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Eyebrow } from "@/design-system/eyebrow";
import { Glyph } from "@/design-system/glyph";
import { Icon } from "@/design-system/icon";
import { Pill } from "@/design-system/pill";

/**
 * W10 · AI proposal — evidence, reasoning, a reviewable diff; the apply
 * button belongs to the human (the four laws, ADR-005). Apply paths land
 * with the assistant in Phase 2 — disabled with the reason, never hidden.
 */
function InsightsPage() {
  const { service } = Route.useParams();
  const { env } = Route.useSearch();

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          before={<Glyph id="s-db" />}
          title={service}
          sub={<span className="mono">PostgreSQL 16 · Query insights</span>}
        >
          <span className="envpill">
            <Icon id="s-env" />
            {env}
            <Icon id="s-chevd" className="h-[11px] w-[11px]" />
          </span>
        </Pghead>

        <div className="flex gap-3.5">
          <div className="prop flex-1">
            <div className="phead">
              <Icon id="s-ai" />
              Proposal · Add index to <span className="mono">orders</span>
              <span className="ml-auto flex items-center gap-2">
                <span className="chip">Ask a follow-up</span>
                <Pill tone="ai">prp_7c31a2 · awaiting your review</Pill>
              </span>
            </div>
            <div className="flex flex-col gap-3.5 p-4">
              <div>
                <Eyebrow>Evidence — what I read</Eyebrow>
                <div className="logwell mt-2">
                  <div>
                    <span className="t">query&nbsp;&nbsp;&nbsp;</span>SELECT * FROM orders WHERE
                    customer_id = $1 ORDER BY created_at DESC
                  </div>
                  <div>
                    <span className="t">plan&nbsp;&nbsp;&nbsp;&nbsp;</span>Seq Scan on orders
                    (rows=1,204,318) · Sort · <span className="lv-w">642 ms avg</span> since deploy
                    #142
                  </div>
                  <div>
                    <span className="t">metrics&nbsp;</span>calls 214/min (was 3/min) · 61% of db
                    time · connections 96% of pool
                  </div>
                </div>
              </div>
              <div>
                <Eyebrow>Reasoning</Eyebrow>
                <p className="mt-1.5 text-[12px] leading-relaxed text-ink2">
                  Deploy <span className="mono">#142</span> made this query hot on every gift-card
                  page load. <span className="mono">orders.customer_id</span> has no index, so each
                  call scans 1.2M rows, holding connections long enough to saturate the pool — the
                  cause of the api latency alert.
                </p>
              </div>
              <div>
                <Eyebrow>
                  Proposed migration{" "}
                  <span className="font-normal normal-case tracking-normal">
                    — a draft, not an action
                  </span>
                </Eyebrow>
                <div className="diff mt-2">
                  <div className="dl ctxl">-- 0049_idx_orders_customer.sql</div>
                  <div className="dl add">
                    + CREATE INDEX CONCURRENTLY idx_orders_customer_created
                  </div>
                  <div className="dl add">
                    + &nbsp;&nbsp;ON orders (customer_id, created_at DESC);
                  </div>
                </div>
              </div>
              <div className="flex gap-5 text-[11.5px]">
                <span>
                  Predicted: <b>642 ms → ~4 ms</b>
                </span>
                <span>
                  Index size: <b>~96 MB</b> (+$0.02/mo)
                </span>
                <span>
                  Build: <b>non-blocking</b>, ~3 min
                </span>
              </div>
              <div className="flex items-center gap-2.5 border-hair border-t pt-3">
                <Btn
                  variant="p"
                  disabled
                  disabledReason="The assistant apply path lands in Phase 2"
                >
                  Open in preview branch
                </Btn>
                <Btn
                  variant="s"
                  disabled
                  disabledReason="The assistant apply path lands in Phase 2"
                >
                  Apply to production…
                </Btn>
                <Btn
                  variant="gh"
                  disabled
                  disabledReason="The assistant apply path lands in Phase 2"
                >
                  Dismiss (logged)
                </Btn>
                <span className="ml-auto text-[10.5px] text-ink3">
                  Applied changes are audited as <b>you, via assistant</b> — same permissions, same
                  confirmations.
                </span>
              </div>
            </div>
          </div>

          <div className="flex w-[300px] shrink-0 flex-col gap-3">
            <Card className="flex flex-col gap-2 p-3.5">
              <Eyebrow>The four laws, enforced here</Eyebrow>
              {[
                "AI proposes; you dispose — this card is a draft artifact.",
                "Explained & auditable — evidence and reasoning shown above.",
                "Reads broadly, writes narrowly — within your permissions only.",
                "Optional — an org Policy can turn this panel off entirely.",
              ].map((law) => (
                <div key={law} className="flex gap-2 text-[11.5px] text-ink2">
                  <span className="text-ok">✓</span>
                  {law}
                </div>
              ))}
            </Card>
            <Card className="flex flex-col gap-2 p-3.5">
              <Eyebrow>Suggested path</Eyebrow>
              <p className="text-[11.5px] leading-relaxed text-ink2">
                1 · Apply in <span className="mono">preview/pr-142</span> branch · 2 · Verify plan
                flips to Index Scan · 3 · Promote migration with deploy{" "}
                <span className="mono">#143</span>
              </p>
            </Card>
            <Card className="p-3.5">
              <p className="text-[11.5px] leading-relaxed text-ink3">
                Never proposed one-click: IAM, secrets, network exposure, deletions. Those the
                assistant only <i>describes how</i> to do.
              </p>
            </Card>
          </div>
        </div>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/insights")({
  component: InsightsPage,
});
