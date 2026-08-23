package testenv

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// repoRoot is five levels up from services/api/internal/platform/testenv.
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

// ciWorkflow is as much of ci.yml as the gate assertions need.
//
// PARSED, not grepped, and that distinction is the whole finding. A
// `strings.Contains` over the file text cannot tell code from a corpse: it
// matches a needle that survives only in a `#` comment, in a step that has been
// commented out, in a job guarded by `if: false`, or in a workflow whose
// triggers were removed. Every one of those was demonstrated against the first
// version of this test — including deleting `-race` from all three module lines
// while the three comments justifying it kept the needle alive. The
// better-documented a gate is, the more reliably a text match lies about it.
type ciWorkflow struct {
	On   map[string]any `yaml:"on"`
	Jobs map[string]struct {
		If    string `yaml:"if"`
		Steps []struct {
			Name string            `yaml:"name"`
			Run  string            `yaml:"run"`
			Env  map[string]string `yaml:"env"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

// ciGate is one gate: which job owns it, and how to recognise it in that job.
// Exactly one of runContains / envKey is set.
type ciGate struct {
	task, job, why string
	runContains    string // a substring of a step's `run:` script
	envKey         string // an env key that must be present AND non-empty
}

// ciGates is every gate this pipeline relies on, with the task that added it.
//
// A HAND-MAINTAINED LIST, deliberately. The other instances of this shape in
// this repo were fixed by DERIVING (`git ls-files '*/go.mod'`, a tree walk for
// testcontainers users) because a source of truth existed. There is none for
// "which gates ought to exist" — that is a decision, so this list IS the
// assertion rather than a duplicate of one.
var ciGates = []ciGate{
	// --- the go job -------------------------------------------------------
	{task: "O7", job: "go", runContains: "make gen-sql",
		why: "the drift gate must regenerate sqlc, or a .sql edit that never reached the generator ships a query nobody reviewed"},
	{task: "§16/§17", job: "go", runContains: "git diff --cached --exit-code -- services packages apps",
		why: "the generators are a no-op that looks busy without the diff that compares their output"},
	{task: "O13", job: "go", runContains: "gofmt -l",
		why: "go vet does not check formatting, so nothing else in this pipeline reports it"},
	{task: "O13", job: "go", runContains: "git ls-files '*/go.mod'",
		why: "the gofmt module list must stay DERIVED — a hardcoded list makes a fifth module invisible"},
	{task: "O13", job: "go", runContains: "go test -race -timeout 30m ./...",
		why: "the detector had never run in CI; O14 reached the base branch and sat there a month"},
	{task: "O23", job: "go", envKey: "STELOIT_REQUIRE_CONTAINERS",
		why: "without it a missing container runtime is a SKIP and the job goes green having run nothing"},
	// --- the validate job -------------------------------------------------
	{task: "spec-sync", job: "validate", runContains: "node scripts/spec-sync/validate.mjs",
		why: "task frontmatter, deps and caps are otherwise unchecked"},
	{task: "O6f", job: "validate", runContains: "protect-authority.test.sh",
		why: "the authority-path hook's own regression tests"},
	{task: "Q3/§17", job: "validate", runContains: "git diff --cached --exit-code -- apps packages docs",
		why: "generated client drift must fail the build"},
	{task: "console", job: "validate", runContains: "pnpm --filter console test",
		why: "the console suite"},
	// --- the infra job ----------------------------------------------------
	{task: "infra", job: "infra", runContains: "terraform fmt -check -recursive infra",
		why: "terraform formatting"},
	{task: "infra", job: "infra", runContains: "terraform -chdir=infra/envs/dev validate",
		why: "a duplicate module call had this job red and hid every later step (O22)"},
}

// Every gate must be armed AND in a job that actually runs.
func TestCIWorkflowArmsEveryGate(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("cannot read ci.yml: %v — this check must not pass by failing to look", err)
	}
	var wf ciWorkflow
	if err := yaml.Unmarshal(b, &wf); err != nil {
		t.Fatalf("ci.yml does not parse: %v", err)
	}

	// The workflow must still fire on a PR. Removing the triggers leaves every
	// gate textually present and never executed. YAML 1.1 folds the bare key
	// `on` to boolean true, so accept either spelling.
	triggers, ok := wf.On["pull_request"]
	if !ok {
		if m, isMap := wf.On["true"].(map[string]any); isMap {
			_, ok = m["pull_request"]
		}
	}
	_ = triggers
	if !ok && !bytes.Contains(b, []byte("pull_request:")) {
		t.Error("ci.yml no longer runs on pull_request — every gate below is armed and never fires")
	}

	for _, g := range ciGates {
		job, present := wf.Jobs[g.job]
		if !present {
			t.Errorf("ci.yml has no job %q, which owns the %s gate (%s)", g.job, g.task, g.why)
			continue
		}
		// A gate in a job guarded by `if:` can be switched off without touching
		// the gate. `if: false # temporarily disabled` was one of the surviving
		// mutations.
		if job.If != "" {
			t.Errorf("job %q carries `if: %s` — every gate it owns can be disabled without touching the gate", g.job, job.If)
		}
		found := false
		for _, st := range job.Steps {
			if g.runContains != "" && strings.Contains(st.Run, g.runContains) {
				found = true
				break
			}
			if g.envKey != "" {
				// Present AND non-empty: `STELOIT_REQUIRE_CONTAINERS: ""`
				// disarms the gate while satisfying any presence check.
				if v, has := st.Env[g.envKey]; has && v != "" {
					found = true
					break
				}
			}
		}
		if !found {
			what := g.runContains
			if what == "" {
				what = "env " + g.envKey + " (non-empty)"
			}
			t.Errorf("job %q no longer arms %q (added by %s)\n  why it is there: %s\n"+
				"  If this gate was deliberately removed, remove it from ciGates in the same commit "+
				"and say why in the PR — do not delete this assertion to make the build green.",
				g.job, what, g.task, g.why)
		}
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
