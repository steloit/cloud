/* Billing — org cost & plan (PDS-009 surfaces) */
(function () {
  const ctx0 = Shell.init({ section: "org-billing", scope: "org" });
  if (!ctx0) return;
  const content = document.getElementById("content");
  const esc = s => String(s).replace(/[&<>"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));
  const B = DATA.billing;

  content.innerHTML = `
    <h1 class="page-title">Billing <span class="tag-ver">v0.5</span></h1>
    <p class="mini-note" style="margin-bottom:16px">Cost renders as fact, never judgment — no red totals, no green savings theater (DES-004 hue law). Estimates are ~marked; actuals aren't.</p>

    <div class="plan-card" style="margin-bottom:20px">
      <div><div class="mini-note">Plan</div><div class="plan-big">${esc(B.plan)}</div></div>
      <div><div class="mini-note">This period (actual)</div><div class="plan-big">$${B.periodTotal.toFixed(2)}</div></div>
      <div><div class="mini-note">Forecast</div><div class="plan-big">~$${B.forecast.toFixed(2)}</div></div>
      <div><div class="mini-note">Renews</div><div class="u-mono">${esc(B.renews)}</div></div>
      <div style="margin-left:auto;display:flex;gap:8px">
        <button class="btn btn--ghost" id="manage">Manage services</button>
        <button class="btn btn--ghost" id="plans">Compare plans</button>
      </div>
    </div>

    <div class="two-col">
      <div>
        <div class="ch-card" id="daily" style="margin-bottom:16px"></div>
        <section class="card"><h3 class="card-title">This period by project</h3>
          ${B.byProject.map(p => `
            <details class="bp" ${p.mtd ? "open" : ""}><summary class="split-row" style="cursor:pointer"><span>${esc(p.project)}</span><b>$${p.mtd.toFixed(2)}</b></summary>
              <div style="padding:4px 0 8px 14px">${p.split.map(([k, v]) => `<div class="split-row"><span class="u-mono u-12">${esc(k)}</span><b>$${v.toFixed(2)}</b></div>`).join("") || `<div class="mini-note">Archived — $0 compute.</div>`}</div>
            </details>`).join("")}
        </section>
      </div>
      <aside>
        <section class="card" style="margin-bottom:16px"><h3 class="card-title">Budget alert</h3>
          <div class="field"><span class="field-label">Notify when the period passes</span>
            <div style="display:flex;gap:8px"><input class="input u-mono" id="budget" value="$${B.budget.amount}"><button class="btn btn--ghost" id="save-budget">Save</button></div></div>
          <p class="mini-note">${esc(B.budget.note)}</p>
        </section>
        <section class="card"><h3 class="card-title">Invoices</h3>
          <table class="tbl tbl--flat"><tbody>
            ${B.invoices.map(inv => `<tr><td class="u-mono u-12">${esc(inv.id)}</td><td>${esc(inv.period)}</td><td class="u-mono">${esc(inv.total)}</td>
              <td><span class="chip-pill chip-pill--ok">${esc(inv.status)}</span></td>
              <td class="tbl-actions"><button class="btn btn--s btn--ghost" data-inv="${esc(inv.id)}">${Shell.icon("doc","ic--s")} PDF</button></td></tr>`).join("")}
          </tbody></table>
        </section>
      </aside>
    </div>`;

  Charts.bars(document.getElementById("daily"), { title: "Daily spend · last 30 days", note: "actuals, all projects", vals: B.daily, h: 150, tip: (v, i) => `Day ${i + 1}: $${v.toFixed(2)}` });

  document.getElementById("save-budget").addEventListener("click", () => Shell.toast("Budget alert saved — notifies Owner + Admins via Billing category."));
  document.getElementById("manage").addEventListener("click", () => location.href = "index.html");
  document.getElementById("plans").addEventListener("click", () => {
    Shell.dialog({ title: "Plans", body: `
      <table class="tbl tbl--flat"><tbody>
        <tr><td><b>Free</b></td><td class="mini-note">1 project · dev sizes · community support</td><td class="u-mono">$0</td></tr>
        <tr><td><b>Pro</b> <span class="chip-pill chip-pill--ok">current</span></td><td class="mini-note">Unlimited projects · 7-day PITR · email support</td><td class="u-mono">usage</td></tr>
        <tr><td><b>Business</b></td><td class="mini-note">SSO · 30-day PITR · priority support</td><td class="u-mono">usage + $99/mo</td></tr>
        <tr><td><b>Enterprise</b></td><td class="mini-note">BYOC · custom policies · SLAs</td><td class="u-mono">talk to us</td></tr>
      </tbody></table>
      <p class="mini-note" style="margin-top:8px">Equal-weight actions, no pressure: managing existing spend is always beside upgrading (E-QUOTA-402 rule).</p>`,
      confirmLabel: "Close", onConfirm: c => c() });
  });
  content.querySelectorAll("[data-inv]").forEach(b => b.addEventListener("click", () => Shell.toast(`${esc(b.dataset.inv)}.pdf — download stubbed in the prototype.`)));

  Shell.protoStates([
    { label: "Billing data delayed (partial)", fn: () => { document.getElementById("daily").innerHTML = `<div class="err-frame err--section"><b>Billing data delayed.</b><span class="mini-note">Charts return as the pipeline catches up — totals above are unaffected. · ref <span class="u-mono">stl_req_b8812</span></span></div>`; } },
    { label: "Reset", fn: () => location.reload() },
  ]);
})();
