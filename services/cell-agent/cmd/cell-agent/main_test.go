package main

import (
	"bytes"
	"context"
	"errors"
	"github.com/steloit/cloud/services/cell-agent/internal/kube"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
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

}

// run's SUCCESS path, which the error cases above cannot reach. Four of run's
// decisions were unpinned survivors: the ACK fallback renderer, the
// POLL_INTERVAL_SECONDS parse, the in-cluster credential requirement, and the
// signal context. An earlier comment here claimed "the ACK branch is covered by
// the positive control in bootConfig's test" — bootConfig's test never calls
// run(), so that annotation was false. Two of the four are pinned here; the
// in-cluster branch needs a cluster and is recorded in the Outcome instead.
func TestRunTakesTheAckBranchAndHonoursThePollInterval(t *testing.T) {
	// Hermetic: kube.NewInCluster reads these directly, so on a pod-based runner
	// run() would take the in-cluster branch and fail on GSA/WAL instead of
	// reaching ACK. Safe polarity either way (a false failure, never a false
	// pass), but a CI move to self-hosted pods should not break this.
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	env := map[string]string{
		"RECONCILER_CELL":       "cell-7",
		"CONTROL_PLANE_URL":     "http://cp",
		"RECONCILER_SECRET":     "s3cret",
		"POLL_INTERVAL_SECONDS": "3",
	}
	if err := run(ctx, func(k string) string { return env[k] }, log); err != nil {
		t.Fatalf("run failed on a valid configuration: %v", err)
	}
	out := buf.String()
	// Outside a cluster kube.NewInCluster fails, so run must fall back to ACK —
	// and SAY SO. A silent fallback looks like a working agent that provisions
	// nothing, which is the whole reason the branch logs at Warn.
	if !strings.Contains(out, "NOTHING is provisioned") {
		t.Fatalf("run did not announce the ACK fallback: %s", out)
	}
	// POLL_INTERVAL_SECONDS must reach the loop, not be parsed and dropped.
	if !strings.Contains(out, "3s") {
		t.Fatalf("POLL_INTERVAL_SECONDS=3 did not reach the interval: %s", out)
	}
	if !strings.Contains(out, "cell-7") {
		t.Fatalf("the validated cell did not reach the agent: %s", out)
	}
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
	// A DEADLINE, because this test's own failure mode is the one it exists to
	// catch. Without it, any mutation that lets a bad config through leaves the
	// child booting into a.Run with a real signal context, and CombinedOutput
	// blocks until Go's 10m test timeout at ~1% CPU — mutation 33's failure mode,
	// moved from the test that was fixed into the test added to replace it.
	// WaitDelay bounds the wait for the child's pipes after the context fires.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestMainExitsNonZeroWhenRunFails")
	cmd.WaitDelay = 5 * time.Second
	cmd.Env = append(os.Environ(),
		"CELL_AGENT_MAIN_UNDER_TEST=1",
		"RECONCILER_CELL=cell_0", // not an RFC1123 label — bootConfig must refuse it
		"CONTROL_PLANE_URL=http://cp",
		"RECONCILER_SECRET=s3cret",
	)
	out, err := cmd.CombinedOutput()

	if ctx.Err() != nil {
		t.Fatalf("main did not exit within the deadline — it booted past validation and blocked "+
			"in a.Run. output=%s", out)
	}
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

// SIGTERM must become a GRACEFUL stop, and nothing pinned that in either
// direction. Four mutations were green: deleting signal.NotifyContext and passing
// the parent straight to a.Run, swapping NotifyContext for context.WithCancel,
// dropping `defer stop()`, and hoisting the whole thing into main(). A pod that
// no longer converts SIGTERM into a graceful stop is killed mid-converge, and CI
// would not have noticed.
//
// The hoist matters specifically: with the signal context created before boot, a
// SIGTERM arriving during boot is absorbed, boot completes, a.Run returns at once
// and the process exits 0 — which an orchestrator reads as "completed" rather
// than "terminated".
func TestMainStopsGracefullyOnSIGTERM(t *testing.T) {
	if os.Getenv("CELL_AGENT_SIGTERM_UNDER_TEST") == "1" {
		main()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestMainStopsGracefullyOnSIGTERM")
	cmd.WaitDelay = 5 * time.Second
	cmd.Env = append(os.Environ(),
		// Hermetic, exactly as the ACK test above. cmd.Env inherits os.Environ(),
		// so on a pod-based runner the child takes the in-cluster branch and dies
		// on CELL_GSA_EMAIL instead of reaching the loop — measured as a 20s
		// failure. One guard, two call sites; the sibling had it and this did not.
		"KUBERNETES_SERVICE_HOST=",
		"KUBERNETES_SERVICE_PORT=",
		"CELL_AGENT_SIGTERM_UNDER_TEST=1",
		"RECONCILER_CELL=cell-0",
		"CONTROL_PLANE_URL=http://127.0.0.1:1", // refused fast; the agent logs and keeps polling
		"RECONCILER_SECRET=s3cret",
		"POLL_INTERVAL_SECONDS=1",
	)
	// A LOCKED buffer. exec writes the child's output from its own goroutine while
	// this test polls it for "cell-agent starting", so a plain bytes.Buffer is a
	// data race — caught by -race under -count=2, which is why that gate exists.
	out := &syncBuffer{}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// The early-fatal paths below return without Wait(), leaking the copy
	// goroutine and its pipe fds. cancel() kills the process; this reaps it.
	waited := false
	defer func() {
		if !waited {
			_ = cmd.Wait()
		}
	}()

	// Wait until the agent is past boot and actually in the loop, so the signal
	// exercises a.Run's context rather than the boot path.
	deadline := time.Now().Add(20 * time.Second)
	for !strings.Contains(out.String(), "cell-agent starting") {
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			t.Fatalf("agent never reached the loop: %s", out.String())
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	err := cmd.Wait()
	waited = true

	if ctx.Err() != nil {
		t.Fatalf("SIGTERM did not stop the agent within the deadline — the signal is not "+
			"wired to a.Run's context. output=%s", out.String())
	}
	if err != nil {
		t.Fatalf("SIGTERM must be a graceful stop (exit 0), got %v. output=%s", err, out.String())
	}
	if !strings.Contains(out.String(), "cell-agent stopped") {
		t.Fatalf("the agent exited without logging a clean stop: %s", out.String())
	}
}

// syncBuffer is a bytes.Buffer that may be read while a child process writes to
// it. Only what this file needs: Write and String.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// THE IN-CLUSTER ARM — the only arm that runs on a cell, and it was dead code to
// this suite: substituting a panic for render.NewCNPGRenderer was a GREEN change,
// and the ACK test above explicitly forces the other branch, so 100% of run()'s
// renderer-selection coverage sat on the path that never runs in production.
//
// It was excused as "needs a real cluster". It does not: kube.NewInCluster reads
// two env vars and two files, so a temp SA dir and t.Setenv reach it in
// milliseconds. The excuse was the finding.
func TestRunTakesTheInClusterBranchAndRequiresGSAandWAL(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte("tok"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A syntactically valid CA, or the client refuses for the wrong reason.
	if err := os.WriteFile(filepath.Join(dir, "ca.crt"), []byte(testCAPEM), 0o600); err != nil {
		t.Fatal(err)
	}
	kube.SetSADirForTest(t, dir)
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "443")

	base := map[string]string{
		"RECONCILER_CELL":   "cell-0",
		"CONTROL_PLANE_URL": "http://cp",
		"RECONCILER_SECRET": "s3cret",
	}
	getenv := func(extra map[string]string) func(string) string {
		m := map[string]string{}
		for k, v := range base {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return func(k string) string { return m[k] }
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// WITHOUT the credentials the agent must refuse to boot. What that guard
	// protects: with it gone the renderer is built with an empty GSA and bucket,
	// rendering `iam.gke.io/gcp-service-account:` (empty) and
	// `destinationPath: gs:///<name>` — no workload identity, no WAL archiving,
	// therefore no base backup and NO PITR (ADR-0007 F3) — while the cluster
	// still reaches ready and reports success. Silent loss of restorability.
	for _, missing := range []map[string]string{
		{"CELL_WAL_BUCKET": "b"},                           // no GSA
		{"CELL_GSA_EMAIL": "sa@p.iam.gserviceaccount.com"}, // no bucket
		{}, // neither
	} {
		err := run(ctx, getenv(missing), slog.New(slog.NewTextHandler(io.Discard, nil)))
		if err == nil {
			t.Fatalf("in-cluster boot accepted %v — an agent with no WAL bucket provisions "+
				"clusters that reach ready and can never be restored", missing)
		}
		if !strings.Contains(err.Error(), "CELL_GSA_EMAIL") || !strings.Contains(err.Error(), "CELL_WAL_BUCKET") {
			t.Fatalf("the error must name both variables: %v", err)
		}
	}

	// WITH them, the in-cluster branch is taken — and says so. The ACK warning
	// must NOT appear, or this test would pass on the fallback path.
	var buf bytes.Buffer
	err := run(ctx, getenv(map[string]string{
		"CELL_GSA_EMAIL":  "sa@p.iam.gserviceaccount.com",
		"CELL_WAL_BUCKET": "steloit-dev-wal-customer",
	}), slog.New(slog.NewTextHandler(&buf, nil)))
	if err != nil {
		t.Fatalf("in-cluster boot failed with credentials present: %v", err)
	}
	if !strings.Contains(buf.String(), "in-cluster, real apply") {
		t.Fatalf("run did not take the in-cluster branch: %s", buf.String())
	}
	if strings.Contains(buf.String(), "NOTHING is provisioned") {
		t.Fatalf("run fell back to ACK while in-cluster: %s", buf.String())
	}
}

// testCAPEM is a throwaway self-signed cert: NewInCluster requires the CA to
// parse, and a bogus string would make the test pass for the wrong reason.
const testCAPEM = `-----BEGIN CERTIFICATE-----
MIIDBTCCAe2gAwIBAgIUGQIaQJw8jwr4dUf7XqH7oz7l3AwwDQYJKoZIhvcNAQEL
BQAwEjEQMA4GA1UEAwwHdGVzdC1jYTAeFw0yNjA4MjMwODMxMjBaFw0yNjA4MjQw
ODMxMjBaMBIxEDAOBgNVBAMMB3Rlc3QtY2EwggEiMA0GCSqGSIb3DQEBAQUAA4IB
DwAwggEKAoIBAQC/V4JAZNm9d9QTTjFVBBhZ8I9ahQulJmp7w0GXJXIkSdIeYaIg
y2LvIt9gVkp6gyLC6pYiMAONsecKNIwxN9j3OvZ9eAM3kxpwXly3gnmqmbcpSVLm
H4UNpMmKry7ET1SqlWFh8tCaXNb//3xaUj+iqaLL+kiMUuM8XVqjPkzFSGRQR7nk
UKWZ49Jss6OKh9bc+z4X+6JNQ9dHbByXfzP1UEArxT4uhZT4AHKhvzY9vH7D7Cea
aWmpvOHc6ufIUYBTAKCbAiv0RT+Aurm0tYSLijpL2eXBuWToAd2TAwnzWicxdo2e
e97pfar8qzFC1JQMDCYN94kLLmxUT6hzsL9DAgMBAAGjUzBRMB0GA1UdDgQWBBRm
gVBI+9FPNpemgPOK9n0p6tn7kDAfBgNVHSMEGDAWgBRmgVBI+9FPNpemgPOK9n0p
6tn7kDAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQBGyXvVGiCX
9GCzL9iWcXjUP2hNSAjo3+bBphXQkVN7d2PCfKrXPdnu4SxM/GB1v2dSRlktGm1/
LH1eeJgciBM0VvIQ5mQJWNBH2LAbSeGptjooDsktnHif3dDyf8SuvFwIDJIy1fv4
5/dPukUyGnNWQhpkSfRWdchbd5EeeHfb2+UHPsZpJ0pBGeMZ9VlxMERcArxM9W96
q4tjhJdddSDnOcNdKwQ1l3wrus6WFKr5kXch5FpPSesQajAofb0RzGGjpsfvxkow
JZgRQCMK5Xlk4KwYZES83QCB8vrRjXZrLVsGFyiniP8RUVpPp0nti5urky6HzVsb
mwuNuMvAyUvC
-----END CERTIFICATE-----`
