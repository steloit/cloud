// Package testenv decides what a MISSING container runtime means.
//
// On a laptop it means "skip these, they need Docker" — the right answer, and
// what the three integration helpers have always done. In CI it must mean
// FAIL, because a skip there is indistinguishable at the job level from a pass:
// the package prints `ok`, the job is green, and nothing ran.
//
// That is not hypothetical harm. O13 added two gates whose value rests entirely
// on these containers actually starting — `-race` (added because the detector
// had never run, and the race it was added for lived in one of these very
// packages) and `-timeout 30m` (derived from internal/identity measuring ~350s).
// Under a silent skip, `-race` degrades to a unit-only detector and the timeout
// reasoning is moot, while CI reports success for both.
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
		t.Fatalf("container runtime unavailable and %s is set: %v\n\n"+
			"This is CI, where a skip is indistinguishable from a pass: the package prints ok, "+
			"the job is green, and nothing ran. The -race and -timeout gates both rest on these "+
			"containers starting, so a silent skip voids them while reporting success.",
			RequireContainersVar, err)
	}
	t.Skipf("container runtime unavailable (CI runs this, and fails if it cannot): %v", err)
}
