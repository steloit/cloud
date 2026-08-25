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
// would omit both, and STARTUP_FAILURE is what an infrastructure problem looks
// like — the Actions billing outage produced jobs that never ran a step.

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

const STATUS_CONTEXT = new Map([
  ["SUCCESS", SUCCESS],
  ["FAILURE", FAILURE],
  ["ERROR", FAILURE],
  ["PENDING", RUNNING],
  ["EXPECTED", RUNNING],
]);

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

  // StatusContext (the legacy commit-status shape) carries `state`, not `status`.
  if (check.__typename === "StatusContext" || (check.state && !check.status)) {
    return { name, state: STATUS_CONTEXT.get(check.state) ?? UNKNOWN };
  }

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
 * @param opts.expectedSha / opts.rollupSha  the seam: checks describe a commit,
 *        and a green answer about a DIFFERENT commit is not an answer.
 */
export function verdict(checks, opts = {}) {
  const { allowSkipped = [], expectedSha, rollupSha } = opts;
  if (!Array.isArray(checks)) return { ready: false, wait: false, reason: "rollup is not a list", states: [] };
  if (checks.length === 0) {
    // The trap that started this: zero checks renders as an empty string and
    // reads like "nothing failing".
    return { ready: false, wait: true, reason: "no checks reported yet — absence is not success", states: [] };
  }
  if (expectedSha && rollupSha && expectedSha !== rollupSha) {
    return {
      ready: false,
      wait: false,
      reason: `checks describe ${rollupSha.slice(0, 7)} but the PR head is ${expectedSha.slice(0, 7)}`,
      states: [],
    };
  }

  const states = checks.map(classify);
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
  return { ready: true, wait: false, reason: `${states.length} checks, all success or allowed-skipped`, states };
}
