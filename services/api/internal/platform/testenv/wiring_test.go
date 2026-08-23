package testenv

import (
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
		If              string `yaml:"if"`
		ContinueOnError bool   `yaml:"continue-on-error"`
		Steps           []struct {
			Name            string            `yaml:"name"`
			If              string            `yaml:"if"`
			ContinueOnError bool              `yaml:"continue-on-error"`
			Run             string            `yaml:"run"`
			Env             map[string]string `yaml:"env"`
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
	{task: "O13", job: "go", runContains: "go test -count=1 -race -timeout 30m ./...",
		why: "the detector had never run in CI; O14 reached the base branch and sat there a month. " +
			"The -count=1 is part of the needle deliberately (US-3.3a): the test cache's " +
			"boundary is the MODULE ROOT, so a fixture outside it — ci.yml five directories up, " +
			"which THIS test reads — is edited without invalidating anything and comes back " +
			"`ok (cached)`. Measured live: disarming a gate is cached-green without the flag and " +
			"FAIL with it, and setup-go restores GOCACHE across commits. It fails OPEN. Note it " +
			"must be pinned through EXECUTABLE text: runContains matches against " +
			"stripShellComments, so a gate can never be armed by a comment"},
	{task: "O13", job: "go", runContains: `out="$(gofmt -l "$m" 2>&1)"`,
		why: "the exit-status/stderr capture: gofmt reports CLEAN on stdout for a file that does not parse"},
	{task: "§17", job: "go", runContains: "make gen-go",
		why: "the oapi contract generator"},
	{task: "§17", job: "go", runContains: "make gen-canon",
		why: "the canon fixtures copy"},
	{task: "T3.4", job: "go", runContains: "cd ../cell-agent && go build ./... && go vet ./... && go test -count=1 -race",
		why: "the cell-agent module is otherwise unbuilt and untested in CI"},
	{task: "E5", job: "go", runContains: "apps/cli && go build ./... && go vet ./... && go test -count=1 -race",
		why: "the CLI module is otherwise unbuilt and untested in CI"},
	{task: "O23", job: "go", envKey: "STELOIT_REQUIRE_CONTAINERS",
		why: "without it a missing container runtime is a SKIP and the job goes green having run nothing"},
	// --- the validate job -------------------------------------------------
	{task: "O6f", job: "validate", runContains: "FOUNDER-RATIFIED",
		why: "authority-paths: docs/product/00-sources/** and decisions.md are human-decision-only (CLAUDE.md hard rule)"},
	// The needle above is a PRESENCE check and cannot see a semantic neutering:
	// flipping this step's final `exit 1` to `exit 0` leaves every marker string
	// intact and fails the gate OPEN. So the failure itself is pinned, by matching
	// the last message together with the exit that follows it.
	{task: "O6f", job: "validate",
		runContains: "on the PR body's first line.\"\nexit 1",
		why:         "authority-paths must FAIL on an unratified change; exit 1 -> exit 0 makes it report and pass"},
	{task: "spec-sync", job: "validate", runContains: "node scripts/spec-sync/validate.mjs",
		why: "task frontmatter, deps and caps are otherwise unchecked"},
	{task: "O6f", job: "validate", runContains: "protect-authority.test.sh",
		why: "the authority-path hook's own regression tests"},
	{task: "Q3/§17", job: "validate", runContains: "git diff --cached --exit-code -- apps packages docs",
		why: "generated client drift must fail the build"},
	{task: "console", job: "validate", runContains: "pnpm --filter console lint",
		why: "console lint"},
	{task: "console", job: "validate", runContains: "pnpm --filter console typecheck",
		why: "console typecheck"},
	{task: "console", job: "validate", runContains: "pnpm --filter console test",
		why: "the console suite"},
	{task: "console", job: "validate", runContains: "pnpm --filter console build",
		why: "the console build"},
	{task: "SDK", job: "validate", runContains: "pnpm --filter @steloit/sdk test",
		why: "the generated SDK's tests"},
	{task: "ADR-026", job: "validate", runContains: "pnpm --filter @steloit/canon test",
		why: "canon invariants — demo data comes from 19-canon only"},
	// --- the infra job ----------------------------------------------------
	{task: "infra", job: "infra", runContains: "terraform fmt -check -recursive infra",
		why: "terraform formatting"},
	{task: "infra", job: "infra", runContains: "terraform -chdir=infra/envs/dev validate",
		why: "a duplicate module call had this job red and hid every later step (O22)"},
	{task: "infra", job: "infra", runContains: "terraform -chdir=infra/envs/cell0 validate",
		why: "cell0 is the second env and was never validated while infra was red (O22)"},
	{task: "T1.2", job: "infra", runContains: "infra/k8s/**/*.yaml",
		why: "every k8s manifest must parse"},
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
	// gate present and never executed.
	//
	// Structured lookup ONLY. An earlier version added a `bytes.Contains(b,
	// "pull_request:")` fallback "in case YAML 1.1 folds the bare key `on` to
	// boolean true" — that is false for yaml.v3, which uses the YAML 1.2 core
	// schema, so the fallback could never help. It could only HURT: it fires
	// exactly when the trigger is genuinely gone, and then any literal
	// `pull_request:` re-arms it. `on: workflow_dispatch:  # was: pull_request:`
	// passed with it.
	if _, ok := wf.On["pull_request"]; !ok {
		t.Errorf("ci.yml no longer runs on pull_request (on: %v) — every gate below is armed and never fires", wf.On)
	}

	for _, g := range ciGates {
		job, present := wf.Jobs[g.job]
		if !present {
			t.Errorf("ci.yml has no job %q, which owns the %s gate (%s)", g.job, g.task, g.why)
			continue
		}
		// A gate in a job guarded by `if:` can be switched off without touching
		// the gate.
		if job.If != "" {
			t.Errorf("job %q carries `if: %s` — every gate it owns can be disabled without touching the gate", g.job, job.If)
		}
		// ...and `continue-on-error: true` lets the gate RUN, FAIL, and the job
		// report success. One line, every gate in the job neutered.
		if job.ContinueOnError {
			t.Errorf("job %q sets continue-on-error — its gates can fail and the job still passes", g.job)
		}
		found := false
		for _, st := range job.Steps {
			hit := false
			if g.runContains != "" && strings.Contains(stripShellComments(st.Run), g.runContains) {
				hit = true
			}
			if g.envKey != "" {
				// Present AND non-empty: `STELOIT_REQUIRE_CONTAINERS: ""`
				// disarms the gate while satisfying any presence check.
				if v, has := st.Env[g.envKey]; has && v != "" {
					hit = true
				}
			}
			if !hit {
				continue
			}
			// The step that CARRIES the gate must itself run and be able to fail.
			// A blanket ban on step-level `if:` is wrong — authority-paths
			// legitimately carries `if: github.event_name == 'pull_request'` — so
			// only a constant-false is rejected.
			if isConstFalse(st.If) {
				t.Errorf("the step arming %q carries `if: %s` — the gate is present and never runs", g.task, st.If)
			}
			if st.ContinueOnError {
				t.Errorf("the step arming %q sets continue-on-error — the gate can fail and the job still passes", g.task)
			}
			found = true
			break
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

// stripShellComments removes `#` lines from inside a `run:` block scalar.
//
// This is where the comment hole MOVED rather than closing. A shell comment
// inside a run: scalar is DATA to the YAML parser, not a YAML comment, so it
// survives parsing and `strings.Contains(st.Run, needle)` matches it exactly as a
// whole-file text match did. Demonstrated: commenting out every executable line
// of the Generate step leaves ZERO commands and both needles satisfied — and the
// suite reported ok.
//
// Worth recording why the earlier "commented-out step -> RED" evidence was
// wrong: commenting out the step's YAML KEYS makes ci.yml unparseable, so that
// mutation hit t.Fatalf("does not parse") and the gate logic never ran. One
// representation died; the class stayed open.
func stripShellComments(run string) string {
	var b strings.Builder
	for _, line := range strings.Split(run, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// isConstFalse reports whether a GitHub Actions `if:` can never be true.
func isConstFalse(expr string) bool {
	switch strings.ToLower(strings.TrimSpace(strings.Trim(strings.TrimSpace(expr), "'\""))) {
	case "false", "0", "${{ false }}":
		return true
	}
	return false
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
