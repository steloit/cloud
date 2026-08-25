package testenv

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

// RequirePostgresPeer proves that the host port a container was published on is
// actually served by THAT container's Postgres, before any test uses it.
//
// WHY THIS EXISTS (O34). Three integration tests were each written off as flaky.
// One preserved its output:
//
//	migrate init: failed to open database: failed to connect to `user=app database=app`:
//	  [::1]:52381 (localhost): dial error: connect: connection refused
//	  127.0.0.1:52381 (localhost): failed to receive message: invalid body length:
//	    expected at most 1073741822, but got 1414811691
//
// 1414811691 is not a length. It is the ASCII bytes `TTP+` — a Postgres client
// reading someone else's response and taking four of its bytes as an int32
// message length. The client was talking to a different process.
//
// MEASURED, not inferred. A harness started 168 containers in waves of 12 and
// probed each published port with a Postgres SSLRequest: 3 were answered by
// something that is not Postgres (~1.8%) — twice with the bytes `HTTP/`, once
// with `01 00 00 00` on port 54167, which `lsof` showed belonging to macOS's
// `rapportd` (Continuity/AirDrop). Crucially, `portsReused` was 0: no port was
// ever handed to two of our containers, so this is NOT testcontainers reuse.
//
// THE MECHANISM. Under colima the Docker daemon lives in a VM. It picks a port
// inside the VM, and lima's hostagent then opens a matching host-side listener
// per published port — 54 `ssh -L` listeners were live during one run. But the
// host port is drawn from the same ephemeral range every other macOS process
// uses (`net.inet.ip.portrange` 49152-65535) and nothing RESERVES it on the
// host. When a host process already holds that number the forward loses, and a
// connection to 127.0.0.1:<port> reaches the squatter. That is also why the
// preserved error shows IPv6 refusing while IPv4 answers garbage: the squatter
// had bound IPv4 only.
//
// SCOPE. This is a developer-machine mechanism. CI runs on ubuntu-latest with
// native Docker, where dockerd bind()s the published port directly and therefore
// reserves it; the VM-forward race does not exist there.
//
// This is a DIAGNOSIS, not a retry. It does not reconnect, restart the container
// or paper over anything — it converts a failure that reads like a Postgres
// protocol bug into one that names the actual cause and the process holding the
// port. The structural fix is to stop allocating hundreds of host ports per run.
func RequirePostgresPeer(t *testing.T, connString string) {
	t.Helper()
	if err := CheckPostgresPeer(connString); err != nil {
		t.Fatal(err)
	}
}

// CheckPostgresPeer is RequirePostgresPeer's testable core. It is separate for
// one reason: a guard whose only entry point calls t.Fatal cannot be asserted to
// FIRE, only to stay quiet — and "the guard is inert" is exactly the failure this
// one exists to prevent elsewhere.
func CheckPostgresPeer(connString string) error {
	u, err := url.Parse(connString)
	if err != nil {
		return fmt.Errorf("testenv: connection string is not a URL: %w", err)
	}
	host, port := u.Hostname(), u.Port()
	if host == "" || port == "" {
		return fmt.Errorf("testenv: connection string has no host:port: %q", connString)
	}

	c, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 5*time.Second)
	if err != nil {
		return fmt.Errorf("testenv: cannot reach the container's published port %s:%s: %v%s",
			host, port, err, squatterHint(port))
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))

	// SSLRequest: int32 length 8, int32 code 80877103. Every Postgres answers
	// with exactly one byte — 'S' or 'N' — and an old or erroring one answers
	// 'E' or 'R'. Anything else is not Postgres.
	req := make([]byte, 8)
	binary.BigEndian.PutUint32(req[0:], 8)
	binary.BigEndian.PutUint32(req[4:], 80877103)
	if _, err := c.Write(req); err != nil {
		return fmt.Errorf("testenv: writing to %s:%s failed: %v%s", host, port, err, squatterHint(port))
	}
	resp := make([]byte, 8)
	n, err := c.Read(resp)
	if err != nil || n == 0 {
		return fmt.Errorf("testenv: %s:%s accepted a connection then said nothing (%v)%s",
			host, port, err, squatterHint(port))
	}
	switch resp[0] {
	case 'S', 'N', 'E', 'R':
		return nil
	}
	return fmt.Errorf("testenv: %s:%s is NOT this container's Postgres — it answered %q (% x).\n"+
		"The published host port is held by another process, so every query in this test "+
		"would talk to it. See O34: pgx reports this as \"invalid body length\" because it "+
		"reads four of those bytes as an int32.%s",
		host, port, string(resp[:n]), resp[:n], squatterHint(port))
}

// squatterHint names the process actually holding the port, when the platform
// can tell us. A diagnosis that says "something else has it" without saying WHAT
// leaves the next person exactly where this one started.
func squatterHint(port string) string {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return ""
	}
	out, err := exec.Command("lsof", "-nP", "-iTCP:"+port).CombinedOutput()
	if err != nil || len(out) == 0 {
		return "\n(lsof could not identify the holder of port " + port + ")"
	}
	return "\nlsof -nP -iTCP:" + port + ":\n" + strings.TrimRight(string(out), "\n")
}
