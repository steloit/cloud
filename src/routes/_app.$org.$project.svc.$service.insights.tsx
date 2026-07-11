import { createFileRoute } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { Banner } from "@/design-system/banner";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Eyebrow } from "@/design-system/eyebrow";
import { Glyph } from "@/design-system/glyph";
import { Icon } from "@/design-system/icon";
import { Pill, type PillTone } from "@/design-system/pill";
import { cn } from "@/lib/utils";

/**
 * D4 · Query Insights (default) + W10 · AI proposal (?proposal=prp_7c31a2).
 * 08-api has no pg_stat_statements endpoint — the D4 rows are frame-fixed
 * canon (spec gap, a finding). The W10 panel preserves the Phase-2 build:
 * evidence, reasoning, a reviewable diff; the apply button belongs to the
 * human (the four laws, ADR-005) — apply paths land with the assistant in
 * Phase 3, disabled with the reason, never hidden.
 */

interface QueryRow {
  query: string;
  calls: string;
  mean: string;
  p95: string;
  total: string;
  plan: { tone: PillTone; label: string } | null;
  regressed?: boolean;
}

const QUERY_ROWS: QueryRow[] = [
  {
    query: "SELECT * FROM orders WHERE customer_id = $1 ORDER BY created_at…",
    calls: "1,284",
    mean: "642 ms",
    p95: "1.4 s",
    total: "13.7 m",
    plan: { tone: "warn", label: "Seq Scan · 1.2M rows" },
    regressed: true,
  },
  {
    query: "UPDATE carts SET items = $1, updated_at = now() WHERE id = $2",
    calls: "9,412",
    mean: "4.1 ms",
    p95: "11 ms",
    total: "38.6 s",
    plan: { tone: "ok", label: "Index" },
  },
  {
    query: "SELECT id, sku, price FROM products WHERE category = $1 LIMIT 24",
    calls: "21,033",
    mean: "1.8 ms",
    p95: "6 ms",
    total: "37.9 s",
    plan: { tone: "ok", label: "Index" },
  },
  {
    query: "INSERT INTO events (type, payload) VALUES ($1, $2)",
    calls: "44,108",
    mean: "0.9 ms",
    p95: "3 ms",
    total: "39.7 s",
    plan: null,
  },
];

/** D4 — the pg_stat_statements list; the regressed row opens the proposal. */
function QueryInsightsList({ openProposal }: { openProposal: () => void }) {
  return (
    <>
      <Pghead
        title="Query Insights"
        sub={<span className="mono">db-main · pg_stat_statements · last 1h · production</span>}
      />

      <div className="chiprow">
        {["1h", "24h", "7d"].map((r) => (
          <span key={r} className={cn("chip", r === "1h" && "on")}>
            {r}
          </span>
        ))}
        <span className="chip">sort: total time ▾</span>
      </div>

      <Banner tone="warn">
        <span>
          Top query regressed at <span className="mono">14:02</span> — right after deploy{" "}
          <span className="mono">#142</span>. The assistant has traced it and drafted proposal{" "}
          <span className="mono">prp_7c31a2</span>.
        </span>
        <span className="flex-1" />
        <Btn variant="s" className="h-6 px-2.5 text-[10.5px]" onClick={openProposal}>
          View proposal →
        </Btn>
      </Banner>

      <div className="tblwrap">
        <table className="tbl">
          <thead>
            <tr>
              <th>Query</th>
              <th>Calls</th>
              <th>Mean</th>
              <th>p95</th>
              <th>Total</th>
              <th>Plan</th>
            </tr>
          </thead>
          <tbody>
            {QUERY_ROWS.map((row) => (
              <tr
                key={row.query}
                className={row.regressed ? "hov cursor-pointer" : undefined}
                style={row.regressed ? { background: "var(--warn-tint)" } : undefined}
                onClick={row.regressed ? openProposal : undefined}
              >
                <td className="mono text-[11px]">{row.query}</td>
                <td className="mono text-[11px]">{row.calls}</td>
                <td className={cn("mono text-[11px]", row.regressed && "text-warn")}>{row.mean}</td>
                <td className={cn("mono text-[11px]", row.regressed && "text-warn")}>{row.p95}</td>
                <td className="mono text-[11px]">{row.total}</td>
                <td>{row.plan ? <Pill tone={row.plan.tone}>{row.plan.label}</Pill> : "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <p className="text-[10.5px] text-ink3">
        Click a row for the full plan, sampled parameters, and history. EXPLAIN runs against a
        branch, never production.
      </p>
    </>
  );
}

/** W10 — the proposal panel, preserved verbatim from the Phase-2 build. */
function ProposalView({ service, env }: { service: string; env: string }) {
  return (
    <>
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
                  <span className="t">metrics&nbsp;</span>calls 214/min (was 3/min) · 61% of db time
                  · connections 96% of pool
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
              <Btn variant="p" disabled disabledReason="The assistant apply path lands in Phase 3">
                Open in preview branch
              </Btn>
              <Btn variant="s" disabled disabledReason="The assistant apply path lands in Phase 3">
                Apply to production…
              </Btn>
              <Btn variant="gh" disabled disabledReason="The assistant apply path lands in Phase 3">
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
    </>
  );
}

function InsightsPage() {
  const { service } = Route.useParams();
  const { env, proposal } = Route.useSearch();
  const navigate = Route.useNavigate();

  const openProposal = () => navigate({ search: (prev) => ({ ...prev, proposal: "prp_7c31a2" }) });

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        {proposal === "prp_7c31a2" ? (
          <ProposalView service={service} env={env} />
        ) : (
          <QueryInsightsList openProposal={openProposal} />
        )}
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/insights")({
  validateSearch: (search: Record<string, unknown>): { proposal?: string } =>
    typeof search.proposal === "string" ? { proposal: search.proposal } : {},
  component: InsightsPage,
});
