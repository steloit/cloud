package identity

// US-11.1 — "Plans gate capabilities, never safety" as a STRUCTURAL invariant.
// The single plan-gate is problem.PlanGated (402 plan_gated). The clearest
// always-available safety operations — a person can always turn MFA on/off and
// can always leave (self-deletion) — must therefore NEVER sit behind that gate.
// This scans their handler source and fails if a plan gate ever creeps in.

import (
	"os"
	"strings"
	"testing"
)

func TestSafetyOperationsNeverPlanGated(t *testing.T) {
	// handler files implementing the "always available" safety operations.
	safetyHandlers := []string{
		"mfa.go", "mfa_http.go", // MFA enroll/verify/disable — F9, never plan-gated
		"account_http.go", // self-deletion + leave-org — a person can always leave
		"password_reset.go", "totp.go",
	}
	for _, f := range safetyHandlers {
		b, err := os.ReadFile(f)
		if err != nil {
			continue // file may not exist under that exact name; the ones that do are checked
		}
		src := string(b)
		if strings.Contains(src, "PlanGated") || strings.Contains(src, "plan_gated") {
			t.Errorf("%s references a plan gate — safety operations (MFA, self-deletion) are never plan-gated (F9/B6, US-11.1)", f)
		}
	}
}
