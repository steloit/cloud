#!/usr/bin/env node
// Every Google API that infra/ actually needs must be in project-base's
// `local.services`.
//
// WHY THIS EXISTS (T1.7). `infra/README.md`'s bootstrap contract is
// Terraform-first: on a clean project, step 2 is
// `terraform apply -target=module.project_base`, with NO manual `gcloud services
// enable`. `project-base` is the sole declarer of `google_project_service` in the
// whole tree, so it owns API enablement — which means its list must be
// sufficient for its OWN resources to apply. It was not:
// `google_project_iam_member.ci_plan_viewer` lives in that module and is served
// by cloudresourcemanager.googleapis.com, which the list omitted. The module
// could not complete its own documented bootstrap step, and nothing said so
// because the procedure actually used inserted a manual enable that masked it.
//
// A LOOKUP TABLE IS NOT SAFE JUST BECAUSE IT IS A TABLE. contexts/provisioning.md:
// "widening a lookup table without widening its consumer turns a loud error into
// a silent success". So an unmapped resource type is a FAILURE here, never a
// skip — adding a `google_*` resource whose API nobody classified must stop CI
// and force the classification, which is the only way this check keeps working
// as infra grows.
import { readFileSync, readdirSync, statSync } from "node:fs";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../infra/", import.meta.url));
const PROJECT_BASE = fileURLToPath(new URL("../../infra/modules/project-base/main.tf", import.meta.url));

// resource-type prefix -> the API that serves it. Longest prefix wins, so
// `google_artifact_registry_repository_iam_member` resolves to artifactregistry
// rather than falling through.
const API_BY_PREFIX = [
  ["google_project_service", null], // serviceusage; enabling it is the bootstrap itself
  ["google_project_iam", "cloudresourcemanager.googleapis.com"], // projects.{get,set}IamPolicy
  ["google_compute_", "compute.googleapis.com"],
  ["google_container_", "container.googleapis.com"],
  ["google_storage_", "storage.googleapis.com"],
  ["google_artifact_registry_", "artifactregistry.googleapis.com"],
  ["google_kms_", "cloudkms.googleapis.com"],
  ["google_dns_", "dns.googleapis.com"],
  ["google_billing_budget", "billingbudgets.googleapis.com"],
  ["google_cloud_scheduler_", "cloudscheduler.googleapis.com"],
  ["google_monitoring_", "monitoring.googleapis.com"],
  ["google_logging_", "logging.googleapis.com"],
  ["google_iam_workload_identity_pool", "iam.googleapis.com"],
  ["google_service_account", "iam.googleapis.com"],
  ["google_secret_manager_", "secretmanager.googleapis.com"],
];

const walk = (dir) =>
  readdirSync(dir).flatMap((f) => {
    const p = dir + f;
    return statSync(p).isDirectory() ? walk(p + "/") : p.endsWith(".tf") ? [p] : [];
  });

const errors = [];

// The declared list, read from the ONE place that declares it.
const baseSrc = readFileSync(PROJECT_BASE, "utf8");
const block = baseSrc.match(/services\s*=\s*\[([\s\S]*?)\]/);
if (!block) {
  console.error("required-apis: could not find `services = [...]` in project-base/main.tf");
  process.exit(1);
}
const declared = new Set([...block[1].matchAll(/"([^"]+)"/g)].map((m) => m[1]));

// Every resource type infra/ actually declares.
const used = new Map(); // api -> [resource types that need it]
for (const file of walk(root)) {
  const src = readFileSync(file, "utf8");
  for (const m of src.matchAll(/^resource\s+"(google[a-z_]*)"/gm)) {
    const type = m[1];
    const hit = API_BY_PREFIX.filter(([p]) => type.startsWith(p)).sort((a, b) => b[0].length - a[0].length)[0];
    if (!hit) {
      errors.push(
        `${type} (${file.replace(root, "infra/")}) is not classified in API_BY_PREFIX. ` +
          `Add it with the API that serves it — an unclassified resource is exactly how ` +
          `cloudresourcemanager went missing.`
      );
      continue;
    }
    const [, api] = hit;
    if (api === null) continue;
    if (!used.has(api)) used.set(api, new Set());
    used.get(api).add(type);
  }
}

for (const [api, types] of [...used].sort()) {
  if (!declared.has(api)) {
    errors.push(
      `${api} is required by ${[...types].sort().join(", ")} but is NOT in ` +
        `project-base's local.services — a clean-project apply cannot create those resources.`
    );
  }
}

if (errors.length) {
  console.error("required-apis: FAIL");
  for (const e of errors) console.error(`  ${e}`);
  process.exit(1);
}
console.log(
  `required-apis: OK — ${used.size} APIs required by infra/, all declared ` +
    `(${declared.size} in the list; the extras are enabled deliberately, which is allowed).`
);
