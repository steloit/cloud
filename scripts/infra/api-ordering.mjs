#!/usr/bin/env node
// Every resource that needs an API must be ordered AFTER the thing that enables it.
//
// WHY (T1.8). `module.project_base.google_project_service.enabled` had ZERO edges
// in either direction, in both envs — nothing waited for API enablement. From-zero
// applies worked only because the documented bootstrap ran a manual
// `gcloud services enable` first, which `infra/README.md`'s contract does not
// include and which is exactly what masked T1.7's missing API.
//
// REACHABILITY, NOT A DIRECT EDGE — and this is the whole design of the check.
// `depends_on = [module.project_base]` orders a consuming module against
// project-base's LEAF resources, which in turn depend on `enabled`. The ordering
// is real and transitive; there is no direct edge and there should not be. An
// earlier draft of T1.8's AC demanded a direct edge: it would have failed on
// correct configuration, and the "fix" would have been a redundant `depends_on`
// added to satisfy a checker. Assert the property (is enablement guaranteed to
// happen first?), never one particular spelling of it.
//
// MEASURED, so it is worth writing down: threading an `apis_ready` output into the
// consuming modules as a variable creates NO edge at all, because a variable no
// resource reads does not order that module's resources. It looks like
// enforcement and enforces nothing. `depends_on` is used because it works.
import { execFileSync } from "node:child_process";
import { cpSync, mkdtempSync, readFileSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const INFRA = fileURLToPath(new URL("../../infra/", import.meta.url));

// One probe per module that consumes an API, named by the resource that would
// actually fail. Adding a module without adding a probe is a gap this cannot see,
// so ENVS lists what must be covered and the check refuses a missing node.
const PROBES = {
  dev: [
    "module.gke_cell.google_container_cluster.cell",
    "module.gke_cell.google_project_iam_member.node_roles",
    "module.network.google_compute_network.vpc",
    "module.network.google_dns_managed_zone.content",
    "module.cnpg.helm_release.cnpg_operator",
    "module.project_base.google_project_iam_member.ci_plan_viewer",
    "module.project_base.google_storage_bucket.artifacts",
    "module.project_base.google_kms_key_ring.core",
  ],
  cell0: [
    "module.gke_cell.google_container_cluster.cell",
    "module.gke_cell.google_project_iam_member.node_roles",
    "module.network.google_compute_network.vpc",
    "module.cnpg.helm_release.cnpg_operator",
    "module.project_base.google_project_iam_member.ci_plan_viewer",
  ],
};
const TARGET = "module.project_base.google_project_service.enabled";

// `terraform graph` refuses to run under `init -backend=false` ("Backend
// initialization required"), which is how CI initializes. So: copy, neutralize the
// backend block IN THE COPY, graph there. The working tree is never touched.
function graphFor(env) {
  const tmp = mkdtempSync(join(tmpdir(), "api-ordering-"));
  try {
    cpSync(INFRA, join(tmp, "infra"), { recursive: true });
    const root = join(tmp, "infra", "envs", env);
    const backend = join(root, "backend.tf");
    writeFileSync(backend, readFileSync(backend, "utf8").replace(/backend\s+"gcs"\s*\{[^}]*\}/s, ""));
    execFileSync("terraform", ["init", "-backend=false", "-input=false"], { cwd: root, stdio: "ignore" });
    return execFileSync("terraform", ["graph"], { cwd: root, encoding: "utf8" });
  } finally {
    rmSync(tmp, { recursive: true, force: true });
  }
}

const errors = [];
for (const [env, probes] of Object.entries(PROBES)) {
  const dot = graphFor(env);
  const edges = new Map();
  for (const m of dot.matchAll(/"([^"]+)"\s*->\s*"([^"]+)"/g)) {
    if (!edges.has(m[1])) edges.set(m[1], new Set());
    edges.get(m[1]).add(m[2]);
  }
  const nodes = new Set([...edges.keys(), ...[...edges.values()].flatMap((s) => [...s])]);
  if (!nodes.has(TARGET)) {
    errors.push(`${env}: ${TARGET} is not in the graph at all — the check would pass vacuously`);
    continue;
  }
  for (const probe of probes) {
    if (!nodes.has(probe)) {
      errors.push(`${env}: ${probe} is not in the graph — renamed or removed. Fix the probe list; a probe that names nothing asserts nothing.`);
      continue;
    }
    const seen = new Set([probe]);
    const stack = [probe];
    let ok = false;
    while (stack.length && !ok) {
      for (const d of edges.get(stack.pop()) ?? []) {
        if (d === TARGET) { ok = true; break; }
        if (!seen.has(d)) { seen.add(d); stack.push(d); }
      }
    }
    if (!ok) {
      errors.push(
        `${env}: ${probe} does NOT depend on ${TARGET}, transitively or otherwise — ` +
          `on a clean project it can be created before its API is enabled.`
      );
    }
  }
}

if (errors.length) {
  console.error("api-ordering: FAIL");
  for (const e of errors) console.error(`  ${e}`);
  process.exit(1);
}
console.log(
  `api-ordering: OK — every probe in both envs is ordered after ${TARGET} ` +
    `(${Object.values(PROBES).flat().length} probes).`
);
