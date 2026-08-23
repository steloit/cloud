package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
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
	withEnv := func(extra map[string]string) func(string) string {
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
		if _, _, _, err := bootConfig(withEnv(map[string]string{"RECONCILER_CELL": bad})); err == nil {
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
		cell, base, token, err := bootConfig(withEnv(extra))
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
		if _, _, _, err := bootConfig(withEnv(map[string]string{missing: ""})); err == nil {
			t.Errorf("boot accepted an empty %s", missing)
		}
	}
}

// main() must HONOUR bootConfig's error. Extracting the config read pinned the
// validator and the call to it; replacing `if err != nil { os.Exit(1) }` with
// `_ = err` was still green, because nothing drove the path that decides what
// to do with the answer.
func TestRunRefusesToStartOnABadBootConfig(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	getenv := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}

	for name, m := range map[string]map[string]string{
		"bad cell":    {"RECONCILER_CELL": "cell_0", "CONTROL_PLANE_URL": "http://cp", "RECONCILER_SECRET": "s"},
		"no url":      {"RECONCILER_SECRET": "s"},
		"no secret":   {"CONTROL_PLANE_URL": "http://cp"},
		"nothing set": {},
	} {
		t.Run(name, func(t *testing.T) {
			// An ALREADY-CANCELLED context, so that a run() which wrongly gets
			// past validation returns instead of blocking in a.Run until the 10m
			// test timeout. The assertion is still "must error": a cancelled
			// context makes a.Run return nil, so a wrongly-accepted config fails
			// here loudly and immediately.
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := run(ctx, getenv(m), log); err == nil {
				t.Fatal("run started the agent on an unusable configuration")
			}
		})
	}

	// And in-cluster credentials are still required when a cluster is present.
	// Outside a cluster kube.NewInCluster fails, so run takes the ACK branch and
	// would block in a.Run — which is why this test drives only the error paths.
	// The ACK branch is covered by the positive control in bootConfig's test.
}

// main() must EXIT NON-ZERO when run() returns an error.
//
// Extracting bootConfig pinned the validator; extracting run pinned the call to
// it. main HONOURING the result is a third representation, and it stayed
// unpinned: replacing `if err := run(...); err != nil` with `; false` was a green
// change, and the agent would then fall off the end of main and exit 0 — a
// crash-looping pod that reports success, which is the shape an orchestrator
// treats as "completed" rather than "failed".
//
// main() cannot be called in-process (it calls os.Exit), so this re-executes the
// test binary as a subprocess, the standard idiom.
func TestMainExitsNonZeroWhenRunFails(t *testing.T) {
	if os.Getenv("CELL_AGENT_MAIN_UNDER_TEST") == "1" {
		main()
		return // unreachable if main exits as it must
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMainExitsNonZeroWhenRunFails")
	cmd.Env = append(os.Environ(),
		"CELL_AGENT_MAIN_UNDER_TEST=1",
		"RECONCILER_CELL=cell_0", // not an RFC1123 label — bootConfig must refuse it
		"CONTROL_PLANE_URL=http://cp",
		"RECONCILER_SECRET=s3cret",
	)
	out, err := cmd.CombinedOutput()

	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("main exited 0 on an unusable configuration — a crash-looping agent that "+
			"reports success. err=%v output=%s", err, out)
	}
	if code := ee.ExitCode(); code != 1 {
		t.Fatalf("main exited %d, want 1", code)
	}
	if !strings.Contains(string(out), "RECONCILER_CELL") {
		t.Fatalf("the failure must name the variable an operator has to fix: %s", out)
	}
}
