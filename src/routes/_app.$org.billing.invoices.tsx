import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { EmptyState } from "@/design-system/empty-state";
import { Icon } from "@/design-system/icon";
import { Pill } from "@/design-system/pill";
import { Skeleton, SkeletonRows } from "@/design-system/skeleton";
import { planLabel, useInvoices } from "@/features/billing/hooks";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { useOrgs } from "@/features/org/hooks";
import type { Invoice } from "@/lib/api";
import { fmtMoney } from "@/lib/fmt";
import { cn } from "@/lib/utils";

/** B3 · Invoices — finalized on the 1st; every one downloadable; an invoice is data. */

function downloadJson(inv: Invoice) {
  const url = URL.createObjectURL(
    new Blob([JSON.stringify(inv, null, 2)], { type: "application/json" }),
  );
  const a = document.createElement("a");
  a.href = url;
  a.download = `${inv.id}.json`;
  a.click();
  URL.revokeObjectURL(url);
}

/** "inv_2026_06" → "Invoice · Jun 2026" via the row's period. */
function invoiceTitle(inv: Invoice): string {
  return `Invoice · ${inv.period}`;
}

/**
 * Representative usage rows per invoice line — the footer's "every line
 * expands to the usage rows behind it" made real. The API's Invoice.lines
 * carries a usage_ref but no per-line usage breakdown endpoint (finding),
 * so these rows are display-grade samples in the B2 meter vocabulary;
 * they illustrate the shape, they don't reconcile to the line total.
 */
const LINE_USAGE: Record<
  string,
  Array<{ meter: string; qty: string; rate: string; amt: string }>
> = {
  "ecommerce — compute · databases · storage · network": [
    { meter: "compute_hours", qty: "1,012 S-hr", rate: "$0.031 / S-hr", amt: "$31.37" },
    { meter: "db_compute_hours", qty: "298 h", rate: "$0.058 / hr", amt: "$17.28" },
    { meter: "db_storage_gb_mo", qty: "84 GB-mo", rate: "$0.12 / GB-mo", amt: "$10.08" },
  ],
  "analytics-pipeline": [
    { meter: "compute_hours", qty: "486 S-hr", rate: "$0.031 / S-hr", amt: "$15.07" },
    { meter: "object_storage_gb_mo", qty: "41 GB-mo", rate: "$0.023 / GB-mo", amt: "$0.94" },
    { meter: "queue_requests_m", qty: "2.1 M", rate: "$0.40 / M", amt: "$0.44" },
  ],
  "internal-tools": [
    { meter: "compute_hours", qty: "212 S-hr", rate: "$0.031 / S-hr", amt: "$6.57" },
    { meter: "egress_gb", qty: "18 GB", rate: "$0.09 / GB", amt: "$1.62" },
  ],
  "Business plan · 20 seats included": [
    { meter: "plan_fee", qty: "1 mo", rate: "$99.00 / mo", amt: "$99.00" },
    { meter: "seats", qty: "12 of 20 included", rate: "—", amt: "$0.00" },
  ],
};

function InvoicesPage() {
  const { org } = Route.useParams();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);
  const invoices = useInvoices(org);
  const [selectedId, setSelectedId] = useState("inv_2026_06");
  // Inline accordion — lines toggle independently, keyed per invoice so
  // switching invoices doesn't carry expansion across.
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const toggleLine = (key: string) =>
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  const selected = (invoices.data ?? []).find((i) => i.id === selectedId);
  const lines = selected?.lines ?? [];
  const subtotal = lines.reduce((sum, l) => sum + (l.cents ?? 0), 0);

  // Four-state grammar (16-qa): pending → skeleton (list + panel), error →
  // failure card, empty → EmptyState, else the list. The detail panel never
  // vanishes silently: no matching invoice → an honest fallback line.
  const isEmpty = invoices.isSuccess && (invoices.data ?? []).length === 0;

  return (
    <>
      <SnavSettings
        org={org}
        orgName={orgRecord?.name ?? org}
        project="ecommerce"
        plan={orgRecord ? planLabel(orgRecord.plan) : "Business"}
        active="b-invoices"
      />
      <main className="main">
        <div className="pgpad">
          <Pghead
            title="Billing · Invoices"
            sub="Finalized on the 1st, paid on the default method — every one downloadable as PDF, CSV and JSON"
          />

          {invoices.isError ? (
            <ApiFailureCard
              title="Invoices didn't load"
              error={invoices.error}
              requestLine={`GET /orgs/${org}/billing/invoices`}
              onRetry={() => invoices.refetch()}
            />
          ) : isEmpty ? (
            <EmptyState
              compact
              icon="s-card"
              title="No invoices yet"
              meaning={
                <>
                  the first invoice finalizes on the 1st — until then the month accrues on Usage
                  (B2), every line queryable before it's ever billed
                </>
              }
              cli="steloit invoices list"
            />
          ) : (
            <div className="flex items-start gap-3.5">
              <div className="tblwrap w-[520px] shrink-0">
                <table className="tbl">
                  <thead>
                    <tr>
                      <th>Invoice</th>
                      <th>Period</th>
                      <th>Amount</th>
                      <th>Status</th>
                    </tr>
                  </thead>
                  <tbody>
                    {invoices.isPending ? (
                      <SkeletonRows cols={4} />
                    ) : (
                      (invoices.data ?? []).map((inv) => {
                        const accruing = inv.status === "accruing";
                        const isSelected = inv.id === selectedId;
                        return (
                          <tr
                            key={inv.id}
                            className="cursor-pointer"
                            onClick={() => {
                              if (!accruing) setSelectedId(inv.id);
                            }}
                          >
                            <td className={`mono ${isSelected ? "bg-steel-tint" : ""}`}>
                              {accruing ? <span className="text-ink3">upcoming</span> : inv.id}
                            </td>
                            <td className={isSelected ? "bg-steel-tint" : ""}>{inv.period}</td>
                            <td className={`mono ${isSelected ? "bg-steel-tint" : ""}`}>
                              {accruing
                                ? `≈ ${fmtMoney(inv.total_cents)}`
                                : fmtMoney(inv.total_cents)}
                            </td>
                            <td className={isSelected ? "bg-steel-tint" : ""}>
                              {accruing ? (
                                <Pill tone="st">accruing · finalizes Aug 1</Pill>
                              ) : (
                                <Pill tone="ok">paid</Pill>
                              )}
                            </td>
                          </tr>
                        );
                      })
                    )}
                  </tbody>
                </table>
              </div>

              {selected ? (
                <Card className="flex min-w-0 flex-1 flex-col gap-3 p-4">
                  <div className="flex items-start gap-2">
                    <div>
                      <div className="text-13 font-semibold">{invoiceTitle(selected)}</div>
                      <div className="mt-0.5 text-11 text-ink3">
                        {selected.id === "inv_2026_06"
                          ? "Jun 1 – Jun 30, 2026 · finalized Jul 1 00:00 UTC"
                          : selected.period}
                      </div>
                    </div>
                    <span className="ml-auto flex items-center gap-2">
                      <Btn variant="s" disabled disabledReason="Rendered formats land in Phase 4">
                        PDF
                      </Btn>
                      <Btn variant="s" disabled disabledReason="Rendered formats land in Phase 4">
                        CSV
                      </Btn>
                      <Btn variant="s" onClick={() => downloadJson(selected)}>
                        JSON
                      </Btn>
                    </span>
                  </div>

                  <div className="flex flex-col">
                    {lines.map((l) => {
                      const key = `${selected.id}:${l.description}`;
                      const open = expanded.has(key);
                      const usage = LINE_USAGE[l.description ?? ""] ?? [];
                      return (
                        <div key={l.description} className="border-hair border-b">
                          <button
                            type="button"
                            className="flex w-full items-baseline gap-2 py-2 text-left text-12p5"
                            aria-expanded={open}
                            onClick={() => toggleLine(key)}
                          >
                            <Icon
                              id="s-chev"
                              className={cn(
                                "h-[10px] w-[10px] self-center text-ink3 transition-transform",
                                open && "rotate-90",
                              )}
                            />
                            <span className="flex-1">{l.description}</span>
                            <span className="mono">{fmtMoney(l.cents ?? 0)}</span>
                          </button>
                          {open ? (
                            <div className="mb-2 ml-[18px] flex flex-col gap-1 rounded-lg bg-surface2 px-3 py-2">
                              {usage.map((u) => (
                                <div
                                  key={u.meter}
                                  className="mono flex items-baseline gap-3 text-11"
                                >
                                  <span className="flex-1">{u.meter}</span>
                                  <span className="text-ink2">{u.qty}</span>
                                  <span className="text-ink3">{u.rate}</span>
                                  <span className="w-14 text-right">{u.amt}</span>
                                </div>
                              ))}
                              <div className="flex items-center justify-between gap-2 pt-1">
                                <span className="text-10p5 text-ink3">
                                  representative rows — the full set stays queryable on Usage (B2)
                                </span>
                                <Btn
                                  variant="gh"
                                  className="h-6 px-2 text-10p5"
                                  disabled
                                  disabledReason="Disputes land with the billing-thread endpoints (finding)"
                                >
                                  Dispute this line…
                                </Btn>
                              </div>
                            </div>
                          ) : null}
                        </div>
                      );
                    })}
                    {/* The FRAME's own subtotal ($344.92) does not equal the sum of its
                      printed lines ($414.92) — rendered from data, arithmetic defect
                      flagged as a finding (the taxed total doesn't reconcile either). */}
                    <div className="flex items-baseline justify-between gap-3 border-hair border-b py-2 text-12p5">
                      <span className="text-ink3">Subtotal</span>
                      <span className="mono">{fmtMoney(subtotal)}</span>
                    </div>
                    {selected.tax ? (
                      <div className="flex items-baseline justify-between gap-3 border-hair border-b py-2 text-12p5">
                        <span className="text-ink3">GST 18% · GSTIN 29ABCDE1234F1Z5</span>
                        <span className="mono">{fmtMoney(selected.tax.cents ?? 0)}</span>
                      </div>
                    ) : null}
                    <div className="flex items-baseline justify-between gap-3 py-2 text-12p5 font-semibold">
                      <span>Total</span>
                      <span className="mono">{fmtMoney(selected.total_cents)}</span>
                    </div>
                  </div>

                  <div className="flex items-center gap-2 rounded-lg bg-surface2 px-3 py-2 text-11p5">
                    <Pill tone="ok">paid</Pill>
                    Visa ·· 4412 · charged Jul 1 09:00 IST · receipt emailed to invoices@acme.dev
                  </div>
                </Card>
              ) : invoices.isPending ? (
                <Skeleton className="h-[260px] min-w-0 flex-1" />
              ) : (
                <Card className="flex min-w-0 flex-1 flex-col p-4">
                  <p className="text-11p5 text-ink3">
                    No invoice selected — pick a finalized invoice on the left to see its lines.
                    Accruing months aren't invoices yet; they finalize on the 1st.
                  </p>
                </Card>
              )}
            </div>
          )}

          <p className="text-11 text-ink3">
            Every line expands to the usage rows behind it (B2 · Usage keeps them queryable forever)
            — an invoice is data, not just a PDF. Disputes: open from the line, not from support
            roulette.
          </p>
        </div>
      </main>
    </>
  );
}

export const Route = createFileRoute("/_app/$org/billing/invoices")({
  component: InvoicesPage,
});
