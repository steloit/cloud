package kube

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testCAPEM(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "steloit-test-ca"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(100 * 365 * 24 * time.Hour),
		IsCA: true, KeyUsage: x509.KeyUsageCertSign, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func saDirWith(t *testing.T, token string, ca []byte) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte(token), 0o600); err != nil {
		t.Fatal(err)
	}
	if ca != nil {
		if err := os.WriteFile(filepath.Join(dir, "ca.crt"), ca, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	prev := saDir
	saDir = dir
	t.Cleanup(func() { saDir = prev })
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "443")
	return dir
}

// THE PRODUCTION CONSTRUCTOR must wire the token as a FILE, not cache its
// contents.
//
// TestAuthRereadsTheRotatedServiceAccountToken proves the re-read mechanism, but
// it builds its client with NewClientForTest and sets tokenFile by hand — so it
// cannot see this wiring at all, and NewInCluster could go back to caching the
// token at boot with the whole module green. Two representations of "the token is
// re-read": the mechanism and the wiring. That test covered the first.
//
// What caching costs is in the type's own comment: GKE projected tokens expire
// (~1h) and are rotated in place, so every apply 401s after the TTL and never
// recovers without a restart.
func TestNewInClusterWiresTheTokenFileRatherThanCachingIt(t *testing.T) {
	saDirWith(t, "boot-token", testCAPEM(t))

	c, err := NewInCluster()
	if err != nil {
		t.Fatal(err)
	}
	if c.tokenFile == "" {
		t.Fatal("NewInCluster did not wire tokenFile — the token is read once at boot and " +
			"every apply 401s after the projected token's ~1h TTL, never recovering")
	}
	if c.token != "" {
		t.Fatalf("NewInCluster cached the token value (%q); the file is the source of truth", c.token)
	}
	if got := filepath.Base(c.tokenFile); got != "token" {
		t.Fatalf("tokenFile is %q", c.tokenFile)
	}
}

// The cluster CA must be read AND validated. Deleting either was green: with no
// CA the agent takes the in-cluster branch with an empty pool, logs "real apply",
// and every converge fails TLS — a working-looking agent that provisions nothing.
func TestNewInClusterRefusesAnUnusableClusterCA(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		saDirWith(t, "tok", nil)
		if _, err := NewInCluster(); err == nil {
			t.Fatal("NewInCluster accepted a missing ca.crt")
		}
	})
	t.Run("not PEM", func(t *testing.T) {
		saDirWith(t, "tok", []byte("this is not a certificate"))
		if _, err := NewInCluster(); err == nil {
			t.Fatal("NewInCluster accepted a ca.crt that is not valid PEM — the RootCAs pool " +
				"would be empty and every converge would fail TLS behind a 'real apply' log line")
		}
	})
	t.Run("valid", func(t *testing.T) {
		saDirWith(t, "tok", testCAPEM(t))
		if _, err := NewInCluster(); err != nil {
			t.Fatalf("NewInCluster refused a valid CA: %v", err)
		}
	})
	t.Run("no cluster", func(t *testing.T) {
		saDirWith(t, "tok", testCAPEM(t))
		t.Setenv("KUBERNETES_SERVICE_HOST", "")
		if _, err := NewInCluster(); err == nil {
			t.Fatal("NewInCluster built a client with no KUBERNETES_SERVICE_HOST")
		}
	})
}

// THE TRANSPORT, END TO END, against a real TLS server.
//
// Round 9 pinned that a missing or unparseable ca.crt is refused. Read,
// validated and INSTALLED are three representations, and only two were covered:
// dropping `RootCAs: pool` from the tls.Config was green, adding
// `InsecureSkipVerify: true` was green, and downgrading the base URL to `http://`
// — the projected ServiceAccount token in cleartext — was green. So was an empty
// `fieldOwner` in the production constructor, which NewClientForTest hides by
// hardcoding its own.
//
// A real handshake pins all of them at once: the client must trust the server
// because of the CA it read, and must not trust one it did not.
func TestNewInClusterActuallyTalksTLSAndCarriesItsIdentity(t *testing.T) {
	var got struct{ auth, query, path string }
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.auth, got.query, got.path = r.Header.Get("Authorization"), r.URL.RawQuery, r.URL.Path
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	host, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	t.Run("trusts the CA it read", func(t *testing.T) {
		saDirWith(t, "projected-token", caPEM)
		t.Setenv("KUBERNETES_SERVICE_HOST", host)
		t.Setenv("KUBERNETES_SERVICE_PORT", port)

		c, err := NewInCluster()
		if err != nil {
			t.Fatal(err)
		}
		ns := []byte("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: env-9f3c1a2b\n")
		if err := c.Apply(context.Background(), "env-9f3c1a2b", [][]byte{ns}); err != nil {
			t.Fatalf("apply over TLS failed — the CA it read is not installed as the trust "+
				"root, or the base URL is not https: %v", err)
		}
		if got.auth != "Bearer projected-token" {
			t.Errorf("authorization %q — the in-cluster client must send its ServiceAccount token", got.auth)
		}
		if !strings.Contains(got.query, "fieldManager=steloit-cell-agent") {
			t.Errorf("query %q — the PRODUCTION constructor must set a field manager; "+
				"NewClientForTest hardcodes its own and hides an empty one here", got.query)
		}
		if !strings.Contains(got.query, "force=true") {
			t.Errorf("query %q — without force the agent is not the authoritative owner", got.query)
		}
	})

	t.Run("refuses a server it has no CA for", func(t *testing.T) {
		// A DIFFERENT, valid CA. This is the arm that kills InsecureSkipVerify and
		// a dropped RootCAs: with either, the handshake below would succeed.
		saDirWith(t, "projected-token", testCAPEM(t))
		t.Setenv("KUBERNETES_SERVICE_HOST", host)
		t.Setenv("KUBERNETES_SERVICE_PORT", port)

		c, err := NewInCluster()
		if err != nil {
			t.Fatal(err)
		}
		ns := []byte("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: env-9f3c1a2b\n")
		err = c.Apply(context.Background(), "env-9f3c1a2b", [][]byte{ns})
		if err == nil {
			t.Fatal("the client trusted a server whose CA it was never given — either " +
				"InsecureSkipVerify is set or RootCAs is not installed, and the agent's " +
				"token goes to anything presenting any certificate")
		}
		if !strings.Contains(err.Error(), "certificate") && !strings.Contains(err.Error(), "x509") {
			t.Fatalf("expected a certificate failure, got: %v", err)
		}
	})

	t.Run("Observe and Delete carry the token too", func(t *testing.T) {
		saDirWith(t, "projected-token", caPEM)
		t.Setenv("KUBERNETES_SERVICE_HOST", host)
		t.Setenv("KUBERNETES_SERVICE_PORT", port)
		c, err := NewInCluster()
		if err != nil {
			t.Fatal(err)
		}
		// Apply's bearer is pinned above; Observe's and Delete's were not, so
		// deleting c.auth(req) from either was a green change.
		got.auth = ""
		if _, err := c.Observe(context.Background(), "env-9f3c1a2b", "db"); err != nil {
			t.Fatal(err)
		}
		if got.auth != "Bearer projected-token" {
			t.Errorf("Observe sent authorization %q", got.auth)
		}
		got.auth = ""
		if err := c.Delete(context.Background(), "env-9f3c1a2b", "Cluster", "db"); err != nil {
			t.Fatal(err)
		}
		if got.auth != "Bearer projected-token" {
			t.Errorf("Delete sent authorization %q", got.auth)
		}
	})
}
