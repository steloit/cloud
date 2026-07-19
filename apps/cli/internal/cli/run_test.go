package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("STELOIT_CONFIG", filepath.Join(dir, "config.json"))
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRoutingAndExitCodes(t *testing.T) {
	isolate(t)
	// no args → usage, exit 2
	code, _, _ := runCLI(t)
	if code != ExitUsage {
		t.Fatalf("bare invocation: %d", code)
	}
	// unknown noun → usage error naming the command
	code, _, errOut := runCLI(t, "frobnicate", "now")
	if code != ExitUsage || !strings.Contains(errOut, "frobnicate") {
		t.Fatalf("unknown: %d %s", code, errOut)
	}
	// version works in all three modes
	code, out, _ := runCLI(t, "version")
	if code != ExitOK || !strings.Contains(out, "steloit dev") {
		t.Fatalf("version: %d %q", code, out)
	}
	code, out, _ = runCLI(t, "version", "--json")
	if code != ExitOK || !strings.Contains(out, `"version"`) {
		t.Fatalf("version --json: %d %q", code, out)
	}
	// --help exits 0
	if code, _, _ = runCLI(t, "--help"); code != ExitOK {
		t.Fatalf("help: %d", code)
	}
	// flag needing a value
	code, _, errOut = runCLI(t, "version", "--project")
	if code != ExitUsage || !strings.Contains(errOut, "needs a value") {
		t.Fatalf("dangling flag: %d %s", code, errOut)
	}
}

func TestInitWritesRepoLink(t *testing.T) {
	dir := isolate(t)
	code, out, _ := runCLI(t, "init", "--project", "prj_abc", "--env", "staging")
	if code != ExitOK || !strings.Contains(out, "prj_abc") || !strings.Contains(out, "staging") {
		t.Fatalf("init: %d %q", code, out)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".steloit"))
	if err != nil {
		t.Fatal(err)
	}
	var link RepoLink
	if err := json.Unmarshal(raw, &link); err != nil || link.Project != "prj_abc" || link.Env != "staging" {
		t.Fatalf("link: %+v %v", link, err)
	}
	// missing project → usage
	if code, _, _ := runCLI(t, "init"); code != ExitUsage {
		t.Fatalf("init without project: %d", code)
	}
}

func TestContextResolutionLadder(t *testing.T) {
	dir := isolate(t)
	cfg := &Config{DefaultOrg: "org_prof", DefaultPrj: "prj_prof", DefaultEnv: "prof-env"}

	// profile only
	c := ResolveContext(map[string]string{}, dir, cfg)
	if c.Project != "prj_prof" || c.Source != "profile" {
		t.Fatalf("profile rung: %+v", c)
	}

	// repo link overrides profile; searched upward from a subdirectory
	if err := os.WriteFile(filepath.Join(dir, ".steloit"), []byte(`{"project":"prj_repo","env":"staging"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	c = ResolveContext(map[string]string{}, sub, cfg)
	if c.Project != "prj_repo" || c.Env != "staging" || c.Source != "repo" {
		t.Fatalf("repo rung: %+v", c)
	}
	// org still fills from profile (per-field ladder), source stays the most explicit
	if c.Org != "org_prof" {
		t.Fatalf("per-field fill: %+v", c)
	}

	// flags beat everything
	c = ResolveContext(map[string]string{"project": "prj_flag"}, dir, cfg)
	if c.Project != "prj_flag" || c.Source != "flags" {
		t.Fatalf("flags rung: %+v", c)
	}
	// env omitted = production forever (ADR-037) — via the echo
	c = ResolveContext(map[string]string{"project": "shop"}, t.TempDir(), &Config{})
	if got := c.Echo(); got != "shop/production ·" {
		t.Fatalf("echo: %q", got)
	}
}
