package identity

// US-11.1 — "Plans gate capabilities, never safety" as a STRUCTURAL invariant.
// The plan gate is problem.PlanGated (402 plan_gated). The clearest
// always-available safety operations — a person can always turn MFA on/off and
// can always leave (self-deletion) — must therefore NEVER sit behind a plan
// gate. This scans their handler source and fails if a plan gate creeps in.

import (
	"os"
	"strings"
	"testing"
)

func TestSafetyOperationsNeverPlanGated(t *testing.T) {
	// Handler files implementing the "always available" safety operations.
	// The core three MUST exist — if one is renamed/moved, this test fails
	// LOUDLY rather than silently skipping (a vacuous pass is worse than none).
	required := []string{
		"mfa.go", "mfa_http.go", // MFA enroll/verify/disable — F9, never plan-gated
		"account_http.go", // self-deletion + leave-org — a person can always leave
	}
	optional := []string{"password_reset.go", "totp.go"}

	// Any signal that a plan gate is in play — not just the problem.PlanGated
	// helper, but the alternate mechanisms too (a raw required-plan field, a
	// RequirePlan middleware) — so the proxy can't be sidestepped.
	gateSignals := []string{"PlanGated", "plan_gated", "RequiredPlan", "RequirePlan", "requiredPlan"}

	scanned := 0
	check := func(f string, mustExist bool) {
		b, err := os.ReadFile(f)
		if err != nil {
			if mustExist {
				t.Fatalf("required safety handler %q missing — this invariant can no longer see it; point the test at the file that now holds MFA/self-deletion", f)
			}
			return
		}
		scanned++
		src := string(b)
		for _, sig := range gateSignals {
			if strings.Contains(src, sig) {
				t.Errorf("%s references a plan gate (%q) — safety operations (MFA, self-deletion) are never plan-gated (F9/B6, US-11.1)", f, sig)
			}
		}
	}
	for _, f := range required {
		check(f, true)
	}
	for _, f := range optional {
		check(f, false)
	}
	if scanned == 0 {
		t.Fatal("scanned zero safety-handler files — the tripwire is inert")
	}
}
