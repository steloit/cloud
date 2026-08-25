// Argument parsing for await-checks, extracted so it can be TESTED.
//
// It lived inline and carried two defects of exactly the class this tooling
// exists to eliminate, both found by review rather than by me:
//
//   1. The PR number was `args.find(a => /^\d+$/.test(a))`, which matches FLAG
//      VALUES: `await-checks.mjs --timeout-min 5 346` polled PR #5 and reported
//      confidently about the wrong pull request.
//   2. `--timeout-min` with a missing or non-numeric value produced NaN, and
//      `Date.now() - started > NaN` is false forever — so the timeout, the
//      mechanism that turns "we never got an answer" into a non-merge, could
//      never fire. A predicate that cannot fail, inside the fix for a predicate
//      that could not fail.
//
// Anything unparseable throws. The caller exits 2 — "could not determine",
// which is neither success nor a red build.

export class ArgError extends Error {}

const FLAGS_WITH_VALUES = new Set(["allow-skipped", "require", "timeout-min", "interval-sec"]);

export function parseArgs(argv) {
  const positional = [];
  const flags = new Map();
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (!a.startsWith("--")) {
      positional.push(a);
      continue;
    }
    const name = a.slice(2);
    if (!FLAGS_WITH_VALUES.has(name)) throw new ArgError(`unknown flag --${name}`);
    const value = argv[i + 1];
    // A flag whose value is missing, or is itself a flag, is an error — never a
    // default. `--allow-skipped` last on the line used to yield the string
    // "undefined" and allow-list a job by that name.
    if (value === undefined || value.startsWith("--")) {
      throw new ArgError(`--${name} needs a value`);
    }
    flags.set(name, value);
    i++; // consume the value so it can never be read as the PR number
  }

  if (positional.length !== 1) {
    throw new ArgError(
      positional.length === 0 ? "no PR number given" : `expected one PR number, got: ${positional.join(" ")}`
    );
  }
  const pr = positional[0];
  if (!/^\d+$/.test(pr)) throw new ArgError(`PR must be a number, got ${JSON.stringify(pr)}`);

  const list = (name) =>
    (flags.get(name) ?? "").split(",").map((s) => s.trim()).filter(Boolean);
  const positive = (name, fallback, scale) => {
    if (!flags.has(name)) return fallback * scale;
    const n = Number(flags.get(name));
    if (!Number.isFinite(n) || n <= 0) {
      throw new ArgError(`--${name} must be a positive number, got ${JSON.stringify(flags.get(name))}`);
    }
    return n * scale;
  };

  return {
    pr,
    allowSkipped: list("allow-skipped"),
    // Required by default: a check that has not been CREATED yet must not read as
    // fine. `--require ""` disables it, which is then a deliberate choice.
    required: flags.has("require") ? list("require") : ["validate", "go", "infra"],
    timeoutMs: positive("timeout-min", 30, 60_000),
    intervalMs: positive("interval-sec", 30, 1000),
  };
}
