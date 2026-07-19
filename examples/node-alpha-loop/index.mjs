// The alpha loop with @steloit/sdk: estimate → create → bind → deploy → events.
//
// Needs a REAL deployment to run end-to-end: a reachable services/api instance
// and a personal token (stp_…) with developer access to an org/project/env.
// Set: STELOIT_API_URL, STELOIT_TOKEN, STELOIT_ORG, STELOIT_PROJECT, STELOIT_ENV.
//
// Run: npm install && npm start
// (via tsx — @steloit/sdk ships as TypeScript source, so a TS-aware loader is
// required; plain `node index.mjs` cannot import it yet.)
//
// P1 boundary, honestly: everything below creates real control-plane rows and
// real spine events, and the estimate gate is enforced by the API. But nothing
// physically provisions yet — the reconciler and build pipeline are P1-gated,
// so the database and web service stay `provisioning`, the deployment stays
// `queued`, the binding stays `pending`, and no billing starts (metering
// starts at `ready`).

import {
  ConflictError,
  fmtMoney,
  QuotaExceededError,
  Steloit,
  SteloitError,
} from "@steloit/sdk";

function required(name) {
  const v = process.env[name];
  if (!v) {
    console.error(`missing ${name} — see the header comment for the required env`);
    process.exit(2);
  }
  return v;
}

const client = new Steloit({
  apiUrl: required("STELOIT_API_URL"),
  token: required("STELOIT_TOKEN"),
  // context is carried, not repeated (mirrors --org/--project/--env)
  org: required("STELOIT_ORG"),         // org_…
  project: required("STELOIT_PROJECT"), // prj_…
  env: required("STELOIT_ENV"),         // env_… (every project is born with production)
});

try {
  const who = await client.auth.session();
  console.log(`connected as ${who.user.email}`);

  // -- 1) estimate → inspect → create (the gate: acceptance is one-shot,
  //       so each create gets its own estimate) -------------------------------
  const dbEstimate = await client.estimates.create({
    services: [
      { product: "postgres", name: "appdb", shape: { size: "dev", storage_gb: 10 } },
    ],
  });
  for (const line of dbEstimate.lines) {
    console.log(`  ${line.name}  ${fmtMoney(line.monthly_cents)}/mo (${line.basis})`);
  }
  console.log(`  total  ${fmtMoney(dbEstimate.monthly_total_cents)}/mo — billing starts at ready, not now`);

  const db = await client.services.create({
    product: "postgres",
    name: "appdb",
    shape: { size: "dev", storage_gb: 10 },
    estimateId: dbEstimate.id, // est_… — nothing provisions without it
  });
  console.log(`created ${db.name} · ${db.id} · ${db.status}`); // provisioning (stays there at P1)

  const webEstimate = await client.estimates.create({
    services: [
      { product: "web", name: "site", shape: { size: "standard-1", instances: 1 } },
    ],
  });
  const web = await client.services.create({
    product: "web",
    name: "site",
    shape: { size: "standard-1", instances: 1 },
    estimateId: webEstimate.id,
  });
  console.log(`created ${web.name} · ${web.id} · ${web.status}`);

  // -- 2) bind (wiring is $0; credentials minted now, injected next deploy) ---
  const binding = await client.bindings.create(web.id, { target: db.id });
  console.log(`bound ${binding.id} · status ${binding.status}`); // pending until a real deploy

  // -- 3) deploy (record is created + numbered; the build pipeline that walks
  //       queued → … → live is P1-gated) --------------------------------------
  const dep = await client.deployments.create({ service: web.id, gitSha: "4f2a91c" });
  console.log(`deployment #${dep.number ?? "?"} ${dep.state} · ${dep.id}`);

  // -- 4) tail the spine over SSE for 10s (id: frames are resume cursors —
  //       the SDK reconnects and resumes automatically) -----------------------
  const stop = new AbortController();
  const timer = setTimeout(() => stop.abort(), 10_000);
  console.log("tailing events for 10s…");
  try {
    for await (const frame of client.events.stream({ signal: stop.signal })) {
      const e = frame.data; // the contract's Event shape
      console.log(`  ${e.at}  ${e.kind}  ${e.action ?? ""}  ${e.actor}`);
    }
  } finally {
    clearTimeout(timer);
  }
  console.log("done — the rows, statuses, and spine events above are real; the infrastructure arrives with the reconciler.");
} catch (err) {
  // typed problem+json — remediation is always present, never swallowed
  if (err instanceof QuotaExceededError) {
    console.error(`quota/plan gate: ${err.message}`);
    if (err.overagePriceCents != null) console.error(`  overage: ${fmtMoney(err.overagePriceCents)}`);
    if (err.requiredPlan) console.error(`  required plan: ${err.requiredPlan}`);
  } else if (err instanceof ConflictError) {
    console.error(`blocked: ${err.message}`);
    for (const r of err.reasons) console.error(`  · ${r.message ?? r.code}`);
  } else if (err instanceof SteloitError) {
    console.error(`${err.name}: ${err.message}`);
  } else {
    console.error(err);
  }
  if (err instanceof SteloitError && err.remediation) console.error(`  → ${err.remediation}`);
  process.exit(1);
}
