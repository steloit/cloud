// Package testenv decides what a MISSING container runtime means.
//
// On a laptop it means "skip these, they need Docker" — the right answer, and
// what the three integration helpers have always done. In CI it must mean
// FAIL, because a skip there is indistinguishable at the job level from a pass:
// the package prints `ok`, the job is green, and nothing ran.
//
// That is not hypothetical harm. O13's `-timeout 30m` is derived from
// internal/identity measuring ~350s — a number that is meaningless for a suite
// that skips, so the gate reports success while proving nothing about the run it
// was sized for.
//
// `-race` is the more careful claim, and an earlier version of this comment got
// it WRONG. It said the race O14/Q10 fixed "lived in one of these very packages",
// implying a silent skip would have hidden it. It would not have: that race was
// in reconcile_test.go's in-memory fakes (18 newFixture uses, zero realDB), so
// the test runs with or without a container and `-race` catches it either way.
// Naming the same PACKAGE on both sides while the distinction that matters is
// unit-test file vs container-test file is exactly the failure CLAUDE.md records
// from O11.
//
// The accurate `-race` argument is prospective: under a silent skip the detector
// never examines the CONTAINER-BACKED suites at all — including
// TestConcurrentWritebackAppliesOnceAgainstRealPostgres and
// TestIdempotencyConcurrentDoubleSubmitHasOneWinner, which are where concurrency
// against real Postgres is exercised. That is where the next such defect lives,
// not where the last one did.
//
// The pattern is not new here: internal/identity/ckm3_checkpoint_test.go already
// gates on STELOIT_CHECKPOINT and fails in t.Cleanup if the test skipped, with a
// comment saying verbatim that "a milestone gate can be green on a machine that
// never executed it". This generalises that to every container-backed suite.
package testenv

import (
	"os"
	"testing"
)

// RequireContainersVar is set in CI (.github/workflows/ci.yml, the go job). When
// it is present, a missing container runtime is a FAILURE rather than a skip.
const RequireContainersVar = "STELOIT_REQUIRE_CONTAINERS"

// Required reports whether this run must actually execute container-backed tests.
func Required() bool { return os.Getenv(RequireContainersVar) != "" }

// SkipOrFail is what an integration helper calls when the runtime is absent.
//
// It never returns: it is a skip locally and a fatal in CI. Callers use it in
// place of t.Skipf so the decision lives in ONE place — three helpers each
// making it independently is how one of them quietly stops making it.
func SkipOrFail(t *testing.T, err error) {
	t.Helper()
	if Required() {
		t.Fatalf("container runtime unavailable and %s is set: %v\n"+
			"A skip is indistinguishable from a pass here — the package prints ok and nothing ran. "+
			"If you are NOT in CI, `unset %s` to go back to skipping.",
			RequireContainersVar, err, RequireContainersVar)
	}
	t.Skipf("container runtime unavailable (CI runs this, and fails if it cannot): %v", err)
}
