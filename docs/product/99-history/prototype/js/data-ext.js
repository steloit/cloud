/* Steloit prototype — data-ext.js
   Platform-phase mock data (P0 depth → P4), extending js/data.js.
   Keyed by "project/env/serviceId" where service-scoped. */

/* v2 service family registered (frames render with version tags) */
DATA.svcTypes.compute = { label: "Compute", icon: "term", desc: "Run your app on the platform. Deploys, previews, scale-to-zero.", price: 5, priceNote: "shared-1", fixed: false, v2: true };

/* ---------------- PDS-001 · PostgreSQL depth ---------------- */
DATA.pg = {
  "ecommerce/production/db-main": {
    version: "PostgreSQL 16.3", size: "starter-1", storage: "8.4 GB / 20 GB", ha: "single node · daily basebackup",
    branches: [
      { name: "main", parent: "—", primary: true, created: "May 2, 2026", size: "8.4 GB", writes: true, costDay: 0.40 },
      { name: "pitr-hotfix-0629", parent: "main @ Jun 29 14:02", primary: false, created: "Jun 29, 2026", size: "8.1 GB", writes: false, costDay: 0.11 },
    ],
    backups: {
      pitrWindow: "7 days (plan: Pro)", lastBase: "Today 03:00 UTC · 41s",
      snapshots: [
        { id: "snap-0705", taken: "Jul 5, 03:00", kind: "automatic", size: "8.4 GB", keep: "7d" },
        { id: "snap-0704", taken: "Jul 4, 03:00", kind: "automatic", size: "8.3 GB", keep: "7d" },
        { id: "snap-pre-migrate", taken: "Jun 28, 11:41", kind: "manual · priya", size: "8.1 GB", keep: "30d" },
      ],
    },
    params: { max_connections: "100 (default)", shared_buffers: "256 MB (default)", work_mem: "16 MB (default)", extensions: "pgvector 0.7 · postgis (off)" },
  },
  "ecommerce/staging/db-main": {
    version: "PostgreSQL 16.3", size: "dev-1", storage: "1.9 GB / 10 GB", ha: "single node",
    branches: [{ name: "main", parent: "—", primary: true, created: "May 4, 2026", size: "1.9 GB", writes: true, costDay: 0.08 }],
    backups: { pitrWindow: "24 hours (dev tier)", lastBase: "Today 03:10 UTC · 12s", snapshots: [{ id: "snap-0705s", taken: "Jul 5, 03:10", kind: "automatic", size: "1.9 GB", keep: "24h" }] },
    params: { max_connections: "50 (default)", shared_buffers: "128 MB (default)", work_mem: "8 MB (default)", extensions: "pgvector 0.7" },
  },
  "analytics/production/db": {
    version: "PostgreSQL 16.3", size: "starter-2", storage: "31.2 GB / 50 GB", ha: "single node · daily basebackup",
    branches: [{ name: "main", parent: "—", primary: true, created: "Mar 18, 2026", size: "31.2 GB", writes: true, costDay: 0.92 }],
    backups: { pitrWindow: "7 days (plan: Pro)", lastBase: "Today 03:00 UTC · 2m 04s", snapshots: [{ id: "snap-0705a", taken: "Jul 5, 03:00", kind: "automatic", size: "31.2 GB", keep: "7d" }] },
    params: { max_connections: "200", shared_buffers: "1 GB", work_mem: "32 MB", extensions: "pgvector 0.7 · timescaledb (off)" },
  },
};

/* ---------------- P2 · Valkey / Storage / Queue depth ---------------- */
DATA.valkey = {
  "ecommerce/production/cache": {
    version: "Valkey 8.0", size: "cache-s · 512 MB", policy: "allkeys-lru", hit: "91.4%", mem: "388 MB / 512 MB",
    keyspaces: [
      { pattern: "sess:*", keys: "48,211", ttl: "24h", mem: "212 MB" },
      { pattern: "cart:*", keys: "9,437", ttl: "7d", mem: "96 MB" },
      { pattern: "rate:*", keys: "31,002", ttl: "60s", mem: "41 MB" },
      { pattern: "cfg:*", keys: "112", ttl: "—", mem: "2 MB" },
    ],
  },
  "ecommerce/staging/cache": {
    version: "Valkey 8.0", size: "cache-s · 512 MB", policy: "allkeys-lru", hit: "62.1%", mem: "497 MB / 512 MB",
    degradedNote: "Memory pressure: evictions exceeded 5%/min for 25 minutes.",
    keyspaces: [{ pattern: "sess:*", keys: "61,844", ttl: "24h", mem: "441 MB" }, { pattern: "fixture:*", keys: "18,300", ttl: "—", mem: "48 MB" }],
  },
};
DATA.storage = {
  "ecommerce/production/media": {
    endpoint: "https://media.acme-ecommerce.steloit.dev", region: "iad", versioning: "on", publicRead: false,
    buckets: [
      { name: "product-images", objects: "18,204", size: "14.1 GB", public: true },
      { name: "user-uploads", objects: "6,110", size: "3.9 GB", public: false },
      { name: "exports", objects: "84", size: "0.2 GB", public: false },
    ],
    topObjects: [
      { key: "product-images/hero/summer-26.webp", size: "4.2 MB", modified: "2d ago" },
      { key: "user-uploads/avatars/u_8814.png", size: "812 KB", modified: "6h ago" },
      { key: "exports/orders-2026-06.csv", size: "18.4 MB", modified: "Jul 1" },
    ],
  },
};
DATA.queue = {
  "ecommerce/production/jobs": {
    engine: "Steloit Queue v1", delivery: "at-least-once", retention: "4 days",
    queues: [
      { name: "emails", depth: 12, rate: "34/min", dlq: 0, consumers: 3 },
      { name: "image-resize", depth: 148, rate: "210/min", dlq: 0, consumers: 6 },
      { name: "webhooks", depth: 3, rate: "9/min", dlq: 4, consumers: 2 },
    ],
    dlqPeek: [
      { id: "msg_91ab", queue: "webhooks", reason: "endpoint 500 ×5", age: "3h" },
      { id: "msg_90fe", queue: "webhooks", reason: "endpoint timeout ×5", age: "5h" },
      { id: "msg_8f21", queue: "webhooks", reason: "endpoint 500 ×5", age: "9h" },
      { id: "msg_8d10", queue: "webhooks", reason: "invalid signature", age: "1d" },
    ],
  },
};

/* ---------------- P1 · Observability ---------------- */
function _series(base, jitter, n) { const a = []; let v = base; for (let i = 0; i < (n || 24); i++) { v = Math.max(0, v + (Math.sin(i / 3) + (((i * 7919) % 13) / 13 - 0.5)) * jitter); a.push(+v.toFixed(1)); } return a; }
DATA.metrics = {
  "ecommerce/production": {
    "db-main": [{ name: "Connections", unit: "", s: _series(34, 6) }, { name: "p95 query", unit: "ms", s: _series(12, 4) }],
    "cache": [{ name: "Hit rate", unit: "%", s: _series(91, 2) }, { name: "Memory", unit: "MB", s: _series(380, 14) }],
    "media": [{ name: "Requests", unit: "/min", s: _series(420, 60) }, { name: "Egress", unit: "MB/min", s: _series(38, 9) }],
    "jobs": [{ name: "Throughput", unit: "msg/min", s: _series(250, 40) }, { name: "Oldest msg age", unit: "s", s: _series(8, 5) }],
  },
  "ecommerce/staging": {
    "db-main": [{ name: "Connections", unit: "", s: _series(6, 2) }, { name: "p95 query", unit: "ms", s: _series(9, 3) }],
    "cache": [{ name: "Hit rate", unit: "%", s: _series(64, 5) }, { name: "Evictions", unit: "/min", s: _series(120, 45) }],
  },
  "analytics/production": {
    "db": [{ name: "Connections", unit: "", s: _series(88, 12) }, { name: "p95 query", unit: "ms", s: _series(45, 18) }],
    "events-store": [{ name: "Requests", unit: "/min", s: _series(900, 200) }, { name: "5xx", unit: "/min", s: _series(4, 3) }],
  },
};
DATA.logs = {
  "ecommerce/production": [
    { t: "14:31:02", lvl: "info", svc: "db-main", msg: "checkpoint complete: wrote 412 buffers (2.5%)" },
    { t: "14:30:58", lvl: "info", svc: "jobs", msg: "queue=image-resize delivered=210 acked=210 window=60s" },
    { t: "14:30:41", lvl: "warn", svc: "jobs", msg: "queue=webhooks delivery failed msg=msg_91ab attempt=5 → dead-letter" },
    { t: "14:30:12", lvl: "info", svc: "media", msg: "PUT user-uploads/avatars/u_8814.png 812KB 201" },
    { t: "14:29:55", lvl: "info", svc: "cache", msg: "evicted=0 expired=1841 hit_rate=91.4%" },
    { t: "14:29:31", lvl: "error", svc: "db-main", msg: "ERROR: duplicate key value violates unique constraint \"orders_idempotency_key\" (retried ok)" },
    { t: "14:28:44", lvl: "info", svc: "db-main", msg: "autovacuum: table \"public.sessions\" 12,004 dead tuples removed" },
    { t: "14:27:19", lvl: "info", svc: "media", msg: "GET product-images/hero/summer-26.webp 4.2MB 200 (cache HIT)" },
  ],
  "ecommerce/staging": [
    { t: "14:30:12", lvl: "warn", svc: "cache", msg: "evictions 132/min exceed 5%/min threshold (25m) — state: degraded" },
    { t: "14:28:03", lvl: "warn", svc: "cache", msg: "memory 497MB/512MB (97%)" },
    { t: "14:20:40", lvl: "info", svc: "db-main", msg: "branch main: idle, 6 connections" },
  ],
  "analytics/production": [
    { t: "14:31:44", lvl: "error", svc: "events-store", msg: "PUT /v1/events 503 upstream saturation (rate limited)" },
    { t: "14:31:20", lvl: "warn", svc: "events-store", msg: "request queue depth 812 (threshold 500)" },
    { t: "14:29:02", lvl: "info", svc: "db", msg: "checkpoint complete: wrote 3,204 buffers" },
  ],
};
DATA.alerts = {
  rules: [
    { id: "al-1", name: "Cache hit rate < 70%", scope: "ecommerce/staging · cache", channel: "in-app + email", state: "firing", since: "25m" },
    { id: "al-2", name: "5xx > 10/min for 5m", scope: "analytics/production · events-store", channel: "in-app + Slack", state: "firing", since: "12m" },
    { id: "al-3", name: "DLQ depth > 0", scope: "ecommerce/production · jobs", channel: "in-app", state: "firing", since: "3h" },
    { id: "al-4", name: "DB storage > 80%", scope: "any postgres · all envs", channel: "in-app + email", state: "ok", since: "—" },
    { id: "al-5", name: "p95 query > 250ms for 10m", scope: "ecommerce/production · db-main", channel: "in-app", state: "ok", since: "—" },
  ],
};

/* ---------------- P1 · IAM (PDS-007 surfaces) ---------------- */
DATA.members = [
  { name: "Priya Sharma", email: "priya@acme.dev", role: "Owner", tfa: true, active: "now", avatar: "PS" },
  { name: "Diego Márquez", email: "diego@acme.dev", role: "Admin", tfa: true, active: "2h ago", avatar: "DM" },
  { name: "Mei Chen", email: "mei@acme.dev", role: "Developer", tfa: true, active: "yesterday", avatar: "MC" },
  { name: "Sam Okafor", email: "sam@acme.dev", role: "Developer", tfa: false, active: "3d ago", avatar: "SO" },
  { name: "Finance bot", email: "billing-export@acme.dev", role: "Billing viewer", tfa: true, active: "Jul 1", avatar: "FB" },
];
DATA.roles = [
  { role: "Owner", desc: "Everything, incl. destroy org, billing, member management.", count: 1 },
  { role: "Admin", desc: "Manage projects, members (not owners), policies. No org destroy.", count: 1 },
  { role: "Developer", desc: "Create/operate services in permitted projects. No billing, no member management.", count: 2 },
  { role: "Billing viewer", desc: "Read invoices, usage, budgets. Nothing else.", count: 1 },
];
DATA.apiKeys = [
  { id: "stl_live_…8k2f", name: "ci-deploy", scope: "ecommerce · deploy", created: "May 12", lastUsed: "14m ago", by: "priya" },
  { id: "stl_live_…m9q1", name: "metrics-export", scope: "org · read-only", created: "Jun 2", lastUsed: "1h ago", by: "diego" },
];

/* ---------------- P3 · Billing (PDS-009) ---------------- */
DATA.billing = {
  plan: "Pro", seatNote: "Unlimited members · usage-based services", renews: "Aug 1, 2026",
  periodTotal: 23.30, forecast: 25.10, credit: 0,
  daily: _series(0.66, 0.18, 30).map(v => +Math.max(0.2, v).toFixed(2)),
  byProject: [
    { project: "ecommerce", mtd: 17.30, split: [["db-main", 9.80], ["cache", 2.20], ["media", 1.68], ["jobs", 0.52], ["staging (all)", 3.10]] },
    { project: "analytics", mtd: 9.10, split: [["db", 7.40], ["events-store", 1.70]] },
    { project: "legacy-crm (archived)", mtd: 0.00, split: [] },
  ],
  invoices: [
    { id: "INV-2026-06", period: "June 2026", total: "$21.84", status: "paid" },
    { id: "INV-2026-05", period: "May 2026", total: "$14.02", status: "paid" },
    { id: "INV-2026-04", period: "April 2026", total: "$9.51", status: "paid" },
  ],
  budget: { amount: 40, action: "notify Owner + Admins", note: "Notification only — Steloit never auto-suspends paid work without you (BIZ-002)." },
};

/* ---------------- P2 · Secrets (PDS-008 slot) ---------------- */
DATA.secrets = {
  "ecommerce/production": [
    { name: "STRIPE_SECRET_KEY", updated: "Jun 21", by: "priya", note: "payments" },
    { name: "RESEND_API_KEY", updated: "Jun 21", by: "priya", note: "email" },
    { name: "SESSION_SIGNING_SECRET", updated: "May 2", by: "system", note: "generated at project create" },
  ],
  "ecommerce/staging": [
    { name: "STRIPE_SECRET_KEY", updated: "Jun 21", by: "priya", note: "test-mode key" },
    { name: "SESSION_SIGNING_SECRET", updated: "May 4", by: "system", note: "generated" },
  ],
  "analytics/production": [{ name: "SEGMENT_WRITE_KEY", updated: "Apr 2", by: "diego", note: "ingest" }],
};

/* ---------------- P3 · Templates (PDS-TPL) ---------------- */
DATA.templates = [
  { id: "saas", name: "SaaS starter", services: ["postgres", "valkey", "storage", "queue"], est: 22, desc: "Accounts, sessions, uploads, background email. The ecommerce project was born from this.", uses: "4.2k" },
  { id: "ai-app", name: "AI application", services: ["postgres", "storage", "queue"], est: 15, desc: "pgvector-ready PostgreSQL, document storage, ingestion queue.", uses: "2.9k" },
  { id: "storefront", name: "Storefront", services: ["postgres", "valkey", "storage"], est: 21, desc: "Catalog DB, cart/session cache, product media.", uses: "1.1k" },
  { id: "pipeline", name: "Event pipeline", services: ["postgres", "queue"], est: 13, desc: "Ingest queue in front of a time-partitioned Postgres.", uses: "640" },
];

/* ---------------- P4 / v2 · Deployments & Preview environments ---------------- */
DATA.deploys = {
  service: "web (compute · shared-1)", repo: "acme/ecommerce", branch: "main", buildCmd: "npm run build",
  rows: [
    { id: "dep_a91", commit: "f3c21a — checkout: fix rounding on partial refunds", env: "production", status: "live", when: "2h ago", by: "mei" },
    { id: "dep_a90", commit: "9b02de — bump next to 15.4", env: "production", status: "rolled-back", when: "yesterday", by: "diego" },
    { id: "dep_a89", commit: "77aa10 — new PDP gallery", env: "staging", status: "live", when: "yesterday", by: "mei" },
  ],
  previews: [
    { pr: "#482 — Gift cards v1", env: "preview-482", db: "branch of production@today (copy-on-write, 0.4 GB delta)", url: "pr-482.acme-ecommerce.steloit.dev", status: "ready", cost: "~$0.31/day" },
    { pr: "#479 — Vector search spike", env: "preview-479", db: "branch of production@Jul 2", url: "pr-479.acme-ecommerce.steloit.dev", status: "ready", cost: "~$0.28/day" },
  ],
};

/* ---------------- P3 · Assistant (PDS-012) canned Q&A ---------------- */
DATA.assistantQA = [
  { match: /branch|copy.*(prod|data)|preview.*db/i, a: "PostgreSQL branches are copy-on-write copies of a database at a point in time — cheap to create, isolated to write to. Your <b>db-main</b> has 2 branches; a new one from <span class=\"u-mono\">main@now</span> would cost about <span class=\"u-mono\">$0.11/day</span> at the current delta size. Create one from the service page → Branches.", cites: ["Concept: Branching", "PRD-001 §4"], links: [["Open db-main → Branches", "service.html?id=db-main&tab=branches"]] },
  { match: /cost|bill|spend|price/i, a: "This organization has spent <span class=\"u-mono\">$23.30</span> this period across 2 active projects; the forecast for the full month is <span class=\"u-mono\">$25.10</span>, under your <span class=\"u-mono\">$40</span> budget alert. The largest line is <b>db-main</b> in ecommerce/production.", cites: ["How estimates work", "FND-010"], links: [["Open Billing", "billing.html"]] },
  { match: /degraded|wrong|cache|slow|alert|firing/i, a: "3 alerts are firing. The oldest: <b>staging cache</b> has been degraded for 25 minutes — evictions exceed 5%/min because memory is at 97%. Typical fixes: raise the size one step (<span class=\"u-mono\">cache-m</span>, +$7/mo, shown before you confirm) or add TTLs to the <span class=\"u-mono\">fixture:*</span> keyspace (48 MB, no TTL).", cites: ["Runbook: Valkey memory pressure"], links: [["Open cache in staging", "service.html?id=cache&project=ecommerce&env=staging"], ["View alerts", "observe.html?tab=alerts"]] },
  { match: /dlq|dead.?letter|webhook/i, a: "The <b>webhooks</b> queue has 4 dead-lettered messages, all endpoint failures to the same host over the last day. You can redrive them after the endpoint recovers — redelivery is at-least-once, so make the handler idempotent.", cites: ["Concept: Queues & DLQ"], links: [["Open jobs → Queues", "service.html?id=jobs&tab=queues"]] },
  { match: /secret|env pull|credential|rotate/i, a: "Secrets live per environment and are injected into bindings at connect time. <span class=\"u-mono\">steloit env pull</span> writes them to <span class=\"u-mono\">.env.steloit</span> locally; rotating a secret updates pulled/synced apps on next pull — manually copied values do not update (that's why the Console warns, C-41).", cites: ["Task: Connect your app", "FND-006"], links: [["Open Secrets", "secrets.html"]] },
];

/* ---------------- DB helper extensions ---------------- */
Object.assign(DB, {
  key(pSlug, eSlug, id) { return `${pSlug}/${eSlug}${id ? "/" + id : ""}`; },
  pg(p, e, id) { return DATA.pg[this.key(p, e, id)] || null; },
  valkey(p, e, id) { return DATA.valkey[this.key(p, e, id)] || null; },
  storageOf(p, e, id) { return DATA.storage[this.key(p, e, id)] || null; },
  queueOf(p, e, id) { return DATA.queue[this.key(p, e, id)] || null; },
  metrics(p, e) { return DATA.metrics[this.key(p, e)] || {}; },
  logs(p, e) { return DATA.logs[this.key(p, e)] || []; },
  secretsOf(p, e) { return DATA.secrets[this.key(p, e)] || []; },
  firingAlerts() { return DATA.alerts.rules.filter(r => r.state === "firing"); },
  assistantAnswer(q) {
    const hit = DATA.assistantQA.find(x => x.match.test(q));
    return hit || { a: "I can answer questions about this organization's projects, costs, alerts, branches, queues, and secrets — and I cite the docs I draw from. I never take actions: every suggestion links to the real, permissioned surface.", cites: ["PDS-012"], links: [["Browse the docs", "docs.html"]] };
  },
});
