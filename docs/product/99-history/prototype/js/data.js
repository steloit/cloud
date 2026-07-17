/* ============================================================
   Steloit prototype — mock data (consistent across all pages)
   ============================================================ */
window.DATA = {
  org: { slug: "acme", name: "Acme" },
  user: { name: "Priya Sharma", email: "priya@acme.dev", initials: "PS", role: "Admin" },
  plan: "Pro",
  regions: [
    { id: "iad", name: "US East (iad)" },
    { id: "sfo", name: "US West (sfo)" },
    { id: "fra", name: "Europe (fra)" },
    { id: "blr", name: "India (blr)" },
  ],
  svcTypes: {
    postgres: { label: "PostgreSQL", icon: "db",     desc: "Your application's database. Users, records, relations.", price: 12, priceNote: "starter-1", fixed: true },
    valkey:   { label: "Valkey",     icon: "bolt",   desc: "In-memory speed. Sessions, caching, rate limits.",        price: 7,  priceNote: "cache-s",   fixed: true },
    storage:  { label: "Object Storage", icon: "box",desc: "Files and uploads. S3-compatible.",                       price: 2,  priceNote: "at 20 GB",  fixed: false },
    queue:    { label: "Queue",      icon: "list",   desc: "Background work. Emails, jobs, events.",                  price: 1,  priceNote: "at 100k msgs", fixed: false },
  },
  projects: [
    {
      slug: "ecommerce", name: "ecommerce", labels: ["team:storefront"], created: "May 2, 2026",
      envs: [
        {
          slug: "production", region: "iad", production: true, appConnected: true,
          services: [
            { id: "db-main", type: "postgres", state: "ready", size: "starter-1", mtd: 9.8, detail: "PG 16 \u00b7 2 branches", spark: [3,4,3,5,6,5,7,6,8,7,9,8] },
            { id: "cache",   type: "valkey",   state: "ready", size: "cache-s",   mtd: 2.2, detail: "512 MB \u00b7 91% hit",  spark: [5,5,6,5,6,7,6,6,7,7,6,7] },
            { id: "media",   type: "storage",  state: "ready", size: "\u2014",    mtd: 1.68, usage: "18.2 GB", detail: "18.2 GB \u00b7 2 buckets", spark: [2,2,3,3,3,4,4,4,5,5,5,6] },
            { id: "jobs",    type: "queue",    state: "ready", size: "\u2014",    mtd: 0.52, detail: "84k msgs \u00b7 0 DLQ",  spark: [4,6,3,7,4,6,5,8,4,6,5,7] },
          ],
          bindings: [
            { id: "b1", from: "app", to: "db-main", scope: "read-write", age: "rotated 6d ago", by: "priya" },
            { id: "b2", from: "app", to: "cache",   scope: "read-write", age: "rotated 6d ago", by: "priya" },
            { id: "b3", from: "app", to: "media",   scope: "read-write", age: "rotated 6d ago", by: "priya" },
            { id: "b4", from: "app", to: "jobs",    scope: "produce",    age: "rotated 2d ago", by: "diego" },
          ],
        },
        {
          slug: "staging", region: "iad", production: false, appConnected: false,
          services: [
            { id: "db-main", type: "postgres", state: "ready",    size: "dev-1", mtd: 2.4, detail: "PG 16 \u00b7 1 branch", spark: [2,3,2,3,2,4,3,3,2,3,3,4] },
            { id: "cache",   type: "valkey",   state: "degraded", size: "cache-s", mtd: 0.9, detail: "memory pressure", spark: [5,6,7,8,8,9,9,9,8,9,9,9] },
          ],
          bindings: [{ id: "b5", from: "app", to: "db-main", scope: "read-write", age: "rotated 21d ago", by: "priya" }],
        },
      ],
    },
    {
      slug: "analytics", name: "analytics", labels: ["team:data"], created: "Jun 11, 2026",
      envs: [
        {
          slug: "production", region: "fra", production: true, appConnected: true,
          services: [
            { id: "db",           type: "postgres", state: "ready",    size: "starter-1", mtd: 6.9, detail: "PG 16", spark: [4,4,5,4,5,5,6,5,6,6,6,7] },
            { id: "events-store", type: "storage",  state: "degraded", size: "\u2014",   mtd: 2.2, usage: "64 GB", detail: "elevated errors", spark: [3,3,4,5,6,7,8,8,9,9,9,9] },
          ],
          bindings: [
            { id: "b6", from: "app", to: "db", scope: "read-write", age: "rotated 12d ago", by: "diego" },
            { id: "b7", from: "app", to: "events-store", scope: "write", age: "rotated 12d ago", by: "diego" },
          ],
        },
      ],
    },
    { slug: "legacy-crm", name: "legacy-crm", labels: [], created: "Feb 9, 2026", archived: true, envs: [] },
  ],
  events: {
    "ecommerce/production": [
      { ic: "check", text: "Backup completed for db-main", t: "2h ago" },
      { ic: "bolt",  text: "cache scaled to cache-s", t: "1d ago" },
      { ic: "bell",  text: "Budget at 80% of $20 monthly limit", t: "2d ago" },
      { ic: "check", text: "jobs provisioned", t: "6d ago" },
      { ic: "key",   text: "Binding app \u2192 jobs created by diego", t: "6d ago" },
      { ic: "check", text: "media provisioned", t: "8d ago" },
      { ic: "users", text: "diego joined the project", t: "12d ago" },
      { ic: "check", text: "Restore drill passed for db-main", t: "14d ago" },
    ],
    "ecommerce/staging": [
      { ic: "warn",  text: "cache degraded \u2014 memory pressure", t: "35m ago" },
      { ic: "check", text: "db-main branch pr-142 retired", t: "3d ago" },
      { ic: "check", text: "Backup completed for db-main", t: "3d ago" },
    ],
    "analytics/production": [
      { ic: "warn",  text: "events-store degraded \u2014 elevated errors", t: "18m ago" },
      { ic: "check", text: "Backup completed for db", t: "5h ago" },
    ],
  },
  notifications: [
    { id: 1, cat: "Alerts",    unread: true,  title: "events-store is degraded", sub: "analytics / production \u00b7 elevated error rate", t: "18m", href: "service.html?project=analytics&env=production&id=events-store" },
    { id: 2, cat: "Alerts",    unread: true,  title: "cache is degraded", sub: "ecommerce / staging \u00b7 memory pressure", t: "35m", href: "service.html?project=ecommerce&env=staging&id=cache" },
    { id: 3, cat: "Billing",   unread: true,  title: "Budget at 80%", sub: "ecommerce \u00b7 $16.10 of $20.00 monthly budget", t: "2d", href: "settings-org.html#billing" },
    { id: 4, cat: "Backups",   unread: false, title: "Backup completed", sub: "ecommerce / production \u00b7 db-main", t: "2h", href: "service.html?project=ecommerce&env=production&id=db-main" },
    { id: 5, cat: "Lifecycle", unread: false, title: "media provisioned", sub: "ecommerce / production", t: "8d", href: "services.html?project=ecommerce&env=production" },
    { id: 6, cat: "Membership",unread: false, title: "Diego Fernandes joined Acme", sub: "invited by you", t: "12d", href: "settings-org.html#members" },
    { id: 7, cat: "System",    unread: false, title: "Maintenance window scheduled", sub: "iad \u00b7 Jul 12, 02:00\u201302:20 UTC \u00b7 no downtime expected", t: "3d", href: "#" },
  ],
  ntfCats: ["All", "Alerts", "Backups", "Billing", "Lifecycle", "Membership", "System"],
  docs: [
    { title: "How Steloit is organized", href: "docs.html" },
    { title: "Create a project", href: "docs.html" },
    { title: "Connect your app", href: "docs.html" },
    { title: "How estimates work", href: "docs.html" },
    { title: "Delete safely", href: "docs.html" },
    { title: "Working with context (CLI)", href: "docs.html" },
  ],
};

/* ---- helpers ---- */
window.DB = {
  projects(includeArchived) {
    return DATA.projects.filter(p => includeArchived ? true : !p.archived);
  },
  project(slug) { return DATA.projects.find(p => p.slug === slug); },
  env(pSlug, eSlug) {
    const p = this.project(pSlug); if (!p) return null;
    return p.envs.find(e => e.slug === eSlug) || null;
  },
  service(pSlug, eSlug, id) {
    const e = this.env(pSlug, eSlug); if (!e) return null;
    return e.services.find(s => s.id === id) || null;
  },
  projectMtd(p) {
    return p.envs.reduce((t, e) => t + e.services.reduce((s, x) => s + x.mtd, 0), 0);
  },
  worstState(services) {
    const rank = { deleting: 5, suspended: 4, degraded: 3, provisioning: 2, ready: 1 };
    return services.reduce((w, s) => (rank[s.state] > rank[w] ? s.state : w), "ready");
  },
  healthSentence(env) { // C-28
    const n = env.services.length;
    if (!n) return "No services yet.";
    const bad = env.services.filter(s => s.state !== "ready");
    if (!bad.length) return `All ${n} service${n > 1 ? "s" : ""} ready.`;
    const k = n - bad.length;
    return `${k} of ${n} ready \u2014 ${bad.map(b => `<strong>${b.id}</strong> ${b.state}`).join(", ")}.`;
  },
  events(pSlug, eSlug) { return DATA.events[`${pSlug}/${eSlug}`] || []; },
};
