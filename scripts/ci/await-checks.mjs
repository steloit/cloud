#!/usr/bin/env node
// Wait for a PR's checks and say, fail-closed, whether it may merge.
//
//   node scripts/ci/await-checks.mjs <pr> [--allow-skipped a,b] [--require a,b]
//                                         [--timeout-min N] [--interval-sec N]
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
import { parseArgs, ArgError } from "./args.mjs";

let opts;
try {
  opts = parseArgs(process.argv.slice(2));
} catch (e) {
  if (!(e instanceof ArgError)) throw e;
  console.error(`await-checks: ${e.message}`);
  console.error("usage: await-checks.mjs <pr> [--allow-skipped a,b] [--require a,b]");
  console.error("                          [--timeout-min N] [--interval-sec N]");
  process.exit(2);
}
const { pr, allowSkipped, required, timeoutMs, intervalMs } = opts;

const gh = (fields) =>
  JSON.parse(execFileSync("gh", ["pr", "view", pr, "--json", fields], { encoding: "utf8" }));
const api = (path) => JSON.parse(execFileSync("gh", ["api", path], { encoding: "utf8" }));
const nwo = JSON.parse(
  execFileSync("gh", ["repo", "view", "--json", "nameWithOwner"], { encoding: "utf8" })
).nameWithOwner;

const started = Date.now();
let last = "";
for (;;) {
  const { headRefOid } = gh("headRefOid");

  // THE SEAM, made structural rather than compared after the fact. An earlier
  // draft read `statusCheckRollup` and compared the run's commit to the PR head —
  // but the GRAPHQL rollup does not carry the run's SHA, so that comparison was
  // `undefined !== undefined`: a seam check that could never fire, the same class
  // of defect as the polling bug this file exists to fix.
  //
  // Asking for the check-runs OF A COMMIT cannot return an answer about a
  // different one, so the property is enforced by the query. The REST payload
  // DOES carry `head_sha` (an earlier O37 note wrongly generalised the GraphQL
  // gap to this endpoint), so it is also asserted below — structural first,
  // checked second.
  // PAGINATED, and the count asserted. This endpoint caps at 30 per page and
  // returns total_count beside check_runs; an unpaginated read silently drops a
  // failing check on page 2, and absence — by this file's own thesis — would then
  // read as success. A fail-OPEN truncation inside a fail-closed guard.
  const page = api(`repos/${nwo}/commits/${headRefOid}/check-runs?per_page=100`);
  const checks = page.check_runs ?? [];

  // THE SEAM, ASSERTED — not merely implied by the URL. Every check run carries
  // `head_sha`, which an earlier revision of O37 wrongly claimed was unavailable
  // (that was true of the GraphQL rollup, not of this endpoint). Querying by SHA
  // makes a wrong answer unlikely; comparing the field makes it detectable.
  const wrongCommit = checks.filter((c) => c.head_sha && c.head_sha !== headRefOid);
  if (wrongCommit.length) {
    console.error(
      `await-checks: ${wrongCommit.length} check run(s) report a different head_sha than ${headRefOid} — refusing.`
    );
    process.exit(2);
  }
  if (typeof page.total_count === "number" && checks.length !== page.total_count) {
    console.error(
      `await-checks: got ${checks.length} check runs but total_count is ${page.total_count} — ` +
        `the list is truncated and a failing check could be missing. Refusing to report ready.`
    );
    process.exit(2);
  }
  const v = verdict(checks, { allowSkipped, require: required });

  if (v.reason !== last) {
    console.log(`[${new Date().toISOString().slice(11, 19)}] ${v.ready ? "READY" : "not ready"} — ${v.reason}`);
    last = v.reason;
  }
  if (v.ready) {
    // THE HEAD CAN MOVE MID-POLL. The SHA-keyed query proves the checks describe
    // THIS commit; it says nothing about whether this commit is still the head by
    // the time anyone acts on the answer. Re-read and compare before saying yes.
    const { headRefOid: nowHead } = gh("headRefOid");
    if (nowHead !== headRefOid) {
      console.error(
        `await-checks: the head moved from ${headRefOid.slice(0, 7)} to ${nowHead.slice(0, 7)} during the poll — ` +
          `the green verdict is about a commit that is no longer what would merge.`
      );
      process.exit(2);
    }
    console.log(`await-checks: PR #${pr} @ ${headRefOid.slice(0, 7)} is mergeable.`);
    // TOCTOU: headRefOid is re-read each tick, so a push during the poll moves the
    // target silently. Pin the merge to the commit that was actually verified.
    console.log(`gh pr merge ${pr} --merge --match-head-commit ${headRefOid}`);
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
