#!/usr/bin/env node
// Wait for a PR's checks and say, fail-closed, whether it may merge.
//
//   node scripts/ci/await-checks.mjs <pr> [--allow-skipped a,b] [--timeout-min N]
//
// Exit 0 ONLY when every check reached an explicit terminal SUCCESS (or a
// SKIPPED that was allow-listed by name) for the PR's CURRENT head commit.
// Exit 1 on any failure or non-success terminal state. Exit 2 on timeout —
// which is NOT success, and is the case the original defect turned into a merge.
//
// The decision lives in check-state.mjs and is unit-tested there; this file is
// the I/O around it, deliberately thin.
import { execFileSync } from "node:child_process";
import { verdict } from "./check-state.mjs";

const args = process.argv.slice(2);
const pr = args.find((a) => /^\d+$/.test(a));
if (!pr) {
  console.error("usage: await-checks.mjs <pr-number> [--allow-skipped a,b] [--timeout-min N]");
  process.exit(2);
}
const flag = (name, fallback) => {
  const i = args.indexOf(`--${name}`);
  return i === -1 ? fallback : args[i + 1];
};
const allowSkipped = String(flag("allow-skipped", "")).split(",").map((s) => s.trim()).filter(Boolean);
const timeoutMs = Number(flag("timeout-min", "30")) * 60_000;
const intervalMs = Number(flag("interval-sec", "30")) * 1000;

const gh = (fields) =>
  JSON.parse(execFileSync("gh", ["pr", "view", pr, "--json", fields], { encoding: "utf8" }));
const api = (path) => JSON.parse(execFileSync("gh", ["api", path], { encoding: "utf8" }));
// The repo this PR belongs to, resolved once from the checkout gh is run in.
const nwo = JSON.parse(
  execFileSync("gh", ["repo", "view", "--json", "nameWithOwner"], { encoding: "utf8" })
).nameWithOwner;

const started = Date.now();
let last = "";
for (;;) {
  const { headRefOid } = gh("headRefOid");

  // THE SEAM, made structural rather than compared after the fact. An earlier
  // draft read `statusCheckRollup` and tried to compare the run's commit to the
  // PR head — but the rollup does not expose the run's SHA, so that comparison
  // was `undefined !== undefined`: a seam check that could never fire, which is
  // the same class of defect as the polling bug this file exists to fix.
  //
  // Asking for the check-runs OF A COMMIT cannot return an answer about a
  // different one. The property is enforced by the query, not by a branch.
  const { check_runs: checks } = api(`repos/${nwo}/commits/${headRefOid}/check-runs`);
  const v = verdict(checks, { allowSkipped });

  if (v.reason !== last) {
    console.log(`[${new Date().toISOString().slice(11, 19)}] ${v.ready ? "READY" : "not ready"} — ${v.reason}`);
    last = v.reason;
  }
  if (v.ready) {
    console.log(`await-checks: PR #${pr} @ ${headRefOid.slice(0, 7)} is mergeable.`);
    process.exit(0);
  }
  if (!v.wait) {
    console.error(`await-checks: PR #${pr} will not become green — ${v.reason}`);
    process.exit(1);
  }
  if (Date.now() - started > timeoutMs) {
    // A timeout is not a pass. The whole point.
    console.error(`await-checks: timed out after ${timeoutMs / 60000} min — ${v.reason}`);
    console.error("A timeout is NOT success. Do not merge on it.");
    process.exit(2);
  }
  execFileSync("sleep", [String(intervalMs / 1000)]);
}
