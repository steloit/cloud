/* Steloit prototype — charts.js
   Token-themed SVG charts (DES-004 §2.16): line/area, bars, log rows.
   Axis text mono-small tertiary; horizontal gridlines only; max 4 series. */
window.Charts = (function () {
  const NS = "http://www.w3.org/2000/svg";
  function esc(s) { return String(s).replace(/[&<>"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c])); }

  function frame(w, h, title, unit) {
    return { w, h, padL: 34, padR: 8, padT: 8, padB: 18, title, unit };
  }
  function scale(vals, f) {
    const max = Math.max(...vals, 1), min = 0;
    return { x: i => f.padL + (i / (vals.length - 1)) * (f.w - f.padL - f.padR), y: v => f.padT + (1 - (v - min) / (max - min)) * (f.h - f.padT - f.padB), max };
  }
  function grid(f, max) {
    let g = "";
    [0, 0.5, 1].forEach(t => {
      const y = f.padT + (1 - t) * (f.h - f.padT - f.padB);
      g += `<line x1="${f.padL}" x2="${f.w - f.padR}" y1="${y}" y2="${y}" class="ch-grid"/>`;
      g += `<text x="${f.padL - 6}" y="${y + 3}" class="ch-axis" text-anchor="end">${fmt(max * t)}</text>`;
    });
    return g;
  }
  function fmt(v) { return v >= 1000 ? (v / 1000).toFixed(1) + "k" : v >= 100 ? Math.round(v) : v >= 10 ? v.toFixed(0) : v.toFixed(1); }

  /* line({el, series:[{name, s:[…]}], unit, h}) — 1 or 2 series */
  function line(host, opts) {
    const f = frame(host.clientWidth || 320, opts.h || 120, opts.title, opts.unit);
    const all = opts.series.flatMap(x => x.s);
    const sc = scale(all, f);
    const cls = ["ch-l1", "ch-l2", "ch-l3", "ch-l4"];
    let paths = "";
    opts.series.forEach((se, si) => {
      const pts = se.s.map((v, i) => `${sc.x(i).toFixed(1)},${sc.y(v).toFixed(1)}`);
      paths += `<path class="${cls[si]}" d="M${pts.join(" L")}" fill="none"/>`;
      const last = se.s[se.s.length - 1];
      paths += `<circle class="${cls[si]}-dot" cx="${sc.x(se.s.length - 1)}" cy="${sc.y(last)}" r="2.5"/>`;
    });
    const legend = opts.series.map((se, si) => `<span class="ch-key"><i class="ch-swatch ${cls[si]}-sw"></i>${esc(se.name)}${opts.unit ? ` <span class="u-faint">${esc(opts.unit)}</span>` : ""} <b class="u-mono">${fmt(se.s[se.s.length - 1])}</b></span>`).join("");
    host.innerHTML = `<div class="ch-head"><span class="ch-title">${esc(opts.title || "")}</span>${legend}</div>
      <svg viewBox="0 0 ${f.w} ${f.h}" preserveAspectRatio="none" role="img" aria-label="${esc(opts.title || "chart")}: latest ${opts.series.map(se => se.name + " " + fmt(se.s[se.s.length - 1]) + (opts.unit || "")).join(", ")}">${grid(f, sc.max)}${paths}</svg>`;
  }

  /* bars({el, vals:[…], h, label}) */
  function bars(host, opts) {
    const f = frame(host.clientWidth || 560, opts.h || 140, opts.title);
    const sc = scale(opts.vals, f);
    const bw = Math.max(3, (f.w - f.padL - f.padR) / opts.vals.length - 2);
    let b = "";
    opts.vals.forEach((v, i) => {
      const x = f.padL + (i / opts.vals.length) * (f.w - f.padL - f.padR);
      b += `<rect class="ch-bar" x="${x.toFixed(1)}" y="${sc.y(v).toFixed(1)}" width="${bw.toFixed(1)}" height="${(f.h - f.padB - sc.y(v)).toFixed(1)}" rx="1.5"><title>${esc(opts.tip ? opts.tip(v, i) : v)}</title></rect>`;
    });
    host.innerHTML = `<div class="ch-head"><span class="ch-title">${esc(opts.title || "")}</span><span class="ch-key u-faint">${esc(opts.note || "")}</span></div>
      <svg viewBox="0 0 ${f.w} ${f.h}" preserveAspectRatio="none" role="img" aria-label="${esc(opts.title || "bar chart")}">${grid(f, sc.max)}${b}</svg>`;
  }

  /* logRows(list, rows) */
  function logRows(host, rows, filter) {
    const f = filter || {};
    const out = rows.filter(r => (!f.lvl || f.lvl === "all" || r.lvl === f.lvl) && (!f.svc || f.svc === "all" || r.svc === f.svc) && (!f.q || r.msg.toLowerCase().includes(f.q.toLowerCase())));
    host.innerHTML = out.length ? out.map(r => `
      <div class="log-row log--${r.lvl}">
        <span class="log-t u-mono">${r.t}</span>
        <span class="log-lvl">${r.lvl}</span>
        <span class="log-svc u-mono">${esc(r.svc)}</span>
        <span class="log-msg u-mono">${esc(r.msg)}</span>
      </div>`).join("") : `<div class="empty-inline">No log lines match. <button class="btn btn--s" data-clearlogs>Clear filters</button></div>`;
  }

  return { line, bars, logRows };
})();
