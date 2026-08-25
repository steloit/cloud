import { test } from "node:test";
import assert from "node:assert/strict";
import { classify, verdict, SUCCESS, FAILURE, RUNNING, SKIPPED, NEUTRAL, STALE, UNKNOWN } from "./check-state.mjs";

const run = (o) => ({ __typename: "CheckRun", name: "go", ...o });

// ---------------------------------------------------------------------------
// THE DEFECT THIS FILE EXISTS FOR.
// ---------------------------------------------------------------------------

test("THE BUG: an in-progress check with an EMPTY conclusion must not read as done", () => {
  // Exactly what `gh pr view --json statusCheckRollup` returned when the bad
  // merge happened: status IN_PROGRESS, conclusion "".
  const c = classify(run({ status: "IN_PROGRESS", conclusion: "" }));
  assert.equal(c.state, RUNNING, "an unfinished check must be RUNNING, not anything terminal");

  const v = verdict([run({ name: "validate", status: "IN_PROGRESS", conclusion: "" }),
                     run({ name: "go", status: "IN_PROGRESS", conclusion: "" }),
                     run({ name: "infra", status: "IN_PROGRESS", conclusion: "" })]);
  assert.equal(v.ready, false, "three unfinished checks must NOT be mergeable");
  assert.equal(v.wait, true, "and the poller must be told to keep waiting, not to give up");
});

test("only an explicit terminal SUCCESS is eligible for merge", () => {
  const green = ["validate", "go", "infra"].map((name) => run({ name, status: "COMPLETED", conclusion: "SUCCESS" }));
  assert.equal(verdict(green).ready, true);

  // Every other terminal conclusion GitHub can produce must NOT be mergeable.
  for (const conclusion of ["FAILURE", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED",
                            "STARTUP_FAILURE", "NEUTRAL", "SKIPPED", "STALE"]) {
    const v = verdict([...green.slice(0, 2), run({ name: "infra", status: "COMPLETED", conclusion })]);
    assert.equal(v.ready, false, `${conclusion} must not be mergeable`);
  }
});

test("COMPLETED with no conclusion is UNKNOWN, never success", () => {
  for (const conclusion of ["", null, undefined]) {
    assert.equal(classify(run({ status: "COMPLETED", conclusion })).state, UNKNOWN);
  }
  const v = verdict([run({ status: "COMPLETED", conclusion: "" })]);
  assert.equal(v.ready, false);
  assert.equal(v.wait, true, "an unknown state waits rather than merging or giving up");
});

// ---------------------------------------------------------------------------
// Absence is not success.
// ---------------------------------------------------------------------------

test("zero checks is NOT green — the empty rollup that reads like nothing failing", () => {
  const v = verdict([]);
  assert.equal(v.ready, false);
  assert.equal(v.wait, true);
  assert.match(v.reason, /absence is not success/);
});

test("a malformed rollup is not green", () => {
  assert.equal(verdict(null).ready, false);
  assert.equal(verdict(undefined).ready, false);
  assert.equal(classify(null).state, UNKNOWN);
  assert.equal(classify("nonsense").state, UNKNOWN);
});

// ---------------------------------------------------------------------------
// Every state GitHub's schema can produce is classified — no silent default.
// ---------------------------------------------------------------------------

test("every CheckConclusionState member maps somewhere, and only SUCCESS is success", () => {
  // Read from the live schema, not invented:
  //   gh api graphql -f query='{__type(name:"CheckConclusionState"){enumValues{name}}}'
  const expected = {
    SUCCESS, FAILURE, TIMED_OUT: FAILURE, CANCELLED: FAILURE, ACTION_REQUIRED: FAILURE,
    STARTUP_FAILURE: FAILURE, NEUTRAL, SKIPPED, STALE,
  };
  for (const [conclusion, want] of Object.entries(expected)) {
    assert.equal(classify(run({ status: "COMPLETED", conclusion })).state, want, conclusion);
  }
  const successes = Object.entries(expected).filter(([, v]) => v === SUCCESS).map(([k]) => k);
  assert.deepEqual(successes, ["SUCCESS"], "exactly one conclusion may mean success");
});

test("a conclusion this code does not recognise fails CLOSED, not open", () => {
  // Found by mutating `CONCLUSION.get(raw) ?? UNKNOWN` to `?? SUCCESS`: the whole
  // suite stayed green. Nothing covered the case that matters most for a guard
  // meant to survive GitHub adding an enum member — the default branch WAS the
  // fail-closed property, and it was untested.
  for (const conclusion of ["DEGRADED", "PARTIAL_SUCCESS", "success-lowercase-typo", "42"]) {
    const c = classify(run({ status: "COMPLETED", conclusion }));
    assert.equal(c.state, UNKNOWN, `${conclusion} must not be classified as anything known`);
  }
  const v = verdict([run({ name: "go", status: "COMPLETED", conclusion: "SOMETHING_GITHUB_ADDED_LATER" })]);
  assert.equal(v.ready, false, "an unrecognised verdict must never be mergeable");
  assert.equal(v.wait, true, "and it must not be read as a terminal failure either");
});

test("every non-COMPLETED CheckStatusState waits", () => {
  for (const status of ["REQUESTED", "QUEUED", "IN_PROGRESS", "WAITING", "PENDING"]) {
    assert.equal(classify(run({ status, conclusion: "" })).state, RUNNING, status);
  }
  // A status this code does not know must not be treated as finished.
  assert.equal(classify(run({ status: "SOMETHING_NEW", conclusion: "SUCCESS" })).state, UNKNOWN);
  assert.equal(classify(run({ conclusion: "SUCCESS" })).state, UNKNOWN, "missing status");
});

test("the REST check-runs payload — lowercase — is what production actually feeds this", () => {
  // Found by mutating away `.toUpperCase()`: the whole suite stayed green while
  // production would have broken, because every other test here uses the GraphQL
  // rollup's UPPERCASE shape and `await-checks.mjs` queries the REST endpoint,
  // which returns lowercase. Testing one layer underneath the seam is not testing
  // the seam. Verbatim from
  //   gh api repos/steloit/cloud/commits/<sha>/check-runs
  const rest = [
    { name: "infra", status: "completed", conclusion: "success" },
    { name: "validate", status: "completed", conclusion: "success" },
    { name: "go", status: "completed", conclusion: "success" },
  ];
  assert.equal(verdict(rest).ready, true, "three real green REST check-runs must be mergeable");

  // …and the same payload's failure and in-flight forms.
  assert.equal(classify({ name: "go", status: "in_progress", conclusion: null }).state, RUNNING);
  assert.equal(classify({ name: "go", status: "queued", conclusion: null }).state, RUNNING);
  assert.equal(classify({ name: "go", status: "completed", conclusion: "failure" }).state, FAILURE);
  assert.equal(classify({ name: "b", status: "completed", conclusion: "skipped" }).state, SKIPPED);
  assert.equal(
    verdict([...rest, { name: "build-sign", status: "completed", conclusion: "skipped" }]).ready,
    false,
    "a lowercase unlisted skip must block exactly as an uppercase one does",
  );
});

test("StatusContext (legacy commit statuses) is classified too", () => {
  const ctx = (state) => ({ __typename: "StatusContext", context: "ci/legacy", state });
  assert.equal(classify(ctx("SUCCESS")).state, SUCCESS);
  assert.equal(classify(ctx("PENDING")).state, RUNNING);
  assert.equal(classify(ctx("EXPECTED")).state, RUNNING);
  assert.equal(classify(ctx("FAILURE")).state, FAILURE);
  assert.equal(classify(ctx("ERROR")).state, FAILURE);
  assert.equal(classify(ctx("WHAT")).state, UNKNOWN);
});

// ---------------------------------------------------------------------------
// SKIPPED is allowed only by name, and the seam is checked.
// ---------------------------------------------------------------------------

test("SKIPPED needs an explicit allow-list entry, by name", () => {
  const checks = [
    run({ name: "validate", status: "COMPLETED", conclusion: "SUCCESS" }),
    run({ name: "build-sign", status: "COMPLETED", conclusion: "SKIPPED" }),
  ];
  assert.equal(verdict(checks).ready, false, "an unlisted skip is not success");
  assert.equal(verdict(checks, { allowSkipped: ["build-sign"] }).ready, true);
  assert.equal(
    verdict(checks, { allowSkipped: ["something-else"] }).ready, false,
    "allow-listing a DIFFERENT job must not excuse this one",
  );
  // The job that matters going silent must not pass because a sibling is listed.
  const goSkipped = [
    run({ name: "validate", status: "COMPLETED", conclusion: "SUCCESS" }),
    run({ name: "go", status: "COMPLETED", conclusion: "SKIPPED" }),
  ];
  assert.equal(verdict(goSkipped, { allowSkipped: ["build-sign"] }).ready, false);
});

test("green checks for a DIFFERENT commit are not an answer about this one", () => {
  const green = [run({ name: "go", status: "COMPLETED", conclusion: "SUCCESS" })];
  assert.equal(verdict(green, { expectedSha: "a".repeat(40), rollupSha: "a".repeat(40) }).ready, true);
  const stale = verdict(green, { expectedSha: "a".repeat(40), rollupSha: "b".repeat(40) });
  assert.equal(stale.ready, false, "a green run for an older head must not merge the new one");
  assert.equal(stale.wait, false, "and it is not a matter of waiting — it needs a new run");
});

// ---------------------------------------------------------------------------
// wait vs no: a poller must neither merge early nor spin on a failure.
// ---------------------------------------------------------------------------

test("a failure stops the poll; an unfinished check continues it", () => {
  const failed = verdict([run({ status: "COMPLETED", conclusion: "FAILURE" })]);
  assert.equal(failed.wait, false, "polling a failed run forever is the other way to get this wrong");
  const running = verdict([run({ status: "QUEUED", conclusion: "" })]);
  assert.equal(running.wait, true);
});

test("one unfinished check among successes still blocks", () => {
  const v = verdict([
    run({ name: "validate", status: "COMPLETED", conclusion: "SUCCESS" }),
    run({ name: "infra", status: "COMPLETED", conclusion: "SUCCESS" }),
    run({ name: "go", status: "IN_PROGRESS", conclusion: "" }),
  ]);
  assert.equal(v.ready, false);
  assert.equal(v.wait, true);
  assert.match(v.reason, /go\(running\)/);
});
