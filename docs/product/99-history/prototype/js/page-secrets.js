/* Secrets — env-scoped (PDS-008 surfaces, ⓵) */
(function () {
  const ctx0 = Shell.init({ section: "services", scope: "project" });
  if (!ctx0) return;
  const content = document.getElementById("content");
  const esc = s => String(s).replace(/[&<>"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));

  window.renderPage = function (cx) {
    const rows = DB.secretsOf(cx.project, cx.env);
    content.innerHTML = `
      <nav class="crumbs" aria-label="Breadcrumb"><a href="settings-env.html${Shell.q()}">Environment settings</a> <span aria-hidden="true">/</span> Secrets</nav>
      <div class="tbl-toolbar"><div><h1 class="page-title">Secrets <span class="tag-ver">v0.5</span></h1>
        <p class="mini-note">Per-environment. Injected into bindings and written locally by <span class="u-mono">steloit env pull</span>. ${cx.e && cx.e.production ? `<span class="primary-tag">production</span> secrets never appear in non-production pulls.` : ""}</p></div>
        <button class="btn btn--primary" id="add">${Shell.icon("plus","ic--s")} Add secret</button></div>
      ${rows.length ? `
      <table class="tbl"><thead><tr><th>Name</th><th>Value</th><th>Updated</th><th>By</th><th></th></tr></thead><tbody>
        ${rows.map((r, i) => `<tr>
          <td class="u-mono">${esc(r.name)}</td>
          <td><span class="u-mono u-faint" id="v-${i}">••••••••••••</span> <button class="btn btn--s btn--ghost" data-reveal="${i}">Reveal</button></td>
          <td class="u-faint">${esc(r.updated)}</td><td class="u-faint">${esc(r.by)}</td>
          <td class="tbl-actions"><button class="btn btn--s btn--ghost" data-rotate="${i}">Rotate…</button><button class="btn btn--s btn--danger-o" data-del="${i}">Delete…</button></td></tr>`).join("")}
      </tbody></table>
      <p class="mini-note">CLI parity: <span class="u-mono">steloit secrets list</span> · <span class="u-mono">steloit secrets set STRIPE_SECRET_KEY --env ${esc(cx.env)}</span></p>`
      : `<div class="empty-inline">No secrets in <b class="u-mono">${esc(cx.env)}</b> yet. Add one, or set it from the CLI — both land in the same place.</div>`}`;

    content.querySelector("#add").addEventListener("click", () => {
      Shell.dialog({ title: `Add secret to ${cx.env}`, body: `
        <label class="field"><span class="field-label">Name</span><input class="input u-mono" value="OPENAI_API_KEY"></label>
        <label class="field"><span class="field-label">Value</span><input class="input u-mono" type="password" placeholder="paste value"></label>
        <p class="mini-note">Apps see it on their next <span class="u-mono">env pull</span> or restart. Values are write-only after save; reveals are audit-logged.</p>`,
        confirmLabel: "Add secret", onConfirm: (close) => { close(); Shell.toast("Secret added — apps pick it up on next pull/restart."); } });
    });
    content.querySelectorAll("[data-reveal]").forEach(b => b.addEventListener("click", () => {
      document.getElementById("v-" + b.dataset.reveal).textContent = "sk_live_9f21…redacted-in-proto";
      b.remove();
      Shell.toast("Revealed — recorded in the audit log (SEC-003).");
    }));
    content.querySelectorAll("[data-rotate]").forEach(b => b.addEventListener("click", () => {
      const r = rows[+b.dataset.rotate];
      Shell.dialog({ title: `Rotate ${r.name}`, body: `Paste the new value. Pulled and synced apps update on next pull; <b>manually copied values won't update</b> (C-41).<label class="field" style="margin-top:10px"><span class="field-label">New value</span><input class="input u-mono" type="password"></label>`, confirmLabel: "Rotate", onConfirm: (close) => { close(); Shell.toast(`<b class="u-mono">${esc(r.name)}</b> rotated.`); } });
    }));
    content.querySelectorAll("[data-del]").forEach(b => b.addEventListener("click", () => {
      const r = rows[+b.dataset.del];
      Shell.dialog({ title: `Delete ${r.name}`, danger: true, body: `Apps depending on <b class="u-mono">${esc(r.name)}</b> in <b class="u-mono">${esc(cx.env)}</b> will fail on their next start. This can't be undone.`, confirmLabel: "Delete secret", onConfirm: (close) => { close(); Shell.toast(`<b class="u-mono">${esc(r.name)}</b> deleted.`); } });
    }));
  };
  renderPage(ctx0);
})();
