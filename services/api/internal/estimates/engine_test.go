package estimates

// Canon regression: the fixtures file is IMPORTED, never retyped. Every
// canon service shape must price to its canonical monthly_estimate_cents,
// and their sum must be the ratified $208 (20800) family invariant.

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/steloit/cloud/services/api/internal/canon"
)

type canonService struct {
	Name                string         `json:"name"`
	Product             string         `json:"product"`
	Shape               map[string]any `json:"shape"`
	MonthlyEstimateCent int64          `json:"monthly_estimate_cents"`
}

func loadCanonServices(t *testing.T) []canonService {
	t.Helper()
	raw, err := os.ReadFile("../../../../docs/product/19-canon/fixtures.json")
	if err != nil {
		t.Fatalf("canon fixtures: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	var svcs []canonService
	for key, v := range doc {
		if len(key) >= 8 && key[:8] == "services" {
			if err := json.Unmarshal(v, &svcs); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(svcs) == 0 {
		t.Fatal("no canon services found")
	}
	return svcs
}

func TestCanonArithmetic(t *testing.T) {
	svcs := loadCanonServices(t)
	var total int64
	for _, s := range svcs {
		line, err := Price(ShapeInput{Product: s.Product, Name: s.Name, Shape: s.Shape})
		if err != nil {
			t.Fatalf("canon %s: %v", s.Name, err)
		}
		if line.MonthlyCents != s.MonthlyEstimateCent {
			t.Fatalf("canon %s prices to %d, canon says %d", s.Name, line.MonthlyCents, s.MonthlyEstimateCent)
		}
		total += line.MonthlyCents
	}
	// The $208 anchor is IMPORTED from the shared canon package (Q2), not
	// retyped — the same number the console/invoice layers assert.
	w, err := canon.Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := w.EcommerceProjectCents(); total != want {
		t.Fatalf("the $208 invariant broke: priced Σ=%d, canon project total=%d", total, want)
	}
	if err := w.CheckArithmetic(); err != nil {
		t.Fatalf("canon arithmetic invariant broke: %v", err)
	}
	t.Logf("canon reproduced: %d services, Σ=%d cents (canon-verified)", len(svcs), total)
}

// checkout-stack (canon T3 capture example): api + worker + jobs + cache = $126.
func TestCheckoutStackTemplate(t *testing.T) {
	svcs := loadCanonServices(t)
	want := map[string]bool{"api": true, "worker": true, "jobs": true, "cache": true}
	var in []ShapeInput
	for _, s := range svcs {
		if want[s.Name] {
			in = append(in, ShapeInput{Product: s.Product, Name: s.Name, Shape: s.Shape})
		}
	}
	if len(in) != 4 {
		t.Fatalf("expected 4 checkout-stack services, got %d", len(in))
	}
	_, total, err := PriceAll(in)
	if err != nil {
		t.Fatal(err)
	}
	if total != 12600 {
		t.Fatalf("checkout-stack must be $126 (12600), got %d", total)
	}
}

func TestEngineRules(t *testing.T) {
	// server-defaulted intents (S11)
	l, err := Price(ShapeInput{Product: "postgres", Shape: map[string]any{"size": "dev"}})
	if err != nil || l.Intent != "database" {
		t.Fatalf("postgres default intent: %+v %v", l, err)
	}
	if l, _ = Price(ShapeInput{Product: "valkey", Shape: map[string]any{}}); l.Intent != "cache" {
		t.Fatalf("valkey default intent: %+v", l)
	}
	if l, _ = Price(ShapeInput{Product: "web", Shape: map[string]any{}}); l.Intent != "app" {
		t.Fatalf("web default intent: %+v", l)
	}
	// explicit intent survives (jobs rides postgres)
	if l, _ = Price(ShapeInput{Product: "postgres", Intent: "jobs", Shape: map[string]any{"size": "dev", "storage_gb": 4}}); l.Intent != "jobs" || l.MonthlyCents != 2100 {
		t.Fatalf("jobs line: %+v", l)
	}
	// HA is +$19 (C2)
	base, _ := Price(ShapeInput{Product: "postgres", Shape: map[string]any{"size": "dev"}})
	ha, _ := Price(ShapeInput{Product: "postgres", Shape: map[string]any{"size": "dev", "ha": true}})
	if ha.MonthlyCents-base.MonthlyCents != 1900 {
		t.Fatalf("HA delta: %d", ha.MonthlyCents-base.MonthlyCents)
	}
	// unknown product / size fail loudly with the field named
	if _, err := Price(ShapeInput{Product: "gpu"}); err == nil {
		t.Fatal("gpu must be rejected (removed from the surface)")
	}
	if _, err := Price(ShapeInput{Product: "postgres", Shape: map[string]any{"size": "mega"}}); err == nil {
		t.Fatal("unknown size accepted")
	}
	// Dev pays for every GB; Standard's canon-derived 50 GB allowance holds
	devTen, _ := Price(ShapeInput{Product: "postgres", Shape: map[string]any{"size": "dev", "storage_gb": 10}})
	if devTen.MonthlyCents != 2400 {
		t.Fatalf("dev+10GB must be $24: %d", devTen.MonthlyCents)
	}
	stdSixty, _ := Price(ShapeInput{Product: "postgres", Shape: map[string]any{"size": "standard", "storage_gb": 60}})
	if stdSixty.MonthlyCents != 5800+10*50 {
		t.Fatalf("standard+60GB: %d", stdSixty.MonthlyCents)
	}
}
