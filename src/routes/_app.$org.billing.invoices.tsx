import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Pill } from "@/design-system/pill";
import { planLabel, useInvoices } from "@/features/billing/hooks";
import { useOrgs } from "@/features/org/hooks";
import type { Invoice } from "@/lib/api";
import { fmtMoney } from "@/lib/fmt";

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

function InvoicesPage() {
  const { org } = Route.useParams();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);
  const invoices = useInvoices(org);
  const [selectedId, setSelectedId] = useState("inv_2026_06");

  const selected = (invoices.data ?? []).find((i) => i.id === selectedId);
  const lines = selected?.lines ?? [];
  const subtotal = lines.reduce((sum, l) => sum + (l.cents ?? 0), 0);

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
                  {(invoices.data ?? []).map((inv) => {
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
                          {accruing ? `≈ ${fmtMoney(inv.total_cents)}` : fmtMoney(inv.total_cents)}
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
                  })}
                </tbody>
              </table>
            </div>

            {selected ? (
              <Card className="flex min-w-0 flex-1 flex-col gap-3 p-4">
                <div className="flex items-start gap-2">
                  <div>
                    <div className="text-[13.5px] font-semibold">{invoiceTitle(selected)}</div>
                    <div className="mt-0.5 text-[11px] text-ink3">
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
                  {lines.map((l) => (
                    <div
                      key={l.description}
                      className="flex items-baseline justify-between gap-3 border-hair border-b py-2 text-[12.5px]"
                    >
                      <span>{l.description}</span>
                      <span className="mono">{fmtMoney(l.cents ?? 0)}</span>
                    </div>
                  ))}
                  {/* The FRAME's own subtotal ($344.92) does not equal the sum of its
                      printed lines ($414.92) — rendered from data, arithmetic defect
                      flagged as a finding (the taxed total doesn't reconcile either). */}
                  <div className="flex items-baseline justify-between gap-3 border-hair border-b py-2 text-[12.5px]">
                    <span className="text-ink3">Subtotal</span>
                    <span className="mono">{fmtMoney(subtotal)}</span>
                  </div>
                  {selected.tax ? (
                    <div className="flex items-baseline justify-between gap-3 border-hair border-b py-2 text-[12.5px]">
                      <span className="text-ink3">GST 18% · GSTIN 29ABCDE1234F1Z5</span>
                      <span className="mono">{fmtMoney(selected.tax.cents ?? 0)}</span>
                    </div>
                  ) : null}
                  <div className="flex items-baseline justify-between gap-3 py-2 text-[12.5px] font-semibold">
                    <span>Total</span>
                    <span className="mono">{fmtMoney(selected.total_cents)}</span>
                  </div>
                </div>

                <div className="flex items-center gap-2 rounded-lg bg-surface2 px-3 py-2 text-[11.5px]">
                  <Pill tone="ok">paid</Pill>
                  Visa ·· 4412 · charged Jul 1 09:00 IST · receipt emailed to invoices@acme.dev
                </div>
              </Card>
            ) : null}
          </div>

          <p className="text-[11px] text-ink3">
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
