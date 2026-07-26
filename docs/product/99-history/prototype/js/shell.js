/* ============================================================
   Steloit prototype — Shell engine
   Renders SH-01 chrome (context bar, nav, ⌘K, notifications),
   owns context resolution (§6.4), keyboard map (§6.11),
   toasts, dialogs, and prototype review controls.
   ============================================================ */
(function () {
  "use strict";

  /* ---------------- Icons (inline SVG sprite) ---------------- */
  const IC = {
    hex: '<path d="M12 2.5 20 7v10l-8 4.5L4 17V7z"/>',
    chev: '<path d="m6 9 6 6 6-6"/>',
    chevR: '<path d="m9 6 6 6-6 6"/>',
    search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>',
    bell: '<path d="M18 9a6 6 0 1 0-12 0c0 6-2.5 7-2.5 7h17S18 15 18 9"/><path d="M10.5 20a2 2 0 0 0 3 0"/>',
    gear: '<circle cx="12" cy="12" r="3"/><path d="M12 2v3m0 14v3M4.9 4.9l2.1 2.1m10 10 2.1 2.1M2 12h3m14 0h3M4.9 19.1 7 17m10-10 2.1-2.1"/>',
    plus: '<path d="M12 5v14M5 12h14"/>',
    check: '<path d="m5 12.5 5 5L19 7"/>',
    x: '<path d="M6 6l12 12M18 6 6 18"/>',
    warn: '<path d="M12 3 2.5 20h19z"/><path d="M12 9v5m0 3v.01"/>',
    info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v6m0-9v.01"/>',
    db: '<ellipse cx="12" cy="5.5" rx="7.5" ry="3"/><path d="M4.5 5.5v13c0 1.7 3.4 3 7.5 3s7.5-1.3 7.5-3v-13M4.5 12c0 1.7 3.4 3 7.5 3s7.5-1.3 7.5-3"/>',
    bolt: '<path d="M13 2 4.5 13.5H11L10 22l8.5-11.5H13z"/>',
    box: '<path d="M21 8.5v9L12 22l-9-4.5v-9L12 4z"/><path d="m3 8.5 9 4.5 9-4.5M12 13v9"/>',
    list: '<path d="M4 6h12M4 12h16M4 18h10"/><circle cx="20" cy="6" r="1.4"/>',
    grid: '<rect x="4" y="4" width="7" height="7" rx="1.5"/><rect x="13" y="4" width="7" height="7" rx="1.5"/><rect x="4" y="13" width="7" height="7" rx="1.5"/><rect x="13" y="13" width="7" height="7" rx="1.5"/>',
    home: '<path d="m3.5 10.5 8.5-7 8.5 7V20a1 1 0 0 1-1 1h-5v-6h-5v6h-5a1 1 0 0 1-1-1z"/>',
    graph: '<circle cx="5.5" cy="12" r="2.5"/><circle cx="18.5" cy="6" r="2.5"/><circle cx="18.5" cy="18" r="2.5"/><path d="M8 11 16 7m-8 6 8 4"/>',
    eye: '<path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12z"/><circle cx="12" cy="12" r="3"/>',
    layers: '<path d="m12 3 9 5-9 5-9-5z"/><path d="m3 13 9 5 9-5"/>',
    key: '<circle cx="8" cy="15" r="4.5"/><path d="m11.5 11.5 8-8M17 6l2.5 2.5M14.5 8.5 17 11"/>',
    users: '<circle cx="9" cy="8" r="3.5"/><path d="M2.5 20c0-3.5 3-5.5 6.5-5.5s6.5 2 6.5 5.5"/><path d="M16 4.8a3.5 3.5 0 0 1 0 6.4M18 14.7c2.1.8 3.5 2.4 3.5 5.3"/>',
    card: '<rect x="2.5" y="5.5" width="19" height="13" rx="2"/><path d="M2.5 10h19"/>',
    doc: '<path d="M6 2.5h8L19 8v13.5H6z"/><path d="M13.5 2.5V8H19"/>',
    copy: '<rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>',
    term: '<rect x="2.5" y="4.5" width="19" height="15" rx="2"/><path d="m6.5 9 3 3-3 3M12 15.5h5"/>',
    sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2.5v2m0 15v2M4.6 4.6 6 6m12 12 1.4 1.4M2.5 12h2m15 0h2M4.6 19.4 6 18M18 6l1.4-1.4"/>',
    moon: '<path d="M20.5 14.5A8.5 8.5 0 0 1 9.5 3.5a8.5 8.5 0 1 0 11 11z"/>',
    kbd: '<rect x="2.5" y="6" width="19" height="12" rx="2"/><path d="M7 10h.01M11 10h.01M15 10h.01M7 14h10"/>',
    arrow: '<path d="M5 12h14m-6-6 6 6-6 6"/>',
    ai: '<path d="M12 3v3m0 12v3M5.6 5.6l2.1 2.1m8.6 8.6 2.1 2.1M3 12h3m12 0h3M5.6 18.4l2.1-2.1m8.6-8.6 2.1-2.1"/><circle cx="12" cy="12" r="3.5"/>',
    external: '<path d="M14 4.5h5.5V10M19.5 4.5 11 13"/><path d="M19.5 14v5a1 1 0 0 1-1 1h-13a1 1 0 0 1-1-1v-13a1 1 0 0 1 1-1h5"/>',
    archive: '<rect x="3" y="4" width="18" height="5" rx="1"/><path d="M5 9v10a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1V9M10 13h4"/>',
    trash: '<path d="M4 7h16M9.5 7V4.5h5V7M6.5 7l1 13h9l1-13"/><path d="M10 11v6m4-6v6"/>',
  };
  function icon(name, cls) {
    return `<svg class="ic ${cls || ""}" viewBox="0 0 24 24" aria-hidden="true">${IC[name] || IC.info}</svg>`;
  }

  /* ---------------- Context resolution (§6.4) ---------------- */
  const params = new URLSearchParams(location.search);
  const ctx = {
    org: DATA.org.slug,
    project: params.get("project") || "ecommerce",
    env: params.get("env") || null,
  };
  function resolveCtx() {
    const p = DB.project(ctx.project);
    if (!p) return { bad: "project" };
    if (p.archived) ctx.env = null;
    else {
      const envs = p.envs.map(e => e.slug);
      if (!ctx.env || !envs.includes(ctx.env)) ctx.env = envs.includes("production") ? "production" : envs[0];
    }
    return { bad: null };
  }
  function q(extra) { // build query preserving context
    const s = new URLSearchParams({ project: ctx.project, env: ctx.env || "" });
    if (extra) Object.entries(extra).forEach(([k, v]) => s.set(k, v));
    return "?" + s.toString();
  }

  /* ---------------- Toasts ---------------- */
  let toastZone;
  function toast(html, ms) {
    if (!toastZone) { toastZone = el("div", "toast-zone"); document.body.appendChild(toastZone); }
    const t = el("div", "toast"); t.setAttribute("role", "status"); t.innerHTML = html;
    toastZone.appendChild(t);
    setTimeout(() => t.remove(), ms || 4200);
  }

  /* ---------------- tiny DOM helpers ---------------- */
  function el(tag, cls, html) { const n = document.createElement(tag); if (cls) n.className = cls; if (html != null) n.innerHTML = html; return n; }
  function esc(s) { return String(s).replace(/[&<>"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c])); }
  const $ = (s, r) => (r || document).querySelector(s);
  const $$ = (s, r) => Array.from((r || document).querySelectorAll(s));

  /* ---------------- Chrome rendering ---------------- */
  let opts = {};
  function renderChrome() {
    const app = $("#app");
    const p = DB.project(ctx.project);
    const env = p && !p.archived ? DB.env(ctx.project, ctx.env) : null;
    const scoped = opts.scope !== "org" && opts.scope !== "user" && p && !p.archived;
    const unread = DATA.notifications.filter(n => n.unread && !readSet.has(n.id)).length;

    const topbar = el("header", "topbar");
    topbar.innerHTML = `
      <a class="logo" href="index.html">${icon("hex", "ic--l hex")}<span>steloit</span></a>
      <nav class="ctx" aria-label="Context">
        <button class="ctx-btn" id="sw-org" aria-haspopup="listbox" aria-expanded="false">
          <span class="ctx-label">${esc(ctx.org)}</span>${icon("chev")}
        </button>
        ${scoped ? `<span class="sep" aria-hidden="true">/</span>
        <button class="ctx-btn" id="sw-proj" aria-haspopup="listbox" aria-expanded="false">
          <span class="ctx-label">${esc(ctx.project)}</span>${icon("chev")}
        </button>
        <span class="sep" aria-hidden="true">/</span>
        <button class="ctx-btn ${env && env.production ? "is-prod" : ""}" id="sw-env" aria-haspopup="listbox" aria-expanded="false">
          <span class="ctx-label">${esc(ctx.env)}</span>
          <span class="tag-region">${env ? env.region : ""}</span>${icon("chev")}
        </button>` : ""}
      </nav>
      <div class="topbar-right">
        <span class="offline-pill" id="offline-pill" role="status">${icon("warn", "ic--s")}<span>Reconnecting\u2026 data as of 14:02</span></span>
        <button class="palette-hint" id="btn-pal">${icon("search", "ic--s")}<span>Search or jump\u2026</span><span class="kbd">\u2318K</span></button>
        <button class="icon-btn" id="btn-ntf" aria-label="Notifications${unread ? `, ${unread} unread` : ""}" aria-haspopup="dialog">
          ${icon("bell")}${unread ? `<span class="count">${unread > 9 ? "9+" : unread}</span>` : ""}
        </button>
        <button class="icon-btn" id="btn-user" aria-label="Your account" aria-haspopup="menu"><span class="avatar">${DATA.user.initials}</span></button>
      </div>`;

    const nav = el("nav", "nav"); nav.setAttribute("aria-label", "Primary");
    const link = (href, ic2, label, key, extra) => {
      const on = opts.section === key;
      return `<a class="nav-link ${on ? "is-on" : ""} ${extra && extra.disabled ? "is-disabled" : ""}" href="${href}" ${on ? 'aria-current="page"' : ""}>
        ${icon(ic2)}<span>${label}</span>${extra && extra.tag ? `<span class="tag-ver">${extra.tag}</span>` : ""}</a>`;
    };
    nav.innerHTML = `
      ${scoped ? `<div class="nav-group">
        <div class="nav-title">Project</div>
        ${link("overview.html" + q(), "home", "Overview", "overview")}
        ${link("services.html" + q(), "layers", "Services", "services")}
        ${link("topology.html" + q(), "graph", "Topology", "topology")}
        ${link("observe.html" + q(), "eye", "Observe", "observe", { tag: "v0.5" })}
        ${link("deployments.html" + q(), "external", "Deployments", "deployments", { tag: "v2" })}
        ${link("settings-project.html" + q(), "gear", "Settings", "proj-settings")}
      </div>` : ""}
      <div class="nav-group">
        <div class="nav-title">Organization</div>
        ${link("index.html", "grid", "Projects", "projects")}
        ${link("templates.html", "copy", "Templates", "templates", { tag: "v0.5" })}
        ${link("members.html", "users", "Members", "org-members")}
        ${link("billing.html", "card", "Billing", "org-billing", { tag: "v0.5" })}
        ${link("assistant.html", "ai", "Assistant", "assistant", { tag: "v0.5" })}
        ${link("settings-org.html#audit", "doc", "Audit", "org-audit", { tag: "v0.5" })}
      </div>
      <div class="nav-foot"><span class="dot-state" aria-hidden="true"></span><span>All systems normal</span></div>`;

    app.prepend(topbar); app.insertBefore(nav, app.children[1]);

    // switchers
    bindSwitcher("#sw-org", orgSwitcherItems, "Switch organization");
    if (scoped) {
      bindSwitcher("#sw-proj", projSwitcherItems, "Switch project");
      bindSwitcher("#sw-env", envSwitcherItems, "Switch environment");
    }
    $("#btn-pal").addEventListener("click", () => openPalette(""));
    $("#btn-ntf").addEventListener("click", toggleNtf);
    $("#btn-user").addEventListener("click", toggleUserMenu);
  }

  /* ---------------- Switcher popovers (LR-1) ---------------- */
  let openPop = null;
  function closePop() { if (openPop) { openPop.pop.remove(); openPop.btn.setAttribute("aria-expanded", "false"); openPop = null; } }
  function bindSwitcher(sel, itemsFn, label) {
    const btn = $(sel); if (!btn) return;
    btn.addEventListener("click", (e) => {
      e.stopPropagation();
      if (openPop && openPop.btn === btn) return closePop();
      closePop(); closeNtf();
      const pop = el("div", "pop is-open");
      pop.setAttribute("role", "listbox"); pop.setAttribute("aria-label", label);
      const r = btn.getBoundingClientRect();
      pop.style.left = Math.min(r.left, innerWidth - 290) + "px";
      pop.style.top = r.bottom + 6 + "px";
      pop.innerHTML = `<div class="pop-search">${icon("search", "ic--s")}<input type="text" placeholder="${label}\u2026" aria-label="${label}"></div>
        <div class="pop-list"></div><div class="pop-foot"></div>`;
      document.body.appendChild(pop);
      openPop = { pop, btn };
      btn.setAttribute("aria-expanded", "true");
      const input = $("input", pop);
      const render = () => { const { list, foot } = itemsFn(input.value.trim().toLowerCase()); $(".pop-list", pop).innerHTML = list; $(".pop-foot", pop).innerHTML = foot || ""; };
      input.addEventListener("input", render); render(); input.focus();
      pop.addEventListener("click", (ev) => {
        const a = ev.target.closest("[data-env]");
        if (a) { ev.preventDefault(); switchEnv(a.dataset.env); closePop(); }
      });
    });
  }
  document.addEventListener("click", (e) => { if (openPop && !openPop.pop.contains(e.target)) closePop(); });

  function orgSwitcherItems(f) {
    return {
      list: `<button class="pop-item is-active"><span class="mark">${icon("check", "ic--s")}</span><span class="mono-name">acme</span><span class="u-right u-faint u-12">${DATA.plan}</span></button>`,
      foot: `<button class="pop-item" onclick="Shell.toast('Creating organizations arrives with plan rules (v0.5).')">${icon("plus", "ic--s")}<span>Create organization</span><span class="u-right tag-ver">v0.5</span></button>`,
    };
  }
  function projSwitcherItems(f) {
    const ps = DB.projects(false).filter(p => !f || p.slug.includes(f));
    const cur = ps.filter(p => p.slug === ctx.project), rest = ps.filter(p => p.slug !== ctx.project);
    const row = (p, active) => {
      const st = p.envs.length ? DB.worstState(p.envs[0].services) : "ready";
      return `<a class="pop-item ${active ? "is-active" : ""}" href="overview.html?project=${p.slug}">
        <span class="mark">${active ? icon("check", "ic--s") : ""}</span>
        <span class="dot-state" style="background:var(--status-${st})" aria-hidden="true"></span>
        <span class="mono-name">${p.slug}</span></a>`;
    };
    return {
      list: (cur.map(p => row(p, true)).join("") + rest.map(p => row(p, false)).join("")) || `<div class="pal-empty">No projects match.</div>`,
      foot: `<a class="pop-item" href="wizard.html">${icon("plus", "ic--s")}<span>New project</span></a>`,
    };
  }
  function envSwitcherItems(f) {
    const p = DB.project(ctx.project);
    const envs = p.envs.filter(e => !f || e.slug.includes(f));
    return {
      list: envs.map(e => `<button class="pop-item ${e.slug === ctx.env ? "is-active" : ""}" data-env="${e.slug}">
          <span class="mark">${e.slug === ctx.env ? icon("check", "ic--s") : ""}</span>
          <span class="mono-name">${e.slug}</span>
          ${e.production ? `<span class="tag-prod">production</span>` : ""}
          <span class="u-right tag-region">${e.region}</span></button>`).join("") || `<div class="pal-empty">No environments match.</div>`,
      foot: `<a class="pop-item" href="environments.html${q({ create: 1 })}">${icon("plus", "ic--s")}<span>New environment</span></a>`,
    };
  }

  /* --------- Environment switch = filter (§6.4 rule 4–5) --------- */
  function switchEnv(envSlug) {
    if (envSlug === ctx.env) return;
    const missing = opts.envRouteCheck && !opts.envRouteCheck(envSlug);
    const content = $(".content");
    const finish = () => {
      ctx.env = envSlug;
      const url = new URL(location.href); url.searchParams.set("env", envSlug);
      if (missing) {
        // C-12: redirect to nearest ancestor
        location.href = "services.html" + q() .replace(/env=[^&]*/, "env=" + envSlug) + "&missing=" + encodeURIComponent(opts.missingName || "");
        return;
      }
      try { history.replaceState(null, "", url); } catch (err) { location.href = url; return; }
      renderContextBits();
      if (window.renderPage) window.renderPage(Shell.ctx());
      if (content) { content.classList.remove("env-fading"); content.classList.add("env-faded-in"); setTimeout(() => content.classList.remove("env-faded-in"), 200); }
      announce(`Now viewing ${ctx.project}, ${envSlug}.`); // §10.1
    };
    if (content) { content.classList.add("env-fading"); setTimeout(finish, getComputedStyle(document.documentElement).getPropertyValue("--motion-fast") === "0ms" ? 0 : 150); }
    else finish();
  }
  function renderContextBits() {
    const p = DB.project(ctx.project); const env = DB.env(ctx.project, ctx.env);
    if (env && opts.scope !== "org" && opts.scope !== "user") {
      const w = env.services.length ? DB.worstState(env.services) : "ready";
      document.title = (w === "ready" ? "\u25CF " : "\u25D0 ") + ctx.project + " \u2014 Steloit";
    }
    const b = $("#sw-env"); if (!b || !env) return;
    b.classList.toggle("is-prod", !!env.production);
    $(".ctx-label", b).textContent = ctx.env; $(".tag-region", b).textContent = env.region;
    $$(".nav-link").forEach(a => { if (a.href.includes("?")) a.href = a.href.replace(/env=[^&]*/, "env=" + ctx.env); });
  }

  /* ---------------- Live region (a11y announcements) ---------------- */
  let live;
  function announce(msg) { if (!live) { live = el("div", "u-sr"); live.setAttribute("aria-live", "polite"); document.body.appendChild(live); } live.textContent = msg; }

  /* ---------------- ⌘K palette (P-5) ---------------- */
  let pal, palItems = [], palIdx = 0;
  function commandRegistry() { // §6.7 — no destructive verbs (R-CMD-1)
    const c = [];
    const go = (label, href) => c.push({ g: "Actions", ic: "arrow", label, href });
    c.push({ g: "Actions", ic: "plus", label: "New project", href: "wizard.html" });
    if (opts.scope !== "org") {
      c.push({ g: "Actions", ic: "plus", label: "Add service \u2014 in " + ctx.project + "/" + ctx.env, href: "wizard.html" + q({ mode: "service" }) });
      c.push({ g: "Actions", ic: "plus", label: "New environment", href: "environments.html" + q({ create: 1 }) });
      go("Go to Overview", "overview.html" + q()); go("Go to Services", "services.html" + q());
      go("Go to Topology", "topology.html" + q()); go("Go to Environments", "environments.html" + q());
      go("Open project settings", "settings-project.html" + q());
      go("Open environment settings", "settings-env.html" + q());
      go("Open Secrets \u2014 " + ctx.env, "secrets.html" + q());
      go("Open Deployments (v2)", "deployments.html" + q());
    }
    go("Open Observe", "observe.html" + q());
    go("Open Billing", "billing.html"); go("Open Members", "members.html");
    go("Browse Templates", "templates.html");
    c.push({ g: "Actions", ic: "ai", label: "Open Assistant", href: "assistant.html" });
    go("Open organization settings", "settings-org.html"); go("Open user settings", "settings-user.html");
    c.push({ g: "Actions", ic: "users", label: "Invite member", href: "members.html" });
    c.push({ g: "Actions", ic: "bell", label: "View notifications", fn: () => { closePalette(); toggleNtf(); } });
    c.push({ g: "Actions", ic: "copy", label: "Copy current URL", fn: () => { try { navigator.clipboard.writeText(location.href); } catch (e) {} toast("URL copied."); closePalette(); } });
    c.push({ g: "Actions", ic: "moon", label: "Toggle theme", fn: () => { Shell.toggleTheme(); closePalette(); } });
    c.push({ g: "Actions", ic: "term", label: "steloit CLI setup", href: "docs.html#cli" });
    return c;
  }
  function searchIndex() { // §4.5
    const out = [];
    DB.projects(true).forEach(p => out.push({ g: "Jump to", ic: "grid", label: p.slug, crumb: "acme", tag: p.archived ? "archived" : "project", href: p.archived ? "index.html?filter=archived" : "overview.html?project=" + p.slug, score: p.slug === ctx.project ? 3 : 1 }));
    DB.projects(false).forEach(p => p.envs.forEach(e => {
      out.push({ g: "Jump to", ic: "layers", label: p.slug + "/" + e.slug, crumb: "environment", tag: "env", href: "overview.html?project=" + p.slug + "&env=" + e.slug, score: p.slug === ctx.project ? 2 : 0 });
      e.services.forEach(s => out.push({ g: "Jump to", ic: DATA.svcTypes[s.type].icon, label: s.id, crumb: p.slug + "/" + e.slug, tag: DATA.svcTypes[s.type].label, href: `service.html?project=${p.slug}&env=${e.slug}&id=${s.id}`, score: (p.slug === ctx.project && e.slug === ctx.env) ? 4 : (p.slug === ctx.project ? 2 : 0) }));
    }));
    ["General", "Members", "Billing", "Policies", "Audit"].forEach(s => out.push({ g: "Jump to", ic: "gear", label: "Org settings \u00b7 " + s, crumb: "settings", tag: "settings", href: "settings-org.html#" + s.toLowerCase(), score: 0 }));
    out.push({ g: "Jump to", ic: "eye", label: "Observe \u00b7 metrics, logs, alerts", crumb: ctx.project + "/" + ctx.env, tag: "v0.5", href: "observe.html" + q(), score: 1 });
    out.push({ g: "Jump to", ic: "key", label: "Secrets \u00b7 " + ctx.env, crumb: ctx.project, tag: "v0.5", href: "secrets.html" + q(), score: 1 });
    out.push({ g: "Jump to", ic: "external", label: "Deployments \u00b7 previews with DB branches", crumb: ctx.project, tag: "v2", href: "deployments.html" + q(), score: 0 });
    out.push({ g: "Jump to", ic: "users", label: "Members", crumb: "acme", tag: "org", href: "members.html", score: 0 });
    out.push({ g: "Jump to", ic: "card", label: "Billing \u00b7 usage & invoices", crumb: "acme", tag: "v0.5", href: "billing.html", score: 0 });
    out.push({ g: "Jump to", ic: "copy", label: "Templates", crumb: "acme", tag: "v0.5", href: "templates.html", score: 0 });
    out.push({ g: "Jump to", ic: "ai", label: "Assistant \u00b7 ask about your org", crumb: "acme", tag: "v0.5", href: "assistant.html", score: 0 });
    DATA.docs.forEach(d => out.push({ g: "Docs", ic: "doc", label: d.title, crumb: "docs", tag: "v0.5", href: d.href, score: -1 }));
    return out;
  }
  function openPalette(prefill) {
    closeNtf(); closePop();
    if (!pal) {
      pal = el("div", "pal-scrim");
      pal.innerHTML = `<div class="pal" role="dialog" aria-modal="true" aria-label="Search and commands">
        <div class="pal-input">${icon("search")}<span class="mode-tag" id="pal-mode"></span>
          <input id="pal-q" type="text" placeholder="Search\u2026  \u00b7  > for commands  \u00b7  ? to ask" aria-label="Search or type a command">
        </div>
        <div class="pal-list" id="pal-list" role="listbox"></div>
        <div class="pal-foot"><span><span class="kbd">\u2191\u2193</span> navigate</span><span><span class="kbd">\u21b5</span> open</span><span><span class="kbd">esc</span> close</span></div></div>`;
      document.body.appendChild(pal);
      pal.addEventListener("click", (e) => { if (e.target === pal) closePalette(); });
      const input = $("#pal-q", pal);
      input.addEventListener("input", renderPal);
      input.addEventListener("keydown", (e) => {
        if (e.key === "ArrowDown") { e.preventDefault(); movePal(1); }
        else if (e.key === "ArrowUp") { e.preventDefault(); movePal(-1); }
        else if (e.key === "Enter") { e.preventDefault(); actPal(); }
      });
    }
    pal.classList.add("is-open");
    const input = $("#pal-q", pal); input.value = prefill || ""; input.focus(); renderPal();
  }
  function closePalette() { if (pal) pal.classList.remove("is-open"); }
  function renderPal() {
    const v = $("#pal-q", pal).value;
    const list = $("#pal-list", pal); const modeTag = $("#pal-mode", pal);
    palIdx = 0;
    if (v.startsWith(">")) {
      modeTag.style.display = "inline-block"; modeTag.textContent = "Commands";
      const f = v.slice(1).trim().toLowerCase();
      palItems = commandRegistry().filter(c => c.label.toLowerCase().includes(f));
      paintPalList(list, palItems, "No matching command.");
    } else if (v.startsWith("?")) {
      modeTag.style.display = "inline-block"; modeTag.textContent = "Ask \u00b7 v0.5";
      palItems = [];
      const qq = v.slice(1).trim();
      list.innerHTML = `<div class="pal-answer">${qq ? askAnswer(qq) : `<div class="pal-empty">Ask about your project \u2014 answers cite the docs and never take actions. <span class="tag-ver">v0.5</span></div>`}</div>`;
    } else {
      modeTag.style.display = "none";
      const f = v.trim().toLowerCase();
      let items = searchIndex();
      if (f) items = items.filter(i => (i.label + " " + i.crumb).toLowerCase().includes(f));
      items.sort((a, b) => b.score - a.score);
      palItems = items.slice(0, 12);
      if (!f) palItems = palItems.slice(0, 7);
      paintPalList(list, palItems, `Nothing in <b>${esc(ctx.project)}</b> matches. <button class="btn btn--s" onclick="Shell.palWiden()">Search all of acme</button>`);
    }
  }
  function paintPalList(list, items, emptyHtml) {
    if (!items.length) { list.innerHTML = `<div class="pal-empty">${emptyHtml}</div>`; return; }
    let html = ""; let lastG = "";
    items.forEach((it, i) => {
      if (it.g !== lastG) { html += `<div class="pal-group">${it.g}</div>`; lastG = it.g; }
      html += `<button class="pal-item ${i === palIdx ? "is-active" : ""}" data-i="${i}" role="option" aria-selected="${i === palIdx}">
        ${icon(it.ic || "arrow", "ic--s")}<span class="u-trunc">${esc(it.label)}</span>
        ${it.crumb ? `<span class="crumb">${esc(it.crumb)}</span>` : ""}${it.tag ? `<span class="type-tag">${esc(it.tag)}</span>` : ""}</button>`;
    });
    list.innerHTML = html;
    $$(".pal-item", list).forEach(b => b.addEventListener("click", () => { palIdx = +b.dataset.i; actPal(); }));
  }
  function movePal(d) {
    if (!palItems.length) return;
    palIdx = (palIdx + d + palItems.length) % palItems.length;
    $$(".pal-item", pal).forEach(b => { const on = +b.dataset.i === palIdx; b.classList.toggle("is-active", on); b.setAttribute("aria-selected", on); if (on) b.scrollIntoView({ block: "nearest" }); });
  }
  function actPal() {
    const it = palItems[palIdx]; if (!it) return;
    if (it.fn) return it.fn();
    if (it.href) location.href = it.href;
  }
  function askAnswer(qq) { // canned, attributed, action-free (G9 / AIC-SHELL-2 stub)
    const low = qq.toLowerCase();
    let a = "In Steloit, an <b>organization</b> holds projects; each <b>project</b> is one application with isolated <b>environments</b>; <b>services</b> live inside an environment. Everything you can click has a CLI and API equivalent.";
    if (low.includes("cost") || low.includes("bill") || low.includes("estimate")) a = "Fixed-price services show exact monthly prices; usage-based items are marked with ~ and estimated at typical use. Nothing bills until you confirm, and every provisioning flow shows its estimate first.";
    else if (low.includes("delete") || low.includes("destroy")) a = "Destroying a project requires typing its name and starts a <b>7-day grace period</b> \u2014 it stays restorable until then. Archiving is the reversible option: services stop, data is kept.";
    else if (low.includes("connect")) a = "Run <code>steloit env pull</code> in your app's directory \u2014 it writes <code>.env.steloit</code> with your connection settings and keeps them fresh when credentials rotate.";
    else if (low.includes("region")) a = "Region is a property of each environment: all services in an environment share it. Need another region? Create a new environment.";
    return `<div class="ans-card"><div class="u-row" style="margin-bottom:8px"><span class="ai-badge">${icon("ai", "ic--s")} Steloit Assistant</span></div>
      ${a}<div class="ans-cites"><span class="chip">${icon("doc", "ic--s")} How Steloit is organized</span><span class="chip">${icon("doc", "ic--s")} Delete safely</span></div>
      <p class="u-12 u-faint" style="margin-top:10px">Answers link to real, permissioned surfaces \u2014 the assistant never executes. Full conversation: PDS-012.</p></div>`;
  }

  /* ---------------- Notifications (P-6) ---------------- */
  const readSet = new Set();
  let ntfPanel, ntfCat = "All";
  function toggleNtf() {
    closePop(); closePalette();
    if (ntfPanel && ntfPanel.classList.contains("is-open")) return closeNtf();
    if (!ntfPanel) {
      ntfPanel = el("aside", "panel ntf-panel");
      ntfPanel.setAttribute("role", "dialog"); ntfPanel.setAttribute("aria-label", "Notifications");
      document.body.appendChild(ntfPanel);
    }
    renderNtf(); ntfPanel.classList.add("is-open");
  }
  function closeNtf() { if (ntfPanel) ntfPanel.classList.remove("is-open"); }
  function renderNtf() {
    const items = DATA.notifications.filter(n => ntfCat === "All" || n.cat === ntfCat);
    ntfPanel.innerHTML = `
      <div class="panel-h"><h2 class="u-14">Notifications</h2>
        <button class="btn btn--ghost btn--s u-right" id="ntf-markall">Mark all read</button>
        <button class="icon-btn" id="ntf-close" aria-label="Close">${icon("x")}</button></div>
      <div class="ntf-chips">${DATA.ntfCats.map(c => `<button class="chip chip--btn ${c === ntfCat ? "chip--on" : ""}" data-cat="${c}">${c}</button>`).join("")}</div>
      <div class="panel-b" style="padding:0">
        ${items.length ? items.map(n => {
          const unread = n.unread && !readSet.has(n.id);
          return `<div class="ntf-item ${unread ? "is-unread" : ""}" data-id="${n.id}" data-href="${n.href}" tabindex="0" role="link">
            <span class="n-dot" aria-hidden="true"></span>
            <div class="u-grow"><div class="n-title">${esc(n.title)}</div><div class="n-sub">${esc(n.sub)}</div></div>
            <time>${n.t}</time></div>`;
        }).join("") : `<div class="empty" style="margin:40px auto"><div class="empty-art">${icon("bell")}</div><p>All quiet. You'll hear about what matters.</p></div>`}
      </div>
      <div class="panel-h" style="border-top:1px solid var(--border);border-bottom:0">
        <a class="u-12" href="settings-user.html#notifications">Notification settings \u2192</a></div>`;
    $("#ntf-close", ntfPanel).addEventListener("click", closeNtf);
    $("#ntf-markall", ntfPanel).addEventListener("click", () => { DATA.notifications.forEach(n => readSet.add(n.id)); renderNtf(); refreshBell(); });
    $$(".chip[data-cat]", ntfPanel).forEach(c => c.addEventListener("click", () => { ntfCat = c.dataset.cat; renderNtf(); }));
    $$(".ntf-item", ntfPanel).forEach(it => {
      const act = () => { readSet.add(+it.dataset.id); refreshBell(); if (it.dataset.href && it.dataset.href !== "#") location.href = it.dataset.href; else renderNtf(); };
      it.addEventListener("click", act);
      it.addEventListener("keydown", e => { if (e.key === "Enter") act(); });
    });
  }
  function refreshBell() {
    const unread = DATA.notifications.filter(n => n.unread && !readSet.has(n.id)).length;
    const btn = $("#btn-ntf"); if (!btn) return;
    btn.innerHTML = `${icon("bell")}${unread ? `<span class="count">${unread > 9 ? "9+" : unread}</span>` : ""}`;
    btn.setAttribute("aria-label", "Notifications" + (unread ? `, ${unread} unread` : ""));
  }

  /* ---------------- User menu ---------------- */
  function toggleUserMenu() {
    closePop(); closeNtf();
    const btn = $("#btn-user");
    const pop = el("div", "pop is-open"); pop.setAttribute("role", "menu");
    const r = btn.getBoundingClientRect();
    pop.style.right = "12px"; pop.style.left = "auto"; pop.style.top = r.bottom + 6 + "px"; pop.style.minWidth = "220px";
    pop.innerHTML = `<div class="pop-list">
      <div style="padding:8px 12px"><div class="u-12" style="font-weight:600">${DATA.user.name}</div><div class="u-12 u-faint">${DATA.user.email}</div></div>
      <a class="pop-item" href="settings-user.html">${icon("gear", "ic--s")}<span>User settings</span></a>
      <button class="pop-item" onclick="Shell.toggleTheme()">${icon("moon", "ic--s")}<span>Toggle theme</span></button>
      <button class="pop-item" onclick="Shell.sheet()">${icon("kbd", "ic--s")}<span>Keyboard shortcuts</span><span class="u-right kbd">?</span></button>
      <a class="pop-item" href="docs.html#cli">${icon("term", "ic--s")}<span>steloit CLI setup</span></a></div>`;
    document.body.appendChild(pop); openPop = { pop, btn };
  }

  /* ---------------- Dialog helper (G3 tiers) ---------------- */
  function dialog({ title, danger, body, confirmLabel, confirmClass, typed, onConfirm, cancelLabel }) {
    const scrim = el("div", "dlg-scrim is-open");
    const id = "dlg-" + Math.random().toString(36).slice(2, 7);
    scrim.innerHTML = `<div class="dlg ${danger ? "dlg--danger" : ""}" role="${danger ? "alertdialog" : "dialog"}" aria-modal="true" aria-labelledby="${id}">
      <div class="dlg-h">${danger ? icon("warn") : ""}<h2 id="${id}">${title}</h2></div>
      <div class="dlg-b">${body}
        ${typed ? `<div class="field" style="margin-top:12px"><label for="${id}-in">Type <b class="u-mono">${esc(typed)}</b> to confirm</label>
          <input id="${id}-in" class="input input--mono" autocomplete="off" spellcheck="false">
          <div class="err u-hide" id="${id}-err">That doesn't match <b>${esc(typed)}</b>.</div></div>` : ""}</div>
      <div class="dlg-f"><button class="btn" data-x>${cancelLabel || "Cancel"}</button>
        <button class="btn ${confirmClass || "btn--primary"}" data-ok ${typed ? "disabled" : ""}>${confirmLabel}</button></div></div>`;
    document.body.appendChild(scrim);
    const dlg = $(".dlg", scrim), ok = $("[data-ok]", scrim), xBtn = $("[data-x]", scrim);
    const close = () => scrim.remove();
    xBtn.addEventListener("click", close);
    scrim.addEventListener("click", e => { if (e.target === scrim) close(); });
    scrim.addEventListener("keydown", e => { if (e.key === "Escape") { e.stopPropagation(); close(); } });
    if (typed) {
      const input = $(`#${id}-in`, scrim);
      input.addEventListener("input", () => { ok.disabled = input.value !== typed; $(`#${id}-err`).classList.add("u-hide"); });
      input.addEventListener("keydown", e => { if (e.key === "Enter" && input.value !== typed) $(`#${id}-err`).classList.remove("u-hide"); });
    }
    ok.addEventListener("click", () => { onConfirm(close, ok); });
    xBtn.focus();
    return { close };
  }

  /* ---------------- Keyboard map (§6.11) ---------------- */
  let lastKey = "", lastT = 0;
  document.addEventListener("keydown", (e) => {
    const tag = (e.target.tagName || "").toLowerCase();
    const typing = tag === "input" || tag === "textarea" || tag === "select" || e.target.isContentEditable;
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") { e.preventDefault(); openPalette(""); return; }
    if ((e.metaKey || e.ctrlKey) && e.key === "\\") { e.preventDefault(); $("#app").classList.toggle("nav-collapsed"); return; }
    if (e.key === "Escape") { closePalette(); closeNtf(); closePop(); document.querySelectorAll(".panel.is-open:not(.ntf-panel)").forEach(p => p.classList.remove("is-open")); return; }
    if (typing) return;
    if (e.key === "?") { e.preventDefault(); sheet(); return; }
    const scoped = opts.scope !== "org" && opts.scope !== "user";
    if (scoped && (e.key === "[" || e.key === "]")) {
      const p = DB.project(ctx.project); if (!p || !p.envs.length) return;
      const i = p.envs.findIndex(x => x.slug === ctx.env);
      const n = (i + (e.key === "]" ? 1 : -1) + p.envs.length) % p.envs.length;
      switchEnv(p.envs[n].slug); return;
    }
    const now = Date.now();
    if (lastKey === "g" && now - lastT < 900) {
      const map = scoped ? { o: "overview.html" + q(), s: "services.html" + q(), t: "topology.html" + q(), e: "environments.html" + q(), ",": "settings-project.html" + q() } : {};
      if (map[e.key]) { location.href = map[e.key]; lastKey = ""; return; }
    }
    if (e.key === "n") { lastKey = "n"; lastT = now; return; }
    if (lastKey === "n" && e.key === "p" && now - lastT < 900) { location.href = "wizard.html"; return; }
    lastKey = e.key; lastT = now;
  });

  function sheet() {
    dialog({
      title: "Keyboard shortcuts",
      body: `<div class="sheet-grid">
        <div class="sh-row"><span>Search &amp; commands</span><span class="keys"><span class="kbd">\u2318</span><span class="kbd">K</span></span></div>
        <div class="sh-row"><span>Collapse navigation</span><span class="keys"><span class="kbd">\u2318</span><span class="kbd">\\</span></span></div>
        <div class="sh-row"><span>Go to Overview</span><span class="keys"><span class="kbd">g</span><span class="kbd">o</span></span></div>
        <div class="sh-row"><span>Cycle environments</span><span class="keys"><span class="kbd">[</span><span class="kbd">]</span></span></div>
        <div class="sh-row"><span>Go to Services</span><span class="keys"><span class="kbd">g</span><span class="kbd">s</span></span></div>
        <div class="sh-row"><span>New project</span><span class="keys"><span class="kbd">n</span><span class="kbd">p</span></span></div>
        <div class="sh-row"><span>Go to Topology</span><span class="keys"><span class="kbd">g</span><span class="kbd">t</span></span></div>
        <div class="sh-row"><span>Project settings</span><span class="keys"><span class="kbd">g</span><span class="kbd">,</span></span></div>
        <div class="sh-row"><span>Go to Environments</span><span class="keys"><span class="kbd">g</span><span class="kbd">e</span></span></div>
        <div class="sh-row"><span>Close layer</span><span class="keys"><span class="kbd">esc</span></span></div>
      </div><p class="u-12 u-faint" style="margin-top:12px">Destructive actions are never a single keystroke.</p>`,
      confirmLabel: "Done", onConfirm: (close) => close(),
    });
  }

  /* ---------------- Theme + proto controls ---------------- */
  let theme = "dark";
  function toggleTheme() { theme = theme === "dark" ? "light" : "dark"; document.documentElement.setAttribute("data-theme", theme); closePop(); }

  let protoStates = [];
  const protoFlags = { theme: false, rm: false, off: false };
  function renderProto() {
    const fab = el("button", "proto-fab", `${icon("eye", "ic--s")}<span>Prototype</span>`);
    fab.setAttribute("aria-haspopup", "true");
    const panel = el("div", "proto-panel");
    panel.innerHTML = `<div class="pp-h">Design-review controls</div><div class="pp-b"></div>`;
    document.body.append(fab, panel);
    const paint = () => {
      $(".pp-b", panel).innerHTML = `
        <div class="pp-label">Global</div>
        <label class="pp-row"><span class="u-grow">Light theme</span><span class="switch"><input type="checkbox" id="pp-theme" ${protoFlags.theme ? "checked" : ""}><span class="tr"></span></span></label>
        <label class="pp-row"><span class="u-grow">Reduced motion</span><span class="switch"><input type="checkbox" id="pp-rm" ${protoFlags.rm ? "checked" : ""}><span class="tr"></span></span></label>
        <label class="pp-row"><span class="u-grow">Offline pill (B-0)</span><span class="switch"><input type="checkbox" id="pp-off" ${protoFlags.off ? "checked" : ""}><span class="tr"></span></span></label>
        ${protoStates.length ? `<div class="pp-label">This screen (state demos)</div>` + protoStates.map((s, i) => `<button class="pp-row" style="width:100%;text-align:left" data-pp="${i}"><span class="u-grow">${s.label}</span>${icon("chevR", "ic--s")}</button>`).join("") : ""}`;
      $("#pp-theme", panel).addEventListener("change", () => { protoFlags.theme = !protoFlags.theme; toggleTheme(); });
      $("#pp-rm", panel).addEventListener("change", e => { protoFlags.rm = e.target.checked; document.documentElement.classList.toggle("reduced-motion", e.target.checked); });
      $("#pp-off", panel).addEventListener("change", e => { protoFlags.off = e.target.checked; $("#offline-pill").classList.toggle("is-on", e.target.checked); });
      $$("[data-pp]", panel).forEach(b => b.addEventListener("click", () => { protoStates[+b.dataset.pp].fn(); panel.classList.remove("is-open"); }));
    };
    fab.addEventListener("click", () => { if (!panel.classList.contains("is-open")) paint(); panel.classList.toggle("is-open"); });
  }

  /* ---------------- init ---------------- */
  window.Shell = {
    init(o) {
      opts = o || {};
      const bad = resolveCtx().bad;
      if (bad && opts.scope !== "org" && opts.scope !== "user" && !opts.allowBadCtx) {
        location.replace("error.html?case=404&what=" + encodeURIComponent(params.get("project") || "project"));
        return null;
      }
      renderChrome(); renderProto(); renderContextBits();
      // C-12 arrival toast (redirected here from a missing route)
      const missing = params.get("missing");
      if (missing) toast(`<b class="u-mono">${esc(missing)}</b> doesn't exist in <b class="u-mono">${esc(ctx.env)}</b> \u2014 showing Services.`);
      return this.ctx();
    },
    ctx() { const p = DB.project(ctx.project); return { org: ctx.org, project: ctx.project, env: ctx.env, p, e: p && !p.archived ? DB.env(ctx.project, ctx.env) : null }; },
    q, icon, toast, dialog, announce, sheet, toggleTheme, switchEnv,
    protoStates(list) { protoStates = list || []; },
    palWiden() { const i = $("#pal-q", pal); renderPal(); i.focus(); },
    esc, el,
  };
})();
