package testenv

// US-3.3f: THE TERRAFORM GATE IS ASSERTED BY RUNNING IT, NOT BY MATCHING IT.
//
// ciGates pins gates with substring needles, which works for "is this command
// present" and CANNOT work for "does this script fail when it should". Measured,
// twice: `fail=0` inserted between the loop and `exit $fail` passed a bare
// `exit $fail` needle; the needle was widened to the loop's tail through the
// exit, and `fail=0` moved ONE LINE UP — inside the loop — passed that too, with
// all three terraform suites failing and the step exiting 0. A substring wall can
// always be stepped around, because the property is about control flow.
//
// So this executes the real step, extracted from ci.yml, under GitHub's actual
// shell semantics (`bash --noprofile --norc -eo pipefail`), against a stub
// `terraform` whose exit code the test chooses. One test, and the whole insertion
// class dies with it.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// infraTestStep returns the `run:` body of the Terraform tests step.
func infraTestStep(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("cannot read ci.yml: %v — this check must not pass by failing to look", err)
	}
	var wf ciWorkflow
	if err := yaml.Unmarshal(b, &wf); err != nil {
		t.Fatalf("ci.yml does not parse: %v", err)
	}
	for _, st := range wf.Jobs["infra"].Steps {
		if strings.Contains(st.Run, "terraform -chdir=") && strings.Contains(st.Run, "test") {
			return st.Run
		}
	}
	t.Fatal("the infra job has no terraform test step — the gate this pins is gone")
	return ""
}

// runInfraStep executes the step in a throwaway git repo with a stub terraform.
func runInfraStep(t *testing.T, script string, terraformExit int, files map[string]string) (int, string) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bin := filepath.Join(dir, ".bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	// THE STUB MUST SUCCEED FOR `init` AND FAIL ONLY FOR `test`. An
	// exit-for-everything stub makes `init` fail, `-e` aborts the script before
	// the loop, and the assertion "the step exits nonzero" is then satisfied by
	// the wrong statement — measured: `fail=0` inside the loop SURVIVED this test
	// until the stub was split, because both runs exited 1 for the same
	// unrelated reason.
	stub := "#!/bin/sh\nfor a in \"$@\"; do\n  [ \"$a\" = test ] && exit " +
		map[bool]string{true: "1", false: "0"}[terraformExit != 0] + "\ndone\nexit 0\n"
	if err := os.WriteFile(filepath.Join(bin, "terraform"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	// A REAL git repo: a copied worktree shares the original index, so `git
	// ls-files` would report the source tree's files and every discovery
	// mutation would be a false pass.
	// No commit: `git ls-files` reads the INDEX and `git grep` searches tracked
	// working-tree files, so `init` + `add` is sufficient — and a commit inherits
	// the developer's global config, which fails outright under
	// `commit.gpgsign = true` (measured: exit 128, "gpg failed to sign").
	// GIT_CONFIG_* are pinned for the same reason.
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}} {
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	sh := filepath.Join(dir, ".step.sh")
	if err := os.WriteFile(sh, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "--noprofile", "--norc", "-eo", "pipefail", sh)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running the step: %v\n%s", err, out)
	}
	return code, string(out)
}

// A well-formed tree: one env and the module, each owning a real test.
func wellFormedTree() map[string]string {
	tf := "run \"enforces\" {\n  command = apply\n  assert {\n    condition = module.gke_cell.datapath_provider == \"ADVANCED_DATAPATH\"\n    error_message = \"x\"\n  }\n}\n"
	return map[string]string{
		"infra/envs/dev/main.tf":                              "module \"gke_cell\" {\n  source = \"../../modules/gke-cell\"\n}\n",
		"infra/envs/dev/tests/cell_enforces.tftest.hcl":       tf,
		"infra/modules/gke-cell/main.tf":                      "resource \"google_container_cluster\" \"cell\" {\n  datapath_provider = \"ADVANCED_DATAPATH\"\n}\n",
		"infra/modules/gke-cell/tests/enforcement.tftest.hcl": tf,
	}
}

func TestTheTerraformGateFailsWhenASuiteFails(t *testing.T) {
	script := infraTestStep(t)

	if code, out := runInfraStep(t, script, 0, wellFormedTree()); code != 0 {
		t.Fatalf("a well-formed tree with passing suites exited %d — the harness is wrong, "+
			"not the gate:\n%s", code, out)
	}
	code, out := runInfraStep(t, script, 1, wellFormedTree())
	if code == 0 {
		t.Errorf("EVERY terraform suite failed and the step exited 0 — the gate fails OPEN.\n%s", out)
	}
}

// The discovery guard, driven through the shapes that defeated the label-based
// version. Each is a semantics-preserving refactor that deleted an env's tests.
func TestTheTerraformGateCatchesAnUncoveredCell(t *testing.T) {
	script := infraTestStep(t)

	for name, mutate := range map[string]func(map[string]string){
		"the module block moved to cell.tf": func(f map[string]string) {
			f["infra/envs/dev/cell.tf"] = f["infra/envs/dev/main.tf"]
			delete(f, "infra/envs/dev/main.tf")
			delete(f, "infra/envs/dev/tests/cell_enforces.tftest.hcl")
		},
		// TWO envs, one relabelled. With a single env, relabelling empties the
		// discovered set and the `-z "$envs"` guard catches it — masking whether
		// discovery itself is label-based. Measured: with only the one-env case,
		// swapping the source regexp back to the label SURVIVED.
		"one of two envs relabelled": func(f map[string]string) {
			f["infra/envs/prod/main.tf"] = f["infra/envs/dev/main.tf"]
			f["infra/envs/prod/tests/cell_enforces.tftest.hcl"] =
				f["infra/envs/dev/tests/cell_enforces.tftest.hcl"]
			f["infra/envs/dev/main.tf"] = strings.Replace(f["infra/envs/dev/main.tf"],
				`module "gke_cell"`, `module "cell"`, 1)
			delete(f, "infra/envs/dev/tests/cell_enforces.tftest.hcl")
		},
		"a cell outside infra/envs": func(f map[string]string) {
			f["infra/cells/cell1/main.tf"] = f["infra/envs/dev/main.tf"]
		},
		// Same masking hazard as the relabel: with one env, moving the block to
		// cell.tf and deleting the tests empties the set. Keep a covered sibling.
		"one of two envs moved to cell.tf": func(f map[string]string) {
			f["infra/envs/prod/main.tf"] = f["infra/envs/dev/main.tf"]
			f["infra/envs/prod/tests/cell_enforces.tftest.hcl"] =
				f["infra/envs/dev/tests/cell_enforces.tftest.hcl"]
			f["infra/envs/dev/cell.tf"] = f["infra/envs/dev/main.tf"]
			delete(f, "infra/envs/dev/main.tf")
			delete(f, "infra/envs/dev/tests/cell_enforces.tftest.hcl")
		},
		"a new env with no tests": func(f map[string]string) {
			f["infra/envs/stage/main.tf"] = f["infra/envs/dev/main.tf"]
		},
		// Mentions datapath_provider but asserts nothing — so the run-block check
		// is the ONLY thing that can catch it. Without this shape, dropping that
		// check is masked by the datapath grep and survives.
		"the suite gutted to zero run blocks": func(f map[string]string) {
			f["infra/envs/dev/tests/cell_enforces.tftest.hcl"] =
				"# datapath_provider is named in a comment and asserted nowhere\nmock_provider \"google\" {}\n"
		},
		"the suite no longer mentions the datapath": func(f map[string]string) {
			f["infra/envs/dev/tests/cell_enforces.tftest.hcl"] =
				"run \"nothing\" {\n  command = apply\n}\n"
		},
	} {
		t.Run(name, func(t *testing.T) {
			files := wellFormedTree()
			mutate(files)
			code, out := runInfraStep(t, script, 0, files)
			if code == 0 {
				t.Errorf("an uncovered cell (%s) left the gate GREEN — an env can stop being "+
					"covered without anyone noticing.\n%s", name, out)
			}
		})
	}
}

// And it must fail closed when discovery finds nothing at all, rather than
// looping zero times and reaching a zero exit.
func TestTheTerraformGateFailsClosedOnEmptyDiscovery(t *testing.T) {
	script := infraTestStep(t)
	for name, files := range map[string]map[string]string{
		"no cell instantiated anywhere": {"infra/README.md": "nothing here\n"},
		"a cell but no test files": {
			"infra/envs/dev/main.tf": "module \"gke_cell\" {\n  source = \"../../modules/gke-cell\"\n}\n",
		},
		// The module and its own tests exist, but NO env instantiates a cell —
		// so `dirs` is non-empty and every expected dir is covered. Only the
		// empty-envs guard can catch this; without this shape, removing that
		// guard is masked by the other checks and survives.
		"the module is tested but no env instantiates a cell": {
			"infra/modules/gke-cell/main.tf":                      "resource \"google_container_cluster\" \"cell\" {\n  datapath_provider = \"ADVANCED_DATAPATH\"\n}\n",
			"infra/modules/gke-cell/tests/enforcement.tftest.hcl": "run \"enforces\" {\n  command = apply\n  assert {\n    condition = google_container_cluster.cell.datapath_provider == \"ADVANCED_DATAPATH\"\n    error_message = \"x\"\n  }\n}\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			code, out := runInfraStep(t, script, 0, files)
			if code == 0 {
				t.Errorf("discovery found nothing and the step exited 0 — the gate did not run "+
					"and said nothing.\n%s", out)
			}
		})
	}
}

// THE CELL'S CA OUTPUT IS PINNED HERE BECAUSE terraform test CANNOT PIN IT.
//
// `cluster_ca_certificate` feeds base64decode() in both envs' kubernetes and helm
// provider blocks. Under mock_provider `master_auth` is an empty list, so the
// expression yields null and the three tftest suites never evaluate it — measured
// four rounds running: pointing the splat at `client_certificate`, and replacing
// the whole value with "", are BOTH green across every suite and the CI step.
// That is the one line the entire one()/try() narrative in outputs.tf is about.
//
// A text pin is the honest instrument here: the property is "this output reads
// the CA from master_auth", and no plan-time assertion available to us evaluates
// it. Stated as a text pin rather than dressed up as behavioural coverage.
func TestTheCellCACertificateOutputStillReadsMasterAuth(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot, "infra", "modules", "gke-cell", "outputs.tf"))
	if err != nil {
		t.Fatalf("cannot read the cell module's outputs: %v", err)
	}
	// STRIP COMMENTS FIRST. A `strings.Contains` over the raw file cannot tell
	// code from a corpse — wiring_test.go says exactly that, 175 lines up, about
	// the sibling gate — and measured: parking the old expression in a `# was:`
	// comment and setting `value = ""` passed this test, fmt, validate and all
	// three terraform suites. The guard had the bypass it exists to prevent.
	src := stripHCLComments(string(b))

	const want = `one(google_container_cluster.cell.master_auth[*].cluster_ca_certificate)`
	if !strings.Contains(src, want) {
		t.Errorf("cluster_ca_certificate is no longer %s.\nBoth envs base64decode() this into "+
			"their kubernetes and helm providers, and no terraform test can catch a change: "+
			"under mock_provider master_auth is empty, so the expression is never evaluated.",
			want)
	}
	// try() was removed as a finding — it swallowed every error in the
	// expression, including one()'s own "must be a list with zero or one
	// elements", and handed a silent null to two provider blocks.
	if strings.Contains(src, "try(one(google_container_cluster.cell.master_auth") {
		t.Error("try() is back around the CA output; it swallows one()'s own error and hands " +
			"a silent null to both envs' provider blocks")
	}
}

// stripHCLComments removes `#` and `//` line comments and /* */ blocks, so a
// text pin asserts what Terraform will EVALUATE rather than what the file
// happens to contain. It does not try to respect strings containing a `#`:
// outputs.tf has none, and a parser that is wrong in the permissive direction
// would reintroduce the hole. If that changes, use hclparse.
func stripHCLComments(src string) string {
	var out strings.Builder
	inBlock := false
	for _, line := range strings.Split(src, "\n") {
		if inBlock {
			if i := strings.Index(line, "*/"); i >= 0 {
				line, inBlock = line[i+2:], false
			} else {
				continue
			}
		}
		if i := strings.Index(line, "/*"); i >= 0 {
			if j := strings.Index(line[i:], "*/"); j >= 0 {
				line = line[:i] + line[i+j+2:]
			} else {
				line, inBlock = line[:i], true
			}
		}
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		out.WriteString(line)
		out.WriteString("\n")
	}
	return out.String()
}

// NOTHING MAY REINTRODUCE `kubernetes_manifest` (T1.4a).
//
// That resource resolves its GroupVersionKind against the live API at PLAN time.
// On a from-zero apply there is no API yet — the cluster is created by the same
// apply — and for a custom resource there is no CRD yet either, so the plan
// fails before anything is created:
//
//	Error: API did not recognize GroupVersionKind from manifest
//	       (CRD may not be installed)
//
// It is an acknowledged, still-open limitation of hashicorp/kubernetes (#1367,
// #2597) and `depends_on` cannot fix it: validation runs before dependency
// resolution. One such resource anywhere in `infra/` puts the whole cell back on
// the `-target` bootstrap that T1.4a removed — and it would look completely
// reasonable in review, because the failure only appears on a from-zero apply
// that nobody runs by hand.
//
// A text guard, and it says so: the property is "this resource type is not used
// here", which no terraform test can assert about a file that would fail to plan.
func TestInfraNeverUsesKubernetesManifest(t *testing.T) {
	root := filepath.Join(repoRoot, "infra")
	var offenders []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".tf") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(b), "\n") {
			// The declaration only — the explanatory comments in cnpg/main.tf
			// name the type deliberately and must stay readable.
			if strings.HasPrefix(strings.TrimSpace(line), `resource "kubernetes_manifest"`) {
				rel, _ := filepath.Rel(repoRoot, path)
				offenders = append(offenders, fmt.Sprintf("%s:%d", rel, i+1))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking infra/: %v — this check must not pass by failing to look", err)
	}
	if len(offenders) > 0 {
		t.Errorf("kubernetes_manifest is back at %v.\nIt needs the Kubernetes API at PLAN time, so a "+
			"from-zero `terraform apply` cannot plan it — the cell returns to the -target bootstrap "+
			"T1.4a removed. Use kubectl_manifest (alekc/kubectl), which defers to apply time.",
			offenders)
	}
	// ...and the check must be looking at something.
	if n := countTF(t, root); n < 5 {
		t.Fatalf("only %d .tf files scanned under infra/ — the walk is not finding the tree", n)
	}
}

func countTF(t *testing.T, root string) int {
	t.Helper()
	n := 0
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".tf") {
			n++
		}
		return nil
	})
	return n
}

// EVERY `provider "kubectl"` MUST SET lazy_load (T1.4a).
//
// alekc/kubectl CONFIGURES at plan time and errors when host/CA are still
// unknown — which is exactly the from-zero case T1.4a exists to fix. Measured
// with the provider block copied verbatim and its host taken from a
// not-yet-applied resource:
//
//	without lazy_load: Plan: 1 to add, then
//	  Error: invalid provider configuration: default cluster has no server defined
//	with lazy_load:    Plan: 2 to add, clean
//
// hashicorp/kubernetes and helm tolerate an unconfigured client at plan; this
// one does not. So removing this one line reintroduces the -target bootstrap in
// a different disguise, and it would read as tidying.
//
// NOTHING ELSE CAN SEE IT. `terraform validate` never configures providers, and
// the env `terraform test` suites mock this provider wholesale — which is what
// makes them pass. This is the only instrument left, so it is a text guard and
// says so.
func TestTheKubectlProviderLoadsLazily(t *testing.T) {
	root := filepath.Join(repoRoot, "infra")
	blocks := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".tf") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		src := string(b)
		i := strings.Index(src, `provider "kubectl" {`)
		for i >= 0 {
			depth, j := 0, i
			for ; j < len(src); j++ {
				if src[j] == '{' {
					depth++
				} else if src[j] == '}' {
					if depth--; depth == 0 {
						break
					}
				}
			}
			body := src[i:min(j+1, len(src))]
			blocks++
			rel, _ := filepath.Rel(repoRoot, path)
			if !strings.Contains(body, "lazy_load") || !strings.Contains(body, "lazy_load = true") {
				t.Errorf("%s: the kubectl provider does not set `lazy_load = true`. It configures at "+
					"PLAN time, so a from-zero apply fails with \"default cluster has no server "+
					"defined\" before anything is created — the -target bootstrap T1.4a removed, "+
					"back in a different disguise.", rel)
			}
			if !strings.Contains(body, "apply_retry_count") {
				t.Errorf("%s: the kubectl provider does not set `apply_retry_count`. The 2.4.1 "+
					"default is a SINGLE attempt, and the CNPG Cluster is applied moments after the "+
					"operator's chart returns — a transient CRD-registration window then fails the "+
					"whole apply.", rel)
			}
			next := strings.Index(src[j:], `provider "kubectl" {`)
			if next < 0 {
				break
			}
			i = j + next
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking infra/: %v — this check must not pass by failing to look", err)
	}
	// Every env that instantiates the cell configures this provider; if the walk
	// finds none, the check is asserting nothing.
	if blocks < 2 {
		t.Fatalf("found %d `provider \"kubectl\"` blocks under infra/, want at least 2 (dev, cell0)", blocks)
	}
}
