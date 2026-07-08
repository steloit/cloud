/* Members — org IAM (PDS-007 surfaces) */
(function () {
  const ctx0 = Shell.init({ section: "org-members", scope: "org" });
  if (!ctx0) return;
  const content = document.getElementById("content");
  let tab = new URLSearchParams(location.search).get("tab") || "members";
  if (!["members", "roles", "api-keys"].includes(tab)) tab = "members";
  const esc = s => String(s).replace(/[&<>"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));

  function render() {
    content.innerHTML = `
      <div class="tbl-toolbar"><div><h1 class="page-title">Members</h1>
        <p class="mini-note">Identity lives at the organization (GOV-002 §2): people, roles, and machine keys. Projects grant, never own.</p></div>
        <button class="btn btn--primary" id="invite">${Shell.icon("plus","ic--s")} Invite member</button></div>
      <div class="tabs" role="tablist">${["members","roles","api-keys"].map(x => `<button class="tab ${x===tab?"is-on":""}" role="tab" aria-selected="${x===tab}" data-tab="${x}">${({members:"Members",roles:"Roles","api-keys":"API keys"})[x]}</button>`).join("")}</div>
      <div id="tabbody"></div>`;
    content.querySelectorAll("[data-tab]").forEach(b => b.addEventListener("click", () => { tab = b.dataset.tab; render(); }));
    content.querySelector("#invite").addEventListener("click", inviteDialog);
    ({ members, roles, "api-keys": keys })[tab](document.getElementById("tabbody"));
  }

  function members(body) {
    body.innerHTML = `
      <table class="tbl"><thead><tr><th>Member</th><th>Role</th><th>2FA</th><th>Last active</th><th></th></tr></thead><tbody>
        ${DATA.members.map((m, i) => `<tr>
          <td><div style="display:flex;gap:10px;align-items:center"><span class="avatar" aria-hidden="true">${m.avatar}</span><div><b>${esc(m.name)}</b><div class="mini-note">${esc(m.email)}</div></div></div></td>
          <td>${m.role === "Owner" ? `<b>Owner</b>` : `<select class="input input--s" data-role="${i}" aria-label="Role for ${esc(m.name)}">${DATA.roles.map(r => `<option ${r.role === m.role ? "selected" : ""}>${r.role}</option>`).join("")}</select>`}</td>
          <td>${m.tfa ? `<span class="chip-pill chip-pill--ok">on</span>` : `<span class="chip-pill chip-pill--warn">off</span>`}</td>
          <td class="u-faint">${esc(m.active)}</td>
          <td class="tbl-actions">${m.role === "Owner" ? "" : `<button class="btn btn--s btn--danger-o" data-remove="${i}">Remove…</button>`}</td></tr>`).join("")}
      </tbody></table>
      <p class="mini-note">CLI parity: <span class="u-mono">steloit org members list</span> · <span class="u-mono">steloit org members invite dev@acme.dev --role developer</span></p>`;
    body.querySelectorAll("[data-role]").forEach(s => s.addEventListener("change", () => {
      const m = DATA.members[+s.dataset.role];
      Shell.dialog({ title: `Change ${m.name}'s role`, body: `From <b>${esc(m.role)}</b> to <b>${esc(s.value)}</b> — takes effect immediately across every project. This is recorded in the audit log.`, confirmLabel: "Change role", onConfirm: (close) => { m.role = s.value; close(); Shell.toast(`${esc(m.name)} is now <b>${esc(s.value)}</b>.`); render(); } });
      s.value = m.role;
    }));
    body.querySelectorAll("[data-remove]").forEach(b => b.addEventListener("click", () => {
      const m = DATA.members[+b.dataset.remove];
      Shell.dialog({ title: `Remove ${m.name}`, danger: true, body: `Removes <b>${esc(m.name)}</b> from <b>acme</b>. Their sessions end now; nothing they created is deleted. They can be re-invited anytime.`, confirmLabel: "Remove member", onConfirm: (close) => { close(); Shell.toast(`${esc(m.name)} removed. <button class="btn btn--s" onclick="Shell.toast('Re-invited.')">Undo</button>`); } });
    }));
  }

  function roles(body) {
    body.innerHTML = `
      <table class="tbl"><thead><tr><th>Role</th><th>What it can do</th><th>Members</th></tr></thead><tbody>
        ${DATA.roles.map(r => `<tr><td><b>${esc(r.role)}</b></td><td class="mini-note" style="font-size:12.5px">${esc(r.desc)}</td><td class="u-mono">${r.count}</td></tr>`).join("")}
      </tbody></table>
      <p class="mini-note">Custom roles arrive with fine-grained policies (FND-007, Enterprise wave v3). Until then these four cover the day-to-day safely.</p>`;
  }

  function keys(body) {
    body.innerHTML = `
      <div class="tbl-toolbar"><p class="mini-note" style="max-width:60ch">Machine credentials for CI and integrations. Keys are shown <b>once</b> at creation — Steloit stores only a hash.</p>
        <button class="btn btn--primary" id="new-key">${Shell.icon("key","ic--s")} New API key</button></div>
      <table class="tbl"><thead><tr><th>Key</th><th>Name</th><th>Scope</th><th>Created</th><th>Last used</th><th></th></tr></thead><tbody id="keyrows">
        ${DATA.apiKeys.map((k, i) => keyRow(k, i)).join("")}
      </tbody></table>`;
    body.querySelector("#new-key").addEventListener("click", () => {
      Shell.dialog({ title: "New API key", body: `
        <label class="field"><span class="field-label">Name</span><input class="input u-mono" id="key-name" value="staging-deploy"></label>
        <label class="field"><span class="field-label">Scope</span><select class="input"><option>ecommerce · deploy</option><option>org · read-only</option><option>analytics · deploy</option></select></label>`,
        confirmLabel: "Create key", onConfirm: (close) => {
          close();
          Shell.dialog({ title: "Copy your key now", body: `<div class="code-block"><code>stl_live_9f2K…u81mQx4CvN</code><button class="btn btn--s btn--ghost" onclick="this.textContent='Copied'">Copy</button></div>
            <p class="mini-note" style="margin-top:10px">This is the only time it's shown. Store it in your CI's secret store — never in the repo.</p>`, confirmLabel: "I've stored it", onConfirm: (c2) => { c2(); Shell.toast("Key created."); } });
        } });
    });
    body.querySelectorAll("[data-revoke]").forEach(b => b.addEventListener("click", () => {
      const k = DATA.apiKeys[+b.dataset.revoke];
      Shell.dialog({ title: `Revoke ${k.name}`, danger: true, body: `Anything using <b class="u-mono">${esc(k.id)}</b> stops authenticating immediately. This can't be undone — create a new key to replace it.`, confirmLabel: "Revoke key", onConfirm: (close) => { close(); Shell.toast(`<b class="u-mono">${esc(k.name)}</b> revoked.`); } });
    }));
  }
  const keyRow = (k, i) => `<tr><td class="u-mono u-12">${esc(k.id)}</td><td><b>${esc(k.name)}</b></td><td class="u-12">${esc(k.scope)}</td><td class="u-faint">${esc(k.created)}</td><td class="u-faint">${esc(k.lastUsed)}</td>
    <td class="tbl-actions"><button class="btn btn--s btn--danger-o" data-revoke="${i}">Revoke…</button></td></tr>`;

  function inviteDialog() {
    Shell.dialog({ title: "Invite to acme", body: `
      <label class="field"><span class="field-label">Email</span><input class="input u-mono" id="inv-email" placeholder="dev@acme.dev"></label>
      <label class="field"><span class="field-label">Role</span><select class="input">${DATA.roles.filter(r => r.role !== "Owner").map(r => `<option>${r.role}</option>`).join("")}</select></label>
      <p class="mini-note">They'll land on their invite link's destination after sign-in — the link is the onboarding (J7).</p>`,
      confirmLabel: "Send invite", onConfirm: (close) => { close(); Shell.toast("Invite sent — it appears under Membership in notifications when accepted."); } });
  }

  render();
})();
