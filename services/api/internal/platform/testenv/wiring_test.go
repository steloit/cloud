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

// ciGates is every gate `ci.yml` must arm, with the task that added it.
//
// A HAND-MAINTAINED LIST, deliberately — and that is worth defending, because
// the two previous instances of this shape were fixed by DERIVING instead. The
// gofmt module list became `git ls-files '*/go.mod'`, and the testcontainers
// caller set became a walk of the tree, in both cases because a source of truth
// existed and a retyped copy could drift from it.
//
// There is no such source here. "Which gates ought to exist" is a decision, not
// a fact discoverable from the tree, so this list IS the assertion rather than a
// duplicate of one. What it must not become is a list that quietly stops
// matching: each entry carries the task that added it, so a failure explains why
// the gate is there rather than merely that a string went missing.
var ciGates = []struct{ task, needle, why string }{
	{"O7", "make gen-sql",
		"the drift gate must regenerate sqlc, or a .sql edit that never reached the generator ships a query nobody reviewed"},
	{"O13", "gofmt -l", // the loop body; the step name could be reworded harmlessly
		"go vet does not check formatting, so nothing else in this pipeline reports it"},
	{"O13", "git ls-files '*/go.mod'",
		"the gofmt module list must stay DERIVED — a hardcoded list makes a fifth module invisible"},
	{"O13", "-race",
		"the detector had never run in CI; O14 reached the base branch and sat there a month"},
	{"O13", "-timeout 30m",
		"Go's default is 10 min PER PACKAGE and internal/identity measures ~350s in CI"},
	{"O23", RequireContainersVar + `: "1"`,
		"without it a missing container runtime is a SKIP, and the job goes green having run nothing"},
}

// Every gate this repository relies on must actually be armed in ci.yml.
//
// Found by auditing the gates added in one session: FIVE were added and only ONE
// was pinned (the container gate, and only because a reviewer asked). Deleting
// `make gen-sql`, the whole gofmt step, and `-race -timeout 30m` from ci.yml left
// the file parsing and the suite reporting `ok` — three gates gone, nothing
// noticed. These gates exist because their absence let real defects through, so
// "removable by deleting four lines of YAML" is not an acceptable state for them.
func TestCIWorkflowArmsEveryGate(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("cannot read ci.yml: %v — this check must not pass by failing to look", err)
	}
	// COMMENTS STRIPPED, and this is the difference between a pin and theatre.
	//
	// Searching the whole file passes when a needle survives only in prose. Every
	// gate here is DOCUMENTED next to itself — `-race` appears in three comments,
	// `-timeout 30m` in two, `gofmt -l` in one — so deleting the executable line
	// and leaving the comment kept the first version of this test green with all
	// three gates gone from the run. Verified by doing exactly that.
	ci := stripYAMLComments(string(b))
	for _, g := range ciGates {
		if !strings.Contains(ci, g.needle) {
			t.Errorf("ci.yml no longer contains %q (added by %s)\n"+
				"  why it is there: %s\n"+
				"  If this gate was deliberately removed, remove it from ciGates in the same commit "+
				"and say why in the PR — do not delete this assertion to make the build green.",
				g.needle, g.task, g.why)
		}
	}
}

// stripYAMLComments removes whole-line `#` comments so a gate needle can only
// match something that actually runs.
//
// Whole-line only, deliberately: an inline `#` inside a shell `run:` block can be
// part of a command (a URL fragment, a printf), and dropping the tail of such a
// line could hide a real gate and produce a FALSE RED. Every comment in ci.yml
// that mentions a gate is a whole-line one, which is the case that matters.
func stripYAMLComments(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
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
