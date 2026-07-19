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
