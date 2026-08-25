// Deciding whether a PR's checks are actually green — fail-closed.
//
// WHY THIS EXISTS. A merge was performed against a rollup that read
// `validate= go= infra=`. The polling predicate tested for the literal string
// "pending"; GitHub reports an unfinished check with an EMPTY conclusion, so
// "not pending" was true on the first tick and the loop exited before anything
// had run. The merge happened to be correct. The verification was not.
//
// THE RULE, and it is the project-wide one: empty, missing, unknown, skipped,
// cancelled and stale are NEVER success. Only an explicit terminal SUCCESS is.
// A predicate with no positive success state is not a predicate.
//
// The enums below are not guessed and not copied from a blog. They are GitHub's
// own, read from the live schema:
//
//   gh api graphql -f query='{ __type(name:"CheckConclusionState"){enumValues{name}} }'
//
// CheckConclusionState: ACTION_REQUIRED TIMED_OUT CANCELLED FAILURE SUCCESS
//                       NEUTRAL SKIPPED STARTUP_FAILURE STALE
// CheckStatusState:     REQUESTED QUEUED IN_PROGRESS COMPLETED WAITING PENDING
// StatusState:          EXPECTED ERROR FAILURE PENDING SUCCESS
//
// STALE and STARTUP_FAILURE are the ones worth noticing: a hand-written list
// omits both. (An earlier revision of this comment cited O35's billing outage as
// a STARTUP_FAILURE example. O35 records the opposite — "not a workflow parse
// error… the jobs exist and are individually refused" — so the citation was a
// claim about a task that task does not make. The enum handling is right either
// way; the justification was not.)

/** Terminal-and-good. The ONLY state that may lead to a merge. */
export const SUCCESS = "success";
/** Terminal-and-bad: stop waiting, this will not become green. */
export const FAILURE = "failure";
/** Not terminal: keep waiting. */
export const RUNNING = "running";
/** Terminal, deliberately not run. NOT success — must be allow-listed by name. */
export const SKIPPED = "skipped";
/** Terminal, no verdict. Never success. */
export const NEUTRAL = "neutral";
/** Superseded by a newer run. Never success — the answer is about older code. */
export const STALE = "stale";
/** Anything this code does not recognise. Fail-closed: never success, never terminal. */
export const UNKNOWN = "unknown";

const CONCLUSION = new Map([
  ["SUCCESS", SUCCESS],
  ["FAILURE", FAILURE],
  ["TIMED_OUT", FAILURE],
  ["CANCELLED", FAILURE],
  ["ACTION_REQUIRED", FAILURE],
  ["STARTUP_FAILURE", FAILURE],
  ["SKIPPED", SKIPPED],
  ["NEUTRAL", NEUTRAL],
  ["STALE", STALE],
]);

const RUNNING_STATUS = new Set(["REQUESTED", "QUEUED", "IN_PROGRESS", "WAITING", "PENDING"]);

// There is deliberately NO StatusContext branch. An earlier revision classified
// legacy commit statuses and unit-tested it — but `await-checks.mjs` queries
// `/commits/{sha}/check-runs`, which returns check runs and never status
// contexts, so nothing production-side could ever reach it. A tested branch with
// no caller claims coverage it does not have; worse, review found it carried the
// exact missing-`toUpperCase()` bug that mutation testing caught one line below,
// left unfixed precisely because no real payload exercised it.
// Measured: this repo has zero legacy statuses (`/commits/main/status` →
// total_count 0). If one is ever required, it must be QUERIED, not merely
// classified — otherwise a red legacy status stays invisible to this guard.

/**
 * classify one entry of a statusCheckRollup.
 *
 * The order of the branches is load-bearing: STATUS is consulted BEFORE
 * conclusion, because a check that is still IN_PROGRESS may carry an empty
 * conclusion and reading conclusion first is exactly the bug this file exists
 * to prevent.
 */
export function classify(check) {
  if (!check || typeof check !== "object") return { name: "<malformed>", state: UNKNOWN };
  const name = typeof check.name === "string" && check.name ? check.name : check.context || "<unnamed>";

  // A payload carrying `state` but no `status` is a legacy commit status, which
  // this guard does not consume (see above). Unknown, never success — so if one
  // ever arrives it blocks rather than passing silently.
  if (check.state && !check.status) return { name, state: UNKNOWN };

  const status = typeof check.status === "string" ? check.status.toUpperCase() : "";
  if (RUNNING_STATUS.has(status)) return { name, state: RUNNING };
  if (status !== "COMPLETED") return { name, state: UNKNOWN }; // missing/garbled status

  const raw = typeof check.conclusion === "string" ? check.conclusion.toUpperCase() : "";
  if (!raw) return { name, state: UNKNOWN }; // COMPLETED with no verdict — never success
  return { name, state: CONCLUSION.get(raw) ?? UNKNOWN };
}

export const isTerminal = (state) => state !== RUNNING && state !== UNKNOWN;

/**
 * Decide whether a PR may be merged on its checks.
 *
 * Returns { ready, wait, reason, states } — `wait` distinguishes "not yet" from
 * "no", so a poller neither merges early nor spins on a failure.
 *
 * @param checks   statusCheckRollup entries
 * @param opts.allowSkipped  check names whose SKIPPED is expected (e.g. an
 *        image job path-gated off). Named individually — a blanket "skipped is
 *        fine" is how a job that stops running becomes invisible.
 * @param opts.require  check names that MUST be present and successful. Absence
 *        of a required name is "not yet", never "fine" — see the note below.
 */
export function verdict(checks, opts = {}) {
  const { allowSkipped = [], require: required = [] } = opts;
  if (!Array.isArray(checks)) return { ready: false, wait: false, reason: "rollup is not a list", states: [] };
  if (checks.length === 0) {
    // The trap that started this: zero checks renders as an empty string and
    // reads like "nothing failing".
    return { ready: false, wait: true, reason: "no checks reported yet — absence is not success", states: [] };
  }
  const states = checks.map(classify);

  // THE ORIGINAL DEFECT, ONE LEVEL UP — found by review, not by me.
  //
  // "Absence is not success" was enforced only for a rollup of length ZERO. Any
  // non-empty SUBSET of the expected jobs was ready. That is constructible here,
  // not theoretical: `image.yml`'s `build-sign` is a SEPARATE workflow, gated on
  // `vars.GCP_PROJECT`, so when that var is unset it completes as `skipped` in
  // about a second — while `ci.yml`'s validate/go/infra check runs are created
  // independently. A poll tick landing in that window sees exactly
  // `[build-sign=skipped]`, and with the documented `--allow-skipped build-sign`
  // that returned ready:true. Exit 0, merge, nothing ran.
  //
  // A check that has not been CREATED yet is indistinguishable from one that will
  // never exist, so the only safe reading is "not yet".
  const present = new Set(states.map((s) => s.name));
  const absent = required.filter((n) => !present.has(n));
  if (absent.length) {
    return {
      ready: false,
      wait: true,
      reason: `required check not reported yet: ${absent.join(", ")}`,
      states,
    };
  }

  const bad = states.filter((s) => s.state === FAILURE);
  if (bad.length) {
    return { ready: false, wait: false, reason: `failed: ${bad.map((s) => s.name).join(", ")}`, states };
  }
  const pending = states.filter((s) => !isTerminal(s.state));
  if (pending.length) {
    return {
      ready: false,
      wait: true,
      reason: `not finished: ${pending.map((s) => `${s.name}(${s.state})`).join(", ")}`,
      states,
    };
  }
  const allow = new Set(allowSkipped);
  const unexpected = states.filter(
    (s) => s.state !== SUCCESS && !(s.state === SKIPPED && allow.has(s.name))
  );
  if (unexpected.length) {
    return {
      ready: false,
      wait: false,
      reason: `not success: ${unexpected.map((s) => `${s.name}=${s.state}`).join(", ")}`,
      states,
    };
  }
  // At least one check must have actually SUCCEEDED. Without this, a rollup whose
  // every entry is an allow-listed skip is green with zero work done — and
  // `--allow-skipped validate,go,infra,build-sign` is a plausible copy-paste.
  if (!states.some((s) => s.state === SUCCESS)) {
    return { ready: false, wait: true, reason: "nothing actually succeeded — every check was skipped", states };
  }
  return { ready: true, wait: false, reason: `${states.length} checks, all success or allowed-skipped`, states };
}
