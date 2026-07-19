// Package canon loads the one demo world (docs/product/19-canon/fixtures.json,
// ADR-026) and exposes the arithmetic invariants from canon.md §invariants so
// no layer retypes them. It is the Go import point the canon-testing pack
// mandates ("import via packages/canon, never read fixtures.json raw"); the
// estimate engine and (when E11 lands) the invoice generator both assert the
// SAME equations against the SAME numbers, so a canon change that breaks the
// math fails every layer at once.
//
// The fixtures are located relative to this source file (runtime.Caller), so it
// is a TEST/dev utility — a deployed binary has no source tree. Production
// paths that need canon numbers embed a synced copy (the matrix.csv pattern).
package canon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// World is the slice of fixtures the arithmetic invariants touch. Money is
// integer cents end-to-end (ADR-025); the JSON tags match fixtures.json.
type World struct {
	Projects     []Project
	Environments []Environment
	Services     []Service
	Billing      Billing
}

type Project struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	MonthlyCostCent int64  `json:"monthly_cost_cents"`
}

type Environment struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	MonthlyCostCent int64  `json:"monthly_cost_cents"`
}

type Service struct {
	Name                string `json:"name"`
	MonthlyEstimateCent int64  `json:"monthly_estimate_cents"`
}

type Billing struct {
	ResourcesCents int64 `json:"resources_cents"`
	PlanFeeCents   int64 `json:"plan_fee_cents"`
	ForecastCents  int64 `json:"forecast_cents"`
}

// fixturesPath resolves docs/product/19-canon/fixtures.json relative to this
// file, so it works from any test's working directory.
func fixturesPath() string {
	_, self, _, _ := runtime.Caller(0)
	// services/api/internal/canon/canon.go → repo root is four dirs up, then
	// three more to escape services/api.
	return filepath.Join(filepath.Dir(self), "..", "..", "..", "..", "docs", "product", "19-canon", "fixtures.json")
}

// Load reads and decodes the canon fixtures. The top-level keys are annotated
// (e.g. "projects (Project, org_acme)"), so sections are matched by prefix.
func Load() (*World, error) {
	raw, err := os.ReadFile(fixturesPath())
	if err != nil {
		return nil, fmt.Errorf("canon: read fixtures: %w", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("canon: parse fixtures: %w", err)
	}
	w := &World{}
	for key, v := range doc {
		switch {
		case strings.HasPrefix(key, "projects"):
			if err := json.Unmarshal(v, &w.Projects); err != nil {
				return nil, fmt.Errorf("canon: projects: %w", err)
			}
		case strings.HasPrefix(key, "environments"):
			if err := json.Unmarshal(v, &w.Environments); err != nil {
				return nil, fmt.Errorf("canon: environments: %w", err)
			}
		case strings.HasPrefix(key, "services"):
			if err := json.Unmarshal(v, &w.Services); err != nil {
				return nil, fmt.Errorf("canon: services: %w", err)
			}
		case strings.HasPrefix(key, "billing_overview"):
			if err := json.Unmarshal(v, &w.Billing); err != nil {
				return nil, fmt.Errorf("canon: billing: %w", err)
			}
		}
	}
	return w, nil
}

// EcommerceProjectCents is the ratified $208 project total (canon.md), read
// from the fixtures so callers never retype it.
func (w *World) EcommerceProjectCents() int64 {
	for _, p := range w.Projects {
		if p.Name == "ecommerce" {
			return p.MonthlyCostCent
		}
	}
	return -1
}

// CheckArithmetic verifies the four canon.md §invariants against the fixtures
// and returns a descriptive error on the first violation (nil = all hold):
//
//	Σ ecommerce services   == ecommerce project total   (61+22+58+24+22+21 = 208)
//	Σ ecommerce environments == ecommerce project total (199.10+6.70+2.20 = 208)
//	Σ project totals       == org resources             (208+96+41+38+0 = 383)
//	resources + plan fee   == org total                 (383+99 = 482)
func (w *World) CheckArithmetic() error {
	proj := w.EcommerceProjectCents()
	if proj <= 0 {
		return fmt.Errorf("canon: ecommerce project not found in fixtures")
	}

	var svcSum int64
	for _, s := range w.Services { // fixtures ship only env_prod (ecommerce) services
		svcSum += s.MonthlyEstimateCent
	}
	if svcSum != proj {
		return fmt.Errorf("Σ services %d ≠ ecommerce project %d", svcSum, proj)
	}

	var envSum int64
	for _, e := range w.Environments { // fixtures ship only prj_ecommerce envs
		envSum += e.MonthlyCostCent
	}
	if envSum != proj {
		return fmt.Errorf("Σ environments %d ≠ ecommerce project %d", envSum, proj)
	}

	var projSum int64
	for _, p := range w.Projects {
		projSum += p.MonthlyCostCent
	}
	if projSum != w.Billing.ResourcesCents {
		return fmt.Errorf("Σ projects %d ≠ org resources %d", projSum, w.Billing.ResourcesCents)
	}

	if got := w.Billing.ResourcesCents + w.Billing.PlanFeeCents; got != w.Billing.ForecastCents {
		return fmt.Errorf("resources %d + plan %d = %d ≠ org total %d",
			w.Billing.ResourcesCents, w.Billing.PlanFeeCents, got, w.Billing.ForecastCents)
	}
	return nil
}
