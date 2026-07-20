package quota

import "testing"

func TestEvaluateSoft(t *testing.T) {
	rate := int64(700) // $7/seat

	// within the allowance → proceed, no overage
	if d := Evaluate(Soft, 5, 3, 1, rate, false); !d.Allowed || d.SoftBlocked || d.OverageConfirmed {
		t.Fatalf("within allowance: %+v", d)
	}
	// exactly AT the allowance (used+delta == allowance) → still allowed
	if d := Evaluate(Soft, 5, 4, 1, rate, false); !d.Allowed {
		t.Fatalf("at allowance should proceed: %+v", d)
	}
	// crossing the allowance with no confirm → soft-blocked, price shown
	d := Evaluate(Soft, 5, 5, 1, rate, false)
	if d.Allowed || !d.SoftBlocked || d.OveragePriceCents != 700 {
		t.Fatalf("soft over no-confirm: %+v", d)
	}
	// same op WITH confirm → proceeds, billed
	if d := Evaluate(Soft, 5, 5, 1, rate, true); !d.Allowed || !d.OverageConfirmed {
		t.Fatalf("soft over with confirm: %+v", d)
	}
	// partial crossing: used 4, add 3, allowance 5 → 2 new overage units billed
	if d := Evaluate(Soft, 5, 4, 3, rate, false); !d.SoftBlocked || d.OveragePriceCents != 2*rate {
		t.Fatalf("partial crossing price: %+v", d)
	}
	// already over: used 7, add 2, allowance 5 → both units are overage (2×rate)
	if d := Evaluate(Soft, 5, 7, 2, rate, false); !d.SoftBlocked || d.OveragePriceCents != 2*rate {
		t.Fatalf("already-over incremental: %+v", d)
	}
}

func TestEvaluateHard(t *testing.T) {
	// hard over → blocked, never a soft price
	d := Evaluate(Hard, 1, 1, 1, 0, false)
	if d.Allowed || !d.HardBlocked || d.SoftBlocked {
		t.Fatalf("hard over: %+v", d)
	}
	// confirm does NOT bypass a hard quota
	if d := Evaluate(Hard, 1, 1, 1, 0, true); d.Allowed {
		t.Fatal("confirm bypassed a hard quota")
	}
	// within → allowed
	if d := Evaluate(Hard, 3, 1, 1, 0, false); !d.Allowed {
		t.Fatalf("hard within: %+v", d)
	}
}

func TestUnlimited(t *testing.T) {
	// allowance < 0 = unlimited: always allowed, never warns
	if d := Evaluate(Soft, -1, 1_000_000, 1, 700, false); !d.Allowed || d.SoftBlocked {
		t.Fatalf("unlimited soft: %+v", d)
	}
	if WarnLevel(-1, 999) != -1 || ShouldWarn(-1, 999) {
		t.Fatal("unlimited must never warn")
	}
}

// Degenerate/adversarial inputs: a negative delta can't fake being under; a
// zero-included free tier bills the first unit; delta=0 is a no-op.
func TestEvaluateEdgeCases(t *testing.T) {
	rate := int64(700)
	// negative delta (a reduction) never incurs overage even when already over
	if d := Evaluate(Soft, 5, 7, -3, rate, false); !d.Allowed || d.SoftBlocked {
		t.Fatalf("negative delta faked/blocked: %+v", d)
	}
	// free tier with 0 included: the first unit is overage
	if d := Evaluate(Soft, 0, 0, 1, rate, false); !d.SoftBlocked || d.OveragePriceCents != rate {
		t.Fatalf("zero-included first unit: %+v", d)
	}
	// delta=0 while under → allowed, no overage
	if d := Evaluate(Soft, 5, 3, 0, rate, false); !d.Allowed {
		t.Fatalf("zero delta under: %+v", d)
	}
	// WarnLevel of a 0 allowance with usage → saturated
	if WarnLevel(0, 1) != 1 || !ShouldWarn(0, 1) {
		t.Fatal("0-allowance with usage should warn")
	}
	if WarnLevel(0, 0) != 0 {
		t.Fatal("0-allowance, 0-usage is 0")
	}
}

func TestWarnAt80(t *testing.T) {
	// 87/100 (the canon egress warn) → warns
	if !ShouldWarn(100, 87) {
		t.Fatal("87/100 should warn")
	}
	if ShouldWarn(100, 79) {
		t.Fatal("79/100 should not warn")
	}
	if !ShouldWarn(100, 80) {
		t.Fatal("exactly 80% should warn")
	}
}

// TestClampOverageToCap — soft overage halts at the hard spend cap (US-11.2/11.7).
func TestClampOverageToCap(t *testing.T) {
	cases := []struct {
		name                            string
		spent, cap, overage, wantAccrue int64
	}{
		{"uncapped accrues fully", 5000, -1, 200, 200},
		{"well under cap accrues fully", 5000, 10000, 200, 200},
		{"partial slice up to the cap", 9900, 10000, 200, 100},
		{"exactly at cap halts", 10000, 10000, 200, 0},
		{"over cap halts", 10500, 10000, 200, 0},
		{"zero overage is zero", 5000, 10000, 0, 0},
	}
	for _, c := range cases {
		if got := ClampOverageToCap(c.spent, c.cap, c.overage); got != c.wantAccrue {
			t.Errorf("%s: ClampOverageToCap(%d,%d,%d)=%d, want %d", c.name, c.spent, c.cap, c.overage, got, c.wantAccrue)
		}
	}
}

// TestWarnMessageScenario2 — QA scenario 2 (canon-pinned Business egress): at
// 87/100 GB the 80% warning fires with the math (percent + the $0.09/GB overage
// price). The per-plan Free/Pro egress allowances stay canon-deferred (plans.json
// $note); this proves the mechanism for the one canon-pinned allowance.
func TestWarnMessageScenario2(t *testing.T) {
	// egress: allowance 100 GB, used 87 GB, overage 9¢/GB (canon).
	w, warn := WarnMessage("egress", 100, 87, 9)
	if !warn {
		t.Fatal("87/100 GB (87%) must fire the 80% warning")
	}
	if w.PercentUsed != 87 {
		t.Fatalf("percent = %d, want 87", w.PercentUsed)
	}
	if w.OverageRate != 9 || w.RemainingUnits != 13 {
		t.Fatalf("math wrong: rate=%d remaining=%d (want 9, 13)", w.OverageRate, w.RemainingUnits)
	}
	// under 80% does NOT warn.
	if _, warn := WarnMessage("egress", 100, 79, 9); warn {
		t.Fatal("79/100 (79%) must not warn")
	}
	// unlimited never warns.
	if _, warn := WarnMessage("events", -1, 999999, 120); warn {
		t.Fatal("unlimited allowance must never warn")
	}
}
