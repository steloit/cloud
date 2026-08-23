package main

import (
	"strings"
	"testing"
)

// The boot-time validation must be pinned where it lives. Deleting the
// ValidateCell call used to be a green change, because this package had no test
// files: the mutation table's "a bad RECONCILER_CELL boots" row mutated the
// VALIDATOR (covered by tenancy's tests) while the parenthetical named the
// WIRING (covered by nothing). Two representations of one property.
func TestBootRefusesACellIdThatCannotBeALabelValue(t *testing.T) {
	valid := map[string]string{
		"CONTROL_PLANE_URL": "http://cp",
		"RECONCILER_SECRET": "s3cret",
	}
	env := func(extra map[string]string) func(string) string {
		m := map[string]string{}
		for k, v := range valid {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return func(k string) string { return m[k] }
	}

	for _, bad := range []string{"cell_0", "Cell-0", "cell 0", "-cell0", strings.Repeat("c", 64)} {
		if _, _, _, err := bootConfig(env(map[string]string{"RECONCILER_CELL": bad})); err == nil {
			t.Errorf("boot accepted RECONCILER_CELL=%q — every converge on the cell would then "+
				"fail with no writeback", bad)
		} else if !strings.Contains(err.Error(), "RECONCILER_CELL") {
			t.Errorf("the error must name the variable an operator has to fix: %v", err)
		}
	}

	// Positive control, and the default: an unset RECONCILER_CELL must still boot.
	for _, good := range []string{"", "cell-0", "cell-7"} {
		extra := map[string]string{}
		if good != "" {
			extra["RECONCILER_CELL"] = good
		}
		cell, base, token, err := bootConfig(env(extra))
		if err != nil {
			t.Fatalf("boot refused a legitimate cell %q: %v", good, err)
		}
		want := good
		if want == "" {
			want = "cell-0"
		}
		if cell != want || base != "http://cp" || token != "s3cret" {
			t.Fatalf("bootConfig returned (%q,%q,%q), want (%q,http://cp,s3cret)", cell, base, token, want)
		}
	}

	// The other required variables still fail closed.
	for _, missing := range []string{"CONTROL_PLANE_URL", "RECONCILER_SECRET"} {
		if _, _, _, err := bootConfig(env(map[string]string{missing: ""})); err == nil {
			t.Errorf("boot accepted an empty %s", missing)
		}
	}
}
