/* Deployments — v2 frame (SH-18 reserved slot, GOV-002 flagship: previews with DB branches) */
(function () {
  const ctx0 = Shell.init({ section: "deployments", scope: "project" });
  if (!ctx0) return;
  const content = document.getElementById("content");
  const esc = s => String(s).replace(/[&<>"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));
  const D = DATA.deploys;

  window.renderPage = function (cx) {
    content.innerHTML = `
      <h1 class="page-title">Deployments <span class="tag-ver" style="background:var(--status-provisioning-soft);color:var(--status-provisioning)">v2 preview</span></h1>
      <div class="v2-banner">${Shell.icon("info","ic--s")} <span>This is the reserved SH-18 frame rendered with the v2 design: Compute + Git integration + <b>preview environments that branch your production database</b> — the GOV-002 flagship. Interactions here are design demos.</span></div>

      <section class="card" style="margin-bottom:18px">
        <div class="tbl-toolbar"><h3 class="card-title" style="margin:0">Service: <span class="u-mono">${esc(D.service)}</span></h3>
          <span class="mini-note">${esc(D.repo)} · <span class="u-mono">${esc(D.branch)}</span> · build: <span class="u-mono">${esc(D.buildCmd)}</span></span></div>
        <table class="tbl"><thead><tr><th>Deploy</th><th>Commit</th><th>Env</th><th>Status</th><th>When</th><th></th></tr></thead><tbody>
          ${D.rows.map(r => `<tr>
            <td class="u-mono u-12">${esc(r.id)}</td><td class="u-mono u-12">${esc(r.commit)}</td><td class="u-mono">${esc(r.env)}</td>
            <td>${r.status === "live" ? `<span class="chip-pill chip-pill--ok">live</span>` : `<span class="chip-pill">${esc(r.status)}</span>`}</td>
            <td class="u-faint">${esc(r.when)} · ${esc(r.by)}</td>
            <td class="tbl-actions">${r.status === "live" && r.env === "production" ? `<button class="btn btn--s btn--danger-o" data-rollback="${r.id}">Roll back…</button>` : ""}</td></tr>`).join("")}
        </tbody></table>
      </section>

      <section class="card">
        <div class="tbl-toolbar"><h3 class="card-title" style="margin:0">Preview environments</h3>
          <span class="mini-note">One per open PR. Each gets a compute instance <b>and a copy-on-write branch of production's database</b> — real data shape, zero risk, cents per day.</span></div>
        <div style="display:grid;gap:10px">
          ${D.previews.map(p => `
            <div class="preview-card">
              <div style="display:flex;gap:10px;align-items:center"><b>${esc(p.pr)}</b><span class="chip-pill chip-pill--ok">${esc(p.status)}</span><span class="cost-chip" style="margin-left:auto">${esc(p.cost)}</span></div>
              <a class="preview-url" href="#" onclick="Shell.toast('Preview URL — external in the real product.');return false">${esc(p.url)}</a>
              <div class="mini-note">env <span class="u-mono">${esc(p.env)}</span> · db: ${esc(p.db)}</div>
              <div style="display:flex;gap:8px;margin-top:4px">
                <button class="btn btn--s btn--ghost" data-promote="${esc(p.pr)}">Promote to staging…</button>
                <button class="btn btn--s btn--danger-o" data-teardown="${esc(p.env)}">Tear down…</button>
              </div>
            </div>`).join("")}
        </div>
      </section>`;

    content.querySelectorAll("[data-rollback]").forEach(b => b.addEventListener("click", () => {
      Shell.dialog({ title: "Roll back production", body: `Re-deploys <b class="u-mono">dep_a90</b> (the previous live build). Traffic shifts atomically; the database is untouched — schema changes need their own migration plan (shown when relevant).`, confirmLabel: "Roll back", onConfirm: c => { c(); Shell.toast("Rolled back — previous build is live."); } });
    }));
    content.querySelectorAll("[data-promote]").forEach(b => b.addEventListener("click", () => Shell.toast(`Promotion flow for <b>${esc(b.dataset.promote)}</b> — v2 design demo.`)));
    content.querySelectorAll("[data-teardown]").forEach(b => b.addEventListener("click", () => {
      Shell.dialog({ title: `Tear down ${b.dataset.teardown}`, danger: true, body: `Deletes the preview compute and its database branch. The PR and production are untouched. Recreated automatically if the PR updates.`, confirmLabel: "Tear down", onConfirm: c => { c(); Shell.toast("Preview torn down."); } });
    }));
  };
  renderPage(ctx0);
})();
