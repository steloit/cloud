/* Templates — PDS-TPL gallery (P3 ⓵) */
(function () {
  const ctx0 = Shell.init({ section: "templates", scope: "org" });
  if (!ctx0) return;
  const content = document.getElementById("content");
  const esc = s => String(s).replace(/[&<>"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));
  content.innerHTML = `
    <h1 class="page-title">Templates <span class="tag-ver">v0.5</span></h1>
    <p class="mini-note" style="margin-bottom:18px">A template is a saved answer to the wizard — services plus sensible configuration. Using one lands you on the normal review step: same estimate, same confirm, nothing hidden.</p>
    <div class="proj-grid">
      ${DATA.templates.map(t => `
        <article class="card card--hover" style="display:grid;gap:8px">
          <div style="display:flex;align-items:center;gap:8px"><b>${esc(t.name)}</b><span class="mini-note" style="margin-left:auto">${esc(t.uses)} uses</span></div>
          <p class="mini-note" style="font-size:12.5px;min-height:34px">${esc(t.desc)}</p>
          <div style="display:flex;gap:6px;flex-wrap:wrap">${t.services.map(s => `<span class="chip-pill">${Shell.icon(DATA.svcTypes[s].icon, "ic--s")} ${DATA.svcTypes[s].label}</span>`).join("")}</div>
          <div style="display:flex;align-items:center;gap:10px;margin-top:4px">
            <span class="cost-chip">~$${t.est}/mo</span>
            <button class="btn btn--primary btn--s" style="margin-left:auto" data-use="${t.id}">Use template</button>
          </div>
        </article>`).join("")}
    </div>`;
  content.querySelectorAll("[data-use]").forEach(b => b.addEventListener("click", () => {
    const t = DATA.templates.find(x => x.id === b.dataset.use);
    location.href = `wizard.html?template=${t.id}&services=${t.services.join(",")}`;
  }));
})();
