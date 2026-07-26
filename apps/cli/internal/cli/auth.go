package cli

// T5.2: auth — token paste (cli.md; device flow is a later refinement). The
// token is VERIFIED against /v1/auth/session before it is stored (0600);
// reveal-once holds: the CLI never prints a stored token back.

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	contracts "github.com/steloit/cloud/packages/contracts/go"
)

func init() {
	register("auth", "login", "store a personal token (verified first)", runAuthLogin)
	register("auth", "logout", "forget the stored token", runAuthLogout)
	register("connect", "verify", "check this machine's connection (A9)", runConnectVerify)
}

// apiClient builds the generated client (sdk.md: generated core, thin
// ergonomics) with the bearer editor.
func apiClient(cfg *Config, token string) (*contracts.ClientWithResponses, error) {
	return contracts.NewClientWithResponses(
		strings.TrimSuffix(cfg.APIURL, "/")+"/v1",
		contracts.WithHTTPClient(&http.Client{Timeout: 15 * time.Second}),
		contracts.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			return nil
		}),
	)
}

// whoami verifies a token and returns the account email.
func whoami(cfg *Config, token string) (string, int) {
	client, err := apiClient(cfg, token)
	if err != nil {
		return "", ExitGeneric
	}
	resp, err := client.GetSessionWithResponse(context.Background())
	if err != nil {
		return "", ExitGeneric
	}
	switch {
	case resp.JSON200 != nil:
		return resp.JSON200.User.Email, ExitOK
	case resp.HTTPResponse.StatusCode == 401:
		return "", ExitPermission
	default:
		return "", ExitGeneric
	}
}

func runAuthLogin(inv *Invocation) int {
	token := inv.Flags["token"]
	if token == "" {
		// paste flow: read one line from stdin (pipe `steloit auth login <
		// tokenfile` or paste interactively). The value is a secret — it is
		// verified, stored 0600, and never echoed back by any command.
		fmt.Fprint(inv.Stderr, "Paste your personal token (stp_…): ")
		sc := bufio.NewScanner(stdinReader)
		if !sc.Scan() {
			fmt.Fprintln(inv.Stderr, "\nsteloit: no token provided")
			return ExitUsage
		}
		token = strings.TrimSpace(sc.Text())
	}
	if !strings.HasPrefix(token, "stp_") {
		fmt.Fprintln(inv.Stderr, "steloit: that does not look like a Steloit token (stp_…) — create one under your profile or `steloit token create`")
		return ExitUsage
	}
	if api := inv.Flags["api"]; api != "" {
		inv.Config.APIURL = api
	}
	email, code := whoami(inv.Config, token)
	if code != ExitOK {
		if code == ExitPermission {
			fmt.Fprintln(inv.Stderr, "✕ token rejected — it may be revoked or expired. Create a new one under your profile.")
		} else {
			fmt.Fprintf(inv.Stderr, "✕ could not reach %s — check the URL (--api) and your network.\n", inv.Config.APIURL)
		}
		return code
	}
	inv.Config.Token = token
	if err := inv.Config.Save(); err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: saving config: %v\n", err)
		return ExitGeneric
	}
	if inv.Quiet() {
		return ExitOK
	}
	fmt.Fprintf(inv.Stdout, "✓ connected as %s · token stored (only a hash lives server-side)\n", email)
	return ExitOK
}

func runAuthLogout(inv *Invocation) int {
	if inv.Config.Token == "" {
		fmt.Fprintln(inv.Stdout, "· nothing stored")
		return ExitOK
	}
	inv.Config.Token = ""
	if err := inv.Config.Save(); err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	fmt.Fprintln(inv.Stdout, "✓ token forgotten locally — revoke it under your profile to kill it server-side")
	return ExitOK
}

// runConnectVerify — A9's moment, terminal form.
func runConnectVerify(inv *Invocation) int {
	if inv.Config.Token == "" {
		fmt.Fprintln(inv.Stderr, "✕ not connected — run `steloit auth login`")
		return ExitPermission
	}
	email, code := whoami(inv.Config, inv.Config.Token)
	if code != ExitOK {
		fmt.Fprintln(inv.Stderr, "✕ stored token no longer works — run `steloit auth login` with a fresh one")
		return code
	}
	if inv.JSON() {
		fmt.Fprintf(inv.Stdout, "{\"connected\":true,\"email\":%q}\n", email)
		return ExitOK
	}
	fmt.Fprintf(inv.Stdout, "✓ connected just now · personal token · %s · manage under your profile\n", email)
	return ExitOK
}
