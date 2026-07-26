/* ============================================================
   Steloit prototype — Topology renderer (P-4, §6.6)
   Deterministic layered layout: external consumers → services.
   Keyboard: Tab in → arrows between nodes → Enter opens →
   b lists bindings → Esc exits. "List view" = same data as lists.
   ============================================================ */
(function () {
  "use strict";
  const NW = 168, NH = 56, GX = 140, GY = 26;

  window.Topology = function (host, envCtx, options) {
    const o = Object.assign({ embed: false, maxNodes: 99, onOpenService: null }, options || {});
    const env = envCtx.e, ctxq = Shell.q();
    let mode = "graph", focusIdx = -1;

    function build() {
      host.classList.add("topo-wrap", o.embed ? "is-embed" : "is-full");
      if (!env || !env.services.length) return renderEmpty();
      mode === "graph" ? renderGraph() : renderList();
    }

    function nodes() {
      const svcs = env.services.slice(0, o.maxNodes);
      const hasApp = env.bindings && env.bindings.length;
      const col1 = svcs.map((s, i) => ({ kind: "svc", s, col: 1, row: i }));
      const col0 = hasApp ? [{ kind: "app", col: 0, row: Math.max(0, Math.floor((svcs.length - 1) / 2)) }] : [];
      return col0.concat(col1);
    }
    function pos(n, rows1) {
      const x = 40 + n.col * (NW + GX);
      const totalH = rows1 * NH + (rows1 - 1) * GY;
      const y = n.kind === "app"
        ? 30 + totalH / 2 - NH / 2
        : 30 + n.row * (NH + GY);
      return { x, y };
    }

    function renderEmpty() {
      host.innerHTML = `<div class="empty" style="margin:0;height:100%;justify-content:center">
        <div class="empty-art">${Shell.icon("graph")}</div>
        <h2>Your project's shape appears here</h2>
        <p>Add a service to draw the first node.</p>
        <a class="btn btn--primary" href="wizard.html${ctxq}&mode=service">${Shell.icon("plus", "ic--s")} Add service</a></div>`;
    }

    function renderGraph() {
      const ns = nodes();
      const svcNodes = ns.filter(n => n.kind === "svc");
      const rows1 = svcNodes.length;
      const W = 40 + 2 * NW + GX + 40, H = 60 + rows1 * NH + (rows1 - 1) * GY;
      const app = ns.find(n => n.kind === "app");
      const idx = {}; svcNodes.forEach(n => idx[n.s.id] = n);

      let edges = "";
      (env.bindings || []).forEach(b => {
        const to = idx[b.to]; if (!to || !app) return;
        const p1 = pos(app, rows1), p2 = pos(to, rows1);
        const x1 = p1.x + NW, y1 = p1.y + NH / 2, x2 = p2.x, y2 = p2.y + NH / 2;
        const mx = (x1 + x2) / 2;
        const prov = to.s.state === "provisioning" ? "is-provisioning" : "";
        edges += `<g class="topo-edge-g" data-binding="${b.id}" tabindex="-1">
          <path class="topo-edge-hit" d="M${x1},${y1} C${mx},${y1} ${mx},${y2} ${x2 - 8},${y2}"/>
          <path class="topo-edge ${prov}" d="M${x1},${y1} C${mx},${y1} ${mx},${y2} ${x2 - 8},${y2}"/>
          <path class="topo-arrow" d="M${x2 - 9},${y2 - 4} L${x2 - 1},${y2} L${x2 - 9},${y2 + 4} Z"/></g>`;
      });

      const nodeSvg = ns.map((n, i) => {
        const p = pos(n, rows1);
        if (n.kind === "app") {
          return `<g class="topo-node is-ghost" data-i="${i}" tabindex="-1" role="img" aria-label="your app, external consumer" transform="translate(${p.x},${p.y})">
            <rect class="n-box" width="${NW}" height="${NH}" rx="10"/>
            <g transform="translate(14,20)" ><g class="ic-holder" style="color:var(--text-faint)">${inlineIcon("external", 18, 18)}</g></g>
            <text class="n-name" x="42" y="26">your app</text>
            <text class="n-type" x="42" y="42">external \u00b7 env pull</text></g>`;
        }
        const s = n.s, t = DATA.svcTypes[s.type];
        return `<g class="topo-node" data-i="${i}" data-svc="${s.id}" tabindex="-1" role="img" aria-label="${s.id}, ${t.label}, ${s.state}" transform="translate(${p.x},${p.y})" style="animation-delay:${i * 40}ms">
          <rect class="n-box" width="${NW}" height="${NH}" rx="10"/>
          <rect class="n-ring" x="-2" y="-2" width="${NW + 4}" height="${NH + 4}" rx="12" stroke="var(--status-${s.state})" opacity="${s.state === "ready" ? ".45" : ".9"}"/>
          <g transform="translate(14,19)"><g style="color:var(--text-dim)">${inlineIcon(t.icon, 18, 18)}</g></g>
          <text class="n-name" x="42" y="24">${s.id}</text>
          <text class="n-type" x="42" y="40">${t.label}</text>
          <text class="n-state" x="${NW - 10}" y="24" text-anchor="end" fill="var(--status-${s.state})">${s.state}</text></g>`;
      }).join("");

      host.innerHTML = `
        <svg class="topo-svg topo-settle" viewBox="0 0 ${W} ${H}" role="group" aria-label="Project topology: ${env.services.length} services${env.bindings ? ", " + env.bindings.length + " bindings" : ""}" tabindex="0">
          ${edges}${nodeSvg}</svg>
        <div class="topo-tools">
          ${o.embed ? `<a class="btn btn--ghost btn--s" href="topology.html${ctxq}">Open full \u2192</a>`
          : `<button class="btn btn--ghost btn--s" data-tool="list" aria-pressed="false">List view</button>
             <a class="btn btn--ghost btn--s" href="wizard.html${ctxq}&mode=service">${Shell.icon("plus", "ic--s")} Add service</a>
             <button class="btn btn--ghost btn--s" data-tool="bind" ${env.services.length < 2 ? `aria-disabled="true" title="Binding connects two services. Add another service first."` : ""}>${Shell.icon("key", "ic--s")} Bind <span class="tag-ver">v0.5</span></button>`}
        </div>
        ${o.embed ? "" : `<div class="topo-hint">Tab into the map \u00b7 arrows move \u00b7 enter opens \u00b7 b lists bindings${env.appConnected ? "" : ` \u00b7 <a href=\"connect.html${ctxq}\">Connect your code \u2192</a>`}</div>`}`;

      const svg = host.querySelector("svg");
      const nodeEls = Array.from(host.querySelectorAll(".topo-node"));
      nodeEls.forEach(g => {
        g.addEventListener("click", () => {
          const id = g.dataset.svc;
          if (id) openSvcPanel(id); else Shell.toast("The external app connects via <b>env pull</b> \u2014 see Connect.");
        });
      });
      Array.from(host.querySelectorAll(".topo-edge-g")).forEach(g => g.addEventListener("click", () => openBindingPanel(g.dataset.binding)));

      // Keyboard traversal (§10.3)
      svg.addEventListener("keydown", (e) => {
        if (["ArrowRight", "ArrowDown"].includes(e.key)) { e.preventDefault(); focusNode(Math.min(nodeEls.length - 1, focusIdx + 1), nodeEls); }
        else if (["ArrowLeft", "ArrowUp"].includes(e.key)) { e.preventDefault(); focusNode(Math.max(0, focusIdx - 1), nodeEls); }
        else if (e.key === "Enter" && focusIdx >= 0) { nodeEls[focusIdx].dispatchEvent(new Event("click")); }
        else if (e.key.toLowerCase() === "b" && focusIdx >= 0) {
          const id = nodeEls[focusIdx].dataset.svc; if (!id) return;
          const bs = (env.bindings || []).filter(b => b.to === id);
          if (bs.length) openBindingPanel(bs[0].id); else Shell.toast("No bindings on this service.");
        }
      });
      svg.addEventListener("focus", () => { if (focusIdx < 0) focusNode(0, nodeEls); });
      svg.addEventListener("blur", () => { nodeEls.forEach(n => n.classList.remove("is-focus")); focusIdx = -1; });

      const listBtn = host.querySelector('[data-tool="list"]');
      if (listBtn) listBtn.addEventListener("click", () => { mode = "list"; build(); });
      const bindBtn = host.querySelector('[data-tool="bind"]');
      if (bindBtn) bindBtn.addEventListener("click", () => {
        if (bindBtn.getAttribute("aria-disabled")) return Shell.toast("Binding connects two services. Add another service first.");
        Shell.toast("Bind flow arrives with the multi-service data layer (v0.5).");
      });
    }

    function focusNode(i, nodeEls) {
      focusIdx = i;
      nodeEls.forEach((n, j) => n.classList.toggle("is-focus", j === i));
      const n = nodeEls[i];
      Shell.announce(n.getAttribute("aria-label"));
    }

    function renderList() {
      const rows = env.services.map(s => {
        const t = DATA.svcTypes[s.type];
        const bs = (env.bindings || []).filter(b => b.to === s.id);
        return `<li><a href="service.html${ctxq}&id=${s.id}" class="u-mono">${s.id}</a>
          <span class="u-dim">\u2014 ${t.label}, ${s.state}</span>
          ${bs.length ? `<ul>${bs.map(b => `<li class="u-12 u-dim">bound from <span class="u-mono">${b.from}</span> \u00b7 ${b.scope} \u00b7 ${b.age}</li>`).join("")}</ul>` : ""}</li>`;
      }).join("");
      host.innerHTML = `<div class="topo-list"><div class="u-row" style="margin-bottom:8px">
          <h3>Topology \u2014 list view</h3>
          <button class="btn btn--ghost btn--s u-right" data-tool="graph">Graph view</button></div>
        <p class="u-12 u-dim" style="margin-bottom:6px">The same information as the map, with no graph dependence.</p>
        <ul>${rows}</ul></div>`;
      host.querySelector('[data-tool="graph"]').addEventListener("click", () => { mode = "graph"; build(); });
    }

    /* side panels */
    let panel;
    function ensurePanel() {
      if (!panel) { panel = Shell.el("aside", "panel"); panel.setAttribute("role", "dialog"); document.body.appendChild(panel); }
      return panel;
    }
    function openSvcPanel(id) {
      const s = env.services.find(x => x.id === id); const t = DATA.svcTypes[s.type];
      const p = ensurePanel();
      p.innerHTML = `<div class="panel-h">${Shell.icon(t.icon)}<h2 class="u-mono u-14">${s.id}</h2>
          <span class="badge badge--${s.state}"><span class="dot"></span>${s.state}</span>
          <button class="icon-btn u-right" data-x aria-label="Close">${Shell.icon("x")}</button></div>
        <div class="panel-b"><dl class="kv">
            <dt>Type</dt><dd>${t.label}</dd><dt>Size</dt><dd class="u-mono">${s.size}</dd>
            <dt>Detail</dt><dd>${s.detail || "\u2014"}</dd>
            <dt>Cost mtd</dt><dd class="u-mono">$${s.mtd.toFixed(2)}</dd>
            <dt>Bindings</dt><dd>${(env.bindings || []).filter(b => b.to === id).length}</dd></dl>
          <div style="margin-top:16px"><a class="btn" href="service.html${ctxq}&id=${id}">Open service page ${Shell.icon("arrow", "ic--s")}</a></div>
          <p class="u-12 u-faint" style="margin-top:12px">Lifecycle verbs (scale, back up, restore) live on the service page \u2014 PDS-001 territory.</p></div>`;
      p.classList.add("is-open");
      p.querySelector("[data-x]").addEventListener("click", () => p.classList.remove("is-open"));
    }
    function openBindingPanel(bid) {
      const b = (env.bindings || []).find(x => x.id === bid);
      const p = ensurePanel();
      p.innerHTML = `<div class="panel-h">${Shell.icon("key")}<h2 class="u-14">Binding</h2>
          <button class="icon-btn u-right" data-x aria-label="Close">${Shell.icon("x")}</button></div>
        <div class="panel-b"><dl class="kv">
            <dt>Consumer</dt><dd class="u-mono">${b.from}</dd><dt>Target</dt><dd class="u-mono">${b.to}</dd>
            <dt>Scope</dt><dd>${b.scope}</dd><dt>Credentials</dt><dd>${b.age}</dd><dt>Created by</dt><dd>${b.by}</dd></dl>
          <div style="margin-top:16px"><button class="btn btn--danger-outline" data-unbind>Unbind\u2026</button></div>
          <p class="u-12 u-faint" style="margin-top:12px">Re-binding later creates new credentials.</p></div>`;
      p.classList.add("is-open");
      p.querySelector("[data-x]").addEventListener("click", () => p.classList.remove("is-open"));
      p.querySelector("[data-unbind]").addEventListener("click", () => {
        Shell.dialog({
          title: `Unbind ${b.from} \u2192 ${b.to}?`, danger: true, typed: b.to,
          body: `<p>The consumer loses access immediately. Injected configuration is removed on its next restart. Re-binding creates <b>new credentials</b>.</p>`,
          confirmLabel: "Unbind", confirmClass: "btn--danger",
          onConfirm: (close) => { close(); p.classList.remove("is-open"); Shell.toast(`Unbound <b class="u-mono">${b.from} \u2192 ${b.to}</b> (prototype \u2014 resets on reload).`); },
        });
      });
    }

    function inlineIcon(name, w, h) {
      const paths = { db: '<ellipse cx="12" cy="5.5" rx="7.5" ry="3"/><path d="M4.5 5.5v13c0 1.7 3.4 3 7.5 3s7.5-1.3 7.5-3v-13M4.5 12c0 1.7 3.4 3 7.5 3s7.5-1.3 7.5-3"/>', bolt: '<path d="M13 2 4.5 13.5H11L10 22l8.5-11.5H13z"/>', box: '<path d="M21 8.5v9L12 22l-9-4.5v-9L12 4z"/><path d="m3 8.5 9 4.5 9-4.5M12 13v9"/>', list: '<path d="M4 6h12M4 12h16M4 18h10"/><circle cx="20" cy="6" r="1.4"/>', external: '<path d="M14 4.5h5.5V10M19.5 4.5 11 13"/><path d="M19.5 14v5a1 1 0 0 1-1 1h-13a1 1 0 0 1-1-1v-13a1 1 0 0 1 1-1h5"/>' };
      return `<svg width="${w}" height="${h}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">${paths[name] || ""}</svg>`;
    }

    build();
    return { rerender: build };
  };
})();
