import { test } from "node:test";
import assert from "node:assert/strict";
import { classify, verdict, SUCCESS, FAILURE, RUNNING, SKIPPED, NEUTRAL, STALE, UNKNOWN } from "./check-state.mjs";
import { parseArgs, ArgError } from "./args.mjs";

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

test("a legacy commit status is UNKNOWN — this guard does not consume them", () => {
  // The branch that classified these was deleted: `await-checks.mjs` queries
  // /check-runs, which never returns a status context, so nothing production-side
  // reached it — and review found it carried the same missing-toUpperCase() bug
  // mutation caught next door, unfixed because no real payload exercised it.
  // Unknown is the safe reading: if one ever arrives it BLOCKS.
  const ctx = (state) => ({ __typename: "StatusContext", context: "ci/legacy", state });
  for (const state of ["SUCCESS", "success", "PENDING", "FAILURE"]) {
    assert.equal(classify(ctx(state)).state, UNKNOWN, state);
  }
  assert.equal(verdict([ctx("SUCCESS")]).ready, false, "a legacy status must never be sufficient");
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

test("a required check that has not been created yet blocks, and WAITS", () => {
  // The blocker review found: "absence is not success" was enforced only for a
  // rollup of length zero, so any non-empty SUBSET was ready. Reproduced against
  // this repo's own workflows — `build-sign` lives in a separate workflow and
  // completes as skipped in ~1s, so a tick can see only that.
  const onlySkip = [{ name: "build-sign", status: "completed", conclusion: "skipped" }];
  const v = verdict(onlySkip, { allowSkipped: ["build-sign"], require: ["validate", "go", "infra"] });
  assert.equal(v.ready, false, "one allow-listed skip must not merge a PR nothing ran on");
  assert.equal(v.wait, true, "a check not yet CREATED is 'not yet', not 'never'");
  assert.match(v.reason, /required check not reported yet/);

  // A subset of the required set blocks too.
  const partial = [{ name: "validate", status: "completed", conclusion: "success" }];
  assert.equal(verdict(partial, { require: ["validate", "go", "infra"] }).ready, false);
  // And the full set, all green, passes — the guard must still be able to say yes.
  const full = ["validate", "go", "infra"].map((name) => ({ name, status: "completed", conclusion: "success" }));
  assert.equal(verdict(full, { require: ["validate", "go", "infra"] }).ready, true);
});

test("an all-skipped rollup is not success, however generous the allow-list", () => {
  const skipped = ["validate", "go", "infra"].map((name) => ({ name, status: "completed", conclusion: "skipped" }));
  const v = verdict(skipped, { allowSkipped: ["validate", "go", "infra"], require: [] });
  assert.equal(v.ready, false, "zero work done is not a pass");
  assert.match(v.reason, /nothing actually succeeded/);
});

test("a FAILURE alongside a still-running check stops the poll rather than waiting it out", () => {
  // Survived mutation: flipping `if (bad.length)` to `if (false)` left the suite
  // green, because no test had a failure and a running check in one rollup — the
  // only state where that branch is observable, and the common real one (go fails
  // at 2 min while infra is still going). Without it the poller spins to the
  // 30-minute timeout instead of reporting the failure.
  const v = verdict([
    { name: "go", status: "completed", conclusion: "failure" },
    { name: "infra", status: "in_progress", conclusion: null },
  ]);
  assert.equal(v.ready, false);
  assert.equal(v.wait, false, "a known failure must not be polled until timeout");
  assert.match(v.reason, /failed: go/);
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

// ---------------------------------------------------------------------------
// Survivors found by an independent QA mutation sweep (37 mutations, 14 real
// survivors). Each of these pins a branch that was previously unobservable.
// ---------------------------------------------------------------------------

test("a payload carrying BOTH state and status is read by its status", () => {
  // Mutating `(check.state && !check.status)` to `(check.state)` survived: a
  // RUNNING check that happens to carry a stray `state` would have been read by
  // that field instead. Consulting the wrong field first is the original bug's
  // exact shape, one layer over.
  assert.equal(
    classify({ name: "go", state: "SUCCESS", status: "IN_PROGRESS", conclusion: "" }).state,
    RUNNING,
    "a live status must beat a stray state field",
  );
  assert.equal(
    classify({ name: "go", state: "FAILURE", status: "COMPLETED", conclusion: "SUCCESS" }).state,
    SUCCESS,
  );
});

test("the allow-list excuses a SKIP, not the job", () => {
  // Mutating the predicate to `!allow.has(s.name)` survived, which widens
  // allowSkipped from "this job's skip is expected" to "this job may return
  // anything" — so an allow-listed job going NEUTRAL or STALE would merge.
  for (const conclusion of ["NEUTRAL", "STALE", "CANCELLED"]) {
    const v = verdict(
      [
        { name: "validate", status: "COMPLETED", conclusion: "SUCCESS" },
        { name: "build-sign", status: "COMPLETED", conclusion },
      ],
      { allowSkipped: ["build-sign"] },
    );
    assert.equal(v.ready, false, `an allow-listed job returning ${conclusion} must not merge`);
  }
});

test("an unnamed check cannot be allow-listed by accident", () => {
  const c = classify({ name: "", status: "COMPLETED", conclusion: "SKIPPED" });
  assert.equal(c.name, "<unnamed>", "an empty name must not collapse into a real one");
  assert.equal(
    verdict([{ name: "", status: "COMPLETED", conclusion: "SKIPPED" }], { allowSkipped: [""] }).ready,
    false,
    "allow-listing the empty string must not excuse an unnamed skip",
  );
});

// ---------------------------------------------------------------------------
// Argument parsing — extracted precisely so these are testable at all.
// ---------------------------------------------------------------------------

test("a flag VALUE is never mistaken for the PR number", () => {
  // Measured: `args.find(a => /^\d+$/.test(a))` on ["--timeout-min","5","346"]
  // returned 5, so the poller reported confidently about the wrong PR.
  assert.equal(parseArgs(["--timeout-min", "5", "346"]).pr, "346");
  assert.equal(parseArgs(["346", "--timeout-min", "5"]).pr, "346");
});

test("a timeout that cannot be parsed is refused, never treated as no timeout", () => {
  // Number(undefined) * 60_000 is NaN, and `elapsed > NaN` is false forever — so
  // the timeout, the mechanism that turns "no answer" into a non-merge, could
  // never fire. O37's own defect class inside O37's fix.
  for (const argv of [["346", "--timeout-min"], ["346", "--timeout-min", "abc"],
                      ["346", "--timeout-min", "0"], ["346", "--timeout-min", "-5"],
                      ["346", "--interval-sec", "NaN"]]) {
    assert.throws(() => parseArgs(argv), ArgError, argv.join(" "));
  }
  assert.equal(Number.isFinite(parseArgs(["346"]).timeoutMs), true);
  assert.ok(parseArgs(["346"]).timeoutMs > 0, "the default timeout must be a real bound");
});

test("a flag whose value is another flag is an error, not a value", () => {
  // `--allow-skipped` last on the line used to yield the string "undefined" and
  // allow-list a job by that name.
  assert.throws(() => parseArgs(["346", "--allow-skipped"]), ArgError);
  assert.throws(() => parseArgs(["346", "--allow-skipped", "--timeout-min", "5"]), ArgError);
  assert.throws(() => parseArgs(["346", "--unknown", "x"]), ArgError);
  assert.throws(() => parseArgs([]), ArgError);
  assert.throws(() => parseArgs(["346", "347"]), ArgError);
  assert.throws(() => parseArgs(["not-a-number"]), ArgError);
});

test("required defaults to the real CI jobs, and is disabled only deliberately", () => {
  assert.deepEqual(parseArgs(["346"]).required, ["validate", "go", "infra"]);
  assert.deepEqual(parseArgs(["346", "--require", "a,b"]).required, ["a", "b"]);
  assert.deepEqual(parseArgs(["346", "--require", ""]).required, [], "an explicit opt-out is allowed");
});
