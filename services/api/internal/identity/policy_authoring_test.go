package identity

import "testing"

// Pure unit coverage for the per-key enforcement vocabulary (no DB) — the guard
// that keeps ai-assistant on enabled|opt_in|disabled and rule policies on
// warn|enforce, warn-first by default.
func TestValidatePolicyEnforcement(t *testing.T) {
	cases := []struct {
		key, in, want string
		ok            bool
	}{
		{"ai-assistant", "", "opt_in", true},
		{"ai-assistant", "disabled", "disabled", true},
		{"ai-assistant", "enabled", "enabled", true},
		{"ai-assistant", "warn", "", false},    // rule vocab rejected for ai
		{"ai-assistant", "enforce", "", false},
		{"allowed-regions", "", "warn", true},  // warn-first default
		{"allowed-regions", "warn", "warn", true},
		{"allowed-regions", "enforce", "enforce", true},
		{"allowed-regions", "enabled", "", false},  // ai vocab rejected for rule
		{"allowed-regions", "disabled", "", false},
		{"allowed-regions", "bogus", "", false},
	}
	for _, c := range cases {
		got, err := ValidatePolicyEnforcement(c.key, c.in)
		if c.ok {
			if err != nil || got != c.want {
				t.Errorf("%s/%q → (%q,%v), want (%q,nil)", c.key, c.in, got, err, c.want)
			}
		} else if err == nil {
			t.Errorf("%s/%q accepted, want rejection", c.key, c.in)
		}
	}
}
