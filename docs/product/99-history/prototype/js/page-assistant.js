/* Assistant — PDS-012 ask surface (⓵). Laws: cites sources, links to real
   permissioned surfaces, never executes. Violet appears only here + badges. */
(function () {
  const ctx0 = Shell.init({ section: "assistant", scope: "org" });
  if (!ctx0) return;
  const content = document.getElementById("content");
  const esc = s => String(s).replace(/[&<>"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));
  const thread = [];

  content.innerHTML = `
    <div class="ask-wrap">
      <h1 class="page-title" style="display:flex;gap:10px;align-items:center">${Shell.icon("ai","ic--l")} Steloit Assistant <span class="tag-ver">v0.5</span></h1>
      <p class="mini-note">Answers about <b>your</b> organization — grounded in its live state and the docs, with citations. It never takes actions: every suggestion is a link to the real, permissioned surface (G9/§12). Disabled entirely if the org policy turns AI off.</p>
      <div class="ask-thread" id="thread" aria-live="polite">
        <div class="ask-a">Ask me about costs, alerts, branches, queues, or secrets. Try: <button class="btn btn--s btn--ghost" data-try="Why is staging degraded?">Why is staging degraded?</button> <button class="btn btn--s btn--ghost" data-try="What are we spending this month?">What are we spending?</button> <button class="btn btn--s btn--ghost" data-try="How do database branches work?">How do branches work?</button></div>
      </div>
      <form class="ask-input" id="askform">
        <input id="askq" placeholder="Ask about this organization…" aria-label="Ask the assistant">
        <button class="btn btn--primary" type="submit">Ask</button>
      </form>
    </div>`;

  const threadEl = document.getElementById("thread");
  function ask(q) {
    threadEl.insertAdjacentHTML("beforeend", `<div class="ask-q">${esc(q)}</div><div class="ask-a is-thinking" id="pending"><span class="u-faint">Reading your org's state…</span></div>`);
    if (threadEl.lastElementChild.scrollIntoView) threadEl.lastElementChild.scrollIntoView({ block: "nearest" });
    setTimeout(() => {
      const hit = DB.assistantAnswer(q);
      document.getElementById("pending").outerHTML = `
        <div class="ask-a">
          <div style="display:flex;gap:8px;align-items:center;margin-bottom:8px"><span class="ai-badge">${Shell.icon("ai","ic--s")} Steloit Assistant</span></div>
          ${hit.a}
          <div class="ask-cites">${(hit.cites || []).map(c => `<a class="ask-cite" href="docs.html#${encodeURIComponent(c.toLowerCase().replace(/[^a-z0-9]+/g,'-'))}">${esc(c)}</a>`).join("")}</div>
          ${(hit.links || []).length ? `<div class="ask-links">${hit.links.map(([t, h]) => `<a href="${h.includes("?") ? h + "&" : h + "?"}project=${ctx0.project || "ecommerce"}&env=${ctx0.env || "production"}">${esc(t)} →</a>`).join("")}</div>` : ""}
        </div>`;
      if (threadEl.lastElementChild.scrollIntoView) threadEl.lastElementChild.scrollIntoView({ block: "nearest" });
      Shell.announce("Assistant answered.");
    }, 900);
  }
  document.getElementById("askform").addEventListener("submit", (e) => {
    e.preventDefault();
    const q = document.getElementById("askq").value.trim();
    if (!q) return; document.getElementById("askq").value = "";
    ask(q);
  });
  content.querySelectorAll("[data-try]").forEach(b => b.addEventListener("click", () => ask(b.dataset.try)));

  Shell.protoStates([
    { label: "Provider error (E-AI-503)", fn: () => { threadEl.insertAdjacentHTML("beforeend", `<div class="ask-a"><b>Couldn't reach the assistant.</b> <span class="mini-note">Your question is preserved — retry, or find it yourself: everything I'd cite lives in <a href="docs.html">the docs</a>. · ref <span class="u-mono">stl_req_ai9921</span></span></div>`); } },
    { label: "Policy off (surface vanishes)", fn: () => { content.innerHTML = `<div class="empty-inline" style="margin-top:40px">AI features are disabled by this organization's policy (FND-007).<br><span class="mini-note">No teaser, no upsell — the surface simply doesn't exist here. This demo state shows what a policy-off org sees if they hit the URL directly.</span></div>`; } },
  ]);
})();
