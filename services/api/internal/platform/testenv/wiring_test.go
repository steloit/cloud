package testenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot is four levels up from services/api/internal/platform/testenv.
const repoRoot = "../../../../.."

// The Go constant and ci.yml are coupled by NOTHING BUT STRING EQUALITY, so
// this pins them together.
//
// Found by mutation: renaming RequireContainersVar to "STELOIT_CONTAINERS_REQUIRED"
// left `go build`, `go vet` and `gofmt -l` all clean, and every container suite
// went straight back to printing `ok` with the runtime broken. A one-line tidy-up
// silently restores the exact defect this package exists to remove, and CI stays
// green reporting it.
//
// ci.yml records this same lesson about ITSELF twenty lines above the env block:
// the gofmt module list was hardcoded twice and had to become
// `git ls-files '*/go.mod'` because "a fifth module would silently get no gofmt
// ... and nothing would say so". This is that shape, in that file, again.
func TestCIWorkflowArmsTheContainerGate(t *testing.T) {
	path := filepath.Join(repoRoot, ".github", "workflows", "ci.yml")
	b, err := os.ReadFile(path)
	if err != nil {
		// t.Fatal, not t.Skip: a missing file must not read as "clean". ci.yml
		// itself carries that lesson for the authority-paths step, where a failing
		// command yielded an empty changed-set that reported "untouched".
		t.Fatalf("cannot read %s: %v — this check must not pass by failing to look", path, err)
	}
	want := RequireContainersVar + `: "1"`
	if !strings.Contains(string(b), want) {
		t.Fatalf("ci.yml does not contain %q — it no longer arms the container gate, "+
			"so every container-backed suite is back to skipping green in CI. "+
			"If the constant was renamed, rename it in ci.yml too.", want)
	}
}

// Required's semantics, pinned separately from the wiring: the gate is armed by
// PRESENCE, not by value.
func TestRequiredReadsTheVar(t *testing.T) {
	t.Setenv(RequireContainersVar, "1")
	if !Required() {
		t.Fatal("the var is set but Required() is false — the gate is disarmed")
	}
	t.Setenv(RequireContainersVar, "")
	if Required() {
		t.Fatal("the var is empty but Required() is true — local runs would fail instead of skipping")
	}
}

// Every file that starts a testcontainer must route its runtime check through
// SkipOrFail. DERIVED from the tree, never a list of the three that exist today
// — a list is precisely what lets the fourth one, added next month, bypass the
// gate invisibly.
//
// Found by mutation: reverting ONE helper to the old t.Skipf and dropping the
// import left gofmt, vet, the build and validate.mjs all clean, and that package
// printed `ok` with the gate fully armed and the runtime broken.
func TestEveryTestcontainersUserRoutesThroughTestenv(t *testing.T) {
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if n := d.Name(); n == ".git" || n == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		body := string(src)
		// Exclude THIS package: wiring_test.go contains the very strings it
		// greps for, so it matches itself. That inflates the count and — worse —
		// keeps the vacuity guard below satisfied even if every real caller were
		// deleted, which is the exact class of lie this package exists to stop.
		if strings.Contains(filepath.ToSlash(path), "/internal/platform/testenv/") {
			return nil
		}
		if !strings.Contains(body, "testcontainers-go") {
			return nil
		}
		checked++
		rel, _ := filepath.Rel(root, path)
		if !strings.Contains(body, "platform/testenv") {
			t.Errorf("%s starts testcontainers but does not import platform/testenv — "+
				"its runtime check cannot be routing through SkipOrFail, so this suite "+
				"skips green in CI", rel)
			return nil
		}
		if strings.Contains(body, "t.Skipf(\"container runtime") || strings.Contains(body, "t.Skip(\"container runtime") {
			t.Errorf("%s still calls t.Skip directly for the runtime check — "+
				"call testenv.SkipOrFail so the decision stays in one place", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	// A walk that matched nothing would pass vacuously, which is the same class
	// of lie this whole package is about.
	if checked < 3 {
		t.Fatalf("found only %d files importing testcontainers-go — there are at least three "+
			"(reconcile, identity, platform/db), so this check is not looking where it thinks it is", checked)
	}
	t.Logf("checked %d testcontainers-using files", checked)
}
